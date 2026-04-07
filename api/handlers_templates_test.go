package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureTemplate = `{
  "providerId": "prov.example",
  "providerName": "Prov",
  "serviceId": "svc1",
  "serviceName": "Service One",
  "version": 1,
  "records": [
    {"type": "TXT", "host": "@", "data": "verify=%token%", "ttl": 3600}
  ]
}`

func writeFixtureTemplate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	provDir := filepath.Join(dir, "prov.example")
	if err := os.MkdirAll(provDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(provDir, "prov.example.svc1.json")
	if err := os.WriteFile(path, []byte(fixtureTemplate), 0o600); err != nil {
		t.Fatal(err)
	}
	// Sentinel file to mark cache as fresh (negative TTL disables refresh anyway).
	sentinel := filepath.Join(dir, ".dns-entree-synced")
	_ = os.WriteFile(sentinel, nil, 0o600)
	return dir
}

func TestHandleTemplatesList(t *testing.T) {
	dir := writeFixtureTemplate(t)
	s := NewServer(Options{TemplateCacheDir: dir})
	req := httptest.NewRequest("GET", "/v1/templates", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "svc1") {
		t.Fatalf("missing svc1: %s", rec.Body.String())
	}
}

func TestHandleTemplatesList_EmptyCache(t *testing.T) {
	dir := t.TempDir()
	s := NewServer(Options{TemplateCacheDir: dir})
	req := httptest.NewRequest("GET", "/v1/templates", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleTemplateGet_Found(t *testing.T) {
	dir := writeFixtureTemplate(t)
	s := NewServer(Options{TemplateCacheDir: dir})
	req := httptest.NewRequest("GET", "/v1/templates/prov.example/svc1", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Service One") {
		t.Fatalf("missing Service One: %s", rec.Body.String())
	}
}

func TestHandleTemplateGet_NotFound(t *testing.T) {
	dir := writeFixtureTemplate(t)
	s := NewServer(Options{TemplateCacheDir: dir})
	req := httptest.NewRequest("GET", "/v1/templates/prov.example/missing", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), CodeTemplateNotFound) {
		t.Fatalf("missing code: %s", rec.Body.String())
	}
}

func TestHandleTemplateResolve(t *testing.T) {
	dir := writeFixtureTemplate(t)
	s := NewServer(Options{TemplateCacheDir: dir})
	body, _ := json.Marshal(map[string]any{"vars": map[string]string{"token": "abc123"}})
	req := httptest.NewRequest("POST", "/v1/templates/prov.example/svc1/resolve", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "verify=abc123") {
		t.Fatalf("missing resolved value: %s", rec.Body.String())
	}
}

func TestHandleTemplateResolve_MissingVar(t *testing.T) {
	dir := writeFixtureTemplate(t)
	s := NewServer(Options{TemplateCacheDir: dir})
	body, _ := json.Marshal(map[string]any{"vars": map[string]string{}})
	req := httptest.NewRequest("POST", "/v1/templates/prov.example/svc1/resolve", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
