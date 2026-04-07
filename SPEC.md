# dns-entree

## Open-source DNS automation. Because $249/month to push a TXT record is insane.

**Date:** 2026-04-06
**Repo:** github.com/spoofcanary/dns-entree
**License:** MIT
**Language:** Go
**Status:** Spec v2 - revised against the upstream project codebase audit

---

## What it does

A Go library and optional hosted API that lets any SaaS product push DNS records to a customer's domain. Detect the DNS provider, authenticate, push records, verify. Four steps, one library, zero vendor lock-in.

The customer types their domain. dns-entree identifies the DNS provider from NS records, presents the best connection method (Domain Connect, OAuth, API key, or guided manual), pushes the records, and confirms via authoritative nameserver query. Done.

Any Domain Connect template from the official `Domain-Connect/Templates` repo works out of the box. HubSpot's, Microsoft's, Shopify's, Zoho's, yours. No proprietary wrapper. No partnership required. No toll booth.

## Why it exists

Entri charges $249/month to do this. GoDaddy routed Domain Connect through Entri exclusively. IONOS invested EUR5M in Entri and started redirecting service providers to them. 24% of .com DNS is now paywalled behind a company that captured an open protocol.

Multiple service providers confirmed that Entri expects a custom integration using their proprietary API. You implement Domain Connect, submit your template, get it approved, and get told none of that matters. Pay Entri, use their API instead.

dns-entree is the exit. Self-host it for free. Use the hosted API for $29/month. Or fork it and build whatever you want. Your approved Domain Connect templates work directly. The protocol works the way it was designed to work.

---

## Extraction Origin

dns-entree extracts generic DNS automation from The upstream project'sproduction codebase. This is battle-tested code handling real customer domains across Cloudflare, Route53, GoDaddy, and Google Cloud DNS.

### What extracts from the upstream project (proven, production code)

- Provider interface and 4 implementations (Cloudflare, Route53, GoDaddy, Google Cloud DNS)
- NS-based provider detection with RDAP fallback (16 nameserver patterns)
- Domain Connect discovery with SSRF protection
- Domain Connect URL signing (RSA-SHA256 PKCS1v15)
- SPF merge algorithm (handles empty, broken, valid, already-merged)
- Authoritative DNS verification with recursive fallback
- Post-push verification

### What's built new for dns-entree

- Domain Connect template engine (parse, resolve, apply any JSON template)
- Template sync from official GitHub repo
- Template-to-records resolution with `txtConflictMatchingMode` handling
- CLI tool
- HTTP API (hosted mode)
- Additional provider implementations (community-driven)

### What stays in the upstream project (product-specific)

