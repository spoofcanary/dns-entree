package dctest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"

	"github.com/spoofcanary/dns-entree/domainconnect"
	"github.com/spoofcanary/dns-entree/domainconnect/dctest"
)

func TestRoundTrip_DiscoverSignApplyVerify(t *testing.T) {
	srv := dctest.NewServer()
	defer srv.Close()

	// 1. Discover: mock TXT + settings endpoint
	txtHost := srv.TXTRecord()
	result, err := domainconnect.Discover(context.Background(), "example.com",
		domainconnect.WithHTTPClient(srv.HTTPClient()),
		domainconnect.WithTXTResolver(func(_ context.Context, _ string) ([]string, error) {
			return []string{txtHost}, nil
		}),
		domainconnect.WithResolver(func(_ context.Context, host string) ([]net.IP, error) {
			// Resolve the mock server's host to a non-private IP so SSRF check passes
			return []net.IP{net.ParseIP("93.184.216.34")}, nil
		}),
	)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if !result.Supported {
		t.Fatal("expected Supported=true")
	}
	if result.ProviderName != "DC Test Provider" {
		t.Errorf("ProviderName = %q, want %q", result.ProviderName, "DC Test Provider")
	}
	if result.URLAsyncUX != srv.URL {
		t.Errorf("URLAsyncUX = %q, want %q", result.URLAsyncUX, srv.URL)
	}

	// 2. Build signed apply URL
	applyURL, err := domainconnect.BuildApplyURL(domainconnect.ApplyURLOpts{
		URLAsyncUX:  result.URLAsyncUX,
		ProviderID:  result.ProviderID,
		ServiceID:   "sendcanary.com.email-setup",
		Domain:      "example.com",
		Params:      map[string]string{"spfRecord": "v=spf1 -all"},
		PrivateKey:  srv.PrivateKey(),
		KeyHost:     "keys.example.com",
		RedirectURI: "https://app.example.com/callback",
	})
	if err != nil {
		t.Fatalf("BuildApplyURL: %v", err)
	}

	// 3. Hit the apply URL on the mock server (use TLS client)
	resp, err := srv.HTTPClient().Get(applyURL)
	if err != nil {
		t.Fatalf("GET apply URL: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("apply returned %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode apply response: %v", err)
	}
	if body["status"] != "applied" {
		t.Errorf("status = %q, want %q", body["status"], "applied")
	}

	// 4. Verify the mock captured the request with valid signature
	reqs := srv.ApplyRequests()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 apply request, got %d", len(reqs))
	}
	req := reqs[0]
	if req.Domain != "example.com" {
		t.Errorf("Domain = %q, want %q", req.Domain, "example.com")
	}
	if req.ProviderID != "dctest" {
		t.Errorf("ProviderID = %q, want %q", req.ProviderID, "dctest")
	}
	if !req.SigValid {
		t.Error("signature verification failed on mock server")
	}
}

func TestRoundTrip_InvalidSignature(t *testing.T) {
	srv := dctest.NewServer()
	defer srv.Close()

	// Build a URL with a bogus signature
	u := srv.URL + "/v2/domainTemplates/providers/dctest/services/test/apply?domain=example.com&sig=INVALIDSIG&key=test"

	resp, err := srv.HTTPClient().Get(u)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()

	reqs := srv.ApplyRequests()
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	if reqs[0].SigValid {
		t.Error("expected SigValid=false for bogus signature")
	}
}

func TestRoundTrip_CustomApplyHandler(t *testing.T) {
	srv := dctest.NewServer(
		dctest.WithApplyHandler(func(req dctest.ApplyRequest) error {
			if req.Domain == "blocked.com" {
				return fmt.Errorf("domain blocked")
			}
			return nil
		}),
	)
	defer srv.Close()

	u := srv.URL + "/v2/domainTemplates/providers/dctest/services/test/apply?domain=blocked.com"
	resp, err := srv.HTTPClient().Get(u)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestServer_Reset(t *testing.T) {
	srv := dctest.NewServer()
	defer srv.Close()

	u := srv.URL + "/v2/domainTemplates/providers/dctest/services/test/apply?domain=a.com"
	srv.HTTPClient().Get(u)
	srv.HTTPClient().Get(u)

	if n := len(srv.ApplyRequests()); n != 2 {
		t.Fatalf("expected 2, got %d", n)
	}
	srv.Reset()
	if n := len(srv.ApplyRequests()); n != 0 {
		t.Fatalf("expected 0 after reset, got %d", n)
	}
}

func TestServer_PublicKeyPEM(t *testing.T) {
	srv := dctest.NewServer()
	defer srv.Close()

	pem := srv.PublicKeyPEM()
	if !contains(pem, "PUBLIC KEY") {
		t.Errorf("PublicKeyPEM missing header: %s", pem[:40])
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsAt(s, sub))
}

func containsAt(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
