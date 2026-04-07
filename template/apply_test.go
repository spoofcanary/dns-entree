package template

import (
	"context"
	"errors"
	"strings"
	"testing"

	entree "github.com/spoofcanary/dns-entree"
)

// fakeProvider is a minimal in-memory Provider for ApplyTemplate tests.
type fakeProvider struct {
	records   map[string][]entree.Record // keyed by record type
	setCalls  []entree.Record
	delCalls  []string
	getErr    error
	setErr    error
	delErr    error
	failOnSet string // record name that should fail SetRecord
	idCounter int
}

func newFakeProvider() *fakeProvider {
	return &fakeProvider{records: map[string][]entree.Record{}}
}

func (f *fakeProvider) Name() string { return "fake" }
func (f *fakeProvider) Slug() string { return "fake" }
func (f *fakeProvider) Verify(ctx context.Context) ([]entree.Zone, error) {
	return nil, nil
}
func (f *fakeProvider) GetRecords(ctx context.Context, domain, recordType string) ([]entree.Record, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	src := f.records[recordType]
	out := make([]entree.Record, len(src))
	copy(out, src)
	return out, nil
}
func (f *fakeProvider) SetRecord(ctx context.Context, domain string, record entree.Record) error {
	if f.setErr != nil {
		return f.setErr
	}
	if f.failOnSet != "" && record.Name == f.failOnSet {
		return errors.New("synthetic set failure")
	}
	f.setCalls = append(f.setCalls, record)
	if record.ID == "" {
		f.idCounter++
		record.ID = "id-" + itoa(f.idCounter)
	}
	existing := f.records[record.Type]
	for i := range existing {
		if existing[i].Name == record.Name && existing[i].Content == record.Content {
			existing[i] = record
			f.records[record.Type] = existing
			return nil
		}
	}
	f.records[record.Type] = append(existing, record)
	return nil
}
func (f *fakeProvider) DeleteRecord(ctx context.Context, domain, recordID string) error {
	if f.delErr != nil {
		return f.delErr
	}
	f.delCalls = append(f.delCalls, recordID)
	for typ, recs := range f.records {
		filtered := recs[:0]
		for _, r := range recs {
			if r.ID != recordID {
				filtered = append(filtered, r)
			}
		}
		f.records[typ] = filtered
	}
	return nil
}
func (f *fakeProvider) ApplyRecords(ctx context.Context, domain string, records []entree.Record) error {
	for _, r := range records {
		if err := f.SetRecord(ctx, domain, r); err != nil {
			return err
		}
	}
	return nil
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}

func stubVerifyAlways(t *testing.T) {
	t.Helper()
	restore := entree.SetVerifyFuncForTest(func(ctx context.Context, domain string, opts entree.VerifyOpts) (entree.VerifyResult, error) {
		return entree.VerifyResult{Verified: true, CurrentValue: opts.Contains, Method: "stub"}, nil
	})
	t.Cleanup(restore)
}

func mkTemplate(records ...TemplateRecord) *Template {
	return &Template{Records: records}
}

func TestApplyTemplate_PrefixConflict(t *testing.T) {
	stubVerifyAlways(t)
	fp := newFakeProvider()
	fp.records["TXT"] = []entree.Record{
		{ID: "old1", Type: "TXT", Name: "_dmarc.example.com", Content: "v=DMARC1; p=none;"},
	}
	svc := entree.NewPushService(fp)
	tmpl := mkTemplate(TemplateRecord{
		Type: "TXT", Host: "_dmarc.example.com", Data: "v=DMARC1; p=reject;",
		TxtConflictMatchingMode: "Prefix", TxtConflictMatchingPrefix: "v=DMARC1",
	})
	results, err := ApplyTemplate(context.Background(), svc, "example.com", tmpl, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results=%d", len(results))
	}
	if len(fp.delCalls) != 1 || fp.delCalls[0] != "old1" {
		t.Errorf("delCalls=%v", fp.delCalls)
	}
	if len(fp.setCalls) != 1 || fp.setCalls[0].Content != "v=DMARC1; p=reject;" {
		t.Errorf("setCalls=%+v", fp.setCalls)
	}
}

