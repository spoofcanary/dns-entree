package entree

import (
	"context"
	"fmt"
)

// DefaultApplyRecords applies records one at a time via SetRecord.
// Providers can call this from their ApplyRecords method as a default implementation.
// Providers with batch APIs (e.g. Route53 ChangeBatch) can override with their own.
func DefaultApplyRecords(p Provider, ctx context.Context, domain string, records []Record) error {
	for _, r := range records {
		if err := p.SetRecord(ctx, domain, r); err != nil {
			return fmt.Errorf("apply record %s %s: %w", r.Type, r.Name, err)
		}
	}
	return nil
}
