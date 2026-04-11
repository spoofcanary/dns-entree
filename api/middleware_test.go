package api

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestCredentialRedactionEmpirical sends a real credential header through the
// full middleware chain and asserts the value never appears in the slog
// buffer (D-30, T-06-01).
func TestCredentialRedactionEmpirical(t *testing.T) {
	const tokenValue = "secret-token-xyz-do-not-log"
	const headerName = "X-Entree-Cloudflare-Token"

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	s := NewServer(Options{Logger: logger})

	// Register a probe handler that reports whether the credential survived
	// the scrubbing chain (it should, via the credentials context).
	s.Mux().HandleFunc("POST /probe", func(w http.ResponseWriter, r *http.Request) {
		_, creds, err := parseCredentialHeaders(r)
		if err != nil {
			t.Errorf("parseCredentialHeaders: %v", err)
			http.Error(w, "bad", 500)
			return
		}
		if creds.APIToken != tokenValue {
			t.Errorf("handler did not see credential value via context (got %q)", creds.APIToken)
		}
		// Confirm scrubbing: r.Header itself must be empty for that name.
		if v := r.Header.Get(headerName); v != "" {
			t.Errorf("scrubbed header still present on r.Header: %q", v)
		}
		w.WriteHeader(204)
	})

	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/probe", nil)
	req.Header.Set("X-Entree-Provider", "cloudflare")
	req.Header.Set(headerName, tokenValue)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	logged := buf.String()
	if strings.Contains(logged, tokenValue) {
		t.Fatalf("credential value leaked into slog buffer: %s", logged)
	}
	if strings.Contains(strings.ToLower(logged), strings.ToLower(headerName)) {
		t.Fatalf("credential header NAME leaked into slog buffer: %s", logged)
	}
}

func TestRecoverMiddlewareCatchesPanic(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	s := NewServer(Options{Logger: logger})
	s.Mux().HandleFunc("GET /boom", func(w http.ResponseWriter, r *http.Request) {
		panic("kaboom")
	})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/boom")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 500 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `"INTERNAL"`) {
		t.Fatalf("body=%s", body)
	}
	if strings.Contains(string(body), "goroutine ") {
		t.Fatalf("stack leaked into client body: %s", body)
	}
}

func TestCORSDisabledByDefault(t *testing.T) {
	ts := newTestServer(t, Options{})
	req, _ := http.NewRequest("GET", ts.URL+"/healthz", nil)
	req.Header.Set("Origin", "https://app.example")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if v := resp.Header.Get("Access-Control-Allow-Origin"); v != "" {
		t.Fatalf("CORS leaked when disabled: %q", v)
	}
}

func TestCORSAllowlist(t *testing.T) {
	ts := newTestServer(t, Options{CORSOrigins: []string{"https://app.example"}})
	req, _ := http.NewRequest("GET", ts.URL+"/healthz", nil)
	req.Header.Set("Origin", "https://app.example")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://app.example" {
		t.Fatalf("ACA-Origin=%q", got)
	}

	// Mismatched origin: header omitted.
	req2, _ := http.NewRequest("GET", ts.URL+"/healthz", nil)
	req2.Header.Set("Origin", "https://evil.example")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if got := resp2.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("ACA-Origin leaked for unallowed origin: %q", got)
	}
}

func TestCORSPreflight(t *testing.T) {
	ts := newTestServer(t, Options{CORSOrigins: []string{"https://app.example"}})
	req, _ := http.NewRequest("OPTIONS", ts.URL+"/healthz", nil)
	req.Header.Set("Origin", "https://app.example")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Fatalf("preflight status=%d", resp.StatusCode)
	}
}

func TestRequestIDEcho(t *testing.T) {
	ts := newTestServer(t, Options{})
	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("X-Request-ID") == "" {
		t.Fatal("missing X-Request-ID")
	}
}
