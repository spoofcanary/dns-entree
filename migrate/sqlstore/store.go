//go:build sqlite

// Package sqlstore provides an optional SQLite-backed MigrationStore.
//
// This package is gated behind the `sqlite` build tag so that default builds
// of dns-entree do not link modernc.org/sqlite (D-07, D-08). Callers opt in
// with `go build -tags=sqlite`.
package sqlstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spoofcanary/dns-entree/migrate"

	_ "modernc.org/sqlite"
)

const schemaDDL = `
CREATE TABLE IF NOT EXISTS migrations (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    status TEXT NOT NULL,
    domain TEXT NOT NULL,
    source_slug TEXT NOT NULL,
    target_slug TEXT NOT NULL,
    target_zone_id TEXT NOT NULL,
    preview BLOB,
    preview_records BLOB,
    target_nameservers BLOB,
    apply_results BLOB,
    ns_change_instructions TEXT NOT NULL,
    error_message TEXT NOT NULL,
    credential_blob BLOB,
    access_token TEXT NOT NULL,
    expires_at INTEGER NOT NULL,
    version INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_migrations_tenant ON migrations(tenant_id);
CREATE INDEX IF NOT EXISTS idx_migrations_status ON migrations(status);
CREATE INDEX IF NOT EXISTS idx_migrations_expires ON migrations(expires_at);
`

// sqliteStore implements migrate.MigrationStore against a modernc.org/sqlite
// database. Concurrent writers are serialized by WAL mode and a 5s busy
// timeout; optimistic locking uses `UPDATE ... WHERE id = ? AND version = ?`.
type sqliteStore struct {
	db *sql.DB
}

var _ migrate.MigrationStore = (*sqliteStore)(nil)

// New opens (or creates) a SQLite database at the given path and returns a
// MigrationStore backed by it. Schema and indexes are created on first run.
func New(path string) (migrate.MigrationStore, error) {
	if path == "" {
		return nil, errors.New("sqlstore: path is empty")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sqlstore: open: %w", err)
	}
	// Single writer + WAL readers; keep the pool small to avoid SQLITE_BUSY.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlstore: set WAL: %w", err)
	}
	if _, err := db.Exec(`PRAGMA busy_timeout=5000`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlstore: set busy_timeout: %w", err)
	}
	if _, err := db.Exec(schemaDDL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlstore: init schema: %w", err)
	}
	return &sqliteStore{db: db}, nil
}

func marshalJSON(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

func unmarshalJSON(data []byte, v any) error {
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, v)
}

// Create inserts a new row. Returns an error if the ID already exists.
func (s *sqliteStore) Create(ctx context.Context, m *migrate.StoredMigration) error {
	if m == nil || m.ID == "" {
		return errors.New("sqlstore: Create: missing ID")
	}
	now := time.Now().UTC()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	m.UpdatedAt = now
	if m.Version == 0 {
		m.Version = 1
	}

	previewBlob, err := marshalJSON(m.Preview)
	if err != nil {
		return fmt.Errorf("sqlstore: marshal preview: %w", err)
	}
	previewRecordsBlob, err := marshalJSON(m.PreviewRecords)
	if err != nil {
		return fmt.Errorf("sqlstore: marshal preview_records: %w", err)
	}
	nsBlob, err := marshalJSON(m.TargetNameservers)
	if err != nil {
		return fmt.Errorf("sqlstore: marshal target_nameservers: %w", err)
	}
	applyBlob, err := marshalJSON(m.ApplyResults)
	if err != nil {
		return fmt.Errorf("sqlstore: marshal apply_results: %w", err)
	}

	const q = `
INSERT INTO migrations (
    id, tenant_id, status, domain, source_slug, target_slug, target_zone_id,
    preview, preview_records, target_nameservers, apply_results,
    ns_change_instructions, error_message, credential_blob, access_token,
    expires_at, version, created_at, updated_at
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`
	_, err = s.db.ExecContext(ctx, q,
		m.ID, m.TenantID, m.Status, m.Domain, m.SourceSlug, m.TargetSlug, m.TargetZoneID,
		previewBlob, previewRecordsBlob, nsBlob, applyBlob,
		m.NSChangeInstructions, m.ErrorMessage, m.CredentialBlob, m.AccessToken,
		m.ExpiresAt.UnixNano(), m.Version, m.CreatedAt.UnixNano(), m.UpdatedAt.UnixNano(),
	)
	if err != nil {
		return fmt.Errorf("sqlstore: insert: %w", err)
	}
	return nil
}

