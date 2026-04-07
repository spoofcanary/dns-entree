package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestParseVarsFlagsOnly(t *testing.T) {
	got, err := ParseVarsFlags([]string{"a=1", "b=2"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if got["a"] != "1" || got["b"] != "2" || len(got) != 2 {
		t.Fatalf("got %v", got)
	}
}

func TestParseVarsFileOnly(t *testing.T) {
	got, err := ParseVarsFlags(nil, "testdata/vars_valid.json")
	if err != nil {
		t.Fatal(err)
	}
	if got["a"] != "1" || got["b"] != "2" || got["c"] != "3" {
		t.Fatalf("got %v", got)
	}
}

func TestParseVarsFlagOverridesFile(t *testing.T) {
	got, err := ParseVarsFlags([]string{"a=override"}, "testdata/vars_valid.json")
	if err != nil {
		t.Fatal(err)
	}
	if got["a"] != "override" {
		t.Fatalf("want override, got %q", got["a"])
	}
	if got["b"] != "2" {
		t.Fatalf("want file value preserved, got %q", got["b"])
	}
}

func TestParseVarsInvalidFlag(t *testing.T) {
	_, err := ParseVarsFlags([]string{"noequals"}, "")
	var ue *UserError
	if !errors.As(err, &ue) || ue.Code != "INVALID_VAR" {
		t.Fatalf("want INVALID_VAR UserError, got %v", err)
	}
}

func TestParseVarsInjectionRejected(t *testing.T) {
	for _, bad := range []string{"a=line1\nline2", "a=x\x00y", "a=x\ry"} {
		_, err := ParseVarsFlags([]string{bad}, "")
		var ue *UserError
		if !errors.As(err, &ue) || ue.Code != "INVALID_VAR_VALUE" {
			t.Fatalf("%q: want INVALID_VAR_VALUE, got %v", bad, err)
		}
	}
}

func TestParseVarsFileMalformed(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(p, []byte(`{"a": 123}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ParseVarsFlags(nil, p)
	var ue *UserError
	if !errors.As(err, &ue) || ue.Code != "INVALID_VARS_FILE" {
		t.Fatalf("want INVALID_VARS_FILE, got %v", err)
	}
}
