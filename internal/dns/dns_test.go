package dns_test

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/miekg/dns"
	appdns "github.com/RushikeshIche/ResolverX/internal/dns"
	"github.com/RushikeshIche/ResolverX/internal/metrics"
	"github.com/RushikeshIche/ResolverX/internal/records"
	"github.com/RushikeshIche/ResolverX/internal/storage"
)

// startMockUpstream boots a temporary in-process DNS server on a random loopback port.
func startMockUpstream(t *testing.T, answers map[string]string) string {
	mux := dns.NewServeMux()
	mux.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.RecursionAvailable = true

		if len(r.Question) > 0 {
			q := r.Question[0]
			if ip, ok := answers[q.Name]; ok {
				m.Answer = append(m.Answer, &dns.A{
					Hdr: dns.RR_Header{
						Name:   q.Name,
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    60,
					},
					A: net.ParseIP(ip).To4(),
				})
			} else {
				m.Rcode = dns.RcodeNameError
			}
		}
		w.WriteMsg(m)
	})

	server := &dns.Server{
		Addr:    "127.0.0.1:0",
		Net:     "udp",
		Handler: mux,
	}

	started := make(chan struct{})
	server.NotifyStartedFunc = func() {
		close(started)
	}

	go func() {
		_ = server.ListenAndServe()
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("mock upstream server failed to start")
	}

	t.Cleanup(func() {
		_ = server.Shutdown()
	})

	return server.PacketConn.LocalAddr().String()
}

func TestEndToEndDNSResolution(t *testing.T) {
	// 1. Mock upstream server
	mockUpstreamAddr := startMockUpstream(t, map[string]string{
		"publicsite.com.": "93.184.216.34",
	})

	// 2. Storage & Records
	store, err := storage.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}
	defer store.Close()

	recSvc := records.NewService(store)
	cache := appdns.NewCache(100)
	forwarder := appdns.NewForwarder([]string{mockUpstreamAddr}, 2*time.Second)
	met := metrics.NewCollector(50)

	// Invalidate cache when record changes
	recSvc.OnChange(func(domain string) {
		cache.Invalidate(domain)
	})

	handler := appdns.NewHandler(recSvc, cache, forwarder, met)

	// Pick a free random loopback port for the test server
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to get free port: %v", err)
	}
	serverAddr := pc.LocalAddr().String()
	pc.Close()

	server := appdns.NewServer(serverAddr, handler)
	if err := server.Start(); err != nil {
		t.Fatalf("failed to start DNS server: %v", err)
	}
	defer server.Stop()

	// Wait for server to bind
	time.Sleep(100 * time.Millisecond)

	dnsClient := &dns.Client{Timeout: 2 * time.Second}

	// 3. Test Local Custom Record
	ctx := context.Background()
	rec := &records.Record{
		Domain: "myapp.test",
		Type:   records.TypeA,
		Value:  "127.0.0.1",
		TTL:    300,
	}
	if err := recSvc.Create(ctx, rec); err != nil {
		t.Fatalf("failed to create custom record: %v", err)
	}

	req := new(dns.Msg)
	req.SetQuestion("myapp.test.", dns.TypeA)
	resp, _, err := dnsClient.Exchange(req, serverAddr)
	if err != nil {
		t.Fatalf("query for local record failed: %v", err)
	}
	if len(resp.Answer) != 1 {
		t.Fatalf("expected 1 answer for local record, got %d", len(resp.Answer))
	}
	if aRec, ok := resp.Answer[0].(*dns.A); !ok || aRec.A.String() != "127.0.0.1" {
		t.Errorf("expected 127.0.0.1, got %v", resp.Answer[0])
	}

	// 4. Test Upstream Forwarding
	reqUpstream := new(dns.Msg)
	reqUpstream.SetQuestion("publicsite.com.", dns.TypeA)
	respUpstream, _, err := dnsClient.Exchange(reqUpstream, serverAddr)
	if err != nil {
		t.Fatalf("query for upstream failed: %v", err)
	}
	if len(respUpstream.Answer) != 1 {
		t.Fatalf("expected 1 answer from upstream, got %d", len(respUpstream.Answer))
	}
	if aRec, ok := respUpstream.Answer[0].(*dns.A); !ok || aRec.A.String() != "93.184.216.34" {
		t.Errorf("expected 93.184.216.34, got %v", respUpstream.Answer[0])
	}

	// 5. Test Cache Hit on repeat query
	respCached, _, err := dnsClient.Exchange(reqUpstream, serverAddr)
	if err != nil {
		t.Fatalf("query for cached domain failed: %v", err)
	}
	if len(respCached.Answer) != 1 {
		t.Fatalf("expected 1 cached answer, got %d", len(respCached.Answer))
	}

	stats := met.GetStats()
	if stats.CacheHits == 0 {
		t.Errorf("expected cache hit count to be > 0, got %d", stats.CacheHits)
	}

	// 6. Test Cache Invalidation on Mutation
	// Add dev.test
	devRec := &records.Record{
		Domain: "dev.test",
		Type:   records.TypeA,
		Value:  "192.168.1.10",
		TTL:    300,
	}
	if err := recSvc.Create(ctx, devRec); err != nil {
		t.Fatalf("failed to create dev.test: %v", err)
	}

	// Update dev.test to 192.168.1.20
	devRec.Value = "192.168.1.20"
	if err := recSvc.Update(ctx, devRec); err != nil {
		t.Fatalf("failed to update dev.test: %v", err)
	}

	time.Sleep(50 * time.Millisecond) // Allow invalidation callback

	reqDev := new(dns.Msg)
	reqDev.SetQuestion("dev.test.", dns.TypeA)
	respDev, _, err := dnsClient.Exchange(reqDev, serverAddr)
	if err != nil {
		t.Fatalf("query for dev.test failed: %v", err)
	}
	if len(respDev.Answer) != 1 {
		t.Fatalf("expected 1 answer for dev.test, got %d", len(respDev.Answer))
	}
	if aRec, ok := respDev.Answer[0].(*dns.A); !ok || aRec.A.String() != "192.168.1.20" {
		t.Errorf("expected updated value 192.168.1.20, got %v", respDev.Answer[0])
	}
}

