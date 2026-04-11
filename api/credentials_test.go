package api

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseCredentialsCloudflareOK(t *testing.T) {
	r := httptest.NewRequest("POST", "/x", nil)
	r.Header.Set("X-Entree-Provider", "cloudflare")
	r.Header.Set("X-Entree-Cloudflare-Token", "tok-abc")
	slug, c, err := parseCredentialHeaders(r)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if slug != "cloudflare" || c.APIToken != "tok-abc" {
		t.Fatalf("slug=%s token=%s", slug, c.APIToken)
	}
}

func TestParseCredentialsMissingProvider(t *testing.T) {
	r := httptest.NewRequest("POST", "/x", nil)
	_, _, err := parseCredentialHeaders(r)
	if err == nil || err.code != CodeMissingCredentials {
		t.Fatalf("err=%+v", err)
	}
}

func TestParseCredentialsMissingHeaderNamesValueOnly(t *testing.T) {
	r := httptest.NewRequest("POST", "/x", nil)
	r.Header.Set("X-Entree-Provider", "route53")
	r.Header.Set("X-Entree-AWS-Access-Key-Id", "AKIAEXAMPLE-DO-NOT-LOG")
	// secret intentionally absent
	_, _, err := parseCredentialHeaders(r)
	if err == nil || err.code != CodeMissingCredentials {
		t.Fatalf("err=%+v", err)
	}
	// The error must NOT echo the access key value.
	for k, v := range err.details {
		if s, ok := v.(string); ok && strings.Contains(s, "AKIAEXAMPLE") {
			t.Fatalf("details[%s] leaked credential value: %v", k, v)
		}
	}
}

func TestParseCredentialsUnknownProvider(t *testing.T) {
	r := httptest.NewRequest("POST", "/x", nil)
	r.Header.Set("X-Entree-Provider", "bogus")
	_, _, err := parseCredentialHeaders(r)
	if err == nil || err.code != CodeBadRequest {
		t.Fatalf("err=%+v", err)
	}
}

func TestParseCredentialsGCDNSBadBase64(t *testing.T) {
	r := httptest.NewRequest("POST", "/x", nil)
	r.Header.Set("X-Entree-Provider", "google_cloud_dns")
	r.Header.Set("X-Entree-GCDNS-Service-Account-JSON", "!!!not-base64!!!")
	_, _, err := parseCredentialHeaders(r)
	if err == nil || err.code != CodeMissingCredentials {
		t.Fatalf("err=%+v", err)
	}
}