// scanRow decodes one row from a *sql.Row or *sql.Rows into a StoredMigration.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanRow(r rowScanner) (*migrate.StoredMigration, error) {
	var (
		m                                                  migrate.StoredMigration
		previewBlob, previewRecordsBlob, nsBlob, applyBlob []byte
		credentialBlob                                     []byte
		expiresAtNS, createdAtNS, updatedAtNS              int64
	)
	if err := r.Scan(
		&m.ID, &m.TenantID, &m.Status, &m.Domain, &m.SourceSlug, &m.TargetSlug, &m.TargetZoneID,
		&previewBlob, &previewRecordsBlob, &nsBlob, &applyBlob,
		&m.NSChangeInstructions, &m.ErrorMessage, &credentialBlob, &m.AccessToken,
		&expiresAtNS, &m.Version, &createdAtNS, &updatedAtNS,
	); err != nil {
		return nil, err
	}
	m.CredentialBlob = credentialBlob
	m.ExpiresAt = time.Unix(0, expiresAtNS).UTC()
	m.CreatedAt = time.Unix(0, createdAtNS).UTC()
	m.UpdatedAt = time.Unix(0, updatedAtNS).UTC()
	if err := unmarshalJSON(previewBlob, &m.Preview); err != nil {
		return nil, fmt.Errorf("sqlstore: decode preview: %w", err)
	}
	if err := unmarshalJSON(previewRecordsBlob, &m.PreviewRecords); err != nil {
		return nil, fmt.Errorf("sqlstore: decode preview_records: %w", err)
	}
	if err := unmarshalJSON(nsBlob, &m.TargetNameservers); err != nil {
		return nil, fmt.Errorf("sqlstore: decode target_nameservers: %w", err)
	}
	if err := unmarshalJSON(applyBlob, &m.ApplyResults); err != nil {
		return nil, fmt.Errorf("sqlstore: decode apply_results: %w", err)
	}
	return &m, nil
}

const selectCols = `id, tenant_id, status, domain, source_slug, target_slug, target_zone_id,
    preview, preview_records, target_nameservers, apply_results,
    ns_change_instructions, error_message, credential_blob, access_token,
    expires_at, version, created_at, updated_at`

// Get loads a row by ID. Returns ErrNotFound if missing, ErrExpired if the
// row's ExpiresAt has already passed.
func (s *sqliteStore) Get(ctx context.Context, id string) (*migrate.StoredMigration, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+selectCols+` FROM migrations WHERE id = ?`, id)
	m, err := scanRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, migrate.ErrNotFound
		}
		return nil, fmt.Errorf("sqlstore: get: %w", err)
	}
	if !m.ExpiresAt.IsZero() && m.ExpiresAt.Before(time.Now().UTC()) {
		return nil, migrate.ErrExpired
	}
	return m, nil
}

