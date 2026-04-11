package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestEnvelopeDriftSuccess asserts the api success envelope is byte-identical
// to the CLI formatter shape (cmd/entree/output.go). The api package cannot
// import package main, so we re-derive the shape from the same struct fields
// and ordering and compare against a hand-written canonical form. Any silent
// drift in field name or order trips this test.
func TestEnvelopeDriftSuccess(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, map[string]any{"hello": "world"})
	got := strings.TrimSpace(rec.Body.String())
	want := `{"ok":true,"schema_version":1,"data":{"hello":"world"}}`
	if got != want {
		t.Fatalf("envelope drift:\n got=%s\nwant=%s", got, want)
	}
	// Sanity: still valid JSON, schema_version is integer 1.
	var v map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	if sv, _ := v["schema_version"].(float64); sv != 1 {
		t.Fatalf("schema_version=%v", v["schema_version"])
	}
}

func TestEnvelopeDriftError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, 400, CodeBadRequest, "boom", map[string]any{"hint": "fix it"})
	got := strings.TrimSpace(rec.Body.String())
	want := `{"ok":false,"schema_version":1,"error":{"code":"BAD_REQUEST","message":"boom","details":{"hint":"fix it"}}}`
	if got != want {
		t.Fatalf("error envelope drift:\n got=%s\nwant=%s", got, want)
	}
}

func TestErrorDetailsScrubbedOfSensitiveKeys(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, 400, CodeMissingCredentials, "nope", map[string]any{
		"token":    "tok-leaked",
		"secret":   "sek-leaked",
		"password": "pw-leaked",
		"value":    "v-leaked",
		"key":      "k-leaked",
		"safe":     "ok",
	})
	body := rec.Body.Bytes()
	for _, leak := range []string{"tok-leaked", "sek-leaked", "pw-leaked", "v-leaked", "k-leaked"} {
		if bytes.Contains(body, []byte(leak)) {
			t.Fatalf("scrubber missed %q: %s", leak, body)
		}
	}
	if !bytes.Contains(body, []byte(`"safe":"ok"`)) {
		t.Fatalf("scrubber removed non-sensitive key: %s", body)
	}
}