func TestApplyTemplate_ExactConflict(t *testing.T) {
	stubVerifyAlways(t)
	fp := newFakeProvider()
	fp.records["TXT"] = []entree.Record{
		{ID: "ex1", Type: "TXT", Name: "host.example.com", Content: "exact-match"},
		{ID: "ex2", Type: "TXT", Name: "host.example.com", Content: "different"},
	}
	svc := entree.NewPushService(fp)
	tmpl := mkTemplate(TemplateRecord{
		Type: "TXT", Host: "host.example.com", Data: "exact-match",
		TxtConflictMatchingMode: "Exact",
	})
	_, err := ApplyTemplate(context.Background(), svc, "example.com", tmpl, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(fp.delCalls) != 1 || fp.delCalls[0] != "ex1" {
		t.Errorf("delCalls=%v", fp.delCalls)
	}
}

func TestApplyTemplate_AllConflict(t *testing.T) {
	stubVerifyAlways(t)
	fp := newFakeProvider()
	fp.records["TXT"] = []entree.Record{
		{ID: "a", Type: "TXT", Name: "h.example.com", Content: "one"},
		{ID: "b", Type: "TXT", Name: "h.example.com", Content: "two"},
		{ID: "c", Type: "TXT", Name: "h.example.com", Content: "three"},
		{ID: "d", Type: "TXT", Name: "other.example.com", Content: "untouched"},
	}
	svc := entree.NewPushService(fp)
	tmpl := mkTemplate(TemplateRecord{
		Type: "TXT", Host: "h.example.com", Data: "fresh",
		TxtConflictMatchingMode: "All",
	})
	_, err := ApplyTemplate(context.Background(), svc, "example.com", tmpl, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(fp.delCalls) != 3 {
		t.Errorf("delCalls=%v", fp.delCalls)
	}
}

func TestApplyTemplate_AllConflictApexWarning(t *testing.T) {
	stubVerifyAlways(t)
	fp := newFakeProvider()
	fp.records["TXT"] = []entree.Record{
		{ID: "spf", Type: "TXT", Name: "@", Content: "v=spf1 include:_spf.google.com ~all"},
		{ID: "gsv", Type: "TXT", Name: "@", Content: "google-site-verification=abc123"},
		{ID: "other", Type: "TXT", Name: "other.example.com", Content: "untouched"},
	}
	svc := entree.NewPushService(fp)
	tmpl := mkTemplate(TemplateRecord{
		Type: "TXT", Host: "@", Data: "v=new-apex",
		TxtConflictMatchingMode: "All",
	})
	results, warnings, err := ApplyTemplateWithReport(context.Background(), svc, "example.com", tmpl, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results=%d", len(results))
	}
	// Operation still proceeds — All is spec-compliant.
	if len(fp.delCalls) != 2 {
		t.Errorf("expected 2 deletions (spf+gsv), got %v", fp.delCalls)
	}
	// Warning must fire and must list the clobbered records.
	if len(warnings) != 1 {
		t.Fatalf("expected exactly 1 warning, got %v", warnings)
	}
	w := warnings[0]
	if !strings.Contains(w, "apex") {
		t.Errorf("warning missing 'apex': %s", w)
	}
	if !strings.Contains(w, "v=spf1") {
		t.Errorf("warning missing SPF content: %s", w)
	}
	if !strings.Contains(w, "google-site-verification") {
		t.Errorf("warning missing GSV content: %s", w)
	}
	if !strings.Contains(w, "SPF") || !strings.Contains(w, "DKIM") || !strings.Contains(w, "DMARC") {
		t.Errorf("warning missing risk callout: %s", w)
	}
}

func TestApplyTemplate_AllConflictNonApexNoWarning(t *testing.T) {
	stubVerifyAlways(t)
	fp := newFakeProvider()
	fp.records["TXT"] = []entree.Record{
		{ID: "a", Type: "TXT", Name: "_dmarc.example.com", Content: "v=DMARC1; p=none"},
	}
	svc := entree.NewPushService(fp)
	tmpl := mkTemplate(TemplateRecord{
		Type: "TXT", Host: "_dmarc.example.com", Data: "v=DMARC1; p=reject",
		TxtConflictMatchingMode: "All",
	})
	_, warnings, err := ApplyTemplateWithReport(context.Background(), svc, "example.com", tmpl, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for non-apex, got %v", warnings)
	}
}

func TestApplyTemplate_NoneConflict(t *testing.T) {
	stubVerifyAlways(t)
	fp := newFakeProvider()
	fp.records["TXT"] = []entree.Record{
		{ID: "x", Type: "TXT", Name: "h.example.com", Content: "stay"},
	}
	svc := entree.NewPushService(fp)
	tmpl := mkTemplate(TemplateRecord{
		Type: "TXT", Host: "h.example.com", Data: "added",
		TxtConflictMatchingMode: "None",
	})
	_, err := ApplyTemplate(context.Background(), svc, "example.com", tmpl, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(fp.delCalls) != 0 {
		t.Errorf("expected no deletes, got %v", fp.delCalls)
	}
}

func TestApplyTemplate_CNAMERouting(t *testing.T) {
	stubVerifyAlways(t)
	fp := newFakeProvider()
	svc := entree.NewPushService(fp)
	tmpl := mkTemplate(TemplateRecord{
		Type: "CNAME", Host: "www.example.com", PointsTo: "target.example.net",
	})
	_, err := ApplyTemplate(context.Background(), svc, "example.com", tmpl, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(fp.setCalls) != 1 || fp.setCalls[0].Type != "CNAME" {
		t.Fatalf("setCalls=%+v", fp.setCalls)
	}
}

func TestApplyTemplate_ARouting(t *testing.T) {
	stubVerifyAlways(t)
	fp := newFakeProvider()
	svc := entree.NewPushService(fp)
	tmpl := mkTemplate(TemplateRecord{
		Type: "A", Host: "a.example.com", PointsTo: "192.0.2.1",
	})
	_, err := ApplyTemplate(context.Background(), svc, "example.com", tmpl, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(fp.setCalls) != 1 || fp.setCalls[0].Type != "A" {
		t.Fatalf("setCalls=%+v", fp.setCalls)
	}
}

func TestApplyTemplate_MXPriorityPreserved(t *testing.T) {
	stubVerifyAlways(t)
	fp := newFakeProvider()
	svc := entree.NewPushService(fp)
	tmpl := mkTemplate(TemplateRecord{
		Type: "MX", Host: "example.com", PointsTo: "mail.example.net", Priority: flexInt{Value: 10},
	})
	_, err := ApplyTemplate(context.Background(), svc, "example.com", tmpl, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(fp.setCalls) != 1 || fp.setCalls[0].Priority != 10 {
		t.Fatalf("setCalls=%+v", fp.setCalls)
	}
}

func TestApplyTemplate_SRVAllFields(t *testing.T) {
	stubVerifyAlways(t)
	fp := newFakeProvider()
	svc := entree.NewPushService(fp)
	tmpl := mkTemplate(TemplateRecord{
		Type: "SRV", Host: "_sip._tcp.example.com", PointsTo: "sip.example.net",
		Priority: flexInt{Value: 10}, Weight: flexInt{Value: 20}, Port: flexInt{Value: 5060}, Service: "_sip", Protocol: "_tcp",
	})
	_, err := ApplyTemplate(context.Background(), svc, "example.com", tmpl, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(fp.setCalls) != 1 {
		t.Fatalf("setCalls=%d", len(fp.setCalls))
	}
	got := fp.setCalls[0]
	if got.Priority != 10 || got.Weight != 20 || got.Port != 5060 || got.Service != "_sip" || got.Protocol != "_tcp" {
		t.Errorf("SRV fields lost: %+v", got)
	}
}

func TestApplyTemplate_SPFMRouting(t *testing.T) {
	stubVerifyAlways(t)
	fp := newFakeProvider()
	svc := entree.NewPushService(fp)
	tmpl := mkTemplate(TemplateRecord{
		Type: "SPFM", Host: "example.com", Data: "include:_spf.foo.com",
	})
	_, err := ApplyTemplate(context.Background(), svc, "example.com", tmpl, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(fp.setCalls) != 1 {
		t.Fatalf("setCalls=%d", len(fp.setCalls))
	}
	got := fp.setCalls[0]
	if got.Type != "TXT" || !strings.Contains(got.Content, "include:_spf.foo.com") {
		t.Errorf("SPFM did not produce expected TXT: %+v", got)
	}
}

func TestApplyTemplate_ApexCNAME(t *testing.T) {
	stubVerifyAlways(t)
	fp := newFakeProvider()
	svc := entree.NewPushService(fp)
	tmpl := mkTemplate(TemplateRecord{
		Type: "CNAME", Host: "alias.example.com", PointsTo: "@",
	})
	_, err := ApplyTemplate(context.Background(), svc, "example.com", tmpl, nil)
	if err != nil {
		t.Fatalf("apex CNAME pointsTo=@ should resolve, got: %v", err)
	}
	if len(fp.setCalls) != 1 || fp.setCalls[0].Content != "@" {
		t.Errorf("setCalls=%+v", fp.setCalls)
	}
}

func TestApplyTemplate_SPFMEmptyDataSkipped(t *testing.T) {
	stubVerifyAlways(t)
	fp := newFakeProvider()
	svc := entree.NewPushService(fp)
	tmpl := mkTemplate(TemplateRecord{
		Type: "SPFM", Host: "example.com", // no Data, no PointsTo
	})
	results, err := ApplyTemplate(context.Background(), svc, "example.com", tmpl, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results=%d", len(results))
	}
	if results[0].Status == entree.StatusFailed {
		t.Errorf("expected non-failed status for empty SPFM, got %+v", results[0])
	}
	if len(fp.setCalls) != 0 {
		t.Errorf("expected no set calls, got %+v", fp.setCalls)
	}
}

func TestApplyTemplate_SPFMPointsToFallback(t *testing.T) {
	stubVerifyAlways(t)
	fp := newFakeProvider()
	svc := entree.NewPushService(fp)
	tmpl := mkTemplate(TemplateRecord{
		Type: "SPFM", Host: "example.com", PointsTo: "_spf.fallback.com",
	})
	_, err := ApplyTemplate(context.Background(), svc, "example.com", tmpl, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(fp.setCalls) != 1 || !strings.Contains(fp.setCalls[0].Content, "include:_spf.fallback.com") {
		t.Errorf("expected include from pointsTo fallback, got %+v", fp.setCalls)
	}
}

func TestApplyTemplate_ResolveError(t *testing.T) {
	stubVerifyAlways(t)
	fp := newFakeProvider()
	svc := entree.NewPushService(fp)
	tmpl := mkTemplate(TemplateRecord{
		Type: "TXT", Host: "%missing%.example.com", Data: "x",
	})
	results, err := ApplyTemplate(context.Background(), svc, "example.com", tmpl, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if results != nil {
		t.Errorf("expected nil results, got %v", results)
	}
	if len(fp.setCalls) != 0 {
		t.Errorf("expected no pushes")
	}
}

func TestApplyTemplate_PartialFailure(t *testing.T) {
	stubVerifyAlways(t)
	fp := newFakeProvider()
	fp.failOnSet = "fail.example.com"
	svc := entree.NewPushService(fp)
	tmpl := mkTemplate(
		TemplateRecord{Type: "TXT", Host: "ok1.example.com", Data: "a"},
		TemplateRecord{Type: "TXT", Host: "fail.example.com", Data: "b"},
		TemplateRecord{Type: "TXT", Host: "ok2.example.com", Data: "c"},
	)
	results, err := ApplyTemplate(context.Background(), svc, "example.com", tmpl, nil)
	if err == nil {
		t.Fatal("expected joined error")
	}
	if len(results) != 3 {
		t.Fatalf("results=%d", len(results))
	}
	if results[1].Status != entree.StatusFailed {
		t.Errorf("middle result not failed: %+v", results[1])
	}
	if results[0].Status == entree.StatusFailed || results[2].Status == entree.StatusFailed {
		t.Errorf("non-failing results marked failed")
	}
}

func TestApplyTemplate_ConflictDeleteFails(t *testing.T) {
	stubVerifyAlways(t)
	fp := newFakeProvider()
	fp.records["TXT"] = []entree.Record{
		{ID: "x", Type: "TXT", Name: "h.example.com", Content: "v=DMARC1; old"},
	}
	fp.delErr = errors.New("delete boom")
	svc := entree.NewPushService(fp)
	tmpl := mkTemplate(
		TemplateRecord{
			Type: "TXT", Host: "h.example.com", Data: "v=DMARC1; new",
			TxtConflictMatchingMode: "Prefix", TxtConflictMatchingPrefix: "v=DMARC1",
		},
		TemplateRecord{Type: "TXT", Host: "ok.example.com", Data: "ok"},
	)
	results, err := ApplyTemplate(context.Background(), svc, "example.com", tmpl, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if len(results) != 2 {
		t.Fatalf("results=%d", len(results))
	}
	if results[0].Status != entree.StatusFailed {
		t.Errorf("first should be failed: %+v", results[0])
	}
	if results[1].Status == entree.StatusFailed {
		t.Errorf("second should have succeeded: %+v", results[1])
	}
}
