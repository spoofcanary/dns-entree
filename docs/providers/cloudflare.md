# Cloudflare

dns-entree uses the official [cloudflare-go](https://github.com/cloudflare/cloudflare-go) SDK. Authentication is a single API token.

## Credential Setup

1. Sign in at <https://dash.cloudflare.com/profile/api-tokens>.
2. Click **Create Token** -> **Create Custom Token**.
3. Name the token (e.g. `dns-entree`).
4. **Permissions**: add two rows
   - `Zone` / `DNS` / `Edit`
   - `Zone` / `Zone` / `Read`
5. **Zone Resources**: scope to a specific zone (preferred) or "All zones from an account".
6. **Client IP Address Filtering** (optional): restrict to your server IPs.
7. **TTL**: set an expiry to force rotation.
8. Create and copy the token. Cloudflare shows it once.

Store the token in a secret manager; load it as `CLOUDFLARE_API_TOKEN`.

## Scope Minimization

- `Zone:Read` is required for listing zones and resolving zone IDs.
- `Zone:DNS:Edit` covers list / create / update / delete of DNS records.
- Do **not** grant Account-level permissions. The token must be Zone-scoped.
- Do **not** use the legacy Global API Key. dns-entree calls `NewWithAPIToken` exclusively.

## Usage

```go
import (
    entree "github.com/spoofcanary/dns-entree"
    _ "github.com/spoofcanary/dns-entree/providers/cloudflare"
)

prov, err := entree.NewProvider("cloudflare", entree.Credentials{
    APIToken: os.Getenv("CLOUDFLARE_API_TOKEN"),
})
```

Or directly:

```go
import "github.com/spoofcanary/dns-entree/providers/cloudflare"

prov, err := cloudflare.NewProvider(os.Getenv("CLOUDFLARE_API_TOKEN"))
```

## Domain Connect Support

Cloudflare supports the [Domain Connect](https://www.domainconnect.org/) protocol for domains using Cloudflare nameservers. These domains advertise DC support via a `_domainconnect` TXT record pointing to Cloudflare's settings host.

This means dns-entree's `domainconnect.Discover()` will return `Supported: true` for Cloudflare-hosted domains, and you can use the async apply URL flow to let end users self-apply DNS templates without providing API credentials.

See [Domain Connect Guide](../guides/domain-connect.md) for the full discovery and signing workflow.

## Known Limits

- Rate limit: 1200 requests per 5 minutes per user per API. dns-entree does not currently rate-limit client-side; batch large migrations or add a `time.Sleep` between domains.
- Enterprise zones have higher record quotas but the same API shape.
- TTL values below 60 are quietly clamped by Cloudflare.

## Gotchas

- Zone must be in "active" status; pending zones reject writes with 400.
- `_dmarc.example.com` and similar prefixed names must be passed as the full FQDN, not just `_dmarc`.

## Official Docs

- API reference: <https://developers.cloudflare.com/api/>
- API tokens: <https://developers.cloudflare.com/fundamentals/api/get-started/create-token/>
- cloudflare-go: <https://github.com/cloudflare/cloudflare-go>
