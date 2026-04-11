package migrate

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newRow(id, tenant, status string) *StoredMigration {
	return &StoredMigration{
		ID:          id,
		TenantID:    tenant,
		Status:      status,
		Domain:      "example.com",
		TargetSlug:  "cloudflare",
		Preview:     &Zone{Domain: "example.com"},
		AccessToken: "tok-" + id,
		ExpiresAt:   time.Now().UTC().Add(1 * time.Hour),
	}
}

func newJSONStoreT(t *testing.T) *jsonStore {
	t.Helper()
	s, err := NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}
	return s
}

func TestJSONStoreCreateGet(t *testing.T) {
	s := newJSONStoreT(t)
	ctx := context.Background()

	m := newRow("id-1", "tenantA", StatusPreview)
	if err := s.Create(ctx, m); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if m.Version != 1 {
		t.Fatalf("want Version=1, got %d", m.Version)
	}

	got, err := s.Get(ctx, "id-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != "id-1" || got.TenantID != "tenantA" || got.Status != StatusPreview {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if got.Version != 1 {
		t.Fatalf("want Version=1, got %d", got.Version)
	}
}

func TestJSONStoreCreateDuplicate(t *testing.T) {
	s := newJSONStoreT(t)
	ctx := context.Background()

	if err := s.Create(ctx, newRow("dup", "t", StatusPreview)); err != nil {
		t.Fatal(err)
	}
	err := s.Create(ctx, newRow("dup", "t", StatusPreview))
	if err == nil {
		t.Fatal("expected duplicate Create to error")
	}
}

func TestJSONStoreUpdateOptimistic(t *testing.T) {
	s := newJSONStoreT(t)
	ctx := context.Background()

	m := newRow("id-u", "t", StatusPreview)
	if err := s.Create(ctx, m); err != nil {
		t.Fatal(err)
	}

	// correct next version
	m.Version = 2
	m.Status = StatusApplying
	if err := s.Update(ctx, m); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ := s.Get(ctx, "id-u")
	if got.Version != 2 || got.Status != StatusApplying {
		t.Fatalf("stale stored row: %+v", got)
	}

	// stale version: still 2, should mismatch (needs 3)
	stale := *got
	stale.Version = 2
	if err := s.Update(ctx, &stale); !errors.Is(err, ErrVersionMismatch) {
		t.Fatalf("want ErrVersionMismatch, got %v", err)
	}
}

func TestJSONStoreDelete(t *testing.T) {
	s := newJSONStoreT(t)
	ctx := context.Background()

	m := newRow("id-d", "t", StatusPreview)
	if err := s.Create(ctx, m); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, "id-d"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(ctx, "id-d"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if _, err := os.Stat(s.lockPathFor("id-d")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lock sidecar still present: %v", err)
	}
	// idempotent
	if err := s.Delete(ctx, "id-d"); err != nil {
		t.Fatalf("Delete idempotent: %v", err)
	}
}

func TestJSONStoreListFilter(t *testing.T) {
	s := newJSONStoreT(t)
	ctx := context.Background()

	_ = s.Create(ctx, newRow("r1", "tenantA", StatusPreview))
	_ = s.Create(ctx, newRow("r2", "tenantA", StatusComplete))
	_ = s.Create(ctx, newRow("r3", "tenantB", StatusPreview))

	all, err := s.List(ctx, ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("want 3, got %d", len(all))
	}

	aRows, _ := s.List(ctx, ListFilter{TenantID: "tenantA"})
	if len(aRows) != 2 {
		t.Fatalf("tenantA: want 2, got %d", len(aRows))
	}
	completeRows, _ := s.List(ctx, ListFilter{Status: StatusComplete})
	if len(completeRows) != 1 || completeRows[0].ID != "r2" {
		t.Fatalf("Status=complete: want [r2], got %+v", completeRows)
	}
}

func TestJSONStoreListSorted(t *testing.T) {
	s := newJSONStoreT(t)
	ctx := context.Background()

	ids := []string{"a", "b", "c"}
	for _, id := range ids {
		if err := s.Create(ctx, newRow(id, "t", StatusPreview)); err != nil {
			t.Fatal(err)
		}
		time.Sleep(5 * time.Millisecond)
	}
	rows, err := s.List(ctx, ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3, got %d", len(rows))
	}
	for i := 0; i < len(rows)-1; i++ {
		if rows[i].CreatedAt.After(rows[i+1].CreatedAt) {
			t.Fatalf("rows not sorted ascending: %v / %v", rows[i].CreatedAt, rows[i+1].CreatedAt)
		}
	}
}

func TestJSONStoreSweepExpired(t *testing.T) {
	s := newJSONStoreT(t)
	ctx := context.Background()
	now := time.Now().UTC()

	past := newRow("past-1", "t", StatusPreview)
	past.ExpiresAt = now.Add(-1 * time.Hour)
	if err := s.Create(ctx, past); err != nil {
		t.Fatal(err)
	}
	past2 := newRow("past-2", "t", StatusPreview)
	past2.ExpiresAt = now.Add(-5 * time.Minute)
	if err := s.Create(ctx, past2); err != nil {
		t.Fatal(err)
	}
	future := newRow("future-1", "t", StatusPreview)
	future.ExpiresAt = now.Add(1 * time.Hour)
	if err := s.Create(ctx, future); err != nil {
		t.Fatal(err)
	}

	n, err := s.SweepExpired(ctx, now)
	if err != nil {
		t.Fatalf("SweepExpired: %v", err)
	}
	if n != 2 {
		t.Fatalf("want 2 swept, got %d", n)
	}

	rows, _ := s.List(ctx, ListFilter{})
	if len(rows) != 1 || rows[0].ID != "future-1" {
		t.Fatalf("want only future-1, got %+v", rows)
	}
}

func TestJSONStoreConcurrentUpdate(t *testing.T) {
	s := newJSONStoreT(t)
	ctx := context.Background()

	m := newRow("race", "t", StatusPreview)
	if err := s.Create(ctx, m); err != nil {
		t.Fatal(err)
	}

	const n = 10
	var wg sync.WaitGroup
	var ok int64
	var mismatch int64
	base, _ := s.Get(ctx, "race")

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cp := *base
			cp.Version = base.Version + 1
			cp.Status = StatusApplying
			err := s.Update(ctx, &cp)
			if err == nil {
				atomic.AddInt64(&ok, 1)
			} else if errors.Is(err, ErrVersionMismatch) {
				atomic.AddInt64(&mismatch, 1)
			} else {
				t.Errorf("unexpected err: %v", err)
			}
		}()
	}
	wg.Wait()

	if ok != 1 {
		t.Fatalf("want exactly 1 success, got %d", ok)
	}
	if mismatch != n-1 {
		t.Fatalf("want %d mismatches, got %d", n-1, mismatch)
	}
}
