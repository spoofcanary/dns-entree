# dns-entree

> Entri charges $249/month for the appetizer. The entree is free.

Provider-agnostic DNS automation for any record type (TXT, CNAME, A, AAAA, MX, NS, SRV) with idempotent writes, post-push verification, SPF merging, Domain Connect, and a template engine - usable as a Go library, a CLI, and an agent-callable tool. First-class helpers for email authentication (DMARC, DKIM, SPF, BIMI) on top of a generic DNS core.

[![Go Reference](https://pkg.go.dev/badge/github.com/spoofcanary/dns-entree.svg)](https://pkg.go.dev/github.com/spoofcanary/dns-entree)
[![Go Report Card](https://goreportcard.com/badge/github.com/spoofcanary/dns-entree)](https://goreportcard.com/report/github.com/spoofcanary/dns-entree)
[![CI](https://github.com/spoofcanary/dns-entree/actions/workflows/ci.yml/badge.svg)](https://github.com/spoofcanary/dns-entree/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/spoofcanary/dns-entree/branch/main/graph/badge.svg)](https://codecov.io/gh/spoofcanary/dns-entree)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## Library Quickstart

```go
package main

import (
    "context"
    "log"
    "os"

    entree "github.com/spoofcanary/dns-entree"
    "github.com/spoofcanary/dns-entree/providers/cloudflare"
)

func main() {
    p, err := cloudflare.NewProvider(os.Getenv("CF_API_TOKEN"))
    if err != nil {
        log.Fatal(err)
    }

    svc := entree.NewPushService(p)
    res, err := svc.PushTXTRecord(context.Background(),
        "example.com",
        "_dmarc.example.com",
        "v=DMARC1; p=none; rua=mailto:dmarc@example.com",
    )
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("status=%s verified=%t", res.Status, res.Verified)
}
```

`PushService` is idempotent: re-running the same push returns `already_configured` instead of rewriting the record, and every successful write is followed by an authoritative DNS query to confirm the change is live.

## CLI Quickstart

```sh
# Push the same DMARC record via the CLI.
export CF_API_TOKEN=...
entree apply \
  --provider cloudflare \
  --domain example.com \
  --type TXT \
  --name _dmarc.example.com \
  --content 'v=DMARC1; p=none; rua=mailto:dmarc@example.com'
```

Add `--json` to any command for agent-friendly structured output.

## Features

- [x] Unified `Provider` interface across Cloudflare, Route 53, GoDaddy, Google Cloud DNS
- [x] NS-pattern + RDAP-fallback provider detection (`DetectProvider`)
- [x] Idempotent `PushService` with post-push authoritative DNS verification
- [x] SPF include merging with RFC 7208 10-lookup limit warnings
- [x] Authoritative-first `Verify` with recursive fallback
- [x] Domain Connect v2 discovery, RSA-SHA256 signing, signed apply URLs
- [x] Template engine: load, sync from upstream repo, resolve variables, apply with conflict modes
- [x] Zone migration: scrape any provider via AXFR/iterated queries/BIND import, bulk-apply to a new provider, verify, print NS-change instructions
- [x] `entree` CLI for `detect`, `apply`, `verify`, `spf-merge`, `dc-discover`, `templates`, `migrate`, `zone`
- [x] Stateless HTTP API with per-request credentials, OpenAPI 3.1 spec, Prometheus metrics
- [x] Stateful migration API with AES-256-GCM credential encryption, per-migration access tokens, pluggable storage (JSON files or SQLite)

## Zone Migration

Move a domain from any DNS host to any supported provider. dns-entree scrapes the live zone via public DNS (no source API credentials needed), previews the diff, bulk-applies to the target, verifies every record against the new nameservers, and prints registrar-specific NS-change instructions.

```sh
# One command to migrate example.com from GoDaddy to Cloudflare
export CLOUDFLARE_API_TOKEN=...
entree migrate example.com --to cloudflare --yes
```

Or step by step:

```sh
# 1. Export the current zone
entree zone export example.com -o zone.json

# 2. Preview what will be created
entree zone import example.com --from zone.json --to cloudflare --dry-run

# 3. Apply and verify
entree zone import example.com --from zone.json --to cloudflare --yes
```

The migration scrapes via AXFR where available, falls back to iterated label queries against authoritative nameservers, and can also import BIND zone files. Rate-limited to 10 writes/sec by default. See [docs/recipes/migrate-from-bash-scripts.md](docs/recipes/migrate-from-bash-scripts.md).

## Widget

Embed a DNS setup flow in any web page. The widget auto-detects the customer's DNS provider, applies records via Domain Connect or API credentials, and falls back to copy-paste instructions.

```html
<script src="https://unpkg.com/@dns-entree/widget@0.1.0/dist/widget.js"
        integrity="sha384-rechRu251oo6Vw3JU0Mwu0idaavS8bsdFeKwJombJBLPaguIIF9wB8QWedJaRgsT"
        crossorigin="anonymous"></script>
<button onclick="DnsEntree.open({
  domain: 'example.com',
  apiUrl: 'http://localhost:8080',
  records: [
    { type: 'TXT', name: '_dmarc.example.com', content: 'v=DMARC1; p=reject;' }
  ]
})">Set up DNS</button>
```

Three tiers, auto-selected: Domain Connect (zero credentials) > API credentials (provider token) > copy-paste fallback (no backend needed). Shadow DOM isolated, light/dark theme, under 20 KB gzipped.

For enterprise deployments, self-host `widget.js` from your own domain instead of using unpkg. The file is a self-contained IIFE with no external dependencies.

See [docs/guides/widget.md](docs/guides/widget.md) for the full integration guide.

## Supported Providers

| Provider                                         | Slug              | API Auth                       | Notes                                   |
| ------------------------------------------------ | ----------------- | ------------------------------ | --------------------------------------- |
| [Cloudflare](docs/providers/cloudflare.md)       | cloudflare        | Scoped API token               | Zone:Read + DNS:Edit permissions        |
| [Amazon Route 53](docs/providers/route53.md)     | route53           | IAM access/secret key          | Uses ChangeBatch for ApplyRecords       |
| [GoDaddy](docs/providers/godaddy.md)             | godaddy           | API key + secret               | API gated: needs 10+ domains *or* Discount Domain Club (~$2/yr) |
| [Google Cloud DNS](docs/providers/gcdns.md)      | google_cloud_dns  | OAuth2 bearer + project ID     | Caller refreshes the token              |

### Domain Connect

Domain Connect is a protocol, not a provider feature. dns-entree supports [Domain Connect v2](https://www.domainconnect.org/) discovery, RSA-SHA256 signing, and signed apply URL generation for **any DNS host that advertises the protocol** - including Cloudflare, Name.com, 1&1, OVH, and [dozens more](https://www.domainconnect.org/ecosystem). No API credentials needed; the customer approves record changes through their DNS provider's consent UI.

See [docs/guides/domain-connect.md](docs/guides/domain-connect.md) for usage.

### A word about GoDaddy

GoDaddy is hostile to small customers who want API access. Their production API returns HTTP 403 on every request unless the account either manages 10+ domains or subscribes to **Discount Domain Club** (~$2/year). The gate is undocumented, there is no error message explaining it, and GoDaddy simultaneously routes their Domain Connect implementation through a paid third-party aggregator (Entri), so the obvious workaround is also blocked. Every other registrar in this list treats API access as a basic feature.

The good news: $2/year Discount Domain Club lifts the gate immediately. The bad news: you have to pay GoDaddy $2/year for the privilege of owning the domain you already paid them for. If you have the option, migrate DNS to Cloudflare (free) or Route 53 (pay per query) instead. See [docs/providers/godaddy.md](docs/providers/godaddy.md) for details.

## HTTP API

dns-entree also ships as a stateless HTTP server so non-Go callers can use it via plain JSON. Every request carries its own credentials in headers; the server holds nothing between requests and is designed to run behind a reverse proxy that handles TLS and auth.

```sh
# Standalone server binary
go install github.com/spoofcanary/dns-entree/cmd/entree-api@latest
entree-api --listen :8080

# Or use the same server from the CLI binary
entree serve --listen :8080
```

Hit it from anywhere:

```sh
curl -sS http://localhost:8080/v1/detect \
  -H 'Content-Type: application/json' \
  -d '{"domain":"example.com"}'
```

The OpenAPI 3.1 spec is served at `/v1/openapi.yaml`. See [docs/http-api.md](docs/http-api.md) for the deployment model, full flag/env reference, error codes, metrics, and a curl example for every endpoint.

### Stateful Migration

For cross-provider migrations where the customer has to update nameservers at the registrar and you need to poll for propagation, use the stateful workflow: `POST /v1/migrate/preview` returns an `id` + per-migration `access_token`; `POST /v1/migrate/{id}/apply` creates the target zone; `POST /v1/migrate/{id}/verify` runs one authoritative-NS check and can be polled until `matched == total`.

```sh
# Preview -> apply -> verify (poll)
curl -sS http://localhost:8080/v1/migrate/preview -H 'Content-Type: application/json' \
  -d '{"domain":"example.com","target":"cloudflare","target_credentials":{"cloudflare":{"token":"cf_TOKEN"}}}'
curl -sS -X POST http://localhost:8080/v1/migrate/$ID/apply  -H "Authorization: Bearer $TOKEN"
curl -sS -X POST http://localhost:8080/v1/migrate/$ID/verify -H "Authorization: Bearer $TOKEN"
```

The one-shot synchronous `POST /v1/migrate` is deprecated (emits `Deprecation: true` + `Sunset` headers) and will be removed after v0.2.0. See [docs/http-api.md#stateful-migration-workflow](docs/http-api.md#stateful-migration-workflow) for the full reference, including storage configuration, encryption at rest, and the optional SQLite backend.

## Install

```sh
# Library
go get github.com/spoofcanary/dns-entree

# CLI
go install github.com/spoofcanary/dns-entree/cmd/entree@latest

# HTTP server
go install github.com/spoofcanary/dns-entree/cmd/entree-api@latest
```

Requires Go 1.26 or later.

## Documentation

- [Getting started](docs/getting-started.md) - install and first push
- [CLI reference](docs/cli.md) - generated from the cobra command tree
- Guides: [push-service](docs/guides/push-service.md), [spf-merge](docs/guides/spf-merge.md), [domain-connect](docs/guides/domain-connect.md), [templates](docs/guides/templates.md), [provider-detection](docs/guides/provider-detection.md), [widget](docs/guides/widget.md)
- Recipes: [push-dmarc](docs/recipes/push-dmarc.md), [push-dkim](docs/recipes/push-dkim.md), [detect-and-apply](docs/recipes/detect-and-apply.md), [migrate-from-bash-scripts](docs/recipes/migrate-from-bash-scripts.md)
- Providers: [cloudflare](docs/providers/cloudflare.md), [route53](docs/providers/route53.md), [godaddy](docs/providers/godaddy.md), [gcdns](docs/providers/gcdns.md)

## Stability

dns-entree is currently **v0.1.0**. During the v0.x line, breaking changes may land in any release. From **v1.0.0 forward**, the project follows strict Semantic Versioning.

Stable packages (semver-covered from v1.0.0): `entree` (root), `domainconnect/`, `template/`, `providers/*`, `cmd/entree/`.
Unstable: anything under `internal/`.

See [docs/stability.md](docs/stability.md) for details.

## Security

Provider credentials are never persisted in plaintext. The stateful migration API encrypts credentials at rest with AES-256-GCM. The HTTP server has no built-in auth - deploy it behind a reverse proxy with TLS and authentication.

See [SECURITY.md](SECURITY.md) for credential handling details, key management, server deployment guidance, and vulnerability reporting.

## License

MIT. See [LICENSE](LICENSE).
