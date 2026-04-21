package namecheap

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	entree "github.com/spoofcanary/dns-entree"
)

func TestNewProvider_RequiresFields(t *testing.T) {
	if _, err := NewProvider("", "k", "u", "1.1.1.1"); err == nil {
		t.Error("expected error when api_user missing")
	}
	if _, err := NewProvider("u", "", "u", "1.1.1.1"); err == nil {
		t.Error("expected error when api_key missing")
	}
	if _, err := NewProvider("u", "k", "u", ""); err == nil {
		t.Error("expected error when client_ip missing")
	}
}

func TestSplitDomain(t *testing.T) {
	cases := []struct {
		in       string
		sld, tld string
		wantErr  bool
	}{
		{"example.com", "example", "com", false},
		{"lynnenergy.com", "lynnenergy", "com", false},
		{"sub.example.com", "sub", "example.com", false}, // bare-label strategy
		{"example.co.uk", "example", "co.uk", false},
		{"single", "", "", true},
		{"", "", "", true},
	}
	for _, tc := range cases {
		sld, tld, err := splitDomain(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("%s: expected error", tc.in)
			}
			continue
		}
		if sld != tc.sld || tld != tc.tld {
			t.Errorf("%s: got (%q,%q), want (%q,%q)", tc.in, sld, tld, tc.sld, tc.tld)
		}
	}
}

func TestHostNameFor(t *testing.T) {
	cases := []struct {
		record, domain, want string
	}{
		{"example.com", "example.com", "@"},
		{"_dmarc.example.com", "example.com", "_dmarc"},
		{"mail.example.com", "example.com", "mail"},
		{"example.com.", "example.com", "@"}, // trailing dot tolerated
	}
	for _, tc := range cases {
		got := hostNameFor(tc.record, tc.domain)
		if got != tc.want {
			t.Errorf("hostNameFor(%q,%q)=%q want %q", tc.record, tc.domain, got, tc.want)
		}
	}
}

func TestMergeRecord_Append(t *testing.T) {
	hosts := []host{{Name: "@", Type: "A", Address: "1.2.3.4", TTL: "300"}}
	rec := entree.Record{Type: "TXT", Name: "_dmarc.example.com", Content: "v=DMARC1; p=reject;", TTL: 1800}
	out := mergeRecord(hosts, rec, "example.com")
	if len(out) != 2 {
		t.Fatalf("len=%d want 2", len(out))
	}
	if out[1].Name != "_dmarc" || out[1].Type != "TXT" {
		t.Errorf("appended record wrong: %+v", out[1])
	}
}

func TestMergeRecord_Replace(t *testing.T) {
	hosts := []host{
		{Name: "@", Type: "A", Address: "1.2.3.4", TTL: "300"},
		{Name: "_dmarc", Type: "TXT", Address: "v=DMARC1; p=none;", TTL: "1800"},
	}
	rec := entree.Record{Type: "TXT", Name: "_dmarc.example.com", Content: "v=DMARC1; p=reject;", TTL: 1800}
	out := mergeRecord(hosts, rec, "example.com")
	if len(out) != 2 {
		t.Fatalf("len=%d want 2", len(out))
	}
	if !strings.Contains(out[1].Address, "p=reject") {
		t.Errorf("replaced content=%q", out[1].Address)
	}
}

func TestVerify_OKResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		fmt.Fprint(w, `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK">
  <Errors/>
  <CommandResponse>
    <DomainGetListResult>
      <Domain Name="lynnenergy.com" IsExpired="false" AutoRenew="true" Created="01/01/2020" Expires="01/01/2030"/>
      <Domain Name="example.com" IsExpired="false" AutoRenew="true" Created="01/01/2020" Expires="01/01/2030"/>
    </DomainGetListResult>
  </CommandResponse>
</ApiResponse>`)
	}))
	defer srv.Close()

	p, err := NewProvider("u", "k", "u", "1.1.1.1")
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	p.baseURL = srv.URL

	zones, err := p.Verify(context.Background())
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(zones) != 2 || zones[0].Name != "lynnenergy.com" {
		t.Errorf("zones=%+v", zones)
	}
}

func TestVerify_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		fmt.Fprint(w, `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="ERROR">
  <Errors>
    <Error Number="1011102">Parameter ApiKey is missing</Error>
  </Errors>
  <CommandResponse/>
</ApiResponse>`)
	}))
	defer srv.Close()

	p, _ := NewProvider("u", "k", "u", "1.1.1.1")
	p.baseURL = srv.URL

	if _, err := p.Verify(context.Background()); err == nil {
		t.Fatal("expected error")
	} else if !strings.Contains(err.Error(), "1011102") {
		t.Errorf("expected error number in: %v", err)
	}
}

func TestGetRecords(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		fmt.Fprint(w, `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK">
  <Errors/>
  <CommandResponse>
    <DomainDNSGetHostsResult>
      <host HostId="1" Name="@" Type="A" Address="1.2.3.4" MXPref="10" TTL="300" IsActive="true"/>
      <host HostId="2" Name="_dmarc" Type="TXT" Address="v=DMARC1; p=none;" MXPref="10" TTL="1800" IsActive="true"/>
    </DomainDNSGetHostsResult>
  </CommandResponse>
</ApiResponse>`)
	}))
	defer srv.Close()

	p, _ := NewProvider("u", "k", "u", "1.1.1.1")
	p.baseURL = srv.URL

	recs, err := p.GetRecords(context.Background(), "example.com", "TXT")
	if err != nil {
		t.Fatalf("getRecords: %v", err)
	}
	if len(recs) != 1 || recs[0].Name != "_dmarc.example.com" {
		t.Errorf("recs=%+v", recs)
	}
}
