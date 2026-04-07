package template

import (
	"bytes"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
)

func loadFixture(t *testing.T, name string, opts ...LoadOption) *Template {
	t.Helper()
	tmpl, err := LoadTemplateFile(filepath.Join("testdata", name), opts...)
	if err != nil {
		t.Fatalf("load %s: %v", name, err)
	}
	return tmpl
}

func TestResolve_SimpleTXT(t *testing.T) {
	tmpl := loadFixture(t, "simple_txt.json")
	recs, err := tmpl.Resolve(map[string]string{"domain": "example.com"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("len = %d", len(recs))
	}
	if recs[0].Content != "verify=example.com" {
		t.Errorf("Content = %q", recs[0].Content)
	}
	if recs[0].Type != "TXT" || recs[0].Name != "@" || recs[0].TTL != 3600 {
		t.Errorf("record = %+v", recs[0])
	}
}

func TestResolve_MissingVariable(t *testing.T) {
	tmpl := loadFixture(t, "simple_txt.json")
	_, err := tmpl.Resolve(map[string]string{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "domain") {
		t.Errorf("error should name variable: %v", err)
	}
}

func TestResolve_NewlineInjection(t *testing.T) {
	tmpl := loadFixture(t, "simple_txt.json")
	_, err := tmpl.Resolve(map[string]string{"domain": "foo\r\nevil"})
	if err == nil {
		t.Fatal("expected error for newline injection")
	}
}

func TestResolve_NullByte(t *testing.T) {
	tmpl := loadFixture(t, "simple_txt.json")
	_, err := tmpl.Resolve(map[string]string{"domain": "foo\x00bar"})
	if err == nil {
		t.Fatal("expected error for null byte")
	}
}

func TestResolve_InvalidHostLabel(t *testing.T) {
	data := []byte(`{"providerId":"x","records":[{"type":"CNAME","host":"%h%","pointsTo":"target.example.com","ttl":60}]}`)
	tmpl, err := LoadTemplateJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	cases := []string{
		strings.Repeat("a", 64), // label too long
		"-leading",
		".dot",
		"bad host",
	}
	for _, c := range cases {
		if _, err := tmpl.Resolve(map[string]string{"h": c}); err == nil {
			t.Errorf("expected error for host %q", c)
		}
	}
}

func TestResolve_InvalidPointsTo(t *testing.T) {
	data := []byte(`{"providerId":"x","records":[{"type":"CNAME","host":"www","pointsTo":"%t%","ttl":60}]}`)
	tmpl, err := LoadTemplateJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	cases := []string{
		"https://example.com",
		"example.com/path",
		"example.com:8080",
		"example .com",
	}
	for _, c := range cases {
		if _, err := tmpl.Resolve(map[string]string{"t": c}); err == nil {
			t.Errorf("expected error for pointsTo %q", c)
		}
	}
}

func TestResolve_EmptyVariableAllowed(t *testing.T) {
	// Per D-07, empty string is allowed.
	data := []byte(`{"providerId":"x","records":[{"type":"TXT","host":"@","data":"prefix-%v%-suffix","ttl":60}]}`)
	tmpl, err := LoadTemplateJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	recs, err := tmpl.Resolve(map[string]string{"v": ""})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if recs[0].Content != "prefix--suffix" {
		t.Errorf("Content = %q", recs[0].Content)
	}
}

func TestResolve_MultiRecord(t *testing.T) {
	tmpl := loadFixture(t, "multi_record.json")
	recs, err := tmpl.Resolve(map[string]string{
		"target": "spf.example.com",
		"ip":     "192.0.2.1",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(recs) != 3 {
		t.Fatalf("len = %d", len(recs))
	}
	if recs[0].Type != "TXT" || recs[0].Content != "v=spf1 include:spf.example.com ~all" {
		t.Errorf("record 0: %+v", recs[0])
	}
	if recs[1].Type != "CNAME" || recs[1].Content != "spf.example.com" {
		t.Errorf("record 1: %+v", recs[1])
	}
	if recs[2].Type != "A" || recs[2].Content != "192.0.2.1" {
		t.Errorf("record 2: %+v", recs[2])
	}
}

func TestResolve_MXSRV(t *testing.T) {
	tmpl := loadFixture(t, "mx_srv.json")
	recs, err := tmpl.Resolve(map[string]string{
		"mailHost": "mail.example.com",
		"srvHost":  "sip.example.com",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("len = %d", len(recs))
	}
	mx := recs[0]
	if mx.Type != "MX" || mx.Content != "mail.example.com" || mx.Priority != 10 {
		t.Errorf("MX: %+v", mx)
	}
	srv := recs[1]
	if srv.Type != "SRV" || srv.Content != "sip.example.com" ||
		srv.Priority != 5 || srv.Weight != 20 || srv.Port != 5060 ||
		srv.Service != "_sip" || srv.Protocol != "_tcp" {
		t.Errorf("SRV: %+v", srv)
	}
}

func TestResolve_UnknownTypeSkipped(t *testing.T) {
	data := []byte(`{"providerId":"x","records":[
		{"type":"FOO","host":"@","data":"x","ttl":60},
		{"type":"TXT","host":"@","data":"keep","ttl":60}
	]}`)
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	tmpl, err := LoadTemplateJSON(data, WithLogger(logger))
	if err != nil {
		t.Fatal(err)
	}
	recs, err := tmpl.Resolve(map[string]string{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(recs) != 1 || recs[0].Content != "keep" {
		t.Errorf("expected only TXT record, got %+v", recs)
	}
	if !strings.Contains(buf.String(), "FOO") {
		t.Errorf("expected warning for FOO type, got: %s", buf.String())
	}
}

func TestResolve_SubstitutionScopedToVariableFields(t *testing.T) {
	// Vars in Type/Essential/etc must be ignored. We put a literal %x% in
	// essential and ensure no substitution happens (no error from missing var).
	data := []byte(`{"providerId":"x","records":[{"type":"TXT","host":"@","data":"ok","essential":"%x%","ttl":60}]}`)
	tmpl, err := LoadTemplateJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmpl.Resolve(map[string]string{}); err != nil {
		t.Errorf("substitution should not touch Essential, got: %v", err)
	}
}

func TestResolve_ValidationIsPostSubstitution(t *testing.T) {
	// Template literal "%foo%" in data is NOT pre-validated; only the
	// substituted value is.
	data := []byte(`{"providerId":"x","records":[{"type":"TXT","host":"@","data":"%foo%","ttl":60}]}`)
	tmpl, err := LoadTemplateJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	recs, err := tmpl.Resolve(map[string]string{"foo": "clean value"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if recs[0].Content != "clean value" {
		t.Errorf("Content = %q", recs[0].Content)
	}
}

func TestResolve_ConflictPrefixFixture(t *testing.T) {
	tmpl := loadFixture(t, "conflict_prefix.json")
	recs, err := tmpl.Resolve(map[string]string{"spf": "spf.example.com"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(recs) != 1 || recs[0].Content != "v=spf1 include:spf.example.com ~all" {
		t.Errorf("got %+v", recs)
	}
}
