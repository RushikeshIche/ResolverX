package integration_test

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/rushi/ResolverX/internal/app"
	"github.com/rushi/ResolverX/internal/config"
	"github.com/rushi/ResolverX/internal/records"
)

func TestFullApplicationIntegration(t *testing.T) {
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, "config.json")
	dbPath := filepath.Join(tempDir, "records.db")

	// Get a free loopback port
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to allocate free port: %v", err)
	}
	listenAddr := pc.LocalAddr().String()
	pc.Close()

	initialCfg := &config.Config{
		ListenAddress:   listenAddr,
		UpstreamServers: []string{"1.1.1.1:53"},
		CacheEnabled:    true,
		CacheMaxEntries: 500,
		LogQueries:      true,
		DatabasePath:    dbPath,
	}
	if err := config.SaveConfig(cfgPath, initialCfg); err != nil {
		t.Fatalf("failed to save initial config: %v", err)
	}

	coreApp, err := app.NewApp(cfgPath)
	if err != nil {
		t.Fatalf("failed to initialize app: %v", err)
	}
	defer coreApp.Close()

	if err := coreApp.SeedDefaultRecords(); err != nil {
		t.Fatalf("failed to seed records: %v", err)
	}

	if err := coreApp.StartServer(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	dnsClient := &dns.Client{Timeout: 2 * time.Second}

	// 1. Query seeded myapp.test A record
	reqA := new(dns.Msg)
	reqA.SetQuestion("myapp.test.", dns.TypeA)
	respA, _, err := dnsClient.Exchange(reqA, listenAddr)
	if err != nil {
		t.Fatalf("query myapp.test failed: %v", err)
	}
	if len(respA.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(respA.Answer))
	}
	if aRec, ok := respA.Answer[0].(*dns.A); !ok || aRec.A.String() != "127.0.0.1" {
		t.Errorf("expected 127.0.0.1, got %v", respA.Answer[0])
	}

	// 2. Query seeded ipv6.test AAAA record
	reqAAAA := new(dns.Msg)
	reqAAAA.SetQuestion("ipv6.test.", dns.TypeAAAA)
	respAAAA, _, err := dnsClient.Exchange(reqAAAA, listenAddr)
	if err != nil {
		t.Fatalf("query ipv6.test failed: %v", err)
	}
	if len(respAAAA.Answer) != 1 {
		t.Fatalf("expected 1 answer for AAAA, got %d", len(respAAAA.Answer))
	}
	if aaaaRec, ok := respAAAA.Answer[0].(*dns.AAAA); !ok || aaaaRec.AAAA.String() != "::1" {
		t.Errorf("expected ::1, got %v", respAAAA.Answer[0])
	}

	// 3. Query via TCP client
	tcpClient := &dns.Client{Net: "tcp", Timeout: 2 * time.Second}
	respTCP, _, err := tcpClient.Exchange(reqA, listenAddr)
	if err != nil {
		t.Fatalf("TCP query failed: %v", err)
	}
	if len(respTCP.Answer) != 1 {
		t.Fatalf("expected 1 TCP answer, got %d", len(respTCP.Answer))
	}

	// 4. Test mutation and live cache invalidation
	ctx := context.Background()
	newRec := &records.Record{
		Domain: "custom-api.test",
		Type:   records.TypeA,
		Value:  "10.10.10.10",
		TTL:    120,
	}
	if err := coreApp.Records.Create(ctx, newRec); err != nil {
		t.Fatalf("failed to create custom-api.test: %v", err)
	}

	reqCustom := new(dns.Msg)
	reqCustom.SetQuestion("custom-api.test.", dns.TypeA)
	respCustom, _, err := dnsClient.Exchange(reqCustom, listenAddr)
	if err != nil {
		t.Fatalf("query custom-api.test failed: %v", err)
	}
	if len(respCustom.Answer) != 1 || respCustom.Answer[0].(*dns.A).A.String() != "10.10.10.10" {
		t.Errorf("expected 10.10.10.10, got %v", respCustom.Answer)
	}

	// 5. Test metrics recorded
	stats := coreApp.Metrics.GetStats()
	if stats.TotalQueries < 4 {
		t.Errorf("expected at least 4 queries recorded in metrics, got %d", stats.TotalQueries)
	}

	// 6. Test graceful stop and restart
	if err := coreApp.StopServer(); err != nil {
		t.Fatalf("failed to stop server: %v", err)
	}
	if coreApp.IsServerRunning() {
		t.Error("expected server to report not running")
	}

	if err := coreApp.StartServer(); err != nil {
		t.Fatalf("failed to restart server: %v", err)
	}
	if !coreApp.IsServerRunning() {
		t.Error("expected server to report running after restart")
	}
}
