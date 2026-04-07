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
- [x] `entree` CLI for `detect`, `apply`, `verify`, `spf-merge`, `dc-discover`, `templates`

## Supported Providers

| Provider                                         | Slug              | API Auth                       | Domain Connect | Notes                                   |
| ------------------------------------------------ | ----------------- | ------------------------------ | -------------- | --------------------------------------- |
| [Cloudflare](docs/providers/cloudflare.md)       | cloudflare        | Scoped API token               | n/a            | Zone:Read + DNS:Edit permissions        |
| [Amazon Route 53](docs/providers/route53.md)     | route53           | IAM access/secret key          | n/a            | Uses ChangeBatch for ApplyRecords       |
| [GoDaddy](docs/providers/godaddy.md)             | godaddy           | API key + secret               | no (Entri gate) | API gated: needs 10+ domains *or* Discount Domain Club (~$2/yr) |
| [Google Cloud DNS](docs/providers/gcdns.md)      | google_cloud_dns  | OAuth2 bearer + project ID     | n/a            | Caller refreshes the token              |

### A word about GoDaddy

GoDaddy is hostile to small customers who want API access. Their production API returns HTTP 403 on every request unless the account either manages 10+ domains or subscribes to **Discount Domain Club** (~$2/year). The gate is undocumented, there is no error message explaining it, and GoDaddy simultaneously routes their Domain Connect implementation through a paid third-party aggregator (Entri), so the obvious workaround is also blocked. Every other registrar in this list treats API access as a basic feature.

The good news: $2/year Discount Domain Club lifts the gate immediately. The bad news: you have to pay GoDaddy $2/year for the privilege of owning the domain you already paid them for. If you have the option, migrate DNS to Cloudflare (free) or Route 53 (pay per query) instead. See [docs/providers/godaddy.md](docs/providers/godaddy.md) for details.

## Install

```sh
# Library
go get github.com/spoofcanary/dns-entree

# CLI
go install github.com/spoofcanary/dns-entree/cmd/entree@latest
```

Requires Go 1.26 or later.

## Documentation

- [Getting started](docs/getting-started.md) - install and first push
- [CLI reference](docs/cli.md) - generated from the cobra command tree
- Guides: [push-service](docs/guides/push-service.md), [spf-merge](docs/guides/spf-merge.md), [domain-connect](docs/guides/domain-connect.md), [templates](docs/guides/templates.md), [provider-detection](docs/guides/provider-detection.md)
- Recipes: [push-dmarc](docs/recipes/push-dmarc.md), [push-dkim](docs/recipes/push-dkim.md), [detect-and-apply](docs/recipes/detect-and-apply.md), [migrate-from-bash-scripts](docs/recipes/migrate-from-bash-scripts.md)
- Providers: [cloudflare](docs/providers/cloudflare.md), [route53](docs/providers/route53.md), [godaddy](docs/providers/godaddy.md), [gcdns](docs/providers/gcdns.md)

## Stability

dns-entree is currently **v0.1.0-alpha**. During the v0.x line, breaking changes may land in any release. From **v1.0.0 forward**, the project follows strict Semantic Versioning.

Stable packages (semver-covered from v1.0.0): `entree` (root), `domainconnect/`, `template/`, `providers/*`, `cmd/entree/`.
Unstable: anything under `internal/`.

See [docs/stability.md](docs/stability.md) for details.

## License

MIT. See [LICENSE](LICENSE).
