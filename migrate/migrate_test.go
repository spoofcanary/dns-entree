package migrate

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/miekg/dns"
	entree "github.com/spoofcanary/dns-entree"
)

// fakeAdapter is a migrate.Adapter that returns a fixed ZoneInfo. The test
// installs it via RegisterAdapter.
type fakeAdapter struct {
	info ZoneInfo
}

func (f *fakeAdapter) EnsureZone(ctx context.Context, domain string, opts ProviderOpts) (ZoneInfo, error) {
	return f.info, nil
}

// fakeProvider is an entree.Provider that records SetRecord calls and pushes
// each record into a backing fakeZone so the post-apply verifier can read them
// back via DNS.
type fakeProvider struct {
	mu      sync.Mutex
	zone    *fakeZone
	applied []entree.Record
}

func (p *fakeProvider) Name() string                                      { return "fake" }
func (p *fakeProvider) Slug() string                                      { return "fake-migrate" }
func (p *fakeProvider) Verify(ctx context.Context) ([]entree.Zone, error) { return nil, nil }
func (p *fakeProvider) GetRecords(ctx context.Context, domain, recordType string) ([]entree.Record, error) {
	return nil, nil
}
func (p *fakeProvider) DeleteRecord(ctx context.Context, domain, recordID string) error { return nil }
func (p *fakeProvider) ApplyRecords(ctx context.Context, domain string, records []entree.Record) error {
	for _, r := range records {
		if err := p.SetRecord(ctx, domain, r); err != nil {
			return err
		}
	}
	return nil
}
func (p *fakeProvider) SetRecord(ctx context.Context, domain string, r entree.Record) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.applied = append(p.applied, r)

	var rr dns.RR
	var err error
	switch r.Type {
	case "A":
		rr, err = dns.NewRR(r.Name + ". 300 IN A " + r.Content)
	case "TXT":
		rr, err = dns.NewRR(r.Name + ". 300 IN TXT \"" + r.Content + "\"")
	case "CNAME":
		rr, err = dns.NewRR(r.Name + ". 300 IN CNAME " + r.Content + ".")
	}
	if err != nil {
		return err
	}
	if rr != nil {
		p.zone.records = append(p.zone.records, rr)
	}
	return nil
}

func TestMigrate_EndToEnd_FakeSource_FakeTarget(t *testing.T) {
	// Source zone: seeded with 3 records. AXFR disabled, force iterated path.
	src := &fakeZone{
		domain:    "example.com",
		allowAXFR: false,
		records: []dns.RR{
			mustRR(t, "example.com. 3600 IN A 192.0.2.1"),
			mustRR(t, "www.example.com. 3600 IN A 192.0.2.2"),
			mustRR(t, "_dmarc.example.com. 3600 IN TXT \"v=DMARC1; p=none\""),
		},
	}
	srcAddr, stopSrc := startFakeAuth(t, src)
	defer stopSrc()

	// Target zone: empty, will be populated by fakeProvider.SetRecord.
	tgtZone := &fakeZone{domain: "example.com", allowAXFR: false}
	tgtAddr, stopTgt := startFakeAuth(t, tgtZone)
	defer stopTgt()

	// Override NS resolver so verifyRecordAgainstNS dials the target fake.
	origResolver := nsResolver
	nsResolver = func(ctx context.Context, host string) (string, error) {
		return tgtAddr, nil
	}
	defer func() { nsResolver = origResolver }()

	// Install fake adapter returning the target NS hostname.
	RegisterAdapter("fake-migrate-e2e", &fakeAdapter{
		info: ZoneInfo{
			ZoneID:      "z-1",
			Nameservers: []string{"ns1.fake.example."},
			Created:     true,
		},
	})

	prov := &fakeProvider{zone: tgtZone}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	report, err := Migrate(ctx, MigrateOptions{
		Domain:         "example.com",
		TargetSlug:     "fake-migrate-e2e",
		TargetProvider: prov,
		ScrapeOpts: ScrapeOptions{
			Nameservers:      []string{srcAddr},
			OnlyLabels:       []string{"@", "www", "_dmarc"},
			QueriesPerSecond: 500,
			SkipAXFR:         true,
		},
		Apply:              true,
		RatePerSecond:      1000,
		VerifyTimeout:      3 * time.Second,
		QueryTimeout:       2 * time.Second,
		SourceProviderSlug: "godaddy",
		SkipSourceDetect:   true,
	})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if report == nil {
		t.Fatal("nil report")
	}
	if len(report.Preview) == 0 {
		t.Fatal("empty preview")
	}
	if len(prov.applied) == 0 {
		t.Fatal("no records applied to fake target")
	}
	if len(report.Results) == 0 {
		t.Fatal("no per-record results")
	}
	for _, r := range report.Results {
		if r.Status != StatusMatched {
			t.Errorf("expected matched, got %s for %s %s: %s", r.Status, r.Record.Type, r.Record.Name, r.Detail)
		}
	}
	if report.NSChange == "" {
		t.Error("expected NS change instructions")
	}
	if report.TargetZoneStatus != "will_create" {
		t.Errorf("target zone status = %s", report.TargetZoneStatus)
	}
	if report.SourceProvider != "godaddy" {
		t.Errorf("source provider = %s", report.SourceProvider)
	}
}

func TestMigrate_DryRun_NoApply(t *testing.T) {
	src := &fakeZone{
		domain:    "example.com",
		allowAXFR: false,
		records: []dns.RR{
			mustRR(t, "example.com. 3600 IN A 192.0.2.1"),
		},
	}
	srcAddr, stopSrc := startFakeAuth(t, src)
	defer stopSrc()

	RegisterAdapter("fake-migrate-dryrun", &fakeAdapter{
		info: ZoneInfo{Nameservers: []string{"ns1.fake."}, Created: false},
	})

	tgtZone := &fakeZone{domain: "example.com"}
	prov := &fakeProvider{zone: tgtZone}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	report, err := Migrate(ctx, MigrateOptions{
		Domain:         "example.com",
		TargetSlug:     "fake-migrate-dryrun",
		TargetProvider: prov,
		ScrapeOpts: ScrapeOptions{
			Nameservers:      []string{srcAddr},
			OnlyLabels:       []string{"@"},
			QueriesPerSecond: 500,
			SkipAXFR:         true,
		},
		Apply:            false,
		SkipSourceDetect: true,
	})
	if err != nil {
		t.Fatalf("dry-run migrate: %v", err)
	}
	if len(prov.applied) != 0 {
		t.Errorf("dry-run should not apply records, got %d", len(prov.applied))
	}
	if len(report.Results) != 0 {
		t.Errorf("dry-run should not produce per-record results, got %d", len(report.Results))
	}
	if report.TargetZoneStatus != "exists" {
		t.Errorf("target zone status = %s", report.TargetZoneStatus)
	}
	if len(report.Preview) == 0 {
		t.Error("preview should have records")
	}
}