func TestConcurrentQueriesAndUpdates(t *testing.T) {
	store, _ := storage.NewStore(":memory:")
	defer store.Close()

	recSvc := records.NewService(store)
	cache := appdns.NewCache(100)
	forwarder := appdns.NewForwarder([]string{"127.0.0.1:65534"}, 50*time.Millisecond)
	met := metrics.NewCollector(50)

	recSvc.OnChange(func(domain string) {
		cache.Invalidate(domain)
	})

	handler := appdns.NewHandler(recSvc, cache, forwarder, met)

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := pc.LocalAddr().String()
	pc.Close()

	server := appdns.NewServer(addr, handler)
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	defer server.Stop()

	time.Sleep(50 * time.Millisecond)

	ctx := context.Background()
	_ = recSvc.Create(ctx, &records.Record{
		Domain: "concurrent.test",
		Type:   records.TypeA,
		Value:  "127.0.0.1",
	})

	var wg sync.WaitGroup
	// 20 concurrent query clients
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := &dns.Client{Timeout: 1 * time.Second}
			req := new(dns.Msg)
			req.SetQuestion("concurrent.test.", dns.TypeA)
			for j := 0; j < 10; j++ {
				_, _, _ = client.Exchange(req, addr)
			}
		}()
	}

	// 5 concurrent record mutators
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			rec := &records.Record{
				Domain: "dyn.test",
				Type:   records.TypeA,
				Value:  "10.0.0.1",
			}
			_ = recSvc.Create(ctx, rec)
			_ = recSvc.Delete(ctx, rec.ID)
		}(i)
	}

	wg.Wait()
}
