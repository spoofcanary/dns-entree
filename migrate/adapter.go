package migrate

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// ZoneInfo describes a zone on a target provider after EnsureZone.
type ZoneInfo struct {
	ZoneID      string
	Nameservers []string
	Created     bool
}

// ProviderOpts carries provider-specific configuration plus a test endpoint
// override. Credentials are passed via the dedicated fields below.
type ProviderOpts struct {
	// Credential fields (each adapter uses what it needs).
	APIToken  string // Cloudflare
	APIKey    string // GoDaddy
	APISecret string // GoDaddy
	AccessKey string // Route53
	SecretKey string // Route53
	Region    string // Route53 (default us-east-1)
	Token     string // GCDNS bearer
	ProjectID string // GCDNS / Cloudflare account
	AccountID string // Cloudflare account ID

	// Endpoint override for tests. Must be http(s).
	Endpoint string

	// HTTPClient override for tests.
	HTTPClient *http.Client
}

// Adapter is the per-provider zone-create contract used by the migrate
// orchestrator. It is intentionally separate from entree.Provider, which is
// the record-level interface (D-13/D-15).
type Adapter interface {
	EnsureZone(ctx context.Context, domain string, opts ProviderOpts) (ZoneInfo, error)
}

var registry = map[string]Adapter{}

// RegisterAdapter installs an adapter under a slug. Used by init() blocks in
// the per-provider files.
func RegisterAdapter(slug string, a Adapter) {
	registry[slug] = a
}

// GetAdapter returns the adapter for a provider slug.
func GetAdapter(slug string) (Adapter, error) {
	a, ok := registry[slug]
	if !ok {
		return nil, fmt.Errorf("migrate: no adapter for provider %q", slug)
	}
	return a, nil
}

// validateDomain rejects obviously malformed input. Adapters call this before
// touching the network.
func validateDomain(domain string) error {
	if domain == "" {
		return fmt.Errorf("migrate: domain required")
	}
	if strings.ContainsAny(domain, " \t\r\n/?#") {
		return fmt.Errorf("migrate: domain contains invalid characters")
	}
	if strings.Contains(domain, "..") || strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return fmt.Errorf("migrate: malformed domain %q", domain)
	}
	if !strings.Contains(domain, ".") {
		return fmt.Errorf("migrate: domain %q has no TLD", domain)
	}
	return nil
}

// validateEndpoint enforces http(s) only on the test override (T-05b-06).
func validateEndpoint(endpoint string) error {
	if endpoint == "" {
		return nil
	}
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		return fmt.Errorf("migrate: endpoint must be http(s)")
	}
	return nil
}
