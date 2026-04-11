package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	entree "github.com/spoofcanary/dns-entree"
)

const fixturePath = "testdata/credentials_valid.json"

func TestLoadCredentialsFlagPath(t *testing.T) {
	l := NewCredentialsLoader(fixturePath)
	c, err := l.Load("cloudflare")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.APIToken != "cf_test_token" {
		t.Errorf("APIToken = %q", c.APIToken)
	}
}

func TestLoadCredentialsEnvFilePath(t *testing.T) {
	t.Setenv("DNSENTREE_CREDENTIALS_FILE", fixturePath)
	l := NewCredentialsLoader("")
	// Force XDG empty so we don't fall through.
	l.XDGConfigHome = ""
	c, err := l.Load("route53")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.AccessKey != "AKIA" || c.SecretKey != "secret" || c.Region != "us-east-1" {
		t.Errorf("unexpected route53 creds: %+v", c)
	}
}

func TestLoadCredentialsXDGFallback(t *testing.T) {
	dir := t.TempDir()
	xdgFile := filepath.Join(dir, "dns-entree", "credentials.json")
	if err := os.MkdirAll(filepath.Dir(xdgFile), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(xdgFile, data, 0o600); err != nil {
		t.Fatal(err)
	}
	l := &CredentialsLoader{
		EnvLookup:     func(string) string { return "" },
		FileReader:    os.ReadFile,
		XDGConfigHome: dir,
	}
	c, err := l.Load("godaddy")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.APIKey != "gd_key" || c.APISecret != "gd_secret" {
		t.Errorf("unexpected godaddy creds: %+v", c)
	}
}

func TestLoadCredentialsEnvVarsOnly(t *testing.T) {
	env := map[string]string{"DNSENTREE_CLOUDFLARE_TOKEN": "envtok"}
	l := &CredentialsLoader{
		EnvLookup:  func(k string) string { return env[k] },
		FileReader: func(string) ([]byte, error) { return nil, os.ErrNotExist },
	}
	c, err := l.Load("cloudflare")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.APIToken != "envtok" {
		t.Errorf("APIToken = %q", c.APIToken)
	}
}

func TestLoadCredentialsRoute53Env(t *testing.T) {
	env := map[string]string{
		"DNSENTREE_AWS_ACCESS_KEY_ID":     "ak",
		"DNSENTREE_AWS_SECRET_ACCESS_KEY": "sk",
		"DNSENTREE_AWS_REGION":            "us-west-2",
	}
	l := &CredentialsLoader{
		EnvLookup:  func(k string) string { return env[k] },
		FileReader: func(string) ([]byte, error) { return nil, os.ErrNotExist },
	}
	c, err := l.Load("route53")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.AccessKey != "ak" || c.SecretKey != "sk" || c.Region != "us-west-2" {
		t.Errorf("unexpected: %+v", c)
	}
}

func TestLoadCredentialsGoDaddyEnv(t *testing.T) {
	env := map[string]string{
		"DNSENTREE_GODADDY_KEY":    "k",
		"DNSENTREE_GODADDY_SECRET": "s",
	}
	l := &CredentialsLoader{
		EnvLookup:  func(k string) string { return env[k] },
		FileReader: func(string) ([]byte, error) { return nil, os.ErrNotExist },
	}
	c, err := l.Load("godaddy")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.APIKey != "k" || c.APISecret != "s" {
		t.Errorf("unexpected: %+v", c)
	}
}

func TestLoadCredentialsGCDNSEnv(t *testing.T) {
	saPath := "/fake/sa.json"
	env := map[string]string{
		"DNSENTREE_GCDNS_SERVICE_ACCOUNT_JSON": saPath,
		"DNSENTREE_GCDNS_PROJECT_ID":           "proj-x",
	}
	l := &CredentialsLoader{
		EnvLookup: func(k string) string { return env[k] },
		FileReader: func(p string) ([]byte, error) {
			if p == saPath {
				return []byte(`{"type":"service_account"}`), nil
			}
			return nil, os.ErrNotExist
		},
	}
	c, err := l.Load("google_cloud_dns")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Token == "" || c.ProjectID != "proj-x" {
		t.Errorf("unexpected: %+v", c)
	}
}

func TestLoadCredentialsMissing(t *testing.T) {
	l := &CredentialsLoader{
		EnvLookup:  func(string) string { return "" },
		FileReader: func(string) ([]byte, error) { return nil, os.ErrNotExist },
	}
	_, err := l.Load("cloudflare")
	if err == nil {
		t.Fatal("expected error")
	}
	var ue *UserError
	if !errors.As(err, &ue) || ue.Code != "NO_CREDENTIALS" {
		t.Errorf("expected NO_CREDENTIALS UserError, got %v", err)
	}
}

func TestRedactCredentials(t *testing.T) {
	m := Redact(entree.Credentials{APIToken: "abc"})
	if m["api_token"] != "***" {
		t.Errorf("expected ***, got %v", m["api_token"])
	}
}

func TestRedactNeverEmpty(t *testing.T) {
	m := Redact(entree.Credentials{})
	if m["api_token"] != "" {
		t.Errorf("empty field should stay empty, got %v", m["api_token"])
	}
}
