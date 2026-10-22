package dns_test

import (
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
	appdns "github.com/rushi/ResolverX/internal/dns"
)

func createTestAResponse(name string, ipStr string, ttl uint32) *dns.Msg {
	msg := new(dns.Msg)
	msg.SetQuestion(name, dns.TypeA)
	msg.Response = true
	msg.Rcode = dns.RcodeSuccess
	msg.Answer = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    ttl,
			},
			A: net.ParseIP(ipStr).To4(),
		},
	}
	return msg
}

func TestCacheSetAndGet(t *testing.T) {
	c := appdns.NewCache(10)
	msg := createTestAResponse("example.com.", "93.184.216.34", 100)

	c.Set(msg)

	got, found := c.Get("example.com.", dns.TypeA, dns.ClassINET)
	if !found || got == nil {
		t.Fatal("expected cached entry to be found")
	}

	if len(got.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(got.Answer))
	}

	aRecord := got.Answer[0].(*dns.A)
	if aRecord.A.String() != "93.184.216.34" {
		t.Errorf("expected IP 93.184.216.34, got %s", aRecord.A.String())
	}
	if aRecord.Hdr.Ttl > 100 || aRecord.Hdr.Ttl == 0 {
		t.Errorf("expected valid remaining TTL <= 100, got %d", aRecord.Hdr.Ttl)
	}
}

func TestCacheLRUEviction(t *testing.T) {
	c := appdns.NewCache(2) // Max 2 entries

	m1 := createTestAResponse("site1.com.", "1.1.1.1", 300)
	m2 := createTestAResponse("site2.com.", "2.2.2.2", 300)
	m3 := createTestAResponse("site3.com.", "3.3.3.3", 300)

	c.Set(m1)
	c.Set(m2)

	// Access site1 to make site2 the least recently used
	c.Get("site1.com.", dns.TypeA, dns.ClassINET)

	// Inserting site3 should evict site2
	c.Set(m3)

	if c.Size() != 2 {
		t.Fatalf("expected cache size 2, got %d", c.Size())
	}

	// site1 must exist
	if _, found := c.Get("site1.com.", dns.TypeA, dns.ClassINET); !found {
		t.Error("expected site1 to remain in cache")
	}

	// site2 must be evicted
	if _, found := c.Get("site2.com.", dns.TypeA, dns.ClassINET); found {
		t.Error("expected site2 to be evicted by LRU")
	}

	// site3 must exist
	if _, found := c.Get("site3.com.", dns.TypeA, dns.ClassINET); !found {
		t.Error("expected site3 to exist in cache")
	}
}

func TestCacheInvalidation(t *testing.T) {
	c := appdns.NewCache(10)
	m := createTestAResponse("myapp.test.", "127.0.0.1", 300)
	c.Set(m)

	if _, found := c.Get("myapp.test.", dns.TypeA, dns.ClassINET); !found {
		t.Fatal("expected entry in cache before invalidation")
	}

	// Invalidate without trailing dot
	c.Invalidate("myapp.test")

	if _, found := c.Get("myapp.test.", dns.TypeA, dns.ClassINET); found {
		t.Error("expected entry to be removed after invalidation")
	}
}

func TestCacheExpiration(t *testing.T) {
	c := appdns.NewCache(10)
	// Entry with 1 second TTL
	m := createTestAResponse("quick.test.", "10.0.0.1", 1)
	c.Set(m)

	// Wait 1.1s for expiration
	time.Sleep(1100 * time.Millisecond)

	if _, found := c.Get("quick.test.", dns.TypeA, dns.ClassINET); found {
		t.Error("expected entry to expire and not be found")
	}
}
