package migrate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGCDNSAdapter_ExistingZone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
		if r.Method != http.MethodGet || !strings.Contains(r.URL.Path, "/projects/proj-1/managedZones") {
			t.Errorf("unexpected req: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"managedZones":[{"name":"zone-abc","dnsName":"example.com.","nameServers":["ns-cloud-a1.googledomains.com.","ns-cloud-a2.googledomains.com."]}]}`))
	}))
	defer srv.Close()

	a, _ := GetAdapter("gcdns")
	info, err := a.EnsureZone(context.Background(), "example.com", ProviderOpts{
		Token:     "test-token",
		ProjectID: "proj-1",
		Endpoint:  srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if info.Created {
		t.Error("expected Created=false")
	}
	if info.ZoneID != "zone-abc" {
		t.Errorf("zone id = %q", info.ZoneID)
	}
	if len(info.Nameservers) != 2 {
		t.Errorf("ns = %v", info.Nameservers)
	}
}

func TestGCDNSAdapter_CreateZone(t *testing.T) {
	var sawCreate bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"managedZones":[]}`))
		case http.MethodPost:
			sawCreate = true
			_, _ = w.Write([]byte(`{"name":"zone-new","dnsName":"example.com.","nameServers":["ns-cloud-x1.googledomains.com.","ns-cloud-x2.googledomains.com."]}`))
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	}))
	defer srv.Close()

	a, _ := GetAdapter("gcdns")
	info, err := a.EnsureZone(context.Background(), "example.com", ProviderOpts{
		Token:     "test-token",
		ProjectID: "proj-1",
		Endpoint:  srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !sawCreate {
		t.Error("expected POST")
	}
	if !info.Created {
		t.Error("expected Created=true")
	}
	if info.ZoneID != "zone-new" {
		t.Errorf("zone id = %q", info.ZoneID)
	}
	if len(info.Nameservers) != 2 || !strings.HasPrefix(info.Nameservers[0], "ns-cloud-x1") {
		t.Errorf("ns = %v", info.Nameservers)
	}
}

func TestGCDNSAdapter_RejectsBadInputs(t *testing.T) {
	a, _ := GetAdapter("gcdns")
	if _, err := a.EnsureZone(context.Background(), "example.com", ProviderOpts{ProjectID: "p"}); err == nil {
		t.Error("expected token required")
	}
	if _, err := a.EnsureZone(context.Background(), "example.com", ProviderOpts{Token: "t"}); err == nil {
		t.Error("expected projectid required")
	}
	if _, err := a.EnsureZone(context.Background(), "bad..domain", ProviderOpts{Token: "t", ProjectID: "p"}); err == nil {
		t.Error("expected bad domain")
	}
}
