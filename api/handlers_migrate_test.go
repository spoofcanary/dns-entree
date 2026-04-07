package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	entree "github.com/spoofcanary/dns-entree"
	"github.com/spoofcanary/dns-entree/migrate"
)

// --- fake provider + adapter wiring -----------------------------------------

type apiFakeProvider struct {
	mu      sync.Mutex
	applied []entree.Record
	sleep   time.Duration
}

func (f *apiFakeProvider) Name() string { return "apiFake" }
func (f *apiFakeProvider) Slug() string { return "api-fake-migrate" }
func (f *apiFakeProvider) Verify(ctx context.Context) ([]entree.Zone, error) {
	return nil, nil
}
func (f *apiFakeProvider) GetRecords(ctx context.Context, domain, rt string) ([]entree.Record, error) {
	return nil, nil
}
func (f *apiFakeProvider) DeleteRecord(ctx context.Context, domain, id string) error { return nil }
func (f *apiFakeProvider) ApplyRecords(ctx context.Context, domain string, recs []entree.Record) error {
	for _, r := range recs {
		_ = f.SetRecord(ctx, domain, r)
	}
	return nil
}
func (f *apiFakeProvider) SetRecord(ctx context.Context, domain string, r entree.Record) error {
	if f.sleep > 0 {
		select {
		case <-time.After(f.sleep):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.applied = append(f.applied, r)
	return nil
}

type apiFakeAdapter struct{}

func (apiFakeAdapter) EnsureZone(ctx context.Context, domain string, opts migrate.ProviderOpts) (migrate.ZoneInfo, error) {
	return migrate.ZoneInfo{ZoneID: "zid", Nameservers: []string{"ns1.fake."}, Created: true}, nil
}

var apiFakeProviderSingleton = &apiFakeProvider{}

func init() {
	entree.RegisterProvider("api-fake-migrate", func(c entree.Credentials) (entree.Provider, error) {
		return apiFakeProviderSingleton, nil
	})
	entree.RegisterProvider("api-fake-slow", func(c entree.Credentials) (entree.Provider, error) {
		return &apiFakeProvider{sleep: 5 * time.Second}, nil
	})
	migrate.RegisterAdapter("api-fake-migrate", apiFakeAdapter{})
	migrate.RegisterAdapter("api-fake-slow", apiFakeAdapter{})
}

// --- helpers ----------------------------------------------------------------

func makeZoneJSON() *migrate.Zone {
	return &migrate.Zone{
		Domain: "example.com",
		Source: "preloaded",
		Records: []entree.Record{
			{Type: "A", Name: "example.com", Content: "192.0.2.10", TTL: 300},
			{Type: "TXT", Name: "_dmarc.example.com", Content: "v=DMARC1; p=none", TTL: 300},
		},
	}
}

// --- tests ------------------------------------------------------------------

func TestHandleMigrate_DryRun(t *testing.T) {
	s := NewServer(Options{})
	body, _ := json.Marshal(migrateRequest{
		Domain:           "example.com",
		Target:           "api-fake-migrate",
		PreloadedZone:    makeZoneJSON(),
		SkipSourceDetect: true,
		Apply:            false,
		DryRun:           true,
		TargetCredentials: bodyCredentials{
			APIToken: "supersecret-token-value",
		},
	})
	req := httptest.NewRequest("POST", "/v1/migrate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "preloaded") {
		t.Fatalf("missing source: %s", rec.Body.String())
	}
}

func TestHandleMigrate_CredentialRedaction(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	s := NewServer(Options{Logger: logger})
	secret := "this-is-a-super-secret-credential-value-xyz"
	body, _ := json.Marshal(migrateRequest{
		Domain:           "example.com",
		Target:           "api-fake-migrate",
		PreloadedZone:    makeZoneJSON(),
		SkipSourceDetect: true,
		Apply:            false,
		DryRun:           true,
		TargetCredentials: bodyCredentials{
			APIToken: secret,
		},
	})
	req := httptest.NewRequest("POST", "/v1/migrate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(buf.String(), secret) {
		t.Fatalf("credential leaked into logs: %s", buf.String())
	}
}

func TestHandleMigrate_OversizeBody(t *testing.T) {
	s := NewServer(Options{})
	big := bytes.Repeat([]byte("a"), int(BodyLimitLarge)+1024)
	body := append([]byte(`{"domain":"example.com","target":"api-fake-migrate","padding":"`), big...)
	body = append(body, []byte(`"}`)...)
	req := httptest.NewRequest("POST", "/v1/migrate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleMigrate_DeadlineHit(t *testing.T) {
	s := NewServer(Options{RequestTimeout: 50 * time.Millisecond})
	body, _ := json.Marshal(migrateRequest{
		Domain:           "example.com",
		Target:           "api-fake-slow",
		PreloadedZone:    makeZoneJSON(),
		SkipSourceDetect: true,
		Apply:            true,
		RatePerSecond:    1000,
		VerifyTimeoutMs:  100,
		QueryTimeoutMs:   50,
	})
	req := httptest.NewRequest("POST", "/v1/migrate", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "request timeout") {
		t.Fatalf("expected request timeout, got: %s", rec.Body.String())
	}
}

func TestHandleZoneExportImport_RoundTrip(t *testing.T) {
	s := NewServer(Options{})

	// Round trip: build a zone, POST to /v1/zone/import (dry run), verify
	// report is populated. /v1/zone/export requires live DNS; we exercise
	// its validation path instead.
	zone := makeZoneJSON()
	body, _ := json.Marshal(zoneImportRequest{
		Zone:   zone,
		To:     "api-fake-migrate",
		DryRun: true,
	})
	req := httptest.NewRequest("POST", "/v1/zone/import", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "example.com") {
		t.Fatalf("missing domain in report: %s", rec.Body.String())
	}
}

func TestHandleZoneImport_MissingZone(t *testing.T) {
	s := NewServer(Options{})
	body, _ := json.Marshal(map[string]any{"to": "api-fake-migrate"})
	req := httptest.NewRequest("POST", "/v1/zone/import", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleZoneExport_MissingDomain(t *testing.T) {
	s := NewServer(Options{})
	req := httptest.NewRequest("POST", "/v1/zone/export", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}
