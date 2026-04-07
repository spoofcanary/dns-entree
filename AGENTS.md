# AGENTS.md - dns-entree

Machine-readable feature surface for LLM agents. Grep-friendly headings. ASCII only.

## Project Summary

dns-entree is a Go library and CLI for DNS provider operations: idempotent record push with post-push verification, SPF merge, provider detection (NS patterns + RDAP fallback), Domain Connect v2 discovery + signing + template apply. Module path: `github.com/spoofcanary/dns-entree`. Go 1.26.1.

## Feature Matrix

| Feature              | Library | CLI    | Providers                                  |
| -------------------- | ------- | ------ | ------------------------------------------ |
| Push TXT/CNAME/SPF   | yes     | no     | cloudflare, route53, godaddy, gcdns        |
| Push A/AAAA/MX/NS/SRV| yes     | no     | cloudflare, route53, godaddy, gcdns        |
| Provider detection   | yes     | detect | 16 NS patterns + RDAP fallback             |
| SPF merge (pure)     | yes     | spf-merge | n/a (no I/O)                            |
| DNS verification     | yes     | verify | authoritative + recursive fallback         |
| Domain Connect discover | yes  | no     | any DC v2 provider                         |
| Domain Connect sign/apply URL | yes | no | RSA-SHA256 PKCS1v15                      |
| DC template sync/load/resolve/apply | yes | no | Domain-Connect/Templates git repo  |

## Library Entry Points by Task

All imports assume `import entree "github.com/spoofcanary/dns-entree"`.

### Push a TXT record

File: `push.go`

```go
svc := entree.NewPushService(provider)
res, err := svc.PushTXTRecord(ctx, "example.com", "_dmarc.example.com",
    "v=DMARC1; p=none; rua=mailto:dmarc@example.com")
```

`func NewPushService(p entree.Provider) *entree.PushService`
`func (*PushService) PushTXTRecord(ctx, domain, name, content string) (*PushResult, error)`
`func (*PushService) PushCNAMERecord(ctx, domain, name, target string) (*PushResult, error)`
`func (*PushService) PushSPFRecord(ctx, domain string, includes []string) (*PushResult, error)`
`func (*PushService) PushGenericRecord(ctx, domain string, r Record) (*PushResult, error)` (A/AAAA/MX/NS/SRV only)

`PushResult.Status` is one of `created`, `updated`, `already_configured`, `failed`.
Post-push verification runs automatically via `entree.Verify`. `PushResult.Verified`/`VerifyError` report it.

### Merge SPF (pure, no I/O)

File: `spf.go`

```go
res, _ := entree.MergeSPF("v=spf1 include:_spf.google.com ~all",
    []string{"servers.mcsv.net"})
// res.Value, res.Changed, res.LookupCount, res.LookupLimitExceeded, res.Warnings
```

`func MergeSPF(current string, includes []string) (MergeResult, error)`

### Detect provider

File: `detect.go`

```go
det, err := entree.DetectProvider(ctx, "example.com")
// det.Provider, det.Label, det.Supported, det.Nameservers, det.Method
```

`func DetectProvider(ctx, domain string) (*DetectionResult, error)` - performs NS lookup + RDAP fallback.
`func DetectFromNS(hosts []string) DetectionResult` - pure pattern match, no I/O.

### Verify a record

File: `verify.go`

```go
res, err := entree.Verify(ctx, "example.com", entree.VerifyOpts{
    RecordType: "TXT",
    Name:       "_dmarc.example.com",
    Contains:   "v=DMARC1",
})
// res.Verified, res.CurrentValue, res.Method, res.NameserversQueried
```

`func Verify(ctx, domain string, opts VerifyOpts) (VerifyResult, error)`

### Discover Domain Connect

File: `domainconnect/discovery.go`

```go
import "github.com/spoofcanary/dns-entree/domainconnect"

dc, err := domainconnect.Discover(ctx, "example.com")
// dc.Supported, dc.ProviderID, dc.ProviderName, dc.URLSyncUX, dc.URLAsyncUX, dc.URLAPI
```

`func Discover(ctx, domain string, opts ...DiscoverOption) (DiscoveryResult, error)` - SSRF-screened, never returns network errors, returns `Supported: false` on any failure.

### Sign Domain Connect query string

File: `domainconnect/signing.go`

