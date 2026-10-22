package redirects

import (
	"context"
	"log"
	"strings"
	"sync"

	"github.com/RushikeshIche/ResolverX/internal/hostsfile"
	"github.com/RushikeshIche/ResolverX/internal/records"
)

// Store defines persistence requirements for URL redirects.
type Store interface {
	CreateRedirect(ctx context.Context, r *Redirect) error
	UpdateRedirect(ctx context.Context, r *Redirect) error
	DeleteRedirect(ctx context.Context, id string) error
	GetRedirect(ctx context.Context, id string) (*Redirect, error)
	FindRedirectByShortcut(ctx context.Context, shortcut string) (*Redirect, error)
	ListRedirects(ctx context.Context) ([]*Redirect, error)
}

// Service manages URL redirects and coordinates automatic DNS record synchronization.
type Service struct {
	store      Store
	dnsRecords *records.Service
	mu         sync.RWMutex
}

// NewService creates a new redirect service.
func NewService(store Store, dnsRecords *records.Service) *Service {
	return &Service{
		store:      store,
		dnsRecords: dnsRecords,
	}
}

// Create validates and saves a new redirect. If the shortcut contains a dot (domain format),
// it automatically creates a corresponding local DNS A record to 127.0.0.1.
func (s *Service) Create(ctx context.Context, r *Redirect) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ValidateRedirect(r); err != nil {
		return err
	}

	if err := s.store.CreateRedirect(ctx, r); err != nil {
		return err
	}

	s.syncDNSRecord(ctx, r.Shortcut)
	s.syncHostsFile(r.Shortcut)
	return nil
}

// Update validates and updates an existing redirect, keeping shortcuts and target URLs fully modifiable.
func (s *Service) Update(ctx context.Context, r *Redirect) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ValidateRedirect(r); err != nil {
		return err
	}

	// Retrieve old redirect in case shortcut was modified
	old, err := s.store.GetRedirect(ctx, r.ID)
	if err != nil {
		return err
	}

	if err := s.store.UpdateRedirect(ctx, r); err != nil {
		return err
	}

	if old.Shortcut != r.Shortcut {
		// Remove old hosts entry, add new one.
		if hostsfile.IsDomainShortcut(old.Shortcut) {
			if err := hostsfile.RemoveEntry(old.Shortcut); err != nil {
				log.Printf("[hostsfile] Warning removing old entry '%s': %v", old.Shortcut, err)
			}
		}
		s.syncDNSRecord(ctx, r.Shortcut)
		s.syncHostsFile(r.Shortcut)
	}
	return nil
}

// Delete removes a redirect by ID and cleans up its hosts file entry.
func (s *Service) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Fetch shortcut before deletion so we can clean up the hosts entry.
	if r, err := s.store.GetRedirect(ctx, id); err == nil && r != nil {
		if hostsfile.IsDomainShortcut(r.Shortcut) {
			if hErr := hostsfile.RemoveEntry(r.Shortcut); hErr != nil {
				log.Printf("[hostsfile] Warning removing entry '%s': %v", r.Shortcut, hErr)
			}
		}
	}

	return s.store.DeleteRedirect(ctx, id)
}

// Get retrieves a redirect by ID.
func (s *Service) Get(ctx context.Context, id string) (*Redirect, error) {
	return s.store.GetRedirect(ctx, id)
}

// FindByShortcut retrieves a redirect matching the given shortcut.
func (s *Service) FindByShortcut(ctx context.Context, shortcut string) (*Redirect, error) {
	return s.store.FindRedirectByShortcut(ctx, shortcut)
}

// List returns all configured redirects.
func (s *Service) List(ctx context.Context) ([]*Redirect, error) {
	return s.store.ListRedirects(ctx)
}

// syncDNSRecord ensures that domain-style shortcuts (e.g. gitrush.test) resolve to 127.0.0.1 in DNS.
func (s *Service) syncDNSRecord(ctx context.Context, shortcut string) {
	if s.dnsRecords == nil {
		return
	}
	// If shortcut looks like a domain name with a TLD (e.g. gitrush.test, app.local)
	if strings.Contains(shortcut, ".") && !strings.Contains(shortcut, "/") {
		existing, _ := s.dnsRecords.Find(ctx, shortcut, records.TypeA)
		if len(existing) == 0 {
			_ = s.dnsRecords.Create(ctx, &records.Record{
				Domain: shortcut,
				Type:   records.TypeA,
				Value:  "127.0.0.1",
				TTL:    300,
			})
		}
	}
}

// syncHostsFile writes a 127.0.0.1 hosts entry for domain-style shortcuts so
// browsers can resolve them without any OS DNS configuration changes.
func (s *Service) syncHostsFile(shortcut string) {
	if !hostsfile.IsDomainShortcut(shortcut) {
		return
	}
	if err := hostsfile.EnsureEntry(shortcut); err != nil {
		log.Printf("[hostsfile] Warning: could not add entry for '%s': %v (try running as Administrator)", shortcut, err)
	}
}
