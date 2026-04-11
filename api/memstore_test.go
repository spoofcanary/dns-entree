package api

import (
	"context"
	"sync"
	"time"

	entree "github.com/spoofcanary/dns-entree"
	"github.com/spoofcanary/dns-entree/migrate"
)

// memStore is a test-only in-memory MigrationStore implementation.
// Enforces the same optimistic-locking semantics as the JSON backend:
// Update requires m.Version == stored.Version + 1 or returns ErrVersionMismatch.
type memStore struct {
	mu sync.Mutex
	m  map[string]*migrate.StoredMigration
}

func newMemStore() *memStore {
	return &memStore{m: map[string]*migrate.StoredMigration{}}
}

// compile-time assertion
var _ migrate.MigrationStore = (*memStore)(nil)

func cloneRow(r *migrate.StoredMigration) *migrate.StoredMigration {
	if r == nil {
		return nil
	}
	c := *r
	if r.PreviewRecords != nil {
		c.PreviewRecords = append([]entree.Record(nil), r.PreviewRecords...)
	}
	if r.TargetNameservers != nil {
		c.TargetNameservers = append([]string(nil), r.TargetNameservers...)
	}
	if r.CredentialBlob != nil {
		c.CredentialBlob = append([]byte(nil), r.CredentialBlob...)
	}
	return &c
}

func (s *memStore) Create(ctx context.Context, m *migrate.StoredMigration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if m == nil || m.ID == "" {
		return migrate.ErrNotFound
	}
	if _, ok := s.m[m.ID]; ok {
		return errAlreadyExists
	}
	now := time.Now().UTC()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	m.UpdatedAt = now
	if m.Version == 0 {
		m.Version = 1
	}
	s.m[m.ID] = cloneRow(m)
	return nil
}

func (s *memStore) Get(ctx context.Context, id string) (*migrate.StoredMigration, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.m[id]
	if !ok {
		return nil, migrate.ErrNotFound
	}
	if !r.ExpiresAt.IsZero() && r.ExpiresAt.Before(time.Now().UTC()) {
		return nil, migrate.ErrExpired
	}
	return cloneRow(r), nil
}

func (s *memStore) Update(ctx context.Context, m *migrate.StoredMigration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if m == nil || m.ID == "" {
		return migrate.ErrNotFound
	}
	existing, ok := s.m[m.ID]
	if !ok {
		return migrate.ErrNotFound
	}
	if existing.Version+1 != m.Version {
		return migrate.ErrVersionMismatch
	}
	m.CreatedAt = existing.CreatedAt
	m.UpdatedAt = time.Now().UTC()
	s.m[m.ID] = cloneRow(m)
	return nil
}

func (s *memStore) List(ctx context.Context, filter migrate.ListFilter) ([]*migrate.StoredMigration, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	var out []*migrate.StoredMigration
	for _, r := range s.m {
		if !r.ExpiresAt.IsZero() && r.ExpiresAt.Before(now) {
			continue
		}
		if filter.TenantID != "" && r.TenantID != filter.TenantID {
			continue
		}
		if filter.Status != "" && r.Status != filter.Status {
			continue
		}
		out = append(out, cloneRow(r))
	}
	return out, nil
}

func (s *memStore) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
	return nil
}

func (s *memStore) SweepExpired(ctx context.Context, before time.Time) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for id, r := range s.m {
		if !r.ExpiresAt.IsZero() && r.ExpiresAt.Before(before) {
			delete(s.m, id)
			count++
		}
	}
	return count, nil
}

// sentinel used only by memStore Create; tests that care about it check errors.Is.
var errAlreadyExists = errorString("memstore: already exists")

type errorString string

func (e errorString) Error() string { return string(e) }

// direct-access helpers for tests that need to inspect or mutate the
// underlying state (e.g. forcing an expired row).
func (s *memStore) rawGet(id string) *migrate.StoredMigration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[id]
}

func (s *memStore) rawPut(r *migrate.StoredMigration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[r.ID] = r
}