`func LoadPrivateKey(pemData []byte) (*rsa.PrivateKey, error)`
`func SignQueryString(query string, key *rsa.PrivateKey) (sig, keyHash string, err error)`
`func SortAndSignParams(params url.Values, key *rsa.PrivateKey) (sortedQuery, sig, keyHash string, err error)`

Signatures are base64url (no padding). Spaces encoded as `%20`, not `+`.

### Build Domain Connect apply URL

File: `domainconnect/apply_url.go`

```go
url, err := domainconnect.BuildApplyURL(domainconnect.ApplyURLOpts{
    URLAsyncUX: dc.URLAsyncUX,
    ProviderID: "exampleservice.com",
    ServiceID:  "dmarc",
    Domain:     "example.com",
    Params:     map[string]string{"rua": "dmarc@example.com"},
    PrivateKey: key,
    KeyHost:    "_dck1",
})
```

`func BuildApplyURL(opts ApplyURLOpts) (string, error)`

### Load a Domain Connect template

File: `template/template.go`, `template/sync.go`

`func LoadTemplateJSON(data []byte, opts ...LoadOption) (*Template, error)`
`func LoadTemplateFile(path string, opts ...LoadOption) (*Template, error)`
`func LoadTemplate(ctx, providerID, serviceID string, opts ...SyncOption) (*Template, error)` - auto-syncs git cache on TTL miss.
`func SyncTemplates(ctx, opts ...SyncOption) error` - clone or fast-forward Domain-Connect/Templates repo.
`func ListTemplates(opts ...SyncOption) ([]TemplateSummary, error)`

### Resolve template variables

File: `template/resolve.go`

```go
records, err := tmpl.Resolve(map[string]string{
    "domain": "example.com",
    "rua":    "dmarc@example.com",
})
```

`func (*Template) Resolve(vars map[string]string) ([]entree.Record, error)`
`func (*Template) ResolveDetailed(vars map[string]string) ([]ResolvedRecord, error)` - includes TXT conflict mode metadata.

### Apply a template end-to-end

File: `template/apply.go`

```go
results, err := template.ApplyTemplate(ctx, pushSvc, "example.com", tmpl, vars)
```

`func ApplyTemplate(ctx, pushSvc *entree.PushService, domain string, tmpl *Template, vars map[string]string) ([]*entree.PushResult, error)`

Partial-failure semantics: returns one `PushResult` per template record; errors are joined via `errors.Join` but processing continues. SPFM records are auto-routed to `PushSPFRecord`. TXT conflict modes `Prefix`/`Exact`/`All` trigger pre-push deletes.

### Provider constructors

| Provider       | Package                                                | Constructor                                                    |
| -------------- | ------------------------------------------------------ | -------------------------------------------------------------- |
| Cloudflare     | `github.com/spoofcanary/dns-entree/providers/cloudflare` | `cloudflare.NewProvider(apiToken string) (*Provider, error)` |
| Route 53       | `github.com/spoofcanary/dns-entree/providers/route53`    | `route53.NewProvider(accessKey, secretKey, region string) (*Provider, error)` |
| GoDaddy        | `github.com/spoofcanary/dns-entree/providers/godaddy`    | `godaddy.NewProvider(apiKey, apiSecret string) (*Provider, error)` |
| Google Cloud DNS | `github.com/spoofcanary/dns-entree/providers/gcdns`    | `gcdns.NewProvider(accessToken, projectID string) (*Provider, error)` |

All returned types satisfy `entree.Provider`.

## CLI Commands

Binary: `entree` (from `cmd/entree/`). Install: `go install github.com/spoofcanary/dns-entree/cmd/entree@latest`.

### Global Flags

| Flag                  | Default | Purpose                                                              |
| --------------------- | ------- | -------------------------------------------------------------------- |
| `--json`              | false   | Machine-readable JSON output (mutually exclusive with `--quiet`)     |
| `--quiet`             | false   | Suppress non-error output, exit code only                            |
| `--log-level`         | warn    | `debug|info|warn|error|off` (written to stderr)                      |
| `--no-color`          | false   | Disable color in human output                                        |
| `--credentials-file`  |         | Path to credentials JSON file                                        |
| `--provider`          |         | Force provider slug (`cloudflare|route53|godaddy|google_cloud_dns`)  |
| `--yes`               | false   | Confirm destructive operations (REQUIRED under non-TTY)              |
| `--timeout`           | 30s     | Operation timeout                                                    |

Non-TTY runs MUST pass `--yes` for any mutating operation; otherwise the command refuses and exits 2.

