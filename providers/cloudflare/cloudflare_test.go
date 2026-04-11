package cloudflare

import (
	"context"
	"errors"
	"strings"
	"testing"

	cf "github.com/cloudflare/cloudflare-go"
	entree "github.com/spoofcanary/dns-entree"
)

type mockClient struct {
	zones      []cf.Zone
	zoneByName map[string]string
	zoneErr    error

	listRecords   []cf.DNSRecord
	listRecordsFn func(params cf.ListDNSRecordsParams) ([]cf.DNSRecord, error)

	createCalled bool
	createParams cf.CreateDNSRecordParams

	updateCalled bool
	updateParams cf.UpdateDNSRecordParams

	deleteCalled bool
	deleteID     string
}

func (m *mockClient) ListZones(ctx context.Context, z ...string) ([]cf.Zone, error) {
	return m.zones, nil
}
func (m *mockClient) ZoneIDByName(name string) (string, error) {
	if m.zoneByName != nil {
		if id, ok := m.zoneByName[name]; ok {
			return id, nil
		}
	}
	if m.zoneErr != nil {
		return "", m.zoneErr
	}
	return "", errors.New("not found")
}
func (m *mockClient) ListDNSRecords(ctx context.Context, rc *cf.ResourceContainer, params cf.ListDNSRecordsParams) ([]cf.DNSRecord, *cf.ResultInfo, error) {
	if m.listRecordsFn != nil {
		recs, err := m.listRecordsFn(params)
		return recs, nil, err
	}
	return m.listRecords, nil, nil
}
func (m *mockClient) CreateDNSRecord(ctx context.Context, rc *cf.ResourceContainer, params cf.CreateDNSRecordParams) (cf.DNSRecord, error) {
	m.createCalled = true
	m.createParams = params
	return cf.DNSRecord{ID: "new-id"}, nil
}
func (m *mockClient) UpdateDNSRecord(ctx context.Context, rc *cf.ResourceContainer, params cf.UpdateDNSRecordParams) (cf.DNSRecord, error) {
	m.updateCalled = true
	m.updateParams = params
	return cf.DNSRecord{ID: params.ID}, nil
}
func (m *mockClient) DeleteDNSRecord(ctx context.Context, rc *cf.ResourceContainer, id string) error {
	m.deleteCalled = true
	m.deleteID = id
	return nil
}

func TestNewProvider_MissingToken(t *testing.T) {
	_, err := NewProvider("")
	if err == nil || !strings.Contains(err.Error(), "APIToken required") {
		t.Fatalf("expected APIToken required error, got %v", err)
	}
}

func TestName(t *testing.T) {
	p := newProviderWithClient(&mockClient{})
	if p.Name() != "Cloudflare" {
		t.Errorf("Name = %q, want Cloudflare", p.Name())
	}
}

func TestSlug(t *testing.T) {
	p := newProviderWithClient(&mockClient{})
	if p.Slug() != "cloudflare" {
		t.Errorf("Slug = %q, want cloudflare", p.Slug())
	}
}

func TestSetRecord_CreateNew(t *testing.T) {
	m := &mockClient{
		zoneByName:  map[string]string{"example.com": "zone1"},
		listRecords: nil,
	}
	p := newProviderWithClient(m)
	err := p.SetRecord(context.Background(), "example.com", entree.Record{Type: "TXT", Name: "_dmarc.example.com", Content: "v=DMARC1; p=none", TTL: 300})
	if err != nil {
		t.Fatal(err)
	}
	if !m.createCalled {
		t.Fatal("expected CreateDNSRecord to be called")
	}
	if m.createParams.TTL != 300 {
		t.Errorf("TTL = %d, want 300", m.createParams.TTL)
	}
}

func TestSetRecord_UpdateExisting(t *testing.T) {
	m := &mockClient{
		zoneByName:  map[string]string{"example.com": "zone1"},
		listRecords: []cf.DNSRecord{{ID: "rec-existing", Type: "TXT", Name: "_dmarc.example.com"}},
	}
	p := newProviderWithClient(m)
	err := p.SetRecord(context.Background(), "example.com", entree.Record{Type: "TXT", Name: "_dmarc.example.com", Content: "v=DMARC1; p=reject", TTL: 60})
	if err != nil {
		t.Fatal(err)
	}
	if !m.updateCalled {
		t.Fatal("expected UpdateDNSRecord to be called")
	}
	if m.updateParams.ID != "rec-existing" {
		t.Errorf("update ID = %q, want rec-existing", m.updateParams.ID)
	}
}

func TestSetRecord_TTLDefault(t *testing.T) {
	m := &mockClient{zoneByName: map[string]string{"example.com": "zone1"}}
	p := newProviderWithClient(m)
	if err := p.SetRecord(context.Background(), "example.com", entree.Record{Type: "TXT", Name: "x", Content: "y", TTL: 0}); err != nil {
		t.Fatal(err)
	}
	if m.createParams.TTL != 1 {
		t.Errorf("TTL = %d, want 1 (auto)", m.createParams.TTL)
	}
}

func TestGetRecords(t *testing.T) {
	m := &mockClient{
		zoneByName:  map[string]string{"example.com": "zone1"},
		listRecords: []cf.DNSRecord{{ID: "r1", Type: "TXT", Name: "example.com", Content: "v=spf1 -all", TTL: 300}},
	}
	p := newProviderWithClient(m)
	recs, err := p.GetRecords(context.Background(), "example.com", "TXT")
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].Content != "v=spf1 -all" {
		t.Errorf("got %+v", recs)
	}
}

func TestDeleteRecord(t *testing.T) {
	m := &mockClient{zoneByName: map[string]string{"example.com": "zone1"}}
	p := newProviderWithClient(m)
	if err := p.DeleteRecord(context.Background(), "example.com", "rec-1"); err != nil {
		t.Fatal(err)
	}
	if !m.deleteCalled || m.deleteID != "rec-1" {
		t.Errorf("delete called=%v id=%q", m.deleteCalled, m.deleteID)
	}
}

func TestVerify(t *testing.T) {
	m := &mockClient{zones: []cf.Zone{{ID: "z1", Name: "example.com", Status: "active"}}}
	p := newProviderWithClient(m)
	zones, err := p.Verify(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(zones) != 1 || zones[0].Name != "example.com" {
		t.Errorf("got %+v", zones)
	}
}

func TestApplyRecords(t *testing.T) {
	m := &mockClient{zoneByName: map[string]string{"example.com": "zone1"}}
	p := newProviderWithClient(m)
	err := p.ApplyRecords(context.Background(), "example.com", []entree.Record{
		{Type: "TXT", Name: "a", Content: "1"},
		{Type: "TXT", Name: "b", Content: "2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !m.createCalled {
		t.Fatal("expected create to be called")
	}
}

func TestResolveZoneID_SubdomainFallback(t *testing.T) {
	m := &mockClient{zoneByName: map[string]string{"example.com": "zone1"}}
	p := newProviderWithClient(m)
	id, err := p.resolveZoneID("foo.bar.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if id != "zone1" {
		t.Errorf("got %q want zone1", id)
	}
}

func TestInitRegistration(t *testing.T) {
	// Importing this package triggers init(); a non-empty token should produce
	// an error from the real SDK only if invalid -- here we expect either ok
	// or an SDK error, but never "unknown provider".
	_, err := entree.NewProvider("cloudflare", entree.Credentials{APIToken: ""})
	if err == nil || !strings.Contains(err.Error(), "APIToken required") {
		t.Fatalf("expected APIToken required, got %v", err)
	}
}
