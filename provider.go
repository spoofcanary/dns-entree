package entree

import "context"

// Provider is the interface for DNS provider integrations.
// Name() and Slug() return static metadata.
// All operations accept context.Context (QUAL-01).
type Provider interface {
	Name() string
	Slug() string
	Verify(ctx context.Context) ([]Zone, error)
	GetRecords(ctx context.Context, domain, recordType string) ([]Record, error)
	SetRecord(ctx context.Context, domain string, record Record) error
	DeleteRecord(ctx context.Context, domain, recordID string) error
	ApplyRecords(ctx context.Context, domain string, records []Record) error
}
