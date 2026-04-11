package entree_test

import (
	"context"
	"fmt"

	entree "github.com/spoofcanary/dns-entree"
)

// fakeProvider is an in-memory [entree.Provider] used only by the runnable
// example below. It stores records in a map keyed by type so PushService can
// exercise its full upsert + verification path without touching the network.
type fakeProvider struct {
	records map[string][]entree.Record
}

func (f *fakeProvider) Name() string { return "Fake" }
func (f *fakeProvider) Slug() string { return "fake" }

func (f *fakeProvider) Verify(ctx context.Context) ([]entree.Zone, error) {
	return []entree.Zone{{ID: "z1", Name: "example.com", Status: "active"}}, nil
}

func (f *fakeProvider) GetRecords(ctx context.Context, domain, recordType string) ([]entree.Record, error) {
	return f.records[recordType], nil
}

func (f *fakeProvider) SetRecord(ctx context.Context, domain string, record entree.Record) error {
	existing := f.records[record.Type]
	for i := range existing {
		if existing[i].Name == record.Name {
			existing[i] = record
			f.records[record.Type] = existing
			return nil
		}
	}
	f.records[record.Type] = append(existing, record)
	return nil
}

func (f *fakeProvider) DeleteRecord(ctx context.Context, domain, recordID string) error {
	return nil
}

func (f *fakeProvider) ApplyRecords(ctx context.Context, domain string, records []entree.Record) error {
	return entree.DefaultApplyRecords(f, ctx, domain, records)
}

// ExamplePushService demonstrates the end-to-end PushService quickstart:
// build a provider, wrap it in a PushService, and push a TXT record
// idempotently. The second push is a no-op because the record is already
// configured.
func ExamplePushService() {
	// Stub the DNS verify seam so the example is deterministic offline.
	restore := entree.SetVerifyFuncForTest(func(ctx context.Context, domain string, opts entree.VerifyOpts) (entree.VerifyResult, error) {
		return entree.VerifyResult{Verified: true, CurrentValue: opts.Contains, Method: "authoritative"}, nil
	})
	defer restore()

	provider := &fakeProvider{records: map[string][]entree.Record{}}
	svc := entree.NewPushService(provider)
	ctx := context.Background()

	res, err := svc.PushTXTRecord(ctx, "example.com", "_dmarc.example.com",
		"v=DMARC1; p=none; rua=mailto:dmarc@example.com")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("first: status=%s verified=%t\n", res.Status, res.Verified)

	res2, _ := svc.PushTXTRecord(ctx, "example.com", "_dmarc.example.com",
		"v=DMARC1; p=none; rua=mailto:dmarc@example.com")
	fmt.Printf("second: status=%s\n", res2.Status)

	// Output:
	// first: status=created verified=true
	// second: status=already_configured
}
