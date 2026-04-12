package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	entree "github.com/spoofcanary/dns-entree"
	_ "github.com/spoofcanary/dns-entree/internal/fakeprovider"
)

// doJSON posts JSON to the server handler and returns status + parsed envelope.
func doJSON(t *testing.T, h http.Handler, method, path, body string, headers map[string]string) (int, map[string]any, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var env map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	return rec.Code, env, rec.Body.String()
}

func newHandler(t *testing.T, opts Options) (http.Handler, *Server) {
	t.Helper()
	s := NewServer(opts)
	return s.Handler(), s
}

// ---- detect ----

func TestHandleDetect_OK(t *testing.T) {
	prev := detectProviderFn
	detectProviderFn = func(ctx context.Context, domain string) (*entree.DetectionResult, error) {
		return &entree.DetectionResult{
			Provider: entree.ProviderCloudflare, Label: "Cloudflare",
			Supported: true, Nameservers: []string{"ns1.cloudflare.com"}, Method: "ns_pattern",
		}, nil
	}
	defer func() { detectProviderFn = prev }()

	h, _ := newHandler(t, Options{})
	code, env, raw := doJSON(t, h, "POST", "/v1/detect", `{"domain":"example.com"}`, nil)
	if code != 200 {
		t.Fatalf("code=%d body=%s", code, raw)
	}
	if env["ok"] != true {
		t.Fatalf("body=%s", raw)
	}
	data := env["data"].(map[string]any)
	if data["provider"] != "cloudflare" || data["supported"] != true {
		t.Fatalf("data=%v", data)
	}
}

func TestHandleDetect_MissingDomain(t *testing.T) {
	h, _ := newHandler(t, Options{})
	code, _, _ := doJSON(t, h, "POST", "/v1/detect", `{}`, nil)
	if code != 400 {
		t.Fatalf("code=%d", code)
	}
}

