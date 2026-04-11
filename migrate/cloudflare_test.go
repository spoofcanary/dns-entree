package migrate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const cfListEmpty = `{"success":true,"errors":[],"messages":[],"result":[],"result_info":{"page":1,"per_page":20,"total_pages":1,"count":0,"total_count":0}}`

const cfListExisting = `{"success":true,"errors":[],"messages":[],"result":[{"id":"zone-123","name":"example.com","name_servers":["ns1.cf.com","ns2.cf.com"],"status":"active"}],"result_info":{"page":1,"per_page":20,"total_pages":1,"count":1,"total_count":1}}`

const cfCreateOK = `{"success":true,"errors":[],"messages":[],"result":{"id":"zone-new","name":"example.com","name_servers":["ns3.cf.com","ns4.cf.com"],"status":"pending"}}`

func TestCloudflareAdapter_ExistingZone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, "/zones") {
			t.Errorf("unexpected req: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("missing auth header: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(cfListExisting))
	}))
	defer srv.Close()

	a, err := GetAdapter("cloudflare")
	if err != nil {
		t.Fatal(err)
	}
	info, err := a.EnsureZone(context.Background(), "example.com", ProviderOpts{
		APIToken:  "test-token",
		AccountID: "acct-1",
		Endpoint:  srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if info.Created {
		t.Error("expected Created=false")
	}
	if info.ZoneID != "zone-123" {
		t.Errorf("zone id = %q", info.ZoneID)
	}
	if len(info.Nameservers) != 2 || info.Nameservers[0] != "ns1.cf.com" {
		t.Errorf("ns = %v", info.Nameservers)
	}
}

func TestCloudflareAdapter_CreateZone(t *testing.T) {
	var sawCreate bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/zones"):
			_, _ = w.Write([]byte(cfListEmpty))
		case r.Method == http.MethodPost && r.URL.Path == "/zones":
			sawCreate = true
			_, _ = w.Write([]byte(cfCreateOK))
		default:
			t.Errorf("unexpected req: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	a, _ := GetAdapter("cloudflare")
	info, err := a.EnsureZone(context.Background(), "example.com", ProviderOpts{
		APIToken:  "test-token",
		AccountID: "acct-1",
		Endpoint:  srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !sawCreate {
		t.Error("expected POST /zones")
	}
	if !info.Created {
		t.Error("expected Created=true")
	}
	if info.ZoneID != "zone-new" {
		t.Errorf("zone id = %q", info.ZoneID)
	}
	if len(info.Nameservers) != 2 || info.Nameservers[1] != "ns4.cf.com" {
		t.Errorf("ns = %v", info.Nameservers)
	}
}

func TestCloudflareAdapter_RejectsBadInputs(t *testing.T) {
	a, _ := GetAdapter("cloudflare")
	cases := []struct {
		name   string
		domain string
		opts   ProviderOpts
	}{
		{"empty domain", "", ProviderOpts{APIToken: "t", AccountID: "a"}},
		{"bad domain", "ex ample.com", ProviderOpts{APIToken: "t", AccountID: "a"}},
		{"no tld", "localhost", ProviderOpts{APIToken: "t", AccountID: "a"}},
		{"no token", "example.com", ProviderOpts{AccountID: "a"}},
		{"no account", "example.com", ProviderOpts{APIToken: "t"}},
		{"bad endpoint", "example.com", ProviderOpts{APIToken: "t", AccountID: "a", Endpoint: "ftp://x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := a.EnsureZone(context.Background(), tc.domain, tc.opts); err == nil {
				t.Error("expected error")
			}
		})
	}
}