### Command: `entree detect <domain>`

Detect the DNS hosting provider via NS pattern match + RDAP fallback.

JSON response schema (`--json`):

```json
{
  "ok": true,
  "data": {
    "provider": "cloudflare",
    "label": "Cloudflare",
    "supported": true,
    "nameservers": ["rick.ns.cloudflare.com", "rita.ns.cloudflare.com"],
    "method": "ns_pattern"
  }
}
```

Example: `entree --json detect example.com`

### Command: `entree verify <domain> <type> <name>`

Query authoritative nameservers for a record. `<type>` is `TXT|CNAME|MX|A|AAAA`.

Flags:
- `--contains <substring>` - case-insensitive substring match

JSON schema:

```json
{
  "ok": true,
  "data": {
    "verified": true,
    "current_value": "v=DMARC1; p=none",
    "method": "authoritative",
    "nameservers_queried": ["ns1.example.com", "ns2.example.com"]
  }
}
```

Example: `entree --json verify example.com TXT _dmarc.example.com --contains v=DMARC1`

### Command: `entree spf-merge <current> <include> [<include>...]`

Pure SPF merge (no network I/O). Idempotent.

JSON schema:

```json
{
  "ok": true,
  "data": {
    "value": "v=spf1 include:_spf.google.com include:servers.mcsv.net ~all",
    "changed": true,
    "broken_input": false,
    "lookup_count": 2,
    "lookup_limit_exceeded": false,
    "warnings": []
  }
}
```

Example: `entree --json spf-merge "v=spf1 include:_spf.google.com ~all" servers.mcsv.net`

### Command: `entree version`

Prints the build version. Exit 0.

### Commands Not Yet Implemented in CLI

`apply`, `dc-discover`, `templates sync|list|show|resolve` are library-only as of Phase 5a. Use the `_examples/` programs or call the library directly.

## Exit Codes

| Code | Meaning          | When                                                              |
| ---- | ---------------- | ----------------------------------------------------------------- |
| 0    | Success          | Command completed successfully                                    |
| 1    | Runtime error    | Network, provider API, DNS query, or other system-level failure   |
| 2    | User error       | Bad flags, missing credentials, invalid input, non-TTY without `--yes` |

Exit codes are stable across versions. Agents can switch on them directly.

## Credentials Env Vars

| Env Var                                  | Provider         | Purpose                                    |
| ---------------------------------------- | ---------------- | ------------------------------------------ |
| `DNSENTREE_CLOUDFLARE_TOKEN`             | cloudflare       | API token with `Zone:Read` + `DNS:Edit`    |
| `DNSENTREE_AWS_ACCESS_KEY_ID`            | route53          | IAM access key                             |
| `DNSENTREE_AWS_SECRET_ACCESS_KEY`        | route53          | IAM secret                                 |
| `DNSENTREE_AWS_REGION`                   | route53          | Region (default `us-east-1`)               |
| `DNSENTREE_GODADDY_KEY`                  | godaddy          | API key                                    |
| `DNSENTREE_GODADDY_SECRET`               | godaddy          | API secret                                 |
| `DNSENTREE_GCDNS_SERVICE_ACCOUNT_JSON`   | google_cloud_dns | Path to service account JSON file          |
| `DNSENTREE_GCDNS_PROJECT_ID`             | google_cloud_dns | GCP project ID                             |
| `DNSENTREE_CREDENTIALS_FILE`             | any              | Path to JSON credentials file (all providers) |

Credentials priority: `--credentials-file` flag > `DNSENTREE_CREDENTIALS_FILE` > `$XDG_CONFIG_HOME/dns-entree/credentials.json` > per-provider env vars.

## Stability Tiers

| Package                                  | Tier     | Breakage Policy                           |
| ---------------------------------------- | -------- | ----------------------------------------- |
| `github.com/spoofcanary/dns-entree`      | Stable   | Semver-bound from v1.0.0 forward          |
| `.../domainconnect`                      | Stable   | Semver-bound from v1.0.0 forward          |
| `.../template`                           | Stable   | Semver-bound from v1.0.0 forward          |
| `.../providers/*`                        | Stable   | Semver-bound from v1.0.0 forward          |
| `.../cmd/entree`                         | Stable   | Flag names and exit codes covered         |
| `.../internal/*`                         | Unstable | May change without notice                 |

Current version `v0.1.0-alpha`: breaking changes permitted during v0.x adoption window.
