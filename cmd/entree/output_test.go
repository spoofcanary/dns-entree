package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func newTestFormatter(mode Mode) (*Formatter, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	return &Formatter{Mode: mode, Out: out, Err: errBuf}, out, errBuf
}

func TestEmitOKHuman(t *testing.T) {
	f, out, _ := newTestFormatter(ModeHuman)
	if err := f.EmitOK("hello"); err != nil {
		t.Fatalf("EmitOK: %v", err)
	}
	if !strings.Contains(out.String(), "hello") {
		t.Errorf("expected human output to contain 'hello', got %q", out.String())
	}
}

func TestEmitOKJSON(t *testing.T) {
	f, out, _ := newTestFormatter(ModeJSON)
	if err := f.EmitOK(map[string]any{"k": "v"}); err != nil {
		t.Fatalf("EmitOK: %v", err)
	}
	var env map[string]any
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("invalid json: %v (%q)", err, out.String())
	}
	if env["ok"] != true {
		t.Errorf("expected ok=true, got %v", env["ok"])
	}
	if v, _ := env["schema_version"].(float64); v != 1 {
		t.Errorf("expected schema_version=1, got %v", env["schema_version"])
	}
	if _, ok := env["data"]; !ok {
		t.Errorf("missing data field")
	}
}

func TestEmitErrorJSON(t *testing.T) {
	f, out, errBuf := newTestFormatter(ModeJSON)
	if err := f.EmitError("BAD_FLAG", "msg", nil); err != nil {
		t.Fatalf("EmitError: %v", err)
	}
	if errBuf.Len() != 0 {
		t.Errorf("D-P3: error envelope must go to stdout, got stderr=%q", errBuf.String())
	}
	var env map[string]any
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if env["ok"] != false {
		t.Errorf("expected ok=false")
	}
	errObj, ok := env["error"].(map[string]any)
	if !ok {
		t.Fatalf("missing error object")
	}
	if errObj["code"] != "BAD_FLAG" || errObj["message"] != "msg" {
		t.Errorf("unexpected error payload: %+v", errObj)
	}
}

func TestEmitOKQuiet(t *testing.T) {
	f, out, _ := newTestFormatter(ModeQuiet)
	if err := f.EmitOK(map[string]any{"x": 1}); err != nil {
		t.Fatalf("EmitOK: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("quiet OK must emit nothing, got %q", out.String())
	}
}

func TestEmitErrorQuiet(t *testing.T) {
	f, out, errBuf := newTestFormatter(ModeQuiet)
	if err := f.EmitError("X", "y", nil); err != nil {
		t.Fatalf("EmitError: %v", err)
	}
	if out.Len() != 0 || errBuf.Len() != 0 {
		t.Errorf("quiet error must emit nothing, got out=%q err=%q", out.String(), errBuf.String())
	}
}

func TestModeMutuallyExclusive(t *testing.T) {
	if _, err := ParseMode(true, true); err == nil {
		t.Errorf("expected error for json+quiet")
	}
	if m, err := ParseMode(true, false); err != nil || m != ModeJSON {
		t.Errorf("json mode parse failed: %v", err)
	}
	if m, err := ParseMode(false, true); err != nil || m != ModeQuiet {
		t.Errorf("quiet mode parse failed: %v", err)
	}
	if m, err := ParseMode(false, false); err != nil || m != ModeHuman {
		t.Errorf("human mode parse failed: %v", err)
	}
}

func TestSchemaVersionConstant(t *testing.T) {
	if SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1", SchemaVersion)
	}
}
