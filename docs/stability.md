# Stability Tiers

dns-entree declares stability per package so callers know which APIs are
covered by semver and which may change without warning.

## Versioning Promise

The project is currently **v0.1.0-alpha**. Until v1.0.0 ships, breaking changes
may land in any release across both stable and unstable tiers. The v0.x line
exists so we can adjust the API based on real-world usage during the
initial migration and initial adoption.

**From v1.0.0 forward**, the project follows strict [Semantic Versioning](https://semver.org/):
- Stable-tier packages: breaking changes require a major version bump
- Unstable-tier packages: may change in any minor release, no major bump required

## Stable Tier

These packages are part of the public API and will be covered by semver from
v1.0.0 forward.

| Package                          | Description                                     |
| -------------------------------- | ----------------------------------------------- |
| `entree` (root)                  | PushService, SPF merge, verify, detect, apply   |
| `domainconnect/`                 | Domain Connect discovery, signing, apply URLs   |
| `template/`                      | Template engine: load, resolve, apply           |
| `providers/cloudflare`           | Cloudflare provider implementation              |
| `providers/route53`              | AWS Route53 provider implementation             |
| `providers/godaddy`              | GoDaddy provider implementation                 |
| `providers/gcdns`                | Google Cloud DNS provider implementation        |
| `cmd/entree/`                    | `entree` CLI command surface and flags          |

## Unstable Tier

These packages may change in any release, including patch releases. Do not
import them from external code unless you are prepared to update on every
upgrade.

| Package        | Description                                     |
| -------------- | ----------------------------------------------- |
| `internal/...` | All packages under `internal/` are unstable     |

## How to Depend on dns-entree

- Pin to a specific version in `go.mod` during the v0.x line
- Read the `CHANGELOG.md` before upgrading
- Avoid importing anything under `internal/`
- File issues against stable-tier packages if behavior changes unexpectedly
