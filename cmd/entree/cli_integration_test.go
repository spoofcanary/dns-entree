package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var binPath string

func TestMain(m *testing.M) {
	if _, err := exec.LookPath("go"); err != nil {
		// Without a go toolchain we cannot build the binary; skip by exiting 0.
		os.Exit(0)
	}
	dir, err := os.MkdirTemp("", "entree-bin-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	binPath = filepath.Join(dir, "entree")
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("failed to build entree binary: " + err.Error())
	}
	os.Exit(m.Run())
}

func runEntree(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Env = append(os.Environ(), "ENTREE_TEST_NO_VERIFY=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("run entree: %v", err)
		}
	}
	return stdout.String(), stderr.String(), code
}

func assertGolden(t *testing.T, got, goldenPath string) {
	t.Helper()
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v", goldenPath, err)
	}
	if string(want) != got {
		t.Fatalf("golden mismatch for %s\nwant: %q\ngot:  %q", goldenPath, string(want), got)
	}
}

func TestSPFMergeJSON(t *testing.T) {
	stdout, _, code := runEntree(t, "--json", "spf-merge", "v=spf1 ~all", "_spf.example.com")
	if code != 0 {
		t.Fatalf("exit %d stdout=%s", code, stdout)
	}
	assertGolden(t, stdout, "testdata/spfmerge_json.golden")
}

func TestSPFMergeHuman(t *testing.T) {
	stdout, _, code := runEntree(t, "spf-merge", "v=spf1 ~all", "_spf.example.com")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if strings.TrimSpace(stdout) != "v=spf1 include:_spf.example.com ~all" {
		t.Fatalf("unexpected human output: %q", stdout)
	}
}

func TestVerifyUnsupportedType(t *testing.T) {
	stdout, _, code := runEntree(t, "--json", "verify", "example.com", "FOO", "_dmarc.example.com")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d; stdout=%s", code, stdout)
	}
	if !strings.Contains(stdout, `"code":"INVALID_RECORD_TYPE"`) {
		t.Fatalf("expected INVALID_RECORD_TYPE error; got %s", stdout)
	}
}

func TestDetectInvalidDomain(t *testing.T) {
	stdout, _, code := runEntree(t, "--json", "detect", "")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d; stdout=%s", code, stdout)
	}
	if !strings.Contains(stdout, `"code":"INVALID_DOMAIN"`) {
		t.Fatalf("expected INVALID_DOMAIN; got %s", stdout)
	}
}

// ---- apply ----

func applyArgs(extra ...string) []string {
	base := []string{
		"--json", "apply", "example.com",
		"--provider", "fake",
		"--credentials-file", "testdata/credentials_valid.json",
	}
	return append(base, extra...)
}

func TestApplyRecordRequiresYesNonTTY(t *testing.T) {
	stdout, _, code := runEntree(t, applyArgs("--record", "TXT:_dmarc.example.com:v=DMARC1")...)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d; stdout=%s", code, stdout)
	}
	if !strings.Contains(stdout, `"code":"CONFIRM_REQUIRED"`) {
		t.Fatalf("expected CONFIRM_REQUIRED; got %s", stdout)
	}
}

func TestApplyRecordHappyPath(t *testing.T) {
	stdout, _, code := runEntree(t, applyArgs("--record", "TXT:_dmarc.example.com:v=DMARC1", "--yes")...)
	if code != 0 {
		t.Fatalf("exit %d stdout=%s", code, stdout)
	}
	assertGolden(t, stdout, "testdata/apply_record_json.golden")
}

func TestApplyDryRunNoYesNeeded(t *testing.T) {
	stdout, _, code := runEntree(t, applyArgs("--record", "TXT:_dmarc.example.com:v=DMARC1", "--dry-run")...)
	if code != 0 {
		t.Fatalf("exit %d stdout=%s", code, stdout)
	}
	assertGolden(t, stdout, "testdata/apply_dryrun_json.golden")
}

func TestApplyMultipleRecords(t *testing.T) {
	stdout, _, code := runEntree(t, applyArgs(
		"--record", "TXT:_dmarc.example.com:v=DMARC1",
		"--record", "TXT:_mta.example.com:v=STSv1",
		"--yes",
	)...)
	if code != 0 {
		t.Fatalf("exit %d stdout=%s", code, stdout)
	}
	if strings.Count(stdout, `"status":"created"`) != 2 {
		t.Fatalf("expected 2 created results; got %s", stdout)
	}
}

