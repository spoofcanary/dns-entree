package entree

import (
	"context"
	"errors"
	"testing"
)

type fakeProvider struct {
	records  map[string][]Record // keyed by record type
	setCalls []Record
	getErr   error
	setErr   error
}

func newFakeProvider() *fakeProvider {
	return &fakeProvider{records: map[string][]Record{}}
}

func (f *fakeProvider) Name() string { return "fake" }
func (f *fakeProvider) Slug() string { return "fake" }
func (f *fakeProvider) Verify(ctx context.Context) ([]Zone, error) {
	return nil, nil
}
func (f *fakeProvider) GetRecords(ctx context.Context, domain, recordType string) ([]Record, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.records[recordType], nil
}
func (f *fakeProvider) SetRecord(ctx context.Context, domain string, record Record) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.setCalls = append(f.setCalls, record)
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
func (f *fakeProvider) ApplyRecords(ctx context.Context, domain string, records []Record) error {
	for _, r := range records {
		if err := f.SetRecord(ctx, domain, r); err != nil {
			return err
		}
	}
	return nil
}

func stubVerify(t *testing.T, verified bool) {
	t.Helper()
	orig := verifyFunc
	verifyFunc = func(ctx context.Context, domain string, opts VerifyOpts) (VerifyResult, error) {
		return VerifyResult{Verified: verified, CurrentValue: opts.Contains, Method: "stub"}, nil
	}
	t.Cleanup(func() { verifyFunc = orig })
}

func TestPushTXT_Create(t *testing.T) {
	stubVerify(t, true)
	fp := newFakeProvider()
	s := NewPushService(fp)
	res, err := s.PushTXTRecord(context.Background(), "example.com", "_dmarc.example.com", "v=DMARC1; p=none")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusCreated {
		t.Errorf("status=%s", res.Status)
	}
	if len(fp.setCalls) != 1 {
		t.Errorf("setCalls=%d", len(fp.setCalls))
	}
	if !res.Verified {
		t.Error("expected verified")
	}
}

func TestPushTXT_AlreadyConfigured(t *testing.T) {
	stubVerify(t, true)
	fp := newFakeProvider()
	fp.records["TXT"] = []Record{{Type: "TXT", Name: "_dmarc.example.com", Content: "v=DMARC1; p=none"}}
	s := NewPushService(fp)
	res, err := s.PushTXTRecord(context.Background(), "example.com", "_dmarc.example.com", "v=DMARC1; p=none")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusAlreadyConfigured {
		t.Errorf("status=%s", res.Status)
	}
	if len(fp.setCalls) != 0 {
		t.Errorf("setCalls=%d", len(fp.setCalls))
	}
}

func TestPushTXT_Update(t *testing.T) {
	stubVerify(t, true)
	fp := newFakeProvider()
	fp.records["TXT"] = []Record{{Type: "TXT", Name: "_dmarc.example.com", Content: "v=DMARC1; p=none"}}
	s := NewPushService(fp)
	res, err := s.PushTXTRecord(context.Background(), "example.com", "_dmarc.example.com", "v=DMARC1; p=quarantine")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusUpdated {
		t.Errorf("status=%s", res.Status)
	}
	if res.PreviousValue != "v=DMARC1; p=none" {
		t.Errorf("prev=%q", res.PreviousValue)
	}
	if len(fp.setCalls) != 1 {
		t.Errorf("setCalls=%d", len(fp.setCalls))
	}
}

func TestPushTXT_SetRecordError(t *testing.T) {
	stubVerify(t, true)
	fp := newFakeProvider()
	fp.setErr = errors.New("boom")
	s := NewPushService(fp)
	res, err := s.PushTXTRecord(context.Background(), "example.com", "_dmarc.example.com", "x")
	if err == nil {
		t.Fatal("expected error")
	}
	if res.Status != StatusFailed {
		t.Errorf("status=%s", res.Status)
	}
}

