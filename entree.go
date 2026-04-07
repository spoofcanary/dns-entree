package entree

// Record represents a DNS record. Name is canonical FQDN without trailing dot.
// Content is always unquoted (providers handle quoting internally).
type Record struct {
	ID      string // provider-specific record ID (for updates/deletes)
	Type    string // "TXT", "MX", "A", "CNAME", "AAAA", "NS", "SRV"
	Name    string // full record name, e.g. "_dmarc.example.com"
	Content string // record value, always unquoted
	TTL     int    // TTL in seconds (0 = provider default)
}

// Zone represents a DNS zone.
type Zone struct {
	ID     string // provider-specific zone ID
	Name   string // domain name, e.g. "example.com"
	Status string // "active", "pending", etc.
}

// Credentials holds authentication for any provider.
// Each provider uses the fields it needs; unused fields are ignored.
type Credentials struct {
	APIToken  string // Cloudflare API token
	APIKey    string // GoDaddy API key, or Cloudflare global key
	APISecret string // GoDaddy API secret
	AccessKey string // Route53 IAM access key
	SecretKey string // Route53 IAM secret key
	Region    string // Route53 region (default: "us-east-1")
	Token     string // Google Cloud DNS OAuth2 bearer token
	ProjectID string // Google Cloud DNS GCP project ID
}

// ProviderType identifies a DNS provider.
type ProviderType string

const (
	ProviderCloudflare     ProviderType = "cloudflare"
	ProviderRoute53        ProviderType = "route53"
	ProviderGoDaddy        ProviderType = "godaddy"
	ProviderGoogleCloudDNS ProviderType = "google_cloud_dns"
)
