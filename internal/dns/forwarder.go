package dns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// Forwarder manages querying upstream DNS servers with failover.
type Forwarder struct {
	mu        sync.RWMutex
	upstreams []string
	timeout   time.Duration
	client    *dns.Client
}

// NewForwarder initializes an upstream forwarder.
func NewForwarder(upstreams []string, timeout time.Duration) *Forwarder {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &Forwarder{
		upstreams: upstreams,
		timeout:   timeout,
		client: &dns.Client{
			Timeout: timeout,
			Net:     "udp",
		},
	}
}

// SetUpstreams updates the list of upstream DNS servers.
func (f *Forwarder) SetUpstreams(upstreams []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.upstreams = make([]string, len(upstreams))
	copy(f.upstreams, upstreams)
}

// GetUpstreams returns the current upstream DNS servers.
func (f *Forwarder) GetUpstreams() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	copied := make([]string, len(f.upstreams))
	copy(copied, f.upstreams)
	return copied
}

// Forward attempts to resolve the DNS query using the configured upstream servers in order.
func (f *Forwarder) Forward(ctx context.Context, req *dns.Msg) (*dns.Msg, error) {
	f.mu.RLock()
	servers := make([]string, len(f.upstreams))
	copy(servers, f.upstreams)
	f.mu.RUnlock()

	if len(servers) == 0 {
		return nil, errors.New("no upstream servers configured")
	}

	var lastErr error
	for _, server := range servers {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Ensure server has port
		addr := server
		if _, _, err := net.SplitHostPort(addr); err != nil {
			addr = net.JoinHostPort(addr, "53")
		}

		resp, _, err := f.client.ExchangeContext(ctx, req, addr)
		if err == nil && resp != nil {
			return resp, nil
		}
		lastErr = fmt.Errorf("upstream %s failed: %w", addr, err)
	}

	return nil, fmt.Errorf("all upstreams failed: %w", lastErr)
}