func TestPushTXT_VerifyFailure(t *testing.T) {
	stubVerify(t, false)
	fp := newFakeProvider()
	s := NewPushService(fp)
	res, err := s.PushTXTRecord(context.Background(), "example.com", "_dmarc.example.com", "v=DMARC1; p=none")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusCreated {
		t.Errorf("status=%s", res.Status)
	}
	if res.Verified {
		t.Error("expected !verified")
	}
	if res.VerifyError == nil {
		t.Error("expected VerifyError")
	}
}

func TestPushSPF_CreateFromEmpty(t *testing.T) {
	stubVerify(t, true)
	fp := newFakeProvider()
	s := NewPushService(fp)
	res, err := s.PushSPFRecord(context.Background(), "example.com", []string{"_spf.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusCreated {
		t.Errorf("status=%s", res.Status)
	}
	if res.RecordValue != "v=spf1 include:_spf.example.com ~all" {
		t.Errorf("value=%q", res.RecordValue)
	}
}

func TestPushSPF_AlreadyConfigured(t *testing.T) {
	stubVerify(t, true)
	fp := newFakeProvider()
	fp.records["TXT"] = []Record{{Type: "TXT", Name: "example.com", Content: "v=spf1 include:_spf.example.com -all"}}
	s := NewPushService(fp)
	res, err := s.PushSPFRecord(context.Background(), "example.com", []string{"_spf.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusAlreadyConfigured {
		t.Errorf("status=%s", res.Status)
	}
	if len(fp.setCalls) != 0 {
		t.Errorf("setCalls=%d", len(fp.setCalls))
	}
}

func TestPushSPF_PreservesExisting(t *testing.T) {
	stubVerify(t, true)
	fp := newFakeProvider()
	fp.records["TXT"] = []Record{{Type: "TXT", Name: "example.com", Content: "v=spf1 ip4:1.2.3.4 -all"}}
	s := NewPushService(fp)
	res, err := s.PushSPFRecord(context.Background(), "example.com", []string{"_spf.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusUpdated {
		t.Errorf("status=%s", res.Status)
	}
	if !contains(res.RecordValue, "ip4:1.2.3.4") || !contains(res.RecordValue, "include:_spf.example.com") {
		t.Errorf("missing mechanisms: %q", res.RecordValue)
	}
	if res.PreviousValue == "" {
		t.Error("expected previous value")
	}
}

func TestPushCNAME_Create(t *testing.T) {
	stubVerify(t, true)
	fp := newFakeProvider()
	s := NewPushService(fp)
	res, err := s.PushCNAMERecord(context.Background(), "example.com", "mail.example.com", "ghs.googlehosted.com")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusCreated {
		t.Errorf("status=%s", res.Status)
	}
}

func TestPushCNAME_UpdateStale(t *testing.T) {
	stubVerify(t, true)
	fp := newFakeProvider()
	fp.records["CNAME"] = []Record{{Type: "CNAME", Name: "mail.example.com", Content: "old.example.com"}}
	s := NewPushService(fp)
	res, err := s.PushCNAMERecord(context.Background(), "example.com", "mail.example.com", "new.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusUpdated {
		t.Errorf("status=%s", res.Status)
	}
	if res.PreviousValue != "old.example.com" {
		t.Errorf("prev=%q", res.PreviousValue)
	}
}

func TestPushCNAME_TrailingDotIdempotent(t *testing.T) {
	stubVerify(t, true)
	fp := newFakeProvider()
	fp.records["CNAME"] = []Record{{Type: "CNAME", Name: "mail.example.com", Content: "cname.example.com."}}
	s := NewPushService(fp)
	res, err := s.PushCNAMERecord(context.Background(), "example.com", "mail.example.com", "cname.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != StatusAlreadyConfigured {
		t.Errorf("status=%s", res.Status)
	}
	if len(fp.setCalls) != 0 {
		t.Errorf("setCalls=%d", len(fp.setCalls))
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
