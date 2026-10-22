package app

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/RushikeshIche/ResolverX/internal/config"
	"github.com/RushikeshIche/ResolverX/internal/dns"
	"github.com/RushikeshIche/ResolverX/internal/hostsfile"
	"github.com/RushikeshIche/ResolverX/internal/metrics"
	"github.com/RushikeshIche/ResolverX/internal/records"
	"github.com/RushikeshIche/ResolverX/internal/redirects"
	"github.com/RushikeshIche/ResolverX/internal/storage"
)

// App is the core coordinator for ResolverX, managing configuration,
// persistence, DNS server lifecycle, HTTP redirect server, and metrics.
type App struct {
	ConfigPath     string
	Config         *config.Config
	Store          *storage.Store
	Records        *records.Service
	Redirects      *redirects.Service
	Cache          *dns.Cache
	Forwarder      *dns.Forwarder
	Metrics        *metrics.Collector
	Server         *dns.Server
	RedirectServer *redirects.Server
}

// NewApp initializes all underlying components using the provided config path.
func NewApp(configPath string) (*App, error) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	store, err := storage.NewStore(cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open storage: %w", err)
	}

	recSvc := records.NewService(store)
	redirSvc := redirects.NewService(store, recSvc)

	var cache *dns.Cache
	if cfg.CacheEnabled {
		cache = dns.NewCache(cfg.CacheMaxEntries)
	}

	// Auto-invalidate cached answers when custom records change
	recSvc.OnChange(func(domain string) {
		if cache != nil {
			cache.Invalidate(domain)
		}
	})

	forwarder := dns.NewForwarder(cfg.UpstreamServers, 2*time.Second)
	met := metrics.NewCollector(150)

	handler := dns.NewHandler(recSvc, cache, forwarder, met)
	server := dns.NewServer(cfg.ListenAddress, handler)

	var redirServer *redirects.Server
	if cfg.HTTPRedirectEnabled {
		redirServer = redirects.NewServer(cfg.HTTPRedirectAddress, redirSvc)
	}

	return &App{
		ConfigPath:     configPath,
		Config:         cfg,
		Store:          store,
		Records:        recSvc,
		Redirects:      redirSvc,
		Cache:          cache,
		Forwarder:      forwarder,
		Metrics:        met,
		Server:         server,
		RedirectServer: redirServer,
	}, nil
}

// StartServer starts the DNS listener service and HTTP redirect server.
func (a *App) StartServer() error {
	if a.RedirectServer != nil && !a.RedirectServer.IsRunning() {
		_ = a.RedirectServer.Start()
	}
	return a.Server.Start()
}

// StopServer stops the DNS listener service and HTTP redirect server.
func (a *App) StopServer() error {
	if a.RedirectServer != nil && a.RedirectServer.IsRunning() {
		_ = a.RedirectServer.Stop()
	}
	return a.Server.Stop()
}

// IsServerRunning returns true if the DNS server is listening.
func (a *App) IsServerRunning() bool {
	return a.Server.IsRunning()
}

// UpdateConfig persists new configuration and updates running components.
func (a *App) UpdateConfig(newCfg *config.Config) error {
	if err := config.SaveConfig(a.ConfigPath, newCfg); err != nil {
		return err
	}

	a.Config = newCfg
	a.Forwarder.SetUpstreams(newCfg.UpstreamServers)

	// Invalidate or rebuild cache if max entries changed or disabled
	if !newCfg.CacheEnabled {
		if a.Cache != nil {
			a.Cache.Purge()
		}
	}

	return nil
}

// Close gracefully closes storage and stops both the DNS and redirect servers.
func (a *App) Close() error {
	if a.RedirectServer != nil {
		_ = a.RedirectServer.Stop()
	}
	_ = a.Server.Stop()
	return a.Store.Close()
}

// SeedDefaultRecords inserts sample DNS records and sample URL redirects if brand new.
func (a *App) SeedDefaultRecords() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	list, err := a.Records.List(ctx)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		sampleRecords := []*records.Record{
			{
				Domain: "myapp.test",
				Type:   records.TypeA,
				Value:  "127.0.0.1",
				TTL:    300,
			},
			{
				Domain: "dev.test",
				Type:   records.TypeA,
				Value:  "192.168.1.10",
				TTL:    300,
			},
			{
				Domain: "ipv6.test",
				Type:   records.TypeAAAA,
				Value:  "::1",
				TTL:    300,
			},
		}

		for _, r := range sampleRecords {
			_ = a.Records.Create(ctx, r)
		}
	}

	// Seed sample URL redirects if none exist
	if a.Redirects != nil {
		redirs, err := a.Redirects.List(ctx)
		if err == nil && len(redirs) == 0 {
			sampleRedirects := []*redirects.Redirect{
				{
					Shortcut:    "gitrush.test",
					TargetURL:   "https://github.com/RushikeshIche",
					StatusCode:  302,
					Description: "GitHub Profile",
				},
				{
					Shortcut:    "gitrush.com",
					TargetURL:   "https://github.com/RushikeshIche",
					StatusCode:  302,
					Description: "GitHub Profile Shortcut",
				},
				{
					Shortcut:    "resolverx.local",
					TargetURL:   "https://github.com/RushikeshIche/ResolverX",
					StatusCode:  302,
					Description: "ResolverX Repository",
				},
			}
			for _, redir := range sampleRedirects {
				_ = a.Redirects.Create(ctx, redir)
			}
		}
	}

	return nil
}

// SyncHostsEntries ensures every existing domain-style redirect shortcut has a
// 127.0.0.1 entry in the OS hosts file. Called once on startup.
func (a *App) SyncHostsEntries() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	list, err := a.Redirects.List(ctx)
	if err != nil {
		return
	}
	for _, r := range list {
		if hostsfile.IsDomainShortcut(r.Shortcut) {
			if err := hostsfile.EnsureEntry(r.Shortcut); err != nil {
				log.Printf("[hostsfile] Warning syncing '%s': %v", r.Shortcut, err)
			}
		}
	}
}

