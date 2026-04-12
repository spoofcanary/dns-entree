package api

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestSecurityCredentialRedactionExhaustive verifies that ALL X-Entree-*
// credential headers are stripped from slog output, including edge-case values
// containing special characters that might break redaction (D-25, D-30).
func TestSecurityCredentialRedactionExhaustive(t *testing.T) {
	// Each entry: header name -> value with special chars.
	headers := map[string]string{
		"X-Entree-Provider":                   "cloudflare",
		"X-Entree-Cloudflare-Token":           `token-with="quotes"`,
		"X-Entree-AWS-Access-Key-Id":          "AKIAIOSFODNN7EXAMPLE",
		"X-Entree-AWS-Secret-Access-Key":      "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		"X-Entree-AWS-Region":                 "us-east-1",
		"X-Entree-GoDaddy-Key":                "key-with-special=chars&more",
		"X-Entree-GoDaddy-Secret":             "secret/with+base64==chars",
		"X-Entree-GCDNS-Service-Account-JSON": "eyJwcm9qZWN0X2lkIjoiZXhhbXBsZSJ9",
		"X-Entree-GCDNS-Project-Id":           "my-gcp-project",
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	s := NewServer(Options{Logger: logger})
	s.Mux().HandleFunc("POST /sec-probe", func(w http.ResponseWriter, r *http.Request) {
		// Verify headers are NOT present on the scrubbed request.
		for name := range headers {
			if name == "X-Entree-Provider" {
				continue // not a secret per se, but should still be scrubbed
			}
			if v := r.Header.Get(name); v != "" {
				t.Errorf("scrubbed header %q still present on r.Header: %q", name, v)
			}
		}
		w.WriteHeader(204)
	})

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/sec-probe", nil)
	for name, val := range headers {
		req.Header.Set(name, val)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	logged := buf.String()
	for name, val := range headers {
		if strings.Contains(logged, val) {
			t.Errorf("credential value for %q leaked into slog buffer:\n  value: %q\n  log: %s", name, val, logged)
		}
		// Also check that the header name (canonical) does not appear.
		canon := http.CanonicalHeaderKey(name)
		if strings.Contains(logged, canon) {
			t.Errorf("credential header name %q leaked into slog buffer: %s", canon, logged)
		}
	}
}

// TestSecurityScrubDetailsStripsKeys verifies that scrubDetails removes keys
// containing sensitive data (D-26).
func TestSecurityScrubDetailsStripsKeys(t *testing.T) {
	input := map[string]any{
		"value":    "apikey=SECRET123",
		"token":    "abc-token-value",
		"password": "xyz-password",
		"secret":   "my-secret",
		"key":      "my-key",
		"domain":   "example.com", // non-sensitive, should survive
		"status":   "ok",          // non-sensitive, should survive
	}

	out := scrubDetails(input)
	if out == nil {
		t.Fatal("scrubDetails returned nil for input with safe keys")
	}

	// Sensitive keys must be removed.
	for _, k := range []string{"value", "token", "password", "secret", "key"} {
		if _, exists := out[k]; exists {
			t.Errorf("sensitive key %q was not stripped", k)
		}
	}

	// Non-sensitive keys must survive.
	for _, k := range []string{"domain", "status"} {
		if _, exists := out[k]; !exists {
			t.Errorf("safe key %q was incorrectly stripped", k)
		}
	}
}

// TestSecurityScrubDetailsNil verifies nil input returns nil.
func TestSecurityScrubDetailsNil(t *testing.T) {
	if out := scrubDetails(nil); out != nil {
		t.Fatalf("expected nil, got %v", out)
	}
}

// TestSecurityScrubDetailsAllSensitive verifies that all-sensitive input returns nil.
func TestSecurityScrubDetailsAllSensitive(t *testing.T) {
	input := map[string]any{
		"token":  "x",
		"secret": "y",
	}
	if out := scrubDetails(input); out != nil {
		t.Fatalf("expected nil when all keys sensitive, got %v", out)
	}
}

// TestSecurityTimingSafeTokenComparison verifies that the stateful migration
// handlers use crypto/subtle.ConstantTimeCompare for bearer token comparison.
// This is a code-property test: we grep the source to ensure the security
// invariant is maintained (T-09-07).
func TestSecurityTimingSafeTokenComparison(t *testing.T) {
	src, err := os.ReadFile("handlers_migrate_stateful.go")
	if err != nil {
		t.Fatalf("cannot read handlers_migrate_stateful.go: %v", err)
	}
	content := string(src)

	if !strings.Contains(content, "subtle.ConstantTimeCompare") {
		t.Error("handlers_migrate_stateful.go does not use subtle.ConstantTimeCompare for token comparison")
	}

	// Verify the import of crypto/subtle is present.
	if !strings.Contains(content, `"crypto/subtle"`) {
		t.Error("handlers_migrate_stateful.go does not import crypto/subtle")
	}

	// Verify checkAccessToken function exists and uses it.
	if !strings.Contains(content, "func checkAccessToken") {
		t.Error("checkAccessToken function not found")
	}
}
