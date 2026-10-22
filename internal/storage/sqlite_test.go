package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rushi/ResolverX/internal/records"
	"github.com/rushi/ResolverX/internal/storage"
)

func setupTestStore(t *testing.T) *storage.Store {
	store, err := storage.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to initialize in-memory store: %v", err)
	}
	t.Cleanup(func() {
		store.Close()
	})
	return store
}

func TestStoreCRUD(t *testing.T) {
	store := setupTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rec := &records.Record{
		Domain: "myapp.test",
		Type:   records.TypeA,
		Value:  "127.0.0.1",
		TTL:    300,
	}

	// 1. Create
	if err := store.CreateRecord(ctx, rec); err != nil {
		t.Fatalf("failed to create record: %v", err)
	}
	if rec.ID == "" {
		t.Fatal("expected record ID to be populated")
	}

	// 2. Duplicate prevention
	dup := &records.Record{
		Domain: "myapp.test",
		Type:   records.TypeA,
		Value:  "127.0.0.1",
		TTL:    300,
	}
	if err := store.CreateRecord(ctx, dup); !errors.Is(err, storage.ErrDuplicateRecord) {
		t.Fatalf("expected ErrDuplicateRecord, got %v", err)
	}

	// 3. Get
	fetched, err := store.GetRecord(ctx, rec.ID)
	if err != nil {
		t.Fatalf("failed to fetch record: %v", err)
	}
	if fetched.Domain != "myapp.test" || fetched.Value != "127.0.0.1" {
		t.Errorf("fetched record mismatch: %+v", fetched)
	}

	// 4. Find matching domain and type
	matches, err := store.FindRecords(ctx, "myapp.test.", records.TypeA)
	if err != nil {
		t.Fatalf("failed to find records: %v", err)
	}
	if len(matches) != 1 || matches[0].Value != "127.0.0.1" {
		t.Fatalf("expected 1 match with value 127.0.0.1, got %d matches", len(matches))
	}

	// 5. Update
	rec.Value = "127.0.0.2"
	rec.TTL = 600
	if err := store.UpdateRecord(ctx, rec); err != nil {
		t.Fatalf("failed to update record: %v", err)
	}
	updated, err := store.GetRecord(ctx, rec.ID)
	if err != nil {
		t.Fatalf("failed to get updated record: %v", err)
	}
	if updated.Value != "127.0.0.2" || updated.TTL != 600 {
		t.Errorf("updated fields mismatch: %+v", updated)
	}

	// 6. List
	all, err := store.ListRecords(ctx)
	if err != nil {
		t.Fatalf("failed to list records: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 record in list, got %d", len(all))
	}

	// 7. Delete
	if err := store.DeleteRecord(ctx, rec.ID); err != nil {
		t.Fatalf("failed to delete record: %v", err)
	}

	// Verify not found
	_, err = store.GetRecord(ctx, rec.ID)
	if !errors.Is(err, storage.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound after delete, got %v", err)
	}
}
