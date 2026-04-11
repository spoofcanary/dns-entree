# Provider Detection Guide

`entree.DetectProvider` identifies which DNS host a domain is using. It is purely read-only and safe to call before asking a user for credentials.

## API

```go
type DetectionResult struct {
    Provider    ProviderType // slug, e.g. "cloudflare"
    Label       string       // human name, e.g. "Cloudflare"
    Supported   bool         // true if dns-entree has a direct API integration
    Nameservers []string
    Method      string       // "ns_pattern" | "rdap_fallback"
}

result, err := entree.DetectProvider(ctx, "example.com")
```

Pure version, for when you already have NS hostnames:

```go
result := entree.DetectFromNS([]string{"ns1.example-dns.com", "ns2.example-dns.com"})
```

## How Detection Works

### Step 1: NS Pattern Match

`DetectFromNS` iterates the resolved NS hostnames and returns the first match against a built-in table of 16 substrings:

| Substring | Provider |
| --- | --- |
| `cloudflare.com` | Cloudflare |
| `awsdns` | Amazon Route 53 |
| `domaincontrol.com`, `godaddy.com` | GoDaddy |
| `googledomains.com` | Google Cloud DNS |
| `squarespace`, `namecheap`, `hover.com`, `digitalocean.com`, `hetzner.com`, `registrar-servers.com`, `nsone.net`, `dnsv.jp`, `linode.com`, `vultr.com`, `dnsimple.com` | detected but not directly supported |

First match wins. The function is deterministic and does no I/O.

### Step 2: RDAP Fallback

`DetectProvider` calls `net.LookupNS`, runs `DetectFromNS`, and then checks if the result is ambiguous. "Ambiguous" means:

- No NS pattern matched, or
- The match was `google_cloud_dns` (Google Domains NS used to front multiple DNS hosts)

In those cases it queries RDAP (Verisign for `.com`/`.net`, nic.io for `.io`, rdap.org for `.org`, identity digital fallback otherwise) for the registrar name and maps that string back to a provider. The `Method` field changes from `ns_pattern` to `rdap_fallback` when RDAP overrides the initial guess.

RDAP lookups are best-effort with a 5s timeout. On any failure the NS result is returned unchanged.

You can also call `entree.LookupRegistrar(ctx, domain)` directly if you only want the registrar name.

## The Supported Flag

`Supported` is true only for the four providers with direct API integrations: Cloudflare, Route 53, GoDaddy, Google Cloud DNS. Detected-but-unsupported providers still return a `Provider` slug and `Label` so UIs can display "We detected NameCheap" even if dns-entree cannot push records there directly.

For unsupported providers, fall back to:

- [Domain Connect](domain-connect.md) if the provider exposes it, or
- Hand-off instructions telling the user which records to create.

## Example

```go
d, err := entree.DetectProvider(ctx, "example.com")
if err != nil {
    log.Fatalf("lookup ns: %v", err)
}
switch {
case d.Supported:
    log.Printf("%s detected (%s) - can push directly", d.Label, d.Method)
case d.Provider != "":
    log.Printf("%s detected (%s) - no direct API, use Domain Connect or manual", d.Label, d.Method)
default:
    log.Printf("unknown provider, nameservers=%v", d.Nameservers)
}
```