// Update performs an atomic optimistic-locking write: the UPDATE matches on
// (id, version = m.Version - 1). Zero rows affected returns ErrVersionMismatch.
func (s *sqliteStore) Update(ctx context.Context, m *migrate.StoredMigration) error {
	if m == nil || m.ID == "" {
		return errors.New("sqlstore: Update: missing ID")
	}
	previewBlob, err := marshalJSON(m.Preview)
	if err != nil {
		return fmt.Errorf("sqlstore: marshal preview: %w", err)
	}
	previewRecordsBlob, err := marshalJSON(m.PreviewRecords)
	if err != nil {
		return fmt.Errorf("sqlstore: marshal preview_records: %w", err)
	}
	nsBlob, err := marshalJSON(m.TargetNameservers)
	if err != nil {
		return fmt.Errorf("sqlstore: marshal target_nameservers: %w", err)
	}
	applyBlob, err := marshalJSON(m.ApplyResults)
	if err != nil {
		return fmt.Errorf("sqlstore: marshal apply_results: %w", err)
	}
	m.UpdatedAt = time.Now().UTC()
	const q = `
UPDATE migrations SET
    tenant_id = ?, status = ?, domain = ?, source_slug = ?, target_slug = ?, target_zone_id = ?,
    preview = ?, preview_records = ?, target_nameservers = ?, apply_results = ?,
    ns_change_instructions = ?, error_message = ?, credential_blob = ?, access_token = ?,
    expires_at = ?, version = ?, updated_at = ?
WHERE id = ? AND version = ?`
	res, err := s.db.ExecContext(ctx, q,
		m.TenantID, m.Status, m.Domain, m.SourceSlug, m.TargetSlug, m.TargetZoneID,
		previewBlob, previewRecordsBlob, nsBlob, applyBlob,
		m.NSChangeInstructions, m.ErrorMessage, m.CredentialBlob, m.AccessToken,
		m.ExpiresAt.UnixNano(), m.Version, m.UpdatedAt.UnixNano(),
		m.ID, m.Version-1,
	)
	if err != nil {
		return fmt.Errorf("sqlstore: update: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlstore: rows affected: %w", err)
	}
	if n == 0 {
		// Distinguish ErrNotFound from ErrVersionMismatch for caller clarity.
		var exists int
		if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM migrations WHERE id = ?`, m.ID).Scan(&exists); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return migrate.ErrNotFound
			}
			return fmt.Errorf("sqlstore: exists check: %w", err)
		}
		return migrate.ErrVersionMismatch
	}
	return nil
}

// Delete removes a row. Idempotent: no error on missing ID.
func (s *sqliteStore) Delete(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM migrations WHERE id = ?`, id); err != nil {
		return fmt.Errorf("sqlstore: delete: %w", err)
	}
	return nil
}

// List returns rows matching filter, sorted by created_at ascending, with
// optional Limit/Offset. Expired rows are excluded.
func (s *sqliteStore) List(ctx context.Context, filter migrate.ListFilter) ([]*migrate.StoredMigration, error) {
	var (
		where []string
		args  []any
	)
	where = append(where, `expires_at >= ?`)
	args = append(args, time.Now().UTC().UnixNano())
	if filter.TenantID != "" {
		where = append(where, `tenant_id = ?`)
		args = append(args, filter.TenantID)
	}
	if filter.Status != "" {
		where = append(where, `status = ?`)
		args = append(args, filter.Status)
	}
	q := `SELECT ` + selectCols + ` FROM migrations WHERE ` + strings.Join(where, " AND ") + ` ORDER BY created_at ASC`
	if filter.Limit > 0 {
		q += ` LIMIT ?`
		args = append(args, filter.Limit)
		if filter.Offset > 0 {
			q += ` OFFSET ?`
			args = append(args, filter.Offset)
		}
	} else if filter.Offset > 0 {
		// SQLite requires LIMIT before OFFSET; use -1 for "no limit".
		q += ` LIMIT -1 OFFSET ?`
		args = append(args, filter.Offset)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlstore: list: %w", err)
	}
	defer rows.Close()
	var out []*migrate.StoredMigration
	for rows.Next() {
		m, err := scanRow(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlstore: scan: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlstore: rows: %w", err)
	}
	return out, nil
}

// SweepExpired deletes rows with expires_at < before and returns the count.
func (s *sqliteStore) SweepExpired(ctx context.Context, before time.Time) (int, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM migrations WHERE expires_at < ?`, before.UnixNano())
	if err != nil {
		return 0, fmt.Errorf("sqlstore: sweep: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("sqlstore: sweep rows affected: %w", err)
	}
	return int(n), nil
}

// Close releases the underlying database handle.
func (s *sqliteStore) Close() error {
	return s.db.Close()
}
