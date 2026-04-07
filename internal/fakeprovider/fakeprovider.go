// Package fakeprovider registers an in-process fake DNS Provider under slug
// "fake". It is shared by the CLI and HTTP API test suites so both can drive
// the library against a deterministic, side-effect-free backend. The slug is
// inert in production unless explicitly selected.
package fakeprovider

import (
	"context"
	"os"

	entree "github.com/spoofcanary/dns-entree"
)

// Provider is an in-memory entree.Provider implementation.
type Provider struct {
	records map[string][]entree.Record
}

// New constructs an empty in-memory fake provider.
func New() *Provider {
	return &Provider{records: map[string][]entree.Record{}}
}

func (f *Provider) Name() string { return "Fake" }
func (f *Provider) Slug() string { return "fake" }

func (f *Provider) Verify(ctx context.Context) ([]entree.Zone, error) {
	return []entree.Zone{{ID: "fake-zone", Name: "example.com", Status: "active"}}, nil
}

func (f *Provider) GetRecords(ctx context.Context, domain, recordType string) ([]entree.Record, error) {
	return append([]entree.Record(nil), f.records[recordType]...), nil
}

func (f *Provider) SetRecord(ctx context.Context, domain string, r entree.Record) error {
	list := f.records[r.Type]
	for i := range list {
		if list[i].Name == r.Name {
			list[i] = r
			f.records[r.Type] = list
			return nil
		}
	}
	f.records[r.Type] = append(list, r)
	return nil
}

func (f *Provider) DeleteRecord(ctx context.Context, domain, recordID string) error {
	return nil
}

func (f *Provider) ApplyRecords(ctx context.Context, domain string, records []entree.Record) error {
	for _, r := range records {
		_ = f.SetRecord(ctx, domain, r)
	}
	return nil
}

func init() {
	entree.RegisterProvider("fake", func(c entree.Credentials) (entree.Provider, error) {
		return New(), nil
	})
	if os.Getenv("ENTREE_TEST_NO_VERIFY") == "1" {
		entree.SetVerifyFuncForTest(func(ctx context.Context, domain string, opts entree.VerifyOpts) (entree.VerifyResult, error) {
			return entree.VerifyResult{Verified: false, Method: "stubbed"}, nil
		})
	}
}
