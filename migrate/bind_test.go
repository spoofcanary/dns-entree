package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleZone = `$ORIGIN example.com.
$TTL 3600
@   IN SOA  ns1.example.com. hostmaster.example.com. ( 1 7200 3600 1209600 3600 )
@   IN NS   ns1.example.com.
@   IN A    192.0.2.1
www IN A    192.0.2.2
mail IN MX  10 mail.example.com.
_dmarc IN TXT "v=DMARC1; p=none"
alias IN CNAME www.example.com.
`

const includeZone = `$ORIGIN example.com.
$TTL 3600
@   IN SOA  ns1.example.com. hostmaster.example.com. ( 1 7200 3600 1209600 3600 )
$INCLUDE /etc/passwd
`

func writeTemp(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestBINDImport_Basic(t *testing.T) {
	p := writeTemp(t, "z.zone", sampleZone)
	z, err := ImportBINDFile(p, "example.com")
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if z.Source != "bind" {
		t.Errorf("source = %s", z.Source)
	}
	// SOA dropped.
	for _, r := range z.Records {
		if r.Type == "SOA" {
			t.Errorf("SOA should be dropped, got %+v", r)
		}
	}
	want := map[string]bool{
		"A|example.com":         false,
		"A|www.example.com":     false,
		"MX|mail.example.com":   false,
		"TXT|_dmarc.example.com": false,
		"CNAME|alias.example.com": false,
		"NS|example.com":        false,
	}
	for _, r := range z.Records {
		k := r.Type + "|" + r.Name
		if _, ok := want[k]; ok {
			want[k] = true
		}
	}
	for k, v := range want {
		if !v {
			t.Errorf("missing record %s", k)
		}
	}
}

func TestBINDImport_RejectsInclude(t *testing.T) {
	p := writeTemp(t, "z.zone", includeZone)
	_, err := ImportBINDFile(p, "example.com")
	if err == nil {
		t.Fatal("expected error for $INCLUDE, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "include") &&
		!strings.Contains(strings.ToLower(err.Error()), "bind parse") {
		t.Logf("error message: %v", err)
	}
}
