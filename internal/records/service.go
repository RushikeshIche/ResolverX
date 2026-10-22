package records

import (
	"context"
	"fmt"
	"sync"
)

// Store defines the persistence contract required by Service.
type Store interface {
	CreateRecord(ctx context.Context, r *Record) error
	UpdateRecord(ctx context.Context, r *Record) error
	DeleteRecord(ctx context.Context, id string) error
	GetRecord(ctx context.Context, id string) (*Record, error)
	ListRecords(ctx context.Context) ([]*Record, error)
	FindRecords(ctx context.Context, domain string, recType RecordType) ([]*Record, error)
}

// ChangeListener is invoked whenever a record is created, updated, or deleted.
type ChangeListener func(domain string)

// Service provides validated record operations and coordinates cache invalidation.
type Service struct {
	store     Store
	mu        sync.RWMutex
	listeners []ChangeListener
}

// NewService creates a new Record Service with the given storage backend.
func NewService(store Store) *Service {
	return &Service{
		store:     store,
		listeners: make([]ChangeListener, 0),
	}
}

// OnChange registers a callback to be triggered when records change.
func (s *Service) OnChange(listener ChangeListener) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listeners = append(s.listeners, listener)
}

func (s *Service) notifyChange(domain string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, l := range s.listeners {
		if l != nil {
			go l(domain)
		}
	}
}

// Create validates and saves a new record, then triggers change notifications.
func (s *Service) Create(ctx context.Context, r *Record) error {
	if err := ValidateRecord(r); err != nil {
		return err
	}

	if err := s.store.CreateRecord(ctx, r); err != nil {
		return err
	}

	s.notifyChange(r.Domain)
	return nil
}

// Update validates and updates an existing record, notifying listeners of the domain change.
func (s *Service) Update(ctx context.Context, r *Record) error {
	if err := ValidateRecord(r); err != nil {
		return err
	}

	// Fetch old record in case domain changed
	old, err := s.store.GetRecord(ctx, r.ID)
	if err != nil {
		return fmt.Errorf("failed to fetch existing record: %w", err)
	}

	if err := s.store.UpdateRecord(ctx, r); err != nil {
		return err
	}

	s.notifyChange(old.Domain)
	if old.Domain != r.Domain {
		s.notifyChange(r.Domain)
	}
	return nil
}

// Delete removes a record and invalidates any cached entries for its domain.
func (s *Service) Delete(ctx context.Context, id string) error {
	old, err := s.store.GetRecord(ctx, id)
	if err != nil {
		return err
	}

	if err := s.store.DeleteRecord(ctx, id); err != nil {
		return err
	}

	s.notifyChange(old.Domain)
	return nil
}

// Get retrieves a record by ID.
func (s *Service) Get(ctx context.Context, id string) (*Record, error) {
	return s.store.GetRecord(ctx, id)
}

// List returns all configured records.
func (s *Service) List(ctx context.Context) ([]*Record, error) {
	return s.store.ListRecords(ctx)
}

// Find looks up records matching domain and record type.
func (s *Service) Find(ctx context.Context, domain string, recType RecordType) ([]*Record, error) {
	return s.store.FindRecords(ctx, domain, recType)
}
