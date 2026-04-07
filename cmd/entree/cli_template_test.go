package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spoofcanary/dns-entree/domainconnect"
)

// seedFakeCache writes a minimal Domain Connect template JSON at the layout
// expected by template.LoadTemplate: <cacheDir>/<providerID>/<providerID>.<serviceID>.json
// Returns the cache dir path.
func seedFakeCache(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	provDir := filepath.Join(dir, "fakeprov")
	if err := os.MkdirAll(provDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tpl := map[string]any{
		"providerId":   "fakeprov",
		"providerName": "Fake Provider",
		"serviceId":    "svc1",
		"serviceName":  "Fake Service One",
		"version":      1,
		"description":  "Synthetic template for tests",
		"records": []map[string]any{
			{
				"type": "TXT",
				"host": "@",
				"data": "v=spf1 include:%spfinclude% -all",
				"ttl":  300,
			},
			{
				"type":     "CNAME",
				"host":     "www",
				"pointsTo": "%target%",
				"ttl":      3600,
			},
		},
	}
	data, err := json.MarshalIndent(tpl, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(provDir, "fakeprov.svc1.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// runCmd executes a freshly-reset rootCmd with args and captures stdout.
// Because cobra root is package-global, tests must not run in parallel.
func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	// Reset global flag state that tests care about.
	flagJSON = false
	flagQuiet = false
	flagCacheDir = ""
	flagResolveVars = nil
	flagVarsFile = ""
	flagYes = false

	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs(args)
	err := rootCmd.ExecuteContext(context.Background())

	// Formatter writes to its own Out (os.Stdout normally); when --json is set
	// we redirect via an override by reading the formatter from the cmd ctx.
	return buf.String(), err
}

// execWithJSONCapture runs rootCmd with --json and captures stdout from the
// formatter. Returns the JSON bytes from the success envelope.
func execWithJSONCapture(t *testing.T, args ...string) (map[string]any, error) {
	t.Helper()
	// Reset state.
	flagJSON = false
	flagQuiet = false
	flagCacheDir = ""
	flagResolveVars = nil
	flagVarsFile = ""
	flagYes = false

	// Hook the formatter to capture output by installing a PersistentPreRunE
	// wrapper is invasive. Instead, swap os.Stdout.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origStdout := os.Stdout
	os.Stdout = w

	rootCmd.SetOut(os.Stderr)
	rootCmd.SetErr(os.Stderr)
	rootCmd.SetArgs(append([]string{"--json"}, args...))
	runErr := rootCmd.ExecuteContext(context.Background())

	w.Close()
	os.Stdout = origStdout
	var outBuf bytes.Buffer
	_, _ = outBuf.ReadFrom(r)

	var env map[string]any
	if len(outBuf.Bytes()) > 0 {
		if jerr := json.Unmarshal(outBuf.Bytes(), &env); jerr != nil {
			t.Logf("stdout was: %q", outBuf.String())
			t.Fatalf("decode envelope: %v", jerr)
		}
	}
	return env, runErr
}

func TestTemplatesListJSON(t *testing.T) {
	cache := seedFakeCache(t)
	env, err := execWithJSONCapture(t, "templates", "list", "--cache-dir", cache)
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if ok, _ := env["ok"].(bool); !ok {
		t.Fatalf("envelope not ok: %v", env)
	}
	data, _ := env["data"].(map[string]any)
	tpls, _ := data["templates"].([]any)
	if len(tpls) != 1 {
		t.Fatalf("want 1 template, got %d (%v)", len(tpls), tpls)
	}
	first, _ := tpls[0].(map[string]any)
	if first["provider_id"] != "fakeprov" || first["service_id"] != "svc1" {
		t.Fatalf("unexpected: %v", first)
	}
}

func TestTemplatesShowJSON(t *testing.T) {
	cache := seedFakeCache(t)
	env, err := execWithJSONCapture(t, "templates", "show", "fakeprov/svc1", "--cache-dir", cache)
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	data, _ := env["data"].(map[string]any)
	if data["provider_id"] != "fakeprov" {
		t.Fatalf("want fakeprov, got %v", data["provider_id"])
	}
	recs, _ := data["records"].([]any)
	if len(recs) != 2 {
		t.Fatalf("want 2 records, got %d", len(recs))
	}
}

func TestTemplatesResolveJSON(t *testing.T) {
	cache := seedFakeCache(t)
	env, err := execWithJSONCapture(t, "templates", "resolve", "fakeprov/svc1",
		"--cache-dir", cache,
		"--var", "spfinclude=mailer.example.net",
		"--var", "target=www.example.net")
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	data, _ := env["data"].(map[string]any)
	recs, _ := data["records"].([]any)
	if len(recs) != 2 {
		t.Fatalf("want 2 records, got %d: %v", len(recs), data)
	}
	txt, _ := recs[0].(map[string]any)
	if !strings.Contains(txt["content"].(string), "mailer.example.net") {
		t.Fatalf("spf var not substituted: %v", txt)
	}
	cname, _ := recs[1].(map[string]any)
	if cname["content"] != "www.example.net" {
		t.Fatalf("target var not substituted: %v", cname)
	}
}

func TestTemplatesShowMissing(t *testing.T) {
	cache := seedFakeCache(t)
	env, err := execWithJSONCapture(t, "templates", "show", "nope/missing", "--cache-dir", cache)
	if err == nil {
		t.Fatalf("expected error, got envelope %v", env)
	}
	var ue *UserError
	if !errorsAs(err, &ue) || ue.Code != "TEMPLATE_NOT_FOUND" {
		t.Fatalf("want TEMPLATE_NOT_FOUND, got %v", err)
	}
}

func TestDCDiscoverInvalidDomain(t *testing.T) {
	_, err := execWithJSONCapture(t, "dc-discover", "   ")
	if err == nil {
		t.Fatalf("want error")
	}
	var ue *UserError
	if !errorsAs(err, &ue) || ue.Code != "INVALID_DOMAIN" {
		t.Fatalf("want INVALID_DOMAIN, got %v", err)
	}
}

func TestDCDiscoverSeam(t *testing.T) {
	orig := discoverFn
	discoverFn = func(ctx context.Context, domain string) (domainconnect.DiscoveryResult, error) {
		return domainconnect.DiscoveryResult{
			Supported:    true,
			ProviderID:   "fake",
			ProviderName: "Fake DC",
			URLSyncUX:    "https://dc.example/v2/sync",
		}, nil
	}
	defer func() { discoverFn = orig }()

	env, err := execWithJSONCapture(t, "dc-discover", "example.com")
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	data, _ := env["data"].(map[string]any)
	if data["supported"] != true || data["provider_id"] != "fake" {
		t.Fatalf("unexpected: %v", data)
	}
}

func TestTemplatesSyncDryGate(t *testing.T) {
	if os.Getenv("ENTREE_INTEGRATION_NETWORK") != "1" {
		t.Skip("network-gated (set ENTREE_INTEGRATION_NETWORK=1)")
	}
	// Intentionally no assertion body: this is a gate guard only.
}

// errorsAs is a local wrapper around errors.As for test conciseness.
func errorsAs(err error, target any) bool {
	// delegate to stdlib via a tiny inline to avoid an extra import block churn
	return errorsAsImpl(err, target)
}
