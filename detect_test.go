package entree

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDetectFromNS(t *testing.T) {
	cases := []struct {
		name      string
		hosts     []string
		provider  ProviderType
		label     string
		supported bool
	}{
		{"cloudflare", []string{"ns1.cloudflare.com"}, ProviderCloudflare, "Cloudflare", true},
		{"awsdns", []string{"ns-123.awsdns-45.com"}, ProviderRoute53, "Amazon Route 53", true},
		{"domaincontrol", []string{"ns01.domaincontrol.com"}, ProviderGoDaddy, "GoDaddy", true},
		{"godaddy", []string{"ns1.godaddy.com"}, ProviderGoDaddy, "GoDaddy", true},
		{"googledomains", []string{"ns-cloud-a1.googledomains.com"}, ProviderGoogleCloudDNS, "Google Cloud DNS", true},
		{"squarespace", []string{"ns1.squarespace.com"}, ProviderType("squarespace"), "Squarespace", false},
		{"namecheap", []string{"dns1.namecheaphosting.com"}, ProviderType("namecheap"), "Namecheap", false},
		{"hover", []string{"ns1.hover.com"}, ProviderType("hover"), "Hover", false},
		{"digitalocean", []string{"ns1.digitalocean.com"}, ProviderType("digitalocean"), "DigitalOcean", false},
		{"hetzner", []string{"hydrogen.ns.hetzner.com"}, ProviderType("hetzner"), "Hetzner", false},
		{"registrar-servers", []string{"dns1.registrar-servers.com"}, ProviderType("namecheap"), "Namecheap", false},
		{"nsone", []string{"dns1.p01.nsone.net"}, ProviderType("ns1"), "NS1", false},
		{"dnsv", []string{"dns1.dnsv.jp"}, ProviderType("dnsv"), "DNSV", false},
		{"linode", []string{"ns1.linode.com"}, ProviderType("linode"), "Linode", false},
		{"vultr", []string{"ns1.vultr.com"}, ProviderType("vultr"), "Vultr", false},
		{"dnsimple", []string{"ns1.dnsimple.com"}, ProviderType("dnsimple"), "DNSimple", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := DetectFromNS(tc.hosts)
			if r.Provider != tc.provider {
				t.Errorf("provider: got %q want %q", r.Provider, tc.provider)
			}
			if r.Label != tc.label {
				t.Errorf("label: got %q want %q", r.Label, tc.label)
			}
			if r.Supported != tc.supported {
				t.Errorf("supported: got %v want %v", r.Supported, tc.supported)
			}
			if r.Method != "ns_pattern" {
				t.Errorf("method: got %q want ns_pattern", r.Method)
			}
		})
	}
}

func TestDetectFromNS_Unknown(t *testing.T) {
	r := DetectFromNS([]string{"unknown.example"})
	if r.Provider != "" {
		t.Errorf("got provider %q, want empty", r.Provider)
	}
	if r.Supported {
		t.Errorf("unknown provider should not be Supported")
	}
}

func TestDetectFromNS_Supported(t *testing.T) {
	tier1 := []ProviderType{ProviderCloudflare, ProviderRoute53, ProviderGoDaddy, ProviderGoogleCloudDNS}
	hosts := map[ProviderType][]string{
		ProviderCloudflare:     {"ns1.cloudflare.com"},
		ProviderRoute53:        {"ns-1.awsdns-2.com"},
		ProviderGoDaddy:        {"ns01.domaincontrol.com"},
		ProviderGoogleCloudDNS: {"ns-cloud-a1.googledomains.com"},
	}
	for _, p := range tier1 {
		r := DetectFromNS(hosts[p])
		if !r.Supported {
			t.Errorf("%s should be Supported=true", p)
		}
	}
	for _, host := range []string{"ns1.squarespace.com", "ns1.hover.com", "ns1.linode.com"} {
		r := DetectFromNS([]string{host})
		if r.Supported {
			t.Errorf("%s should be Supported=false", host)
		}
	}
}

func TestRegistrarToProvider(t *testing.T) {
	cases := map[string]ProviderType{
		"Squarespace Domains LLC": ProviderType("squarespace"),
		"GoDaddy.com, LLC":        ProviderGoDaddy,
		"Cloudflare, Inc.":        ProviderCloudflare,
		"Amazon Registrar, Inc.":  ProviderRoute53,
		"Google LLC":              ProviderGoogleCloudDNS,
		"Unknown Registrar":       ProviderType(""),
	}
	for in, want := range cases {
		if got := registrarToProvider(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestLookupRegistrar(t *testing.T) {
	body := `{"entities":[{"roles":["registrar"],"vcardArray":["vcard",[["fn",{},"text","Squarespace Domains LLC"]]]}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rdap+json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	orig := rdapEndpoints
	rdapEndpoints = func(domain string) []string { return []string{srv.URL + "/domain/" + domain} }
	defer func() { rdapEndpoints = orig }()

	got := lookupRegistrar(context.Background(), "example.com")
	if got != "Squarespace Domains LLC" {
		t.Errorf("got %q, want Squarespace Domains LLC", got)
	}
}

func TestLookupRegistrar_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 6s timeout test in -short")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(6 * time.Second)
	}))
	defer srv.Close()

	orig := rdapEndpoints
	rdapEndpoints = func(domain string) []string { return []string{srv.URL + "/domain/" + domain} }
	defer func() { rdapEndpoints = orig }()

	got := lookupRegistrar(context.Background(), "example.com")
	if got != "" {
		t.Errorf("expected empty on timeout, got %q", got)
	}
}