func TestApplyInvalidRecordSpec(t *testing.T) {
	stdout, _, code := runEntree(t, applyArgs("--record", "FOO", "--yes")...)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d; stdout=%s", code, stdout)
	}
	if !strings.Contains(stdout, `"code":"INVALID_RECORD_SPEC"`) {
		t.Fatalf("expected INVALID_RECORD_SPEC; got %s", stdout)
	}
}

// ---- apply --template ----

// seedSubprocessCache creates a minimal template cache layout matching what
// template.LoadTemplate expects. Mirrors seedFakeCache in cli_template_test.go
// but usable from subprocess-based tests.
func seedSubprocessCache(t *testing.T) string {
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
		"description":  "Synthetic template for subprocess tests",
		"records": []map[string]any{
			{"type": "TXT", "host": "@", "data": "v=spf1 include:%spfinclude% -all", "ttl": 300},
			{"type": "CNAME", "host": "www", "pointsTo": "%target%", "ttl": 3600},
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

func applyTemplateArgs(cacheDir string, extra ...string) []string {
	base := []string{
		"--json", "apply", "example.com",
		"--provider", "fake",
		"--credentials-file", "testdata/credentials_valid.json",
		"--cache-dir", cacheDir,
		"--template", "fakeprov/svc1",
		"--var", "spfinclude=mailer.example.net",
		"--var", "target=www.example.net",
	}
	return append(base, extra...)
}

func TestApplyTemplateDryRun(t *testing.T) {
	cache := seedSubprocessCache(t)
	stdout, stderr, code := runEntree(t, applyTemplateArgs(cache, "--dry-run")...)
	if code != 0 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("decode envelope: %v; stdout=%s", err, stdout)
	}
	if ok, _ := env["ok"].(bool); !ok {
		t.Fatalf("envelope not ok: %v", env)
	}
	data, _ := env["data"].(map[string]any)
	recs, _ := data["records"].([]any)
	if len(recs) != 2 {
		t.Fatalf("want 2 records, got %d: %v", len(recs), data)
	}
}

func TestApplyTemplateHappyPath(t *testing.T) {
	cache := seedSubprocessCache(t)
	stdout, stderr, code := runEntree(t, applyTemplateArgs(cache, "--yes")...)
	if code != 0 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("decode envelope: %v; stdout=%s", err, stdout)
	}
	if ok, _ := env["ok"].(bool); !ok {
		t.Fatalf("envelope not ok: %v", env)
	}
	data, _ := env["data"].(map[string]any)
	results, _ := data["results"].([]any)
	if len(results) == 0 {
		t.Fatalf("expected results, got %v", data)
	}
}

func TestApplyTemplateMissing(t *testing.T) {
	cache := seedSubprocessCache(t)
	stdout, _, code := runEntree(t,
		"--json", "apply", "example.com",
		"--provider", "fake",
		"--credentials-file", "testdata/credentials_valid.json",
		"--cache-dir", cache,
		"--template", "nope/missing",
		"--yes",
	)
	if code == 0 {
		t.Fatalf("expected non-zero exit; stdout=%s", stdout)
	}
	if !strings.Contains(stdout, `"code":"TEMPLATE_NOT_FOUND"`) {
		t.Fatalf("expected TEMPLATE_NOT_FOUND; got %s", stdout)
	}
}

func TestApplyTemplateRequiresEitherRecordOrTemplate(t *testing.T) {
	stdout, _, code := runEntree(t, applyArgs("--yes")...)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d; stdout=%s", code, stdout)
	}
	if !strings.Contains(stdout, `"code":"NO_OPERATION"`) {
		t.Fatalf("expected NO_OPERATION; got %s", stdout)
	}
}

func TestApplyTemplateConflictRecordAndTemplate(t *testing.T) {
	cache := seedSubprocessCache(t)
	stdout, _, code := runEntree(t, applyTemplateArgs(cache,
		"--record", "TXT:_dmarc.example.com:v=DMARC1",
		"--yes",
	)...)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d; stdout=%s", code, stdout)
	}
	if !strings.Contains(stdout, `"code":"MUTUALLY_EXCLUSIVE_OPS"`) {
		t.Fatalf("expected MUTUALLY_EXCLUSIVE_OPS; got %s", stdout)
	}
}

func TestApplyTemplateNonTTYRequiresYes(t *testing.T) {
	cache := seedSubprocessCache(t)
	stdout, _, code := runEntree(t, applyTemplateArgs(cache)...)
	if code != 2 {
		t.Fatalf("expected exit 2, got %d; stdout=%s", code, stdout)
	}
	if !strings.Contains(stdout, `"code":"CONFIRM_REQUIRED"`) {
		t.Fatalf("expected CONFIRM_REQUIRED; got %s", stdout)
	}
}
