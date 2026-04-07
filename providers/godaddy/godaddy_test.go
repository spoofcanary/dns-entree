package godaddy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	entree "github.com/spoofcanary/dns-entree"
)

func newTestProvider(t *testing.T, h http.HandlerFunc) (*Provider, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	p, err := NewProvider("KEY", "SECRET")
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	p.baseURL = srv.URL
	return p, srv
}

func TestNewProvider_MissingKey(t *testing.T) {
	if _, err := NewProvider("", "secret"); err == nil {
		t.Fatal("expected error for missing APIKey")
	}
}

func TestNewProvider_MissingSecret(t *testing.T) {
	if _, err := NewProvider("key", ""); err == nil {
		t.Fatal("expected error for missing APISecret")
	}
}

func TestName(t *testing.T) {
	p, _ := NewProvider("k", "s")
	if p.Name() != "GoDaddy" {
		t.Fatalf("Name() = %q", p.Name())
	}
}

func TestSlug(t *testing.T) {
	p, _ := NewProvider("k", "s")
	if p.Slug() != "godaddy" {
		t.Fatalf("Slug() = %q", p.Slug())
	}
}

func TestAuthHeader(t *testing.T) {
	var gotAuth string
	p, srv := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("[]"))
	})
	defer srv.Close()
	_, _ = p.Verify(context.Background())
	if gotAuth != "sso-key KEY:SECRET" {
		t.Fatalf("auth header = %q", gotAuth)
	}
}

func TestVerify(t *testing.T) {
	p, srv := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"domain":"example.com","status":"ACTIVE"}]`))
	})
	defer srv.Close()
	zones, err := p.Verify(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(zones) != 1 || zones[0].Name != "example.com" || zones[0].Status != "active" {
		t.Fatalf("zones = %+v", zones)
	}
}

func TestGetRecords_NameExpansion(t *testing.T) {
	p, srv := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"type":"A","name":"www","data":"1.2.3.4","ttl":600}]`))
	})
	defer srv.Close()
	recs, err := p.GetRecords(context.Background(), "example.com", "A")
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].Name != "www.example.com" {
		t.Fatalf("records = %+v", recs)
	}
}

