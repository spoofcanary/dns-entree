package storetest

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spoofcanary/dns-entree/migrate"
)

// Factory returns a fresh, isolated MigrationStore per call. Tests invoke
// it once per sub-test to guarantee no cross-test pollution.
type Factory func(t *testing.T) migrate.MigrationStore

// RunConformance runs the full MigrationStore contract suite against the
// backend produced by factory. Each sub-test uses its own store instance.
func RunConformance(t *testing.T, factory Factory) {
	t.Helper()

	t.Run("create_and_get", func(t *testing.T) {
		s := factory(t)
		ctx := context.Background()
		m := newFixture("m1", "tenant-a", migrate.StatusPreview, time.Now().Add(time.Hour))
		if err := s.Create(ctx, m); err != nil {
			t.Fatalf("Create: %v", err)
		}
		got, err := s.Get(ctx, "m1")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.ID != "m1" || got.TenantID != "tenant-a" || got.Status != migrate.StatusPreview {
			t.Fatalf("round-trip mismatch: %+v", got)
		}
		if got.Domain != "example.com" {
			t.Fatalf("domain mismatch: %q", got.Domain)
		}
	})

	t.Run("get_not_found", func(t *testing.T) {
		s := factory(t)
		_, err := s.Get(context.Background(), "missing")
		if !errors.Is(err, migrate.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("create_duplicate", func(t *testing.T) {
		s := factory(t)
		ctx := context.Background()
		m := newFixture("dup", "t", migrate.StatusPreview, time.Now().Add(time.Hour))
		if err := s.Create(ctx, m); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if err := s.Create(ctx, m); err == nil {
			t.Fatalf("second Create with same ID must error")
		}
	})

	t.Run("update_happy_path", func(t *testing.T) {
		s := factory(t)
		ctx := context.Background()
		m := newFixture("u1", "t", migrate.StatusPreview, time.Now().Add(time.Hour))
		if err := s.Create(ctx, m); err != nil {
			t.Fatalf("Create: %v", err)
		}
		loaded, err := s.Get(ctx, "u1")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		loaded.Status = migrate.StatusApplying
		loaded.Version = loaded.Version + 1
		if err := s.Update(ctx, loaded); err != nil {
			t.Fatalf("Update: %v", err)
		}
		got, err := s.Get(ctx, "u1")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.Status != migrate.StatusApplying {
			t.Fatalf("status not persisted: %q", got.Status)
		}
		if got.Version != loaded.Version {
			t.Fatalf("version not persisted: got %d want %d", got.Version, loaded.Version)
		}
	})

	t.Run("update_stale_version", func(t *testing.T) {
		s := factory(t)
		ctx := context.Background()
		m := newFixture("u2", "t", migrate.StatusPreview, time.Now().Add(time.Hour))
		if err := s.Create(ctx, m); err != nil {
			t.Fatalf("Create: %v", err)
		}
		a, err := s.Get(ctx, "u2")
		if err != nil {
			t.Fatalf("Get a: %v", err)
		}
		b, err := s.Get(ctx, "u2")
		if err != nil {
			t.Fatalf("Get b: %v", err)
		}
		a.Status = migrate.StatusApplying
		a.Version = a.Version + 1
		if err := s.Update(ctx, a); err != nil {
			t.Fatalf("first Update: %v", err)
		}
		b.Status = migrate.StatusFailed
		b.Version = b.Version + 1
		if err := s.Update(ctx, b); !errors.Is(err, migrate.ErrVersionMismatch) {
			t.Fatalf("want ErrVersionMismatch, got %v", err)
		}
	})

	t.Run("update_not_found", func(t *testing.T) {
		s := factory(t)
		ctx := context.Background()
		ghost := newFixture("ghost", "t", migrate.StatusPreview, time.Now().Add(time.Hour))
		ghost.Version = 2
		err := s.Update(ctx, ghost)
		if !errors.Is(err, migrate.ErrNotFound) && !errors.Is(err, migrate.ErrVersionMismatch) {
			t.Fatalf("want ErrNotFound or ErrVersionMismatch, got %v", err)
		}
	})

	t.Run("delete_happy_path", func(t *testing.T) {
		s := factory(t)
		ctx := context.Background()
		m := newFixture("d1", "t", migrate.StatusPreview, time.Now().Add(time.Hour))
		if err := s.Create(ctx, m); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if err := s.Delete(ctx, "d1"); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if _, err := s.Get(ctx, "d1"); !errors.Is(err, migrate.ErrNotFound) {
			t.Fatalf("want ErrNotFound after delete, got %v", err)
		}
	})

	t.Run("delete_idempotent", func(t *testing.T) {
		s := factory(t)
		if err := s.Delete(context.Background(), "never-existed"); err != nil {
			t.Fatalf("Delete on unknown ID must be nil, got %v", err)
		}
	})

	t.Run("list_empty", func(t *testing.T) {
		s := factory(t)
		rows, err := s.List(context.Background(), migrate.ListFilter{})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(rows) != 0 {
			t.Fatalf("want 0 rows, got %d", len(rows))
		}
	})

	t.Run("list_all", func(t *testing.T) {
		s := factory(t)
		ctx := context.Background()
		exp := time.Now().Add(time.Hour)
		for _, id := range []string{"a", "b", "c"} {
			if err := s.Create(ctx, newFixture(id, "t", migrate.StatusPreview, exp)); err != nil {
				t.Fatalf("Create %s: %v", id, err)
			}
		}
		rows, err := s.List(ctx, migrate.ListFilter{})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(rows) != 3 {
			t.Fatalf("want 3 rows, got %d", len(rows))
		}
	})

	t.Run("list_filter_tenant", func(t *testing.T) {
		s := factory(t)
		ctx := context.Background()
		exp := time.Now().Add(time.Hour)
		_ = s.Create(ctx, newFixture("a", "t1", migrate.StatusPreview, exp))
		_ = s.Create(ctx, newFixture("b", "t2", migrate.StatusPreview, exp))
		_ = s.Create(ctx, newFixture("c", "t1", migrate.StatusPreview, exp))
		rows, err := s.List(ctx, migrate.ListFilter{TenantID: "t1"})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("want 2 rows for tenant t1, got %d", len(rows))
		}
		for _, r := range rows {
			if r.TenantID != "t1" {
				t.Fatalf("leaked tenant %q", r.TenantID)
			}
		}
	})

	t.Run("list_filter_status", func(t *testing.T) {
		s := factory(t)
		ctx := context.Background()
		exp := time.Now().Add(time.Hour)
		_ = s.Create(ctx, newFixture("a", "t", migrate.StatusPreview, exp))
		_ = s.Create(ctx, newFixture("b", "t", migrate.StatusComplete, exp))
		_ = s.Create(ctx, newFixture("c", "t", migrate.StatusComplete, exp))
		rows, err := s.List(ctx, migrate.ListFilter{Status: migrate.StatusComplete})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("want 2 complete, got %d", len(rows))
		}
		for _, r := range rows {
			if r.Status != migrate.StatusComplete {
				t.Fatalf("leaked status %q", r.Status)
			}
		}
	})

	t.Run("list_pagination", func(t *testing.T) {
		s := factory(t)
		ctx := context.Background()
		exp := time.Now().Add(time.Hour)
		for _, id := range []string{"a", "b", "c", "d", "e"} {
			if err := s.Create(ctx, newFixture(id, "t", migrate.StatusPreview, exp)); err != nil {
				t.Fatalf("Create %s: %v", id, err)
			}
		}
		rows, err := s.List(ctx, migrate.ListFilter{Limit: 2})
		if err != nil {
			t.Fatalf("List limit: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("want 2 rows with Limit=2, got %d", len(rows))
		}
		rows2, err := s.List(ctx, migrate.ListFilter{Limit: 2, Offset: 2})
		if err != nil {
			t.Fatalf("List offset: %v", err)
		}
		if len(rows2) != 2 {
			t.Fatalf("want 2 rows with Offset=2, got %d", len(rows2))
		}
	})

	t.Run("sweep_expired", func(t *testing.T) {
		s := factory(t)
		ctx := context.Background()
		past := time.Now().Add(-time.Hour)
		future := time.Now().Add(time.Hour)
		_ = s.Create(ctx, newFixture("old1", "t", migrate.StatusPreview, past))
		_ = s.Create(ctx, newFixture("old2", "t", migrate.StatusPreview, past))
		_ = s.Create(ctx, newFixture("new1", "t", migrate.StatusPreview, future))
		n, err := s.SweepExpired(ctx, time.Now())
		if err != nil {
			t.Fatalf("SweepExpired: %v", err)
		}
		if n != 2 {
			t.Fatalf("want sweep count 2, got %d", n)
		}
		if _, err := s.Get(ctx, "old1"); !errors.Is(err, migrate.ErrNotFound) {
			t.Fatalf("old1 still present: %v", err)
		}
		if _, err := s.Get(ctx, "new1"); err != nil {
			t.Fatalf("new1 must still exist: %v", err)
		}
	})

	t.Run("sweep_none_expired", func(t *testing.T) {
		s := factory(t)
		ctx := context.Background()
		future := time.Now().Add(time.Hour)
		_ = s.Create(ctx, newFixture("a", "t", migrate.StatusPreview, future))
		n, err := s.SweepExpired(ctx, time.Now())
		if err != nil {
			t.Fatalf("SweepExpired: %v", err)
		}
		if n != 0 {
			t.Fatalf("want 0, got %d", n)
		}
	})

	t.Run("concurrent_update", func(t *testing.T) {
		s := factory(t)
		ctx := context.Background()
		m := newFixture("race", "t", migrate.StatusPreview, time.Now().Add(time.Hour))
		if err := s.Create(ctx, m); err != nil {
			t.Fatalf("Create: %v", err)
		}
		base, err := s.Get(ctx, "race")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		const workers = 10
		var wg sync.WaitGroup
		var wins int64
		var mismatches int64
		wg.Add(workers)
		for i := 0; i < workers; i++ {
			go func() {
				defer wg.Done()
				clone := *base
				clone.Status = migrate.StatusApplying
				clone.Version = base.Version + 1
				err := s.Update(ctx, &clone)
				if err == nil {
					atomic.AddInt64(&wins, 1)
					return
				}
				if errors.Is(err, migrate.ErrVersionMismatch) {
					atomic.AddInt64(&mismatches, 1)
					return
				}
				t.Errorf("unexpected err: %v", err)
			}()
		}
		wg.Wait()
		if wins != 1 {
			t.Fatalf("want exactly 1 winner, got %d (mismatches=%d)", wins, mismatches)
		}
		if mismatches != workers-1 {
			t.Fatalf("want %d mismatches, got %d", workers-1, mismatches)
		}
	})
}

// newFixture builds a minimally-populated StoredMigration suitable for
// round-tripping through any backend. Preview is a non-nil trivial Zone so
// JSON backends that disallow nil fields still encode cleanly.
func newFixture(id, tenant, status string, expiresAt time.Time) *migrate.StoredMigration {
	now := time.Now().UTC().Truncate(time.Second)
	return &migrate.StoredMigration{
		ID:        id,
		TenantID:  tenant,
		Status:    status,
		Domain:    "example.com",
		Preview:   &migrate.Zone{Domain: "example.com"},
		ExpiresAt: expiresAt,
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
