package main

// In-process fake Provider registered under slug "fake". Intended for CLI
// integration tests and local experimentation; has no network side effects.
// Safe to ship in production binaries -- the slug is inert unless explicitly
// selected via --provider fake.

import (
	"context"
	"os"

	entree "github.com/spoofcanary/dns-entree"
)

type fakeProvider struct {
	records map[string][]entree.Record
}

func newFakeProvider() *fakeProvider {
	return &fakeProvider{records: map[string][]entree.Record{}}
}

func (f *fakeProvider) Name() string { return "Fake" }
func (f *fakeProvider) Slug() string { return "fake" }
func (f *fakeProvider) Verify(ctx context.Context) ([]entree.Zone, error) {
	return []entree.Zone{{ID: "fake-zone", Name: "example.com", Status: "active"}}, nil
}
func (f *fakeProvider) GetRecords(ctx context.Context, domain, recordType string) ([]entree.Record, error) {
	return append([]entree.Record(nil), f.records[recordType]...), nil
}
func (f *fakeProvider) SetRecord(ctx context.Context, domain string, r entree.Record) error {
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
func (f *fakeProvider) DeleteRecord(ctx context.Context, domain, recordID string) error {
	return nil
}
func (f *fakeProvider) ApplyRecords(ctx context.Context, domain string, records []entree.Record) error {
	for _, r := range records {
		_ = f.SetRecord(ctx, domain, r)
	}
	return nil
}

func init() {
	entree.RegisterProvider("fake", func(c entree.Credentials) (entree.Provider, error) {
		return newFakeProvider(), nil
	})
	if os.Getenv("ENTREE_TEST_NO_VERIFY") == "1" {
		entree.SetVerifyFuncForTest(func(ctx context.Context, domain string, opts entree.VerifyOpts) (entree.VerifyResult, error) {
			return entree.VerifyResult{Verified: false, Method: "stubbed"}, nil
		})
	}
}
