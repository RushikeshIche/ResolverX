package app

import (
	"context"
	"fmt"
	"time"

	"github.com/rushi/ResolverX/internal/config"
	"github.com/rushi/ResolverX/internal/dns"
	"github.com/rushi/ResolverX/internal/metrics"
	"github.com/rushi/ResolverX/internal/records"
	"github.com/rushi/ResolverX/internal/storage"
)

// App is the core coordinator for ResolverX, managing configuration,
// persistence, DNS server lifecycle, and metrics.
type App struct {
	ConfigPath string
	Config     *config.Config
	Store      *storage.Store
	Records    *records.Service
	Cache      *dns.Cache
	Forwarder  *dns.Forwarder
	Metrics    *metrics.Collector
	Server     *dns.Server
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

	return &App{
		ConfigPath: configPath,
		Config:     cfg,
		Store:      store,
		Records:    recSvc,
		Cache:      cache,
		Forwarder:  forwarder,
		Metrics:    met,
		Server:     server,
	}, nil
}

// StartServer starts the DNS listener service.
func (a *App) StartServer() error {
	return a.Server.Start()
}

// StopServer stops the DNS listener service.
func (a *App) StopServer() error {
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

// Close gracefully closes storage and stops the DNS server.
func (a *App) Close() error {
	_ = a.Server.Stop()
	return a.Store.Close()
}

// SeedDefaultRecords inserts sample records if the database is brand new.
func (a *App) SeedDefaultRecords() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	list, err := a.Records.List(ctx)
	if err != nil {
		return err
	}
	if len(list) > 0 {
		return nil // already has records
	}

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

	return nil
}