func TestHandleDetect_WrongMethod(t *testing.T) {
	h, _ := newHandler(t, Options{})
	req := httptest.NewRequest("GET", "/v1/detect", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 405 && rec.Code != 404 {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestHandleDetect_WrongContentType(t *testing.T) {
	h, _ := newHandler(t, Options{})
	req := httptest.NewRequest("POST", "/v1/detect", strings.NewReader(`{"domain":"x"}`))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 415 {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestHandleDetect_BadJSON(t *testing.T) {
	h, _ := newHandler(t, Options{})
	code, _, _ := doJSON(t, h, "POST", "/v1/detect", `{not json`, nil)
	if code != 400 {
		t.Fatalf("code=%d", code)
	}
}

func TestHandleDetect_Oversize(t *testing.T) {
	h, _ := newHandler(t, Options{})
	big := strings.Repeat("a", int(BodyLimitDefault)+16)
	body := fmt.Sprintf(`{"domain":"%s"}`, big)
	code, _, _ := doJSON(t, h, "POST", "/v1/detect", body, nil)
	if code != 413 {
		t.Fatalf("code=%d", code)
	}
}

// ---- verify ----

func TestHandleVerify_OK(t *testing.T) {
	prev := verifyFn
	verifyFn = func(ctx context.Context, domain string, opts entree.VerifyOpts) (entree.VerifyResult, error) {
		return entree.VerifyResult{Verified: true, CurrentValue: "v=DMARC1; p=none", Method: "authoritative"}, nil
	}
	defer func() { verifyFn = prev }()

	h, _ := newHandler(t, Options{})
	code, env, raw := doJSON(t, h, "POST", "/v1/verify",
		`{"domain":"example.com","type":"TXT","name":"_dmarc.example.com","contains":"DMARC1"}`, nil)
	if code != 200 {
		t.Fatalf("code=%d body=%s", code, raw)
	}
	if env["ok"] != true {
		t.Fatalf("body=%s", raw)
	}
}

func TestHandleVerify_Mismatch409(t *testing.T) {
	prev := verifyFn
	verifyFn = func(ctx context.Context, domain string, opts entree.VerifyOpts) (entree.VerifyResult, error) {
		return entree.VerifyResult{Verified: false, CurrentValue: "old", Method: "authoritative"}, nil
	}
	defer func() { verifyFn = prev }()

	h, _ := newHandler(t, Options{})
	code, env, raw := doJSON(t, h, "POST", "/v1/verify",
		`{"domain":"example.com","type":"TXT","name":"_dmarc.example.com","contains":"DMARC1"}`, nil)
	if code != 409 {
		t.Fatalf("code=%d body=%s", code, raw)
	}
	errObj := env["error"].(map[string]any)
	if errObj["code"] != CodeVerifyMismatch {
		t.Fatalf("err=%v", errObj)
	}
}

// ---- spf-merge ----

func TestHandleSPFMerge_OK(t *testing.T) {
	h, _ := newHandler(t, Options{})
	code, env, raw := doJSON(t, h, "POST", "/v1/spf-merge",
		`{"current":"v=spf1 -all","includes":["_spf.example.net"]}`, nil)
	if code != 200 {
		t.Fatalf("code=%d body=%s", code, raw)
	}
	data := env["data"].(map[string]any)
	if data["changed"] != true {
		t.Fatalf("data=%v", data)
	}
	if !strings.Contains(data["value"].(string), "include:_spf.example.net") {
		t.Fatalf("value=%v", data["value"])
	}
}

// ---- apply ----

func TestHandleApply_DryRun(t *testing.T) {
	h, _ := newHandler(t, Options{})
	body := `{"domain":"example.com","dry_run":true,"records":[{"type":"TXT","name":"_dmarc.example.com","content":"v=DMARC1; p=none"}]}`
	code, env, raw := doJSON(t, h, "POST", "/v1/apply", body, map[string]string{
		"X-Entree-Provider": "fake",
	})
	if code != 200 {
		t.Fatalf("code=%d body=%s", code, raw)
	}
	data := env["data"].(map[string]any)
	if data["dry_run"] != true {
		t.Fatalf("data=%v", data)
	}
	if _, ok := data["diffs"]; !ok {
		t.Fatalf("no diffs: %v", data)
	}
}

func TestHandleApply_RealPush(t *testing.T) {
	t.Setenv("ENTREE_TEST_NO_VERIFY", "1")
	restore := entree.SetVerifyFuncForTest(func(ctx context.Context, domain string, opts entree.VerifyOpts) (entree.VerifyResult, error) {
		return entree.VerifyResult{Verified: true, Method: "stubbed"}, nil
	})
	defer restore()

	h, _ := newHandler(t, Options{})
	body := `{"domain":"example.com","dry_run":false,"records":[{"type":"TXT","name":"_dmarc.example.com","content":"v=DMARC1; p=none"}]}`
	code, env, raw := doJSON(t, h, "POST", "/v1/apply", body, map[string]string{
		"X-Entree-Provider": "fake",
	})
	if code != 200 {
		t.Fatalf("code=%d body=%s", code, raw)
	}
	data := env["data"].(map[string]any)
	results, _ := data["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("results=%v", results)
	}
	first := results[0].(map[string]any)
	if first["status"] != "created" {
		t.Fatalf("first=%v", first)
	}
}

func TestHandleApply_MissingProviderHeader(t *testing.T) {
	h, _ := newHandler(t, Options{})
	body := `{"domain":"example.com","records":[{"type":"TXT","name":"x.example.com","content":"v"}]}`
	code, env, _ := doJSON(t, h, "POST", "/v1/apply", body, nil)
	if code != 400 {
		t.Fatalf("code=%d", code)
	}
	errObj := env["error"].(map[string]any)
	if errObj["code"] != CodeMissingCredentials {
		t.Fatalf("err=%v", errObj)
	}
}

// ---- credential redaction (e2e) ----

// errorProvider fails SetRecord with a raw error containing secret text.
type errorProvider struct{}

func (errorProvider) Name() string { return "errorp" }
func (errorProvider) Slug() string { return "errorp" }
func (errorProvider) Verify(ctx context.Context) ([]entree.Zone, error) {
	return nil, nil
}
func (errorProvider) GetRecords(ctx context.Context, domain, recordType string) ([]entree.Record, error) {
	return nil, nil
}
func (errorProvider) SetRecord(ctx context.Context, domain string, r entree.Record) error {
	return fmt.Errorf("cloudflare API key leak-me-secret was rejected")
}
func (errorProvider) DeleteRecord(ctx context.Context, domain, recordID string) error { return nil }
func (errorProvider) ApplyRecords(ctx context.Context, domain string, records []entree.Record) error {
	return nil
}

func init() {
	entree.RegisterProvider("errorp", func(c entree.Credentials) (entree.Provider, error) {
		return errorProvider{}, nil
	})
	// errorp has no required headers beyond X-Entree-Provider.
	providerHeaderSpecs["errorp"] = []string{}
}

func TestE2ECoreHandlers_ProviderErrorNotLeaked(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))
	s := NewServer(Options{Logger: logger})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	body := `{"domain":"example.com","dry_run":false,"records":[{"type":"TXT","name":"_dmarc.example.com","content":"v=DMARC1; p=none"}]}`
	req, _ := http.NewRequest("POST", ts.URL+"/v1/apply", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Entree-Provider", "errorp")
	// Credential header with sentinel value that must never appear in log or response.
	req.Header.Set("X-Entree-Cloudflare-Token", "leak-me-header-token")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 500 {
		t.Fatalf("status=%d body=%s", resp.StatusCode, respBody)
	}
	// Raw provider error string must NOT appear in response body.
	if bytes.Contains(respBody, []byte("leak-me-secret")) {
		t.Fatalf("raw provider error leaked: %s", respBody)
	}
	// Response must use fixed code.
	if !bytes.Contains(respBody, []byte(`"code":"PROVIDER_ERROR"`)) {
		t.Fatalf("missing fixed code: %s", respBody)
	}
	// Log buffer must NOT contain the credential header sentinel.
	if bytes.Contains(logBuf.Bytes(), []byte("leak-me-header-token")) {
		t.Fatalf("credential header leaked to log: %s", logBuf.String())
	}
}

func TestE2ECoreHandlers_DryRunNoMutations(t *testing.T) {
	h, _ := newHandler(t, Options{})
	body := `{"domain":"example.com","dry_run":true,"records":[{"type":"TXT","name":"a.example.com","content":"v"}]}`
	code, _, _ := doJSON(t, h, "POST", "/v1/apply", body, map[string]string{"X-Entree-Provider": "fake"})
	if code != 200 {
		t.Fatalf("code=%d", code)
	}
	// Second dry-run should still see no existing records (proves no writes happened).
	code2, env, _ := doJSON(t, h, "POST", "/v1/apply", body, map[string]string{"X-Entree-Provider": "fake"})
	if code2 != 200 {
		t.Fatalf("code=%d", code2)
	}
	data := env["data"].(map[string]any)
	diffs := data["diffs"].([]any)
	first := diffs[0].(map[string]any)
	if first["action"] != "CREATE" {
		t.Fatalf("expected CREATE (no prior mutation), got %v", first)
	}
}

// ---- validation ----

func TestHandleDetect_InvalidDomain(t *testing.T) {
	h, _ := newHandler(t, Options{})
	code, env, _ := doJSON(t, h, "POST", "/v1/detect", `{"domain":"exam ple.com"}`, nil)
	if code != 400 {
		t.Fatalf("code=%d", code)
	}
	errObj := env["error"].(map[string]any)
	if errObj["code"] != CodeBadRequest {
		t.Fatalf("expected BAD_REQUEST, got %v", errObj["code"])
	}
	msg, _ := errObj["message"].(string)
	if !strings.Contains(msg, "invalid domain") {
		t.Fatalf("expected 'invalid domain' in message, got %q", msg)
	}
}

func TestHandleDetect_ValidDomainPassesValidation(t *testing.T) {
	prev := detectProviderFn
	detectProviderFn = func(ctx context.Context, domain string) (*entree.DetectionResult, error) {
		return &entree.DetectionResult{
			Provider: entree.ProviderCloudflare, Label: "Cloudflare",
			Supported: true, Nameservers: []string{"ns1.cloudflare.com"}, Method: "ns_pattern",
		}, nil
	}
	defer func() { detectProviderFn = prev }()

	h, _ := newHandler(t, Options{})
	code, env, raw := doJSON(t, h, "POST", "/v1/detect", `{"domain":"example.com"}`, nil)
	if code != 200 {
		t.Fatalf("code=%d body=%s", code, raw)
	}
	if env["ok"] != true {
		t.Fatalf("body=%s", raw)
	}
}

func TestHandleApply_InvalidRecordValue(t *testing.T) {
	h, _ := newHandler(t, Options{})
	body := `{"domain":"example.com","records":[{"type":"A","name":"test.example.com","content":"not-an-ip"}]}`
	code, env, _ := doJSON(t, h, "POST", "/v1/apply", body, map[string]string{
		"X-Entree-Provider": "fake",
	})
	if code != 400 {
		t.Fatalf("code=%d", code)
	}
	errObj := env["error"].(map[string]any)
	if errObj["code"] != CodeBadRequest {
		t.Fatalf("expected BAD_REQUEST, got %v", errObj["code"])
	}
	msg, _ := errObj["message"].(string)
	if !strings.Contains(msg, "invalid record") {
		t.Fatalf("expected 'invalid record' in message, got %q", msg)
	}
}

func TestHandleVerify_InvalidDomain(t *testing.T) {
	h, _ := newHandler(t, Options{})
	code, env, _ := doJSON(t, h, "POST", "/v1/verify",
		`{"domain":"exam ple.com","type":"TXT","name":"_dmarc.example.com","contains":"test"}`, nil)
	if code != 400 {
		t.Fatalf("code=%d", code)
	}
	errObj := env["error"].(map[string]any)
	if errObj["code"] != CodeBadRequest {
		t.Fatalf("expected BAD_REQUEST, got %v", errObj["code"])
	}
}

func TestHandleApply_InvalidDomain(t *testing.T) {
	h, _ := newHandler(t, Options{})
	body := `{"domain":"exam ple.com","records":[{"type":"TXT","name":"test.example.com","content":"test"}]}`
	code, env, _ := doJSON(t, h, "POST", "/v1/apply", body, map[string]string{
		"X-Entree-Provider": "fake",
	})
	if code != 400 {
		t.Fatalf("code=%d", code)
	}
	errObj := env["error"].(map[string]any)
	if errObj["code"] != CodeBadRequest {
		t.Fatalf("expected BAD_REQUEST, got %v", errObj["code"])
	}
}
