package migrate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGoDaddyAdapter_ExistingDomain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "sso-key ") {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/domains/example.com" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"domain":"example.com","status":"ACTIVE","nameServers":["ns1.godaddy.com","ns2.godaddy.com"]}`))
	}))
	defer srv.Close()

	a, _ := GetAdapter("godaddy")
	info, err := a.EnsureZone(context.Background(), "example.com", ProviderOpts{
		APIKey:    "k",
		APISecret: "s",
		Endpoint:  srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if info.Created {
		t.Error("godaddy never creates")
	}
	if info.ZoneID != "example.com" {
		t.Errorf("zone id = %q", info.ZoneID)
	}
	if len(info.Nameservers) != 2 {
		t.Errorf("ns = %v", info.Nameservers)
	}
}

func TestGoDaddyAdapter_NotRegistered(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	a, _ := GetAdapter("godaddy")
	_, err := a.EnsureZone(context.Background(), "example.com", ProviderOpts{
		APIKey:    "k",
		APISecret: "s",
		Endpoint:  srv.URL,
	})
	if err == nil {
		t.Fatal("expected error for unregistered domain")
	}
	if !strings.Contains(err.Error(), "not registered") {
		t.Errorf("error = %v", err)
	}
}

func TestGoDaddyAdapter_RejectsBadInputs(t *testing.T) {
	a, _ := GetAdapter("godaddy")
	if _, err := a.EnsureZone(context.Background(), "example.com", ProviderOpts{}); err == nil {
		t.Error("expected creds required")
	}
	if _, err := a.EnsureZone(context.Background(), "", ProviderOpts{APIKey: "k", APISecret: "s"}); err == nil {
		t.Error("expected empty domain rejected")
	}
}

func TestGetAdapter_UnknownSlug(t *testing.T) {
	if _, err := GetAdapter("nope"); err == nil {
		t.Error("expected error")
	}
}