- Managed DNS zone operations (The upstream project'sown `dns.sendcanary.com` Cloudflare zone)
- DMARC/SPF/DKIM/BIMI record generation with email-authentication-specific logic
- Onboarding wizard state, enforcement gates, alignment checks
- DMARC report ingestion and sender scoring
- SPF flattening worker
- BIMI SVG validation and hosting
- RFC 7489 authorization record management
- DNS monitoring worker (4-hour scan cycle, change classification)
- DNS re-verification worker (15-minute pending check cycle)

After extraction, the upstream project imports dns-entree as a Go module dependency for all generic DNS operations. Product-specific logic stays in The upstream project'sown packages.

---

## Architecture

```
Any SaaS app (or the upstream project)
     |
     v
dns-entree (Go library or HTTP API)
     |
     +-- Detect provider (NS lookup + RDAP fallback)
     +-- Authenticate (per-provider method)
     +-- Push records (direct API or Domain Connect template)
     +-- Verify (authoritative NS query with recursive fallback)
     +-- Return result
```

Two integration modes:

**Library mode.** Import `github.com/spoofcanary/dns-entree` into your Go application. Call functions directly. No network hop, no API key, no dependency on our infrastructure.

**API mode.** POST to `api.dns-entree.dev`. Get back results. For non-Go applications or teams that don't want to manage provider credentials themselves.

---

## Provider Support

### Tier 1: Direct API (push records programmatically)

Extracted from upstream production. All four have been pushing real customer DNS records.

| Provider | Method | Auth | Market Share | Source |
|----------|--------|------|-------------|--------|
| Cloudflare | REST API v4 | API token | ~15% of DNS | Extracted |
| Route 53 | AWS SDK v2 | IAM access key + secret | ~5% | Extracted |
| GoDaddy | REST API v1 | API key + secret | ~21% of .com | Extracted |
| Google Cloud DNS | REST API v1 | OAuth2 bearer + project ID | ~3% | Extracted |

**GoDaddy note:** API key generation UX is poor (buried in developer portal). The implementation works but customer onboarding friction is high. Domain Connect or guided manual may convert better for GoDaddy domains.

### Tier 2: Domain Connect (one-click consent)

Any DNS provider that implements Domain Connect and honors templates directly:

| Provider | Status |
|----------|--------|
| Cloudflare | Live, tested in production |
| NameSilo | Live, auto-syncs from GitHub repo |
| WordPress.com | Live |
| Vercel | Live |
| Plesk | Live |
| IONOS | Redirects to Entri (bypass via Tier 1 if API available) |
| GoDaddy | Redirects to Entri (bypass via Tier 1 API key) |

### Tier 3: Guided manual

Every other provider. dns-entree returns:

- The exact records to add (resolved from template or raw records)
- Deep link to the provider's DNS management page (if URL pattern is known)
- Provider-specific instructions (if available)
- Authoritative NS verification endpoint for instant confirmation

### Provider detection

Extracted from The upstream project's`ns_detect.go`. Two-phase detection that goes beyond naive NS pattern matching.

1. **NS hostname pattern matching** - 16 patterns covering major providers
2. **RDAP registrar fallback** - for ambiguous or unknown cases

Most DNS detection tools stop at step 1. dns-entree adds RDAP because real-world NS patterns break. When Google Domains migrated to Squarespace in 2024, millions of domains kept their original `googledomains.com` nameservers but were now managed by Squarespace. Pure NS matching returns "Google Cloud DNS" for domains that should show "Squarespace." RDAP queries the actual registrar of record, catching migrations, white-label DNS, and provider changes that NS patterns miss.

```
net.LookupNS(domain) -> match NS hostnames -> known provider?
     |                                              |
     | (ambiguous or unknown)                       | yes -> return
     v                                              |
RDAP registrar lookup -> match registrar name ------+
```

Current NS patterns:

| Pattern | Provider |
|---------|----------|
| `cloudflare.com` | Cloudflare |
| `awsdns` | Route 53 |
| `domaincontrol.com`, `godaddy.com` | GoDaddy |
| `googledomains.com` | Google Cloud DNS (needs RDAP disambiguation) |
| `squarespacedns.com` | Squarespace |
| `registrar-servers.com` | Namecheap |
| `hover.com` | Hover |
| `digitalocean.com` | DigitalOcean |
| `hetzner.com` | Hetzner |
| `ns1.net` | NS1 |
| `linode.com` | Linode |
| `vultr.com` | Vultr |
| `dnsimple.com` | DNSimple |
| `ui-dns.com`, `ui-dns.de` | IONOS |
| `namesilo.com` | NameSilo |
| `porkbun.com` | Porkbun |

Adding a detection pattern is a one-line PR.

---

## Library API

### Quick start: Idempotent push

This is the primary API. Every SaaS developer who needs to push DNS records to a customer's domain reinvents this badly. dns-entree does it correctly: read existing records, merge if needed, write only if changed, verify via authoritative nameservers.

```go
import entree "github.com/spoofcanary/dns-entree"

// 1. Detect the customer's DNS provider
provider, _ := entree.DetectProvider(ctx, "jakesplumbing.com")
// provider.Name = "godaddy", provider.Label = "GoDaddy"

// 2. Connect with credentials
godaddy, _ := entree.NewProvider("godaddy", entree.Credentials{
    APIKey: "...", APISecret: "...",
})

// 3. Push records idiomatically
push := entree.NewPushService(godaddy)

// TXT record: creates if missing, updates if stale, skips if already correct
result, _ := push.PushTXTRecord(ctx, "jakesplumbing.com", "_dmarc",
    "v=DMARC1; p=reject; rua=mailto:dmarc@example.com")
// result.Status = "created" | "updated" | "already_configured" | "failed"
// result.Verified = true (instant authoritative NS check, no 24h wait)
// result.PreviousValue = "" (was missing)

// SPF record: reads existing SPF, merges your include, writes merged result.
// Never removes existing includes. Handles broken/missing/valid SPF.
result, _ = push.PushSPFRecord(ctx, "jakesplumbing.com",
    []string{"include:_spf.google.com", "include:sendgrid.net"})
// Reads: "v=spf1 include:_spf.google.com ~all"
// Merges: "v=spf1 include:_spf.google.com include:sendgrid.net ~all"
// Writes only because sendgrid.net was missing. Verifies after write.

// CNAME record: same check-before-write pattern
result, _ = push.PushCNAMERecord(ctx, "jakesplumbing.com",
    "k1._domainkey", "dkim.mailchimp.com")
```

The `PushService` handles what every SaaS gets wrong:

| Problem | How PushService handles it |
|---------|---------------------------|
| Overwrites customer's existing SPF | Reads first, merges includes, preserves existing mechanisms |
| Creates duplicate records | Checks existing before writing, skips if already correct |
| "Wait 24-48 hours" after push | Verifies via authoritative NS immediately (bypasses cache) |
| Silent failures | Returns status + previous value + verification result |
| SPF with 10+ lookups | Warns but still merges (caller decides policy) |
| Broken/unparseable existing SPF | Creates clean record, sets `BrokenInput=true` flag |

**SPF is the only record type that requires read-before-write.** TXT and CNAME records are simple create-or-replace. SPF must read the existing record, parse it, merge the new include without removing existing includes, and write the merged result. The PushService encapsulates this entire read-merge-write flow.

### Types

Extracted from The upstream project'sexisting `Record` struct. Field names preserved from production code to minimize extraction churn.

```go
type Record struct {
    ID      string // Provider-assigned ID (empty on create)
    Type    string // TXT, CNAME, A, AAAA, MX, NS, SRV
    Name    string // Full record name: "_dmarc.example.com", "@" for apex
    Content string // Record value
    TTL     int    // Seconds. 0 = provider default.
}

type Zone struct {
    ID     string
    Name   string
    Status string
}

type ProviderType string

const (
    ProviderCloudflare    ProviderType = "cloudflare"
    ProviderRoute53       ProviderType = "route53"
    ProviderGoDaddy       ProviderType = "godaddy"
    ProviderGoogleCloudDNS ProviderType = "google_cloud_dns"
)
```

### Provider interface

Extracted from The upstream project's4-method interface, extended with metadata and batch operations. This is plumbing -- most users should use `PushService` instead of calling provider methods directly.

```go
type Provider interface {
    // Metadata
    Name() string         // Human-readable: "Cloudflare"
    Slug() string         // Machine identifier: "cloudflare"

    // Connection
    Verify(ctx context.Context) ([]Zone, error)

    // Record operations
    GetRecords(ctx context.Context, domain, recordType string) ([]Record, error)
    SetRecord(ctx context.Context, domain string, record Record) error
    DeleteRecord(ctx context.Context, domain, recordID string) error

    // Batch operations (default implementation calls SetRecord in loop)
    ApplyRecords(ctx context.Context, domain string, records []Record) error
}
```

**Design decision:** No `Connect() -> *Connection` session model for library mode. Providers are constructed directly with credentials. The session/connection abstraction only exists in the hosted API (where credential lifetime matters). Construct a provider, call methods, done. How Go developers expect a library to work.

### Detection and verification

```go
// Detect which DNS provider hosts a domain
// Two-phase: NS pattern matching + RDAP registrar fallback
result, err := entree.DetectProvider(ctx, "jakesplumbing.com")
// result.Provider = "godaddy"
// result.ProviderLabel = "GoDaddy"
// result.Supported = true
// result.Nameservers = ["ns77.domaincontrol.com", "ns78.domaincontrol.com"]

// Verify via authoritative nameservers (instant, no cache wait)
result, err := entree.Verify(ctx, "jakesplumbing.com", entree.VerifyOpts{
    RecordType: "TXT",
    Name:       "_dmarc",
    Contains:   "v=DMARC1",
})
// result.Verified = true
// result.CurrentValue = "v=DMARC1; p=reject; ..."
// result.Method = "authoritative"          // or "recursive_fallback"
// result.NameserversQueried = ["ns77.domaincontrol.com"]
```

### PushResult

Every push operation returns a `PushResult`:

```go
type PushResult struct {
    Status        string // "created", "updated", "already_configured", "failed"
    RecordName    string // What was pushed: "_dmarc.jakesplumbing.com"
    RecordValue   string // The value that was written
    PreviousValue string // What was there before (empty if new)
    Verified      bool   // Post-push authoritative NS check passed
    VerifyError   error  // Non-nil if verification failed (record still pushed)
}
```

### SPF merge

Extracted from The upstream project's`mergeSPFIncludes`. Generalized to accept any include domain.

```go
merged, err := entree.MergeSPF(currentSPF, "include:_spf.yoursaas.com")
// merged.Value = "v=spf1 include:_spf.google.com include:_spf.yoursaas.com ~all"
// merged.Changed = true
// merged.BrokenInput = false
```

| Input | Output |
|-------|--------|
| No existing SPF | `v=spf1 include:{new} ~all` |
| Valid SPF without the include | Inserts before terminator, preserves existing |
| Already has the include | Returns unchanged, `Changed=false` |
| Broken/unparseable SPF | Fresh record, `BrokenInput=true`, warning |
| SPF with `redirect=` | Handles correctly |
| SPF at 10 lookup limit | Warns but still merges (caller decides) |

### Authoritative DNS verification

Extracted from The upstream project's`AuthoritativeDNSVerify`. Production-proven across thousands of domains.

```go
result, err := entree.Verify(ctx, "jakesplumbing.com", entree.VerifyOpts{
    RecordType: "TXT",
    Name:       "_dmarc",
    Contains:   "v=DMARC1",    // Substring match (case-insensitive)
})
```

**How it works:**

1. `net.LookupNS(domain)` to get authoritative nameservers
2. Query up to 2 NS IPs directly (custom resolver, UDP port 53)
3. 5-second timeout per NS query
4. Return on first match
5. Fallback to recursive resolution if authoritative fails (handles CNAME chains)

Callers get instant verification instead of telling users to "wait 24-48 hours for DNS propagation."

---

## Domain Connect

### Discovery

Extracted from The upstream project's`internal/domainconnect/discovery.go`. Includes SSRF protection.

```go
dc, err := entree.DiscoverDomainConnect(ctx, "jakesplumbing.com")
// dc.Supported = true
// dc.ProviderID = "cloudflare.com"
// dc.ProviderName = "Cloudflare"
// dc.SyncURL = "https://dash.cloudflare.com/cdn-cgi/access/domain-connect/..."
// dc.APIURL = "https://api.cloudflare.com/..."
```

**Discovery flow:**

1. DNS lookup: `_domainconnect.{domain}` TXT record
2. SSRF validation of returned hostname (reject loopback, private, link-local IPs)
3. HTTP GET `https://{hostname}/v2/{domain}/settings` (5-second timeout)
4. Parse provider settings JSON
5. Return result with `Supported=false` on any error (graceful degradation, never fails hard)

**Options for testing:**

```go
dc, err := entree.DiscoverDomainConnect(ctx, domain,
    entree.WithResolver(mockResolver),
    entree.WithHTTPClient(testClient),
)
```

### URL signing

Extracted from The upstream project's`internal/domainconnect/signing.go`.

```go
// Load RSA private key (PKCS8 or PKCS1 PEM)
key, err := entree.LoadDCPrivateKey(pemData)

// Sign a query string (RSA-SHA256 PKCS1v15, base64 output)
sig, err := entree.SignDCQueryString(queryString, key)
```

### Apply URL generation

Builds signed Domain Connect async apply URLs for any template.

```go
tmpl, _ := entree.LoadTemplate("microsoft.com", "o365")

url, err := entree.BuildDCApplyURL(ctx, entree.DCApplyOpts{
    SyncURL:     dc.SyncURL,
    Domain:      "jakesplumbing.com",
    Template:    tmpl,
    Params:      map[string]string{"subdomain": "mail"},
    PrivateKey:  key,
    KeyID:       "my-key-id",
    RedirectURI: "https://myapp.com/callback",
})
// Redirect user to url. They approve on provider's UI. Records applied.
```

---

## Domain Connect Template Engine

**Status: Net-new development.** the upstream project does not parse templates at runtime - it hardcodes template paths as URL query parameters. The template engine is the primary new capability dns-entree adds.

### What the engine does

The official Domain Connect template repository (`github.com/Domain-Connect/Templates`) contains 127+ templates from service providers: Microsoft O365, HubSpot, Shopify, Zoho, Mailjet, Squarespace, WordPress, and dozens more. Each template is a JSON file describing what DNS records a service needs.

dns-entree parses any template from that repo and can:

1. **Resolve** templates to concrete records (substitute variables, compute names)
2. **Apply** resolved records via any Tier 1 provider API (bypassing Domain Connect consent flow entirely)
3. **Generate** signed Domain Connect apply URLs (for Tier 2 providers that support DC natively)

This is what Entri paywalled. dns-entree makes it free.

### Template operations

```go
// Sync all templates from the official repo
err := entree.SyncTemplates(ctx, entree.TemplateOpts{
    RepoURL:  "https://github.com/Domain-Connect/Templates",
    CacheDir: "~/.entree/templates",
})

// Load a template by provider and service ID
tmpl, err := entree.LoadTemplate("hubspot.com", "hubspot-cname-txt")

// Load from a local file (your own templates, not yet in the repo)
tmpl, err := entree.LoadTemplateFile("./my-service.json")

// Load from raw JSON bytes
tmpl, err := entree.LoadTemplateJSON(data)

// Inspect what records a template will create
records, err := tmpl.Resolve(map[string]string{
    "subdomain": "blog",
    "ip":        "203.0.113.42",
})
// Returns []Record with all %variables% substituted
```

### Applying templates via provider API

Two paths depending on whether the DNS provider supports Domain Connect directly:

```go
// Path 1: Provider supports Domain Connect natively
// Generate a signed consent URL. User approves. Provider applies records.
dc, _ := entree.DiscoverDomainConnect(ctx, "jakesplumbing.com")
if dc.Supported {
    url, _ := entree.BuildDCApplyURL(ctx, entree.DCApplyOpts{
        SyncURL:  dc.SyncURL,
        Domain:   "jakesplumbing.com",
        Template: tmpl,
        Params:   map[string]string{"token": "abc123"},
        ...
    })
    // Redirect user to url. They approve. Done.
}

// Path 2: Provider doesn't support DC (or routes through Entri)
// Resolve template to records, push via provider API directly.
provider, _ := entree.NewProvider("godaddy", entree.Credentials{...})
err := entree.ApplyTemplate(ctx, provider, "jakesplumbing.com", tmpl,
    map[string]string{"token": "abc123"})
// Template resolved to records, pushed via GoDaddy API. Entri bypassed.
```

### Template engine internals

```go
type Template struct {
    ProviderID         string            // "hubspot.com"
    ProviderName       string            // "HubSpot"
    ServiceID          string            // "hubspot-cname-txt"
    ServiceName        string            // "HubSpot CNAME and TXT"
    Version            int
    Records            []TemplateRecord
    Variables          []string          // Computed from records
    SyncPubKeyDomain   string
    SyncRedirectDomain string
}

type TemplateRecord struct {
    Type                      string // TXT, CNAME, A, AAAA, MX, NS, SRV
    Host                      string // May contain %variables%
    PointsTo                  string // May contain %variables% (CNAME, MX, NS, SRV)
    Data                      string // May contain %variables% (TXT)
    TTL                       int
    Essential                 string // "OnApply", "Always", or ""
    TxtConflictMatchingMode   string // "Prefix", "None", or ""
    TxtConflictMatchingPrefix string
}
```

| Feature | Handled |
|---------|---------|
| `%variable%` substitution in Host, PointsTo, Data | Yes |
| `txtConflictMatchingMode: Prefix` (replace existing matching prefix) | Yes |
| `txtConflictMatchingMode: None` (add without conflict check) | Yes |
| `essential: OnApply` (required for apply) | Yes |
| `essential: Always` (required always) | Yes |
| All record types (TXT, CNAME, A, AAAA, MX, NS, SRV) | Yes |
| `syncPubKeyDomain` / `syncRedirectDomain` | Yes |
| Group records by host for single-record-per-host providers | Yes |
| TTL from template or provider default | Yes |

### Template as app store

| Scenario | With Entri | With dns-entree |
|----------|-----------|-----------------|
| SaaS wants GoDaddy customers to auto-configure DNS | Pay Entri $249/month | `ApplyTemplate(ctx, provider, domain, tmpl, params)` |
| SaaS has an approved DC template | Template ignored, Entri requires their API | Template works directly on Cloudflare, NameSilo, Vercel, Plesk |
| SaaS wants to support a new DNS provider | Wait for Entri to add it | Implement the Provider interface, submit a PR |
| SaaS wants to support a new service template | Irrelevant, Entri uses custom integration | Sync from official repo. Automatic. |

---

## Provider Interface Details

### Constructor pattern

```go
// Generic constructor (dispatches to provider-specific)
provider, err := entree.NewProvider(slug string, creds entree.Credentials) (Provider, error)

// Or construct directly
provider, err := entree.NewCloudflareProvider(apiToken string)
provider := entree.NewGoDaddyProvider(apiKey, apiSecret string)
provider, err := entree.NewRoute53Provider(accessKey, secretKey, region string)
provider := entree.NewGoogleCloudDNSProvider(accessToken, projectID string)
```

### Credentials

```go
type Credentials struct {
    APIKey     string            // Cloudflare token, GoDaddy key
    APISecret  string            // GoDaddy secret, Route53 secret key
    OAuthToken string            // Google Cloud DNS bearer token
    Region     string            // Route53 region (optional, defaults to us-east-1)
    ProjectID  string            // Google Cloud DNS project ID
    Extra      map[string]string // Future provider-specific fields
}
```

### Provider-specific behaviors

**Cloudflare:**
- Auth: `Authorization: Bearer {apiToken}`
- Zone lookup: `ZoneIDByName` API call
- TTL: defaults to 1 (Cloudflare auto)
- Upsert: checks existing, updates or creates

**Route 53:**
- Auth: AWS IAM (access key + secret key + region)
- Zone lookup: longest suffix match across hosted zones
- Record ID format: `name|type` (pipe-delimited)
- TXT values auto-wrapped in quotes
- TTL: defaults to 300

**GoDaddy:**
- Auth: `sso-key {apiKey}:{apiSecret}` header
- Domain normalization: `@` for apex, strips trailing `.domain`
- TTL minimum: 600 seconds (GoDaddy enforced)
- Record ID format: `name|type`

**Google Cloud DNS:**
- Auth: OAuth2 bearer token + GCP project ID
- Zone lookup: longest suffix match
- Names: trailing `.` appended for API
- TXT values: auto-quoted
- Has `DiscoverGCPProject(ctx, accessToken, domain)` for finding the right project
- Upsert requires delete-then-create (API constraint)

### Adding a new provider

Implement the `Provider` interface. One file per provider. Register in the provider registry.

```go
// providers/digitalocean.go
type DigitalOceanProvider struct { ... }

func (p *DigitalOceanProvider) Name() string { return "DigitalOcean" }
func (p *DigitalOceanProvider) Slug() string { return "digitalocean" }
func (p *DigitalOceanProvider) Verify(ctx context.Context) ([]Zone, error) { ... }
func (p *DigitalOceanProvider) GetRecords(ctx, domain, recordType) ([]Record, error) { ... }
func (p *DigitalOceanProvider) SetRecord(ctx, domain, record) error { ... }
func (p *DigitalOceanProvider) DeleteRecord(ctx, domain, recordID) error { ... }
func (p *DigitalOceanProvider) ApplyRecords(ctx, domain, records) error { ... }

func init() {
    entree.RegisterProvider("digitalocean", func(creds Credentials) (Provider, error) {
        return NewDigitalOceanProvider(creds.APIKey)
    })
}
```

---

## HTTP API (hosted mode)

For non-Go applications. Same operations, REST interface.

The hosted API adds a session layer around providers. Credentials are held in memory for the session duration, never persisted to disk.

```
POST /v1/detect
  {"domain": "jakesplumbing.com"}
  -> {"provider": "godaddy", "provider_label": "GoDaddy",
      "supported": true, "nameservers": ["ns77.domaincontrol.com"],
      "domain_connect": {"supported": false}}

POST /v1/providers
  {"provider": "godaddy", "api_key": "...", "api_secret": "..."}
  -> {"session_id": "sess_abc123", "expires_at": "...", "zones": [...]}

POST /v1/records/apply
  {"session_id": "sess_abc123", "domain": "jakesplumbing.com",
   "records": [...], "verify": true}
  -> {"applied": 3, "verified": 3, "results": [...]}

POST /v1/templates/apply
  {"session_id": "sess_abc123", "domain": "jakesplumbing.com",
   "template": "microsoft.com/o365",
   "params": {"subdomain": "mail"},
   "verify": true}
  -> {"applied": 4, "verified": 4, "template": "microsoft.com/o365"}

POST /v1/verify
  {"domain": "jakesplumbing.com", "record_type": "TXT",
   "name": "_dmarc", "contains": "v=DMARC1"}
  -> {"verified": true, "current_value": "v=DMARC1; ...",
      "method": "authoritative", "nameservers_queried": [...]}

POST /v1/spf/merge
  {"current": "v=spf1 include:_spf.google.com ~all",
   "add_include": "include:_spf.yoursaas.com"}
  -> {"merged": "v=spf1 include:_spf.google.com include:_spf.yoursaas.com ~all",
      "changed": true, "broken_input": false}

POST /v1/domain-connect/discover
  {"domain": "jakesplumbing.com"}
  -> {"supported": true, "provider_id": "cloudflare.com",
      "provider_name": "Cloudflare", "sync_url": "..."}

GET /v1/templates
  -> {"templates": [{"provider_id": "microsoft.com", "service_id": "o365", ...}, ...]}

GET /v1/templates/{provider_id}/{service_id}
  -> {"provider_id": "microsoft.com", "service_id": "o365",
     "records": [...], "variables": [...]}
```

**Auth:** API key in header. `Authorization: Bearer ent_...`

**Rate limits:** 100 requests/minute on hosted. Unlimited self-hosted.

**Session lifetime:** 15 minutes. Credentials held in memory only. Deleted on expiry or explicit close.

---

## CLI

```bash
# Detect provider
$ entree detect jakesplumbing.com
Provider: GoDaddy (godaddy)
Supported: yes (api_key + api_secret)
Domain Connect: no (routes through Entri)
Nameservers: ns77.domaincontrol.com, ns78.domaincontrol.com

# Push records directly
$ entree apply jakesplumbing.com \
    --provider godaddy \
    --api-key "..." \
    --api-secret "..." \
    --record "TXT:_dmarc:v=DMARC1; p=reject; rua=mailto:..."
Applied 1 record via GoDaddy API.
Verified via ns77.domaincontrol.com.

# Apply a Domain Connect template via provider API
$ entree apply jakesplumbing.com \
    --template microsoft.com/o365 \
    --provider godaddy \
    --api-key "..." \
    --api-secret "..."
Resolved 4 records from microsoft.com/o365
Applied via GoDaddy API
Verified 4/4 via authoritative NS

# Verify a record
$ entree verify jakesplumbing.com TXT _dmarc
Found: v=DMARC1; p=reject; rua=mailto:...
Method: authoritative (ns77.domaincontrol.com)

# Merge SPF
$ entree spf-merge "v=spf1 include:_spf.google.com ~all" "include:sendgrid.net"
v=spf1 include:_spf.google.com include:sendgrid.net ~all

# Domain Connect discovery
$ entree dc-discover jakesplumbing.com
Supported: yes
Provider: Cloudflare (cloudflare.com)
Sync URL: https://dash.cloudflare.com/...

# Template management
$ entree templates sync
Synced 127 templates from Domain-Connect/Templates

$ entree templates list
microsoft.com/o365
hubspot.com/hubspot-cname-txt
shopify.com/website
sendcanary.com/email-setup
... 123 more

$ entree templates show microsoft.com/o365
Template: Microsoft Office 365
Provider: microsoft.com
Service: o365
Records: 4 (MX, TXT, TXT, CNAME)
Variables: none

$ entree templates resolve microsoft.com/o365
TXT  @                    v=spf1 include:spf.protection.outlook.com -all
MX   @                    *.mail.protection.outlook.com (priority 0)
TXT  @                    MS=ms00000000
CNAME autodiscover        autodiscover.outlook.com

# Dry run: resolve and print what would be pushed, without pushing
$ entree apply jakesplumbing.com \
    --template microsoft.com/o365 \
    --provider godaddy \
    --api-key "..." \
    --api-secret "..." \
    --dry-run
Would apply 4 records to jakesplumbing.com via GoDaddy API:
  TXT   @            v=spf1 include:spf.protection.outlook.com -all
  MX    @            *.mail.protection.outlook.com (priority 0)
  TXT   @            MS=ms00000000
  CNAME autodiscover autodiscover.outlook.com
No changes made (dry run).
```

The `--dry-run` flag on `entree apply` resolves the template, connects to the provider, reads existing records, shows what would change, and exits without writing. Same value as `templates resolve` but with the full provider context -- you see conflicts, merges, and skips before committing.

---

## Repo Structure

```
dns-entree/
+-- entree.go                 # Top-level: NewProvider, DetectProvider, Verify, MergeSPF
+-- provider.go               # Provider interface, Record, Zone, Credentials types
+-- registry.go               # Provider registry (RegisterProvider, NewProvider)
+-- detect.go                 # NS-based provider detection + RDAP fallback
+-- verify.go                 # Authoritative NS verification + recursive fallback
+-- spf.go                    # SPF merge utility
+-- push.go                   # PushService (idempotent record operations)
+-- template.go               # Template types, Load*, Resolve
+-- template_sync.go          # Sync templates from GitHub repo
+-- template_apply.go         # ApplyTemplate via provider API
+-- providers/
|   +-- cloudflare.go         # Extracted from internal/dns/cloudflare.go
|   +-- godaddy.go            # Extracted from internal/dns/godaddy.go
|   +-- route53.go            # Extracted from internal/dns/route53.go
|   +-- googlecloud.go        # Extracted from internal/dns/google_cloud_dns.go
+-- domainconnect/
|   +-- discovery.go          # Extracted from internal/domainconnect/discovery.go
|   +-- signing.go            # Extracted from internal/domainconnect/signing.go
|   +-- apply_url.go          # Generalized from internal/domainconnect/apply_url.go
+-- api/
|   +-- server.go             # Hosted API server
|   +-- handlers.go           # HTTP handlers
|   +-- session.go            # In-memory session manager
+-- cmd/
|   +-- entree/main.go        # CLI binary
|   +-- entree-server/main.go # API server binary
+-- templates/                # Cached official templates (gitignored)
+-- go.mod
+-- go.sum
+-- LICENSE                   # MIT
+-- README.md
```

---

## Extraction Mapping

Exact source-to-target file mapping for code extraction.

| dns-entree file | Extracted from | Changes needed |
|-----------------|---------------|----------------|
| `provider.go` | `internal/dns/provider.go` | Add `Name()`, `Slug()`, `ApplyRecords()` to interface |
| `detect.go` | `internal/dns/ns_detect.go` | Remove `SupportedProviders` map (all providers are "supported" in entree context). Rename `NSDetectionResult` fields. |
| `verify.go` | `internal/dmarc/dns_verify.go` | Move package, rename `AuthoritativeDNSVerify` to `Verify`, generalize `DNSVerifyResult` to `VerifyResult` |
| `spf.go` | `internal/dns/spf_merge.go` + `push_service.go:mergeSPFIncludes` | Export `mergeSPFIncludes`, remove sendcanary-specific `MergeSPFForDomainConnect`. New public `MergeSPF(current, addInclude)` signature. |
| `push.go` | `internal/dns/push_service.go` | Remove DMARC/DKIM-specific helpers, generalize to `PushTXTRecord`, `PushCNAMERecord`, `PushSPFRecord`. Replace `dmarc.AuthoritativeDNSVerify` with local `Verify`. **Critical:** `PushSPFRecord` must extract the full read-merge-write flow -- it calls `GetRecords` to read existing SPF, pipes through `MergeSPF`, then calls `SetRecord` with the merged result. This is the only record type that reads before writing. The extraction must include `mergeSPFIncludes` (currently unexported in `push_service.go`) and the SPF terminator detection logic. |
| `providers/cloudflare.go` | `internal/dns/cloudflare.go` | Add `Name()`, `Slug()`, `ApplyRecords()` |
| `providers/route53.go` | `internal/dns/route53.go` | Add `Name()`, `Slug()`, `ApplyRecords()` |
| `providers/godaddy.go` | `internal/dns/godaddy.go` | Add `Name()`, `Slug()`, `ApplyRecords()` |
| `providers/googlecloud.go` | `internal/dns/google_cloud_dns.go` | Add `Name()`, `Slug()`, `ApplyRecords()`. Export `DiscoverGCPProject`. |
| `domainconnect/discovery.go` | `internal/domainconnect/discovery.go` | Remove hardcoded onboarded-providers map. Let caller decide what's "onboarded". |
| `domainconnect/signing.go` | `internal/domainconnect/signing.go` | Direct extraction, no changes |
| `domainconnect/apply_url.go` | `internal/domainconnect/apply_url.go` | Generalize from the upstream project-specific `BuildApplyURL`/`BuildManagedApplyURL` to template-driven `BuildDCApplyURL` |
| `template.go` | **New** | Template JSON parser, variable resolver, conflict mode handler |
| `template_sync.go` | **New** | Git clone/pull of Domain-Connect/Templates repo |
| `template_apply.go` | **New** | Resolve template + push via provider API + handle conflicts |
| `registry.go` | **New** | Provider registry for `NewProvider(slug, creds)` dispatch |
| `api/*` | **New** | HTTP API server, session manager |
| `cmd/*` | **New** | CLI and API server binaries |

---

## Development Phases

### Phase 1: Extract core library

Extract production code into new repo. All tests must pass.

- Provider interface + 4 implementations
- NS detection + RDAP
- SPF merge
- Authoritative DNS verification
- Provider registry
- `go.mod`, LICENSE, basic README

**Deliverable:** `go get github.com/spoofcanary/dns-entree` works. Can detect providers, push records, verify.

### Phase 2: Domain Connect

Extract and generalize Domain Connect code.

- Discovery with SSRF protection
- URL signing
- Generalized apply URL builder (template-driven, not hardcoded)

**Deliverable:** Can discover DC support and generate signed apply URLs for any template.

### Phase 3: Template engine

Net-new development. The primary differentiator.

- Template JSON parser
- Variable substitution
- `txtConflictMatchingMode` handling
- Template sync from GitHub
- `ApplyTemplate()` via provider API (resolve + push + verify)
- Template listing and inspection

**Deliverable:** Any template from the official DC repo resolves and applies via any Tier 1 provider.

### Phase 4: CLI

- `entree detect`
- `entree apply` (records and templates)
- `entree verify`
- `entree spf-merge`
- `entree dc-discover`
- `entree templates sync/list/show/resolve`

**Deliverable:** Full CLI for DNS automation from the terminal.

### Phase 5: Hosted API

- HTTP server with session-based provider management
- All library operations exposed as REST endpoints
- API key auth, rate limiting
- Template cache (daily sync)

**Deliverable:** Non-Go applications can use dns-entree via HTTP.

### Phase 6: initial migration

**Blocked on:** Phase 3 (template engine) must be stable first.

The upstream project'sDomain Connect flow currently hardcodes template paths as URL query parameters. After migration, it should use the template engine to resolve records. If the template engine has bugs in conflict handling or variable substitution, The upstream project'swizard breaks for real customers.

**Gate:** Run the template engine against all 127+ templates in the official `Domain-Connect/Templates` repo as a validation suite. Every template must parse, resolve with test variables, and produce valid records. This is the green light for migration.

- Replace `internal/dns/` provider code with `dns-entree` import
- Replace `internal/domainconnect/` with `dns-entree/domainconnect` import
- Move `AuthoritativeDNSVerify` usage to `dns-entree.Verify`
- Migrate wizard DC flow from hardcoded URL params to template engine resolution
- Keep managed DNS, BIMI validator, RFC 7489, monitoring workers in the upstream project
- Integration tests against real providers

**Deliverable:** the upstream project uses dns-entree as a dependency. Zero regression.

---

## Hosted API Pricing

| | Self-hosted | Hosted |
|---|---|---|
| Price | Free (MIT) | $29/month |
| Connections/month | Unlimited | 500 included |
| Overage | N/A | $0.05/connection |
| Providers | All | All |
| Templates | All (self-synced) | All (pre-synced, updated daily) |
| Support | GitHub issues | Email |
| Credential storage | Your servers | Session-only, not persisted |

Entri charges $249/month for 50 connections. dns-entree charges $29/month for 500. Self-host for $0.

---

## What this is and what it isn't

**What it is:** A library that detects DNS providers, pushes records via their APIs, executes Domain Connect templates, and verifies via authoritative nameservers. Detect, authenticate, push, verify.

**What it isn't:** A DNS hosting service. A registrar. A domain marketplace. A monitoring platform. A proprietary wrapper around an open protocol.

We're not Entri. We're the Go module that makes Entri unnecessary.

---

*dns-entree: Open-source DNS automation.*
*Your Domain Connect templates work here. No toll booth.*
