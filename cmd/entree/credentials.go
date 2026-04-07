package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	entree "github.com/spoofcanary/dns-entree"
)

// credentialsFile mirrors the on-disk JSON schema (D-04).
type credentialsFile struct {
	Cloudflare struct {
		APIToken string `json:"api_token"`
	} `json:"cloudflare"`
	Route53 struct {
		AccessKeyID     string `json:"access_key_id"`
		SecretAccessKey string `json:"secret_access_key"`
		Region          string `json:"region"`
	} `json:"route53"`
	GoDaddy struct {
		APIKey    string `json:"api_key"`
		APISecret string `json:"api_secret"`
	} `json:"godaddy"`
	GoogleCloudDNS struct {
		ServiceAccountJSONPath string `json:"service_account_json_path"`
		ProjectID              string `json:"project_id"`
	} `json:"google_cloud_dns"`
}

// CredentialsLoader resolves credentials per D-03 priority chain. All IO is
// pluggable so tests can drive it without touching real $HOME or env.
type CredentialsLoader struct {
	FlagPath      string
	EnvLookup     func(string) string
	FileReader    func(string) ([]byte, error)
	XDGConfigHome string
}

// NewCredentialsLoader builds a loader using real environment + filesystem.
func NewCredentialsLoader(flagPath string) *CredentialsLoader {
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		if home, err := os.UserHomeDir(); err == nil {
			xdg = filepath.Join(home, ".config")
		}
	}
	return &CredentialsLoader{
		FlagPath:      flagPath,
		EnvLookup:     os.Getenv,
		FileReader:    os.ReadFile,
		XDGConfigHome: xdg,
	}
}

// Load resolves credentials for the given provider slug.
func (l *CredentialsLoader) Load(provider string) (entree.Credentials, error) {
	if l.EnvLookup == nil {
		l.EnvLookup = os.Getenv
	}
	if l.FileReader == nil {
		l.FileReader = os.ReadFile
	}

	// Priority 1: --credentials-file flag.
	if l.FlagPath != "" {
		return l.loadFromFile(l.FlagPath, provider)
	}
	// Priority 2: env var pointing at file.
	if p := l.EnvLookup("DNSENTREE_CREDENTIALS_FILE"); p != "" {
		return l.loadFromFile(p, provider)
	}
	// Priority 3: XDG path.
	if l.XDGConfigHome != "" {
		xdgPath := filepath.Join(l.XDGConfigHome, "dns-entree", "credentials.json")
		if data, err := l.FileReader(xdgPath); err == nil {
			return parseAndExtract(data, provider)
		}
	}
	// Priority 4: per-provider env vars.
	if creds, ok := l.loadFromEnv(provider); ok {
		return creds, nil
	}
	return entree.Credentials{}, &UserError{
		Code: "NO_CREDENTIALS",
		Msg:  fmt.Sprintf("no credentials found for provider %s (set --credentials-file or env vars)", provider),
	}
}

func (l *CredentialsLoader) loadFromFile(path, provider string) (entree.Credentials, error) {
	data, err := l.FileReader(path)
	if err != nil {
		return entree.Credentials{}, &UserError{
			Code: "CREDENTIALS_READ_FAILED",
			Msg:  fmt.Sprintf("cannot read credentials file %s", path),
		}
	}
	return parseAndExtract(data, provider)
}

func parseAndExtract(data []byte, provider string) (entree.Credentials, error) {
	var cf credentialsFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return entree.Credentials{}, &UserError{
			Code: "CREDENTIALS_PARSE_FAILED",
			Msg:  "credentials file is not valid JSON",
		}
	}
	switch provider {
	case "cloudflare":
		if cf.Cloudflare.APIToken == "" {
			return entree.Credentials{}, &UserError{Code: "NO_CREDENTIALS", Msg: "cloudflare api_token missing"}
		}
		return entree.Credentials{APIToken: cf.Cloudflare.APIToken}, nil
	case "route53":
		if cf.Route53.AccessKeyID == "" || cf.Route53.SecretAccessKey == "" {
			return entree.Credentials{}, &UserError{Code: "NO_CREDENTIALS", Msg: "route53 access_key_id/secret_access_key missing"}
		}
		return entree.Credentials{
			AccessKey: cf.Route53.AccessKeyID,
			SecretKey: cf.Route53.SecretAccessKey,
			Region:    cf.Route53.Region,
		}, nil
	case "godaddy":
		if cf.GoDaddy.APIKey == "" || cf.GoDaddy.APISecret == "" {
			return entree.Credentials{}, &UserError{Code: "NO_CREDENTIALS", Msg: "godaddy api_key/api_secret missing"}
		}
		return entree.Credentials{APIKey: cf.GoDaddy.APIKey, APISecret: cf.GoDaddy.APISecret}, nil
	case "google_cloud_dns":
		if cf.GoogleCloudDNS.ServiceAccountJSONPath == "" {
			return entree.Credentials{}, &UserError{Code: "NO_CREDENTIALS", Msg: "google_cloud_dns service_account_json_path missing"}
		}
		return entree.Credentials{
			Token:     cf.GoogleCloudDNS.ServiceAccountJSONPath,
			ProjectID: cf.GoogleCloudDNS.ProjectID,
		}, nil
	default:
		return entree.Credentials{}, &UserError{Code: "UNKNOWN_PROVIDER", Msg: fmt.Sprintf("unknown provider %q", provider)}
	}
}

func (l *CredentialsLoader) loadFromEnv(provider string) (entree.Credentials, bool) {
	switch provider {
	case "cloudflare":
		if t := l.EnvLookup("DNSENTREE_CLOUDFLARE_TOKEN"); t != "" {
			return entree.Credentials{APIToken: t}, true
		}
	case "route53":
		ak := l.EnvLookup("DNSENTREE_AWS_ACCESS_KEY_ID")
		sk := l.EnvLookup("DNSENTREE_AWS_SECRET_ACCESS_KEY")
		if ak != "" && sk != "" {
			return entree.Credentials{
				AccessKey: ak,
				SecretKey: sk,
				Region:    l.EnvLookup("DNSENTREE_AWS_REGION"),
			}, true
		}
	case "godaddy":
		k := l.EnvLookup("DNSENTREE_GODADDY_KEY")
		s := l.EnvLookup("DNSENTREE_GODADDY_SECRET")
		if k != "" && s != "" {
			return entree.Credentials{APIKey: k, APISecret: s}, true
		}
	case "google_cloud_dns":
		if p := l.EnvLookup("DNSENTREE_GCDNS_SERVICE_ACCOUNT_JSON"); p != "" {
			data, err := l.FileReader(p)
			if err == nil {
				return entree.Credentials{
					Token:     string(data),
					ProjectID: l.EnvLookup("DNSENTREE_GCDNS_PROJECT_ID"),
				}, true
			}
		}
	}
	return entree.Credentials{}, false
}

// Redact returns a credential map with non-empty secret fields replaced by "***".
// Empty fields stay empty so debug output remains honest (D-05).
func Redact(c entree.Credentials) map[string]any {
	mask := func(s string) string {
		if s == "" {
			return ""
		}
		return "***"
	}
	return map[string]any{
		"api_token":  mask(c.APIToken),
		"api_key":    mask(c.APIKey),
		"api_secret": mask(c.APISecret),
		"access_key": mask(c.AccessKey),
		"secret_key": mask(c.SecretKey),
		"region":     c.Region,
		"token":      mask(c.Token),
		"project_id": c.ProjectID,
	}
}
