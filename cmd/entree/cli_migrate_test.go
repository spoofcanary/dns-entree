package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeZoneJSON writes a schema_version:1 zone export file for import tests.
func writeZoneJSON(t *testing.T, dir string) string {
	t.Helper()
	doc := map[string]any{
		"schema_version": 1,
		"command":        "zone.export",
		"domain":         "example.com",
		"source":         "iterated",
		"nameservers":    []string{"ns1.example.", "ns2.example."},
		"records": []map[string]any{
			{"Type": "TXT", "Name": "_dmarc.example.com", "Content": "v=DMARC1; p=none", "TTL": 300},
			{"Type": "A", "Name": "www.example.com", "Content": "192.0.2.1", "TTL": 300},
		},
	}
	buf, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "zone.json")
	if err := os.WriteFile(p, buf, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestZoneImportMissingFrom(t *testing.T) {
	stdout, _, code := runEntree(t, "--json", "zone", "import", "example.com",
		"--to", "fake", "--credentials-file", "testdata/credentials_valid.json")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d; stdout=%s", code, stdout)
	}
	if !strings.Contains(stdout, `"code":"MISSING_FROM"`) {
		t.Fatalf("expected MISSING_FROM; got %s", stdout)
	}
}

func TestZoneImportMissingTo(t *testing.T) {
	dir := t.TempDir()
	zp := writeZoneJSON(t, dir)
	stdout, _, code := runEntree(t, "--json", "zone", "import", "example.com",
		"--from", zp, "--credentials-file", "testdata/credentials_valid.json")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d; stdout=%s", code, stdout)
	}
	if !strings.Contains(stdout, `"code":"MISSING_TO"`) {
		t.Fatalf("expected MISSING_TO; got %s", stdout)
	}
}

func TestZoneImportDryRun(t *testing.T) {
	dir := t.TempDir()
	zp := writeZoneJSON(t, dir)
	stdout, stderr, code := runEntree(t, "--json", "zone", "import", "example.com",
		"--from", zp, "--to", "fake",
		"--credentials-file", "testdata/credentials_valid.json",
		"--dry-run",
		"--verify-timeout", "1s",
	)
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
	if v, _ := data["schema_version"].(float64); int(v) != 1 {
		t.Fatalf("schema_version=%v", data["schema_version"])
	}
	if applied, _ := data["applied"].(bool); applied {
		t.Fatalf("dry-run should not be applied")
	}
	if tzs, _ := data["target_zone_status"].(string); tzs != "will_create" {
		t.Fatalf("target_zone_status=%v", data["target_zone_status"])
	}
}

func TestZoneImportApply(t *testing.T) {
	dir := t.TempDir()
	zp := writeZoneJSON(t, dir)
	stdout, stderr, code := runEntree(t, "--json", "zone", "import", "example.com",
		"--from", zp, "--to", "fake",
		"--credentials-file", "testdata/credentials_valid.json",
		"--yes",
		"--verify-timeout", "1s",
	)
	if code != 0 {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("decode envelope: %v; stdout=%s", err, stdout)
	}
	data, _ := env["data"].(map[string]any)
	if applied, _ := data["applied"].(bool); !applied {
		t.Fatalf("expected applied=true")
	}
	results, _ := data["results"].([]any)
	if len(results) == 0 {
		t.Fatalf("expected results; got %v", data)
	}
}

func TestZoneImportRequiresYesNonTTY(t *testing.T) {
	// Without --yes and without --dry-run, zone import behaves as preview
	// (dry-run fallback). So we expect ok and applied=false.
	dir := t.TempDir()
	zp := writeZoneJSON(t, dir)
	stdout, _, code := runEntree(t, "--json", "zone", "import", "example.com",
		"--from", zp, "--to", "fake",
		"--credentials-file", "testdata/credentials_valid.json",
		"--verify-timeout", "1s",
	)
	if code != 0 {
		t.Fatalf("exit %d stdout=%s", code, stdout)
	}
	var env map[string]any
	_ = json.Unmarshal([]byte(stdout), &env)
	data, _ := env["data"].(map[string]any)
	if applied, _ := data["applied"].(bool); applied {
		t.Fatalf("without --yes must not apply")
	}
}

func TestMigrateMissingTo(t *testing.T) {
	stdout, _, code := runEntree(t, "--json", "migrate", "example.com")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d; stdout=%s", code, stdout)
	}
	if !strings.Contains(stdout, `"code":"MISSING_TO"`) {
		t.Fatalf("expected MISSING_TO; got %s", stdout)
	}
}

func TestMigrateInvalidRate(t *testing.T) {
	stdout, _, code := runEntree(t, "--json", "migrate", "example.com",
		"--to", "fake", "--rate", "500")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d; stdout=%s", code, stdout)
	}
	if !strings.Contains(stdout, `"code":"INVALID_RATE"`) {
		t.Fatalf("expected INVALID_RATE; got %s", stdout)
	}
}

func TestZoneExportBadFormat(t *testing.T) {
	stdout, _, code := runEntree(t, "--json", "zone", "export", "example.com",
		"--format", "yaml", "--no-axfr")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d; stdout=%s", code, stdout)
	}
	if !strings.Contains(stdout, `"code":"INVALID_FORMAT"`) {
		t.Fatalf("expected INVALID_FORMAT; got %s", stdout)
	}
}
