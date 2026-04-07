package gcdns

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	entree "github.com/spoofcanary/dns-entree"
)

func newTestProvider(t *testing.T, h http.HandlerFunc) (*Provider, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	p, err := NewProvider("TOKEN", "proj-1")
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	p.baseURL = srv.URL
	return p, srv
}

func TestNewProvider_MissingToken(t *testing.T) {
	if _, err := NewProvider("", "proj"); err == nil {
		t.Fatal("expected error")
	}
}

func TestNewProvider_MissingProjectID(t *testing.T) {
	if _, err := NewProvider("tok", ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestName(t *testing.T) {
	p, _ := NewProvider("t", "p")
	if p.Name() != "Google Cloud DNS" {
		t.Fatalf("Name() = %q", p.Name())
	}
}

func TestSlug(t *testing.T) {
	p, _ := NewProvider("t", "p")
	if p.Slug() != "google_cloud_dns" {
		t.Fatalf("Slug() = %q", p.Slug())
	}
}

// zonesHandler returns a handler that serves a single managedZones response
// for any /managedZones request and dispatches other URLs to next.
func zonesHandler(zonesJSON string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/managedZones") && r.Method == "GET" {
			_, _ = w.Write([]byte(zonesJSON))
			return
		}
		if next != nil {
			next(w, r)
			return
		}
		w.WriteHeader(404)
	}
}

func TestVerify(t *testing.T) {
	p, srv := newTestProvider(t, zonesHandler(
		`{"managedZones":[{"name":"zone-1","dnsName":"example.com.","id":"1"}]}`, nil))
	defer srv.Close()
	zones, err := p.Verify(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(zones) != 1 || zones[0].Name != "example.com" || zones[0].ID != "zone-1" {
		t.Fatalf("zones = %+v", zones)
	}
}

func TestGetRecords_TXTUnquoting(t *testing.T) {
	p, srv := newTestProvider(t, zonesHandler(
		`{"managedZones":[{"name":"zone-1","dnsName":"example.com.","id":"1"}]}`,
		func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/rrsets") {
				_, _ = w.Write([]byte(`{"rrsets":[{"name":"example.com.","type":"TXT","ttl":300,"rrdatas":["\"v=spf1 -all\""]}]}`))
				return
			}
			w.WriteHeader(404)
		}))
	defer srv.Close()
	recs, err := p.GetRecords(context.Background(), "example.com", "TXT")
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].Content != "v=spf1 -all" {
		t.Fatalf("recs = %+v", recs)
	}
	if recs[0].Name != "example.com" {
		t.Fatalf("name not stripped: %q", recs[0].Name)
	}
}

func TestGetRecords_TrailingDotStripped(t *testing.T) {
	p, srv := newTestProvider(t, zonesHandler(
		`{"managedZones":[{"name":"zone-1","dnsName":"example.com.","id":"1"}]}`,
		func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"rrsets":[{"name":"www.example.com.","type":"A","ttl":300,"rrdatas":["1.2.3.4"]}]}`))
		}))
	defer srv.Close()
	recs, _ := p.GetRecords(context.Background(), "example.com", "A")
	if recs[0].Name != "www.example.com" {
		t.Fatalf("name = %q", recs[0].Name)
	}
}

func TestSetRecord_DeleteBeforeCreate(t *testing.T) {
	var mu sync.Mutex
	var calls []string
	p, srv := newTestProvider(t, zonesHandler(
		`{"managedZones":[{"name":"zone-1","dnsName":"example.com.","id":"1"}]}`,
		func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			calls = append(calls, r.Method)
			mu.Unlock()
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{}`))
		}))
	defer srv.Close()

	err := p.SetRecord(context.Background(), "example.com", entree.Record{
		Type: "TXT", Name: "example.com", Content: "v=spf1", TTL: 300,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) < 2 || calls[0] != "DELETE" || calls[1] != "POST" {
		t.Fatalf("expected DELETE then POST, got %v", calls)
	}
}

