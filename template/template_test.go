package template

import (
	"path/filepath"
	"testing"
)

func TestLoadTemplateFile_SimpleTXT(t *testing.T) {
	tmpl, err := LoadTemplateFile(filepath.Join("testdata", "simple_txt.json"))
	if err != nil {
		t.Fatalf("LoadTemplateFile: %v", err)
	}
	if tmpl.ProviderID != "test.example" {
		t.Errorf("ProviderID = %q", tmpl.ProviderID)
	}
	if tmpl.ServiceID != "simple-txt" {
		t.Errorf("ServiceID = %q", tmpl.ServiceID)
	}
	if tmpl.Version != 1 {
		t.Errorf("Version = %d", tmpl.Version)
	}
	if len(tmpl.Records) != 1 {
		t.Fatalf("Records len = %d", len(tmpl.Records))
	}
	r := tmpl.Records[0]
	if r.Type != "TXT" || r.Host != "@" || r.Data != "verify=%domain%" || r.TTL != 3600 {
		t.Errorf("unexpected record: %+v", r)
	}
}

func TestLoadTemplateFile_MultiRecord(t *testing.T) {
	tmpl, err := LoadTemplateFile(filepath.Join("testdata", "multi_record.json"))
	if err != nil {
		t.Fatalf("LoadTemplateFile: %v", err)
	}
	if len(tmpl.Records) != 3 {
		t.Fatalf("Records len = %d", len(tmpl.Records))
	}
	if tmpl.Records[0].Type != "TXT" || tmpl.Records[1].Type != "CNAME" || tmpl.Records[2].Type != "A" {
		t.Errorf("record order: %+v", tmpl.Records)
	}
}

func TestLoadTemplateFile_MXSRV(t *testing.T) {
	tmpl, err := LoadTemplateFile(filepath.Join("testdata", "mx_srv.json"))
	if err != nil {
		t.Fatalf("LoadTemplateFile: %v", err)
	}
	if len(tmpl.Records) != 2 {
		t.Fatalf("Records len = %d", len(tmpl.Records))
	}
	mx := tmpl.Records[0]
	if mx.Type != "MX" || mx.Priority != 10 || mx.PointsTo != "%mailHost%" {
		t.Errorf("MX wrong: %+v", mx)
	}
	srv := tmpl.Records[1]
	if srv.Type != "SRV" || srv.Service != "_sip" || srv.Protocol != "_tcp" ||
		srv.Priority != 5 || srv.Weight != 20 || srv.Port != 5060 || srv.Target != "%srvHost%" {
		t.Errorf("SRV wrong: %+v", srv)
	}
}

func TestLoadTemplateFile_ConflictPrefix(t *testing.T) {
	tmpl, err := LoadTemplateFile(filepath.Join("testdata", "conflict_prefix.json"))
	if err != nil {
		t.Fatalf("LoadTemplateFile: %v", err)
	}
	r := tmpl.Records[0]
	if r.TxtConflictMatchingMode != "Prefix" || r.TxtConflictMatchingPrefix != "v=spf1" {
		t.Errorf("conflict fields wrong: %+v", r)
	}
}

func TestLoadTemplateJSON_Lenient(t *testing.T) {
	// Unknown fields must NOT error per D-05.
	data := []byte(`{"providerId":"x","unknownField":42,"records":[{"type":"TXT","host":"@","data":"hi","futureKey":"ok"}]}`)
	tmpl, err := LoadTemplateJSON(data)
	if err != nil {
		t.Fatalf("lenient parse failed: %v", err)
	}
	if tmpl.ProviderID != "x" || len(tmpl.Records) != 1 {
		t.Errorf("unexpected: %+v", tmpl)
	}
}

func TestLoadTemplateJSON_Empty(t *testing.T) {
	if _, err := LoadTemplateJSON(nil); err == nil {
		t.Error("expected error for empty input")
	}
	if _, err := LoadTemplateJSON([]byte("not json")); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadTemplateFile_NotFound(t *testing.T) {
	if _, err := LoadTemplateFile("testdata/nonexistent.json"); err == nil {
		t.Error("expected error for missing file")
	}
}
