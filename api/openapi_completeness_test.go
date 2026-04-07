package api

import (
	"strings"
	"testing"
)

// TestOpenAPISpecCompleteness asserts that every D-09 route is present in
// api/openapi.yaml. We deliberately avoid pulling in a YAML parser and just
// scan for the path keys at column 2.
func TestOpenAPISpecCompleteness(t *testing.T) {
	spec := string(OpenAPISpec())
	if spec == "" {
		t.Fatal("openapi spec is empty")
	}
	for _, k := range []string{"openapi:", "info:", "paths:", "components:"} {
		if !strings.Contains(spec, k) {
			t.Errorf("missing top-level key %q", k)
		}
	}
	if !strings.Contains(spec, "3.1.0") {
		t.Error("openapi version 3.1.0 not declared")
	}

	routes := []string{
		"/healthz:",
		"/readyz:",
		"/metrics:",
		"/v1/openapi.yaml:",
		"/v1/detect:",
		"/v1/verify:",
		"/v1/spf-merge:",
		"/v1/apply:",
		"/v1/apply/template:",
		"/v1/dc/discover:",
		"/v1/dc/apply-url:",
		"/v1/templates/sync:",
		"/v1/templates:",
		"/v1/templates/{provider}/{service}:",
		"/v1/templates/{provider}/{service}/resolve:",
		"/v1/migrate:",
		"/v1/zone/export:",
		"/v1/zone/import:",
	}
	for _, r := range routes {
		if !strings.Contains(spec, r) {
			t.Errorf("openapi spec missing route %s", r)
		}
	}

	codes := []string{
		"MISSING_CREDENTIALS",
		"BAD_REQUEST",
		"PROVIDER_ERROR",
		"VERIFY_MISMATCH",
		"TEMPLATE_NOT_FOUND",
		"RATE_LIMIT_EXCEEDED",
		"INTERNAL",
	}
	for _, c := range codes {
		if !strings.Contains(spec, c) {
			t.Errorf("openapi spec missing error code %s", c)
		}
	}

	for _, s := range []string{"X-Entree-Cloudflare-Token", "X-Entree-AWS-Access-Key-Id", "X-Entree-GoDaddy-Key", "X-Entree-GCDNS-Service-Account-JSON"} {
		if !strings.Contains(spec, s) {
			t.Errorf("openapi spec missing security header %s", s)
		}
	}
}
