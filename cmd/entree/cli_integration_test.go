package main

import (
	"bytes"
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