func TestSetRecord_TXTQuoting(t *testing.T) {
	var postBody string
	p, srv := newTestProvider(t, zonesHandler(
		`{"managedZones":[{"name":"zone-1","dnsName":"example.com.","id":"1"}]}`,
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "POST" {
				b, _ := io.ReadAll(r.Body)
				postBody = string(b)
			}
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{}`))
		}))
	defer srv.Close()

	_ = p.SetRecord(context.Background(), "example.com", entree.Record{
		Type: "TXT", Name: "example.com", Content: "v=spf1", TTL: 300,
	})
	if !strings.Contains(postBody, `\"v=spf1\"`) {
		t.Fatalf("expected quoted TXT, got %s", postBody)
	}
}

func TestSetRecord_TrailingDot(t *testing.T) {
	var postBody string
	p, srv := newTestProvider(t, zonesHandler(
		`{"managedZones":[{"name":"zone-1","dnsName":"example.com.","id":"1"}]}`,
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "POST" {
				b, _ := io.ReadAll(r.Body)
				postBody = string(b)
			}
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{}`))
		}))
	defer srv.Close()

	_ = p.SetRecord(context.Background(), "example.com", entree.Record{
		Type: "A", Name: "example.com", Content: "1.2.3.4", TTL: 300,
	})
	if !strings.Contains(postBody, `"name":"example.com."`) {
		t.Fatalf("expected trailing dot in name, got %s", postBody)
	}
}

func TestSetRecord_DefaultTTL(t *testing.T) {
	var postBody string
	p, srv := newTestProvider(t, zonesHandler(
		`{"managedZones":[{"name":"zone-1","dnsName":"example.com.","id":"1"}]}`,
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "POST" {
				b, _ := io.ReadAll(r.Body)
				postBody = string(b)
			}
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{}`))
		}))
	defer srv.Close()

	_ = p.SetRecord(context.Background(), "example.com", entree.Record{
		Type: "A", Name: "example.com", Content: "1.2.3.4", TTL: 0,
	})
	if !strings.Contains(postBody, `"ttl":300`) {
		t.Fatalf("expected default TTL 300, got %s", postBody)
	}
}

func TestFindZoneID_LongestSuffix(t *testing.T) {
	p, srv := newTestProvider(t, zonesHandler(
		`{"managedZones":[
			{"name":"zone-parent","dnsName":"example.com.","id":"1"},
			{"name":"zone-child","dnsName":"sub.example.com.","id":"2"}
		]}`, nil))
	defer srv.Close()
	id, err := p.findZoneID(context.Background(), "sub.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if id != "zone-child" {
		t.Fatalf("expected zone-child, got %s", id)
	}
}

func TestDeleteRecord_InvalidID(t *testing.T) {
	p, srv := newTestProvider(t, zonesHandler(
		`{"managedZones":[{"name":"zone-1","dnsName":"example.com.","id":"1"}]}`, nil))
	defer srv.Close()
	err := p.DeleteRecord(context.Background(), "example.com", "noseparator")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteRecord_IDParsing(t *testing.T) {
	var gotPath string
	p, srv := newTestProvider(t, zonesHandler(
		`{"managedZones":[{"name":"zone-1","dnsName":"example.com.","id":"1"}]}`,
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "DELETE" {
				gotPath = r.URL.Path
			}
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{}`))
		}))
	defer srv.Close()
	if err := p.DeleteRecord(context.Background(), "example.com", "www.example.com|A"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(gotPath, "/rrsets/www.example.com./A") {
		t.Fatalf("path = %q", gotPath)
	}
}

func TestDiscoverGCPProject(t *testing.T) {
	// Single test server handles both CRM and DNS endpoints by path prefix.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/projects") && !strings.Contains(r.URL.Path, "managedZones"):
			_, _ = w.Write([]byte(`{"projects":[{"projectId":"proj-a"},{"projectId":"proj-b"}]}`))
		case strings.Contains(r.URL.Path, "/projects/proj-a/managedZones"):
			_, _ = w.Write([]byte(`{"managedZones":[]}`))
		case strings.Contains(r.URL.Path, "/projects/proj-b/managedZones"):
			_, _ = w.Write([]byte(`{"managedZones":[{"name":"zone-1"}]}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	id, err := discoverGCPProject(context.Background(), "tok", "example.com", srv.URL, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if id != "proj-b" {
		t.Fatalf("expected proj-b, got %s", id)
	}
}

func TestRegistration(t *testing.T) {
	prov, err := entree.NewProvider("google_cloud_dns", entree.Credentials{Token: "t", ProjectID: "p"})
	if err != nil {
		t.Fatal(err)
	}
	if prov.Slug() != "google_cloud_dns" {
		t.Fatalf("slug = %q", prov.Slug())
	}
}
