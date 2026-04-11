package migrate

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	entree "github.com/spoofcanary/dns-entree"
)

// Sentinel errors returned by MigrationStore implementations.
var (
	ErrNotFound        = errors.New("migrate: migration not found")
	ErrVersionMismatch = errors.New("migrate: version mismatch")
	ErrExpired         = errors.New("migrate: migration expired")
)

// Status constants for StoredMigration.Status (D-04).
const (
	StatusPreview          = "preview"
	StatusApplying         = "applying"
	StatusAwaitingNSChange = "awaiting_ns_change"
	StatusVerifying        = "verifying"
	StatusComplete         = "complete"
	StatusFailed           = "failed"
	StatusExpired          = "expired"
)

// StoredMigration is the persisted row for a stateful migration (D-04).
// TenantID is an opaque blob: the library does not interpret it (D-17).
type StoredMigration struct {
	ID                   string               `json:"id"`
	TenantID             string               `json:"tenant_id"`
	Status               string               `json:"status"`
	Domain               string               `json:"domain"`
	SourceSlug           string               `json:"source_slug"`
	TargetSlug           string               `json:"target_slug"`
	Preview              *Zone                `json:"preview"`
	PreviewRecords       []entree.Record      `json:"preview_records"`
	TargetZoneID         string               `json:"target_zone_id"`
	TargetNameservers    []string             `json:"target_nameservers"`
	ApplyResults         []*entree.PushResult `json:"apply_results"`
	NSChangeInstructions string               `json:"ns_change_instructions"`
	ErrorMessage         string               `json:"error_message"`
	CredentialBlob       []byte               `json:"credential_blob"`
	AccessToken          string               `json:"access_token"`
	ExpiresAt            time.Time            `json:"expires_at"`
	Version              int64                `json:"version"`
	CreatedAt            time.Time            `json:"created_at"`
	UpdatedAt            time.Time            `json:"updated_at"`
}

// ListFilter narrows results returned by MigrationStore.List.
// Zero value returns everything.
type ListFilter struct {
	TenantID string
	Status   string
	Limit    int
	Offset   int
}

// MigrationStore persists StoredMigration rows (D-03). Implementations must
// be safe for concurrent use and enforce optimistic locking in Update via
// StoredMigration.Version (D-05, D-15).
type MigrationStore interface {
	Create(ctx context.Context, m *StoredMigration) error
	Get(ctx context.Context, id string) (*StoredMigration, error)
	Update(ctx context.Context, m *StoredMigration) error
	List(ctx context.Context, filter ListFilter) ([]*StoredMigration, error)
	Delete(ctx context.Context, id string) error
	SweepExpired(ctx context.Context, before time.Time) (int, error)
}

// NewMigrationID returns a UUIDv7 string. UUIDv7 encodes a millisecond
// timestamp in the high bits, so IDs sort by creation time lexicographically
// (D-18).
func NewMigrationID() string {
	return uuid.Must(uuid.NewV7()).String()
}
