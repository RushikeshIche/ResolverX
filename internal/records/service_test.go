package records_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rushi/ResolverX/internal/records"
	"github.com/rushi/ResolverX/internal/storage"
)

func TestServiceValidationAndNotification(t *testing.T) {
	store, err := storage.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}
	defer store.Close()

	svc := records.NewService(store)
	ctx := context.Background()

	var mu sync.Mutex
	changedDomains := make([]string, 0)
	var wg sync.WaitGroup

	svc.OnChange(func(domain string) {
		mu.Lock()
		changedDomains = append(changedDomains, domain)
		mu.Unlock()
		wg.Done()
	})

	// 1. Create with invalid IP -> should fail before hitting store
	badRec := &records.Record{
		Domain: "invalid.test",
		Type:   records.TypeA,
		Value:  "999.999.999.999",
	}
	if err := svc.Create(ctx, badRec); err == nil {
		t.Fatal("expected error for invalid IP, got nil")
	}

	// 2. Create valid record -> should notify
	wg.Add(1)
	validRec := &records.Record{
		Domain: "good.test",
		Type:   records.TypeA,
		Value:  "127.0.0.1",
	}
	if err := svc.Create(ctx, validRec); err != nil {
		t.Fatalf("failed to create record: %v", err)
	}

	// Wait for notification
	waitTimeout(&wg, 2*time.Second, t)

	mu.Lock()
	if len(changedDomains) != 1 || changedDomains[0] != "good.test" {
		t.Fatalf("expected notification for good.test, got: %v", changedDomains)
	}
	mu.Unlock()

	// 3. Update record -> should notify
	wg.Add(1)
	validRec.Value = "127.0.0.2"
	if err := svc.Update(ctx, validRec); err != nil {
		t.Fatalf("failed to update record: %v", err)
	}
	waitTimeout(&wg, 2*time.Second, t)

	// 4. Delete record -> should notify
	wg.Add(1)
	if err := svc.Delete(ctx, validRec.ID); err != nil {
		t.Fatalf("failed to delete record: %v", err)
	}
	waitTimeout(&wg, 2*time.Second, t)
}

func waitTimeout(wg *sync.WaitGroup, timeout time.Duration, t *testing.T) {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return
	case <-time.After(timeout):
		t.Fatal("timed out waiting for change notification")
	}
}