func TestGetRecords_ApexExpansion(t *testing.T) {
	p, srv := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"type":"A","name":"@","data":"1.2.3.4","ttl":600}]`))
	})
	defer srv.Close()
	recs, err := p.GetRecords(context.Background(), "example.com", "A")
	if err != nil {
		t.Fatal(err)
	}
	if recs[0].Name != "example.com" {
		t.Fatalf("apex name = %q", recs[0].Name)
	}
}

// TestSetRecord_PreservesSiblings is the QUAL-04 fix verification.
// When 2 TXT records exist at name+type, SetRecord must PUT all 3 records (not 1).
func TestSetRecord_PreservesSiblings(t *testing.T) {
	var putBody []godaddyRecord
	p, srv := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			// Return 2 existing TXT records at @+TXT
			_, _ = w.Write([]byte(`[
				{"type":"TXT","name":"@","data":"v=spf1 -all","ttl":600},
				{"type":"TXT","name":"@","data":"google-site-verification=abc","ttl":600}
			]`))
		case "PUT":
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &putBody); err != nil {
				t.Fatalf("unmarshal PUT: %v", err)
			}
			w.WriteHeader(200)
		}
	})
	defer srv.Close()

	err := p.SetRecord(context.Background(), "example.com", entree.Record{
		Type:    "TXT",
		Name:    "example.com",
		Content: "new-record",
		TTL:     600,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(putBody) != 3 {
		t.Fatalf("expected 3 records in PUT body, got %d: %+v", len(putBody), putBody)
	}
	// Verify all 3 are present
	seen := map[string]bool{}
	for _, r := range putBody {
		seen[r.Data] = true
	}
	for _, want := range []string{"v=spf1 -all", "google-site-verification=abc", "new-record"} {
		if !seen[want] {
			t.Fatalf("missing record %q in PUT body: %+v", want, putBody)
		}
	}
}

func TestSetRecord_ReplaceExisting(t *testing.T) {
	var putBody []godaddyRecord
	p, srv := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			_, _ = w.Write([]byte(`[
				{"type":"TXT","name":"@","data":"v=spf1 -all","ttl":600}
			]`))
		case "PUT":
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &putBody)
			w.WriteHeader(200)
		}
	})
	defer srv.Close()

	err := p.SetRecord(context.Background(), "example.com", entree.Record{
		Type:    "TXT",
		Name:    "example.com",
		Content: "v=spf1 -all", // same data, should replace not duplicate
		TTL:     1200,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(putBody) != 1 {
		t.Fatalf("expected 1 record (replace), got %d", len(putBody))
	}
	if putBody[0].TTL != 1200 {
		t.Fatalf("expected TTL updated to 1200, got %d", putBody[0].TTL)
	}
}

func TestSetRecord_NameNormalization(t *testing.T) {
	var gotPath string
	p, srv := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			gotPath = r.URL.Path
			w.WriteHeader(200)
			return
		}
		_, _ = w.Write([]byte(`[]`))
	})
	defer srv.Close()
	_ = p.SetRecord(context.Background(), "example.com", entree.Record{
		Type: "TXT", Name: "_dmarc.example.com", Content: "v=DMARC1", TTL: 600,
	})
	if !strings.HasSuffix(gotPath, "/records/TXT/_dmarc") {
		t.Fatalf("path = %q", gotPath)
	}
}

func TestSetRecord_ApexNormalization(t *testing.T) {
	var gotPath string
	p, srv := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			gotPath = r.URL.Path
			w.WriteHeader(200)
			return
		}
		_, _ = w.Write([]byte(`[]`))
	})
	defer srv.Close()
	_ = p.SetRecord(context.Background(), "example.com", entree.Record{
		Type: "TXT", Name: "example.com", Content: "v=spf1", TTL: 600,
	})
	if !strings.HasSuffix(gotPath, "/records/TXT/@") {
		t.Fatalf("path = %q", gotPath)
	}
}

func TestSetRecord_MinTTL(t *testing.T) {
	var putBody []godaddyRecord
	p, srv := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &putBody)
			w.WriteHeader(200)
			return
		}
		_, _ = w.Write([]byte(`[]`))
	})
	defer srv.Close()
	_ = p.SetRecord(context.Background(), "example.com", entree.Record{
		Type: "TXT", Name: "example.com", Content: "v=spf1", TTL: 60,
	})
	if len(putBody) != 1 || putBody[0].TTL != 600 {
		t.Fatalf("expected TTL=600, got %+v", putBody)
	}
}

func TestDeleteRecord_ValidID(t *testing.T) {
	var gotPath, gotMethod string
	p, srv := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.WriteHeader(204)
	})
	defer srv.Close()
	if err := p.DeleteRecord(context.Background(), "example.com", "_dmarc|TXT"); err != nil {
		t.Fatal(err)
	}
	if gotMethod != "DELETE" {
		t.Fatalf("method = %q", gotMethod)
	}
	if !strings.HasSuffix(gotPath, "/records/TXT/_dmarc") {
		t.Fatalf("path = %q", gotPath)
	}
}

func TestDeleteRecord_InvalidID(t *testing.T) {
	p, _ := NewProvider("k", "s")
	if err := p.DeleteRecord(context.Background(), "example.com", "noseparator"); err == nil {
		t.Fatal("expected error")
	}
}

func TestRegistration(t *testing.T) {
	prov, err := entree.NewProvider("godaddy", entree.Credentials{APIKey: "k", APISecret: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if prov.Slug() != "godaddy" {
		t.Fatalf("slug = %q", prov.Slug())
	}
}

// suppress unused import warning when fmt is unused after edits
var _ = fmt.Sprintf
