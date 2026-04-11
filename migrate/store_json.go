package migrate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// jsonStore is the default MigrationStore backend (D-06). It persists one
// JSON file per migration at <dir>/<id>.json and guards concurrent writes
// with a per-row lock sidecar <id>.lock. File modes: dir 0700, data/lock
// files 0600.
//
// Windows note: multi-process access is unsupported. The windows build uses
// an in-process sync.Map-backed lock; single-process deployments are fine.
type jsonStore struct {
	dir string
}

// compile-time assertion
var _ MigrationStore = (*jsonStore)(nil)

// NewJSONStore creates (if needed) the state directory with mode 0700 and
// returns a jsonStore. It writes a probe file to verify the directory is
// writable.
func NewJSONStore(dir string) (*jsonStore, error) {
	if dir == "" {
		return nil, errors.New("migrate: state dir is empty")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("migrate: mkdir state dir: %w", err)
	}
	probe := filepath.Join(dir, ".write-probe")
	if err := os.WriteFile(probe, []byte("ok"), 0o600); err != nil {
		return nil, fmt.Errorf("migrate: state dir not writable: %w", err)
	}
	_ = os.Remove(probe)
	return &jsonStore{dir: dir}, nil
}

func (s *jsonStore) pathFor(id string) string {
	return filepath.Join(s.dir, id+".json")
}

func (s *jsonStore) lockPathFor(id string) string {
	return filepath.Join(s.dir, id+".lock")
}

// readRow loads and decodes a row by ID. Returns ErrNotFound if missing.
func (s *jsonStore) readRow(id string) (*StoredMigration, error) {
	data, err := os.ReadFile(s.pathFor(id))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("migrate: read row %s: %w", id, err)
	}
	var m StoredMigration
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("migrate: decode row %s: %w", id, err)
	}
	return &m, nil
}

// writeRow atomically writes a row via temp file + rename. Caller must hold
// the row lock.
func (s *jsonStore) writeRow(m *StoredMigration) error {
	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("migrate: encode row: %w", err)
	}
	final := s.pathFor(m.ID)
	tmp, err := os.CreateTemp(s.dir, m.ID+".*.tmp")
	if err != nil {
		return fmt.Errorf("migrate: create temp: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("migrate: write temp: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("migrate: chmod temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("migrate: close temp: %w", err)
	}
	if err := os.Rename(tmpName, final); err != nil {
		cleanup()
		return fmt.Errorf("migrate: rename: %w", err)
	}
	return nil
}

// Create writes a new row. Returns an error if the ID already exists.
// Sets Version=1 and stamps CreatedAt/UpdatedAt if the caller left them
// zero.
func (s *jsonStore) Create(ctx context.Context, m *StoredMigration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if m == nil || m.ID == "" {
		return errors.New("migrate: Create: missing ID")
	}
	lk, err := lockFile(s.lockPathFor(m.ID))
	if err != nil {
		return fmt.Errorf("migrate: lock: %w", err)
	}
	defer releaseLock(lk)

	if _, err := os.Stat(s.pathFor(m.ID)); err == nil {
		return fmt.Errorf("migrate: row %s already exists", m.ID)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("migrate: stat: %w", err)
	}
	now := time.Now().UTC()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	m.UpdatedAt = now
	if m.Version == 0 {
		m.Version = 1
	}
	return s.writeRow(m)
}

// Get loads a row by ID. Returns ErrNotFound if missing or ErrExpired if
// ExpiresAt has already passed.
func (s *jsonStore) Get(ctx context.Context, id string) (*StoredMigration, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m, err := s.readRow(id)
	if err != nil {
		return nil, err
	}
	if !m.ExpiresAt.IsZero() && m.ExpiresAt.Before(time.Now().UTC()) {
		return nil, ErrExpired
	}
	return m, nil
}

// Update enforces optimistic locking: the caller must submit
// m.Version == stored.Version + 1. On success the row is written with the
// submitted version and a fresh UpdatedAt.
func (s *jsonStore) Update(ctx context.Context, m *StoredMigration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if m == nil || m.ID == "" {
		return errors.New("migrate: Update: missing ID")
	}
	lk, err := lockFile(s.lockPathFor(m.ID))
	if err != nil {
		return fmt.Errorf("migrate: lock: %w", err)
	}
	defer releaseLock(lk)

	existing, err := s.readRow(m.ID)
	if err != nil {
		return err
	}
	if existing.Version+1 != m.Version {
		return ErrVersionMismatch
	}
	m.CreatedAt = existing.CreatedAt
	m.UpdatedAt = time.Now().UTC()
	return s.writeRow(m)
}

// Delete removes the row and its lock sidecar. Idempotent.
func (s *jsonStore) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	lk, err := lockFile(s.lockPathFor(id))
	if err != nil {
		return fmt.Errorf("migrate: lock: %w", err)
	}
	// remove data file first, then release + remove lock
	if err := os.Remove(s.pathFor(id)); err != nil && !errors.Is(err, os.ErrNotExist) {
		releaseLock(lk)
		return fmt.Errorf("migrate: remove row: %w", err)
	}
	releaseLock(lk)
	if err := os.Remove(s.lockPathFor(id)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("migrate: remove lock: %w", err)
	}
	return nil
}

// List walks the state dir, loads rows matching filter, skips expired rows,
// sorts by CreatedAt ascending, and applies Limit/Offset.
func (s *jsonStore) List(ctx context.Context, filter ListFilter) ([]*StoredMigration, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("migrate: readdir: %w", err)
	}
	now := time.Now().UTC()
	var out []*StoredMigration
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		if strings.HasPrefix(name, ".") {
			continue
		}
		id := strings.TrimSuffix(name, ".json")
		m, err := s.readRow(id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return nil, err
		}
		if !m.ExpiresAt.IsZero() && m.ExpiresAt.Before(now) {
			continue
		}
		if filter.TenantID != "" && m.TenantID != filter.TenantID {
			continue
		}
		if filter.Status != "" && m.Status != filter.Status {
			continue
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	if filter.Offset > 0 {
		if filter.Offset >= len(out) {
			return nil, nil
		}
		out = out[filter.Offset:]
	}
	if filter.Limit > 0 && len(out) > filter.Limit {
		out = out[:filter.Limit]
	}
	return out, nil
}

// SweepExpired deletes all rows where ExpiresAt is before `before` and
// returns the count removed.
func (s *jsonStore) SweepExpired(ctx context.Context, before time.Time) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return 0, fmt.Errorf("migrate: readdir: %w", err)
	}
	count := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		if strings.HasPrefix(name, ".") {
			continue
		}
		id := strings.TrimSuffix(name, ".json")
		m, err := s.readRow(id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return count, err
		}
		if m.ExpiresAt.IsZero() || !m.ExpiresAt.Before(before) {
			continue
		}
		if err := s.Delete(ctx, id); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// releaseLock closes the lock handle, ignoring errors (best-effort).
func releaseLock(c io.Closer) {
	if c != nil {
		_ = c.Close()
	}
}
