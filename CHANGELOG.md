# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
from v1.0.0 onward. During the v0.x line, breaking changes may land in any release.

## [Unreleased]

## [0.2.0] - 2026-04-20

### Added
- New `esp` package: Email Service Provider detection and classification
  - `ClassifyDomain(ctx, domain, spf, selectors)` - full pipeline (SPF + DKIM)
  - `ClassifyFromSPF(spf)` - synchronous, no-DNS top-level include classification
  - `ClassifyFromSPFRecursive(ctx, resolver, spf)` - recursive SPF walk with SES-in-chain detection
  - `ClassifyFromDKIM(ctx, resolver, domain, selectors)` - DKIM CNAME target classification
- SES-backed sender detection: flags Resend, Loops, Bento, Customer.io, Salesforce transactional,
  Stripe transactional, and raw Amazon SES as `Infrastructure: ses`, enabling consumers to route
  to the correct integration (AWS IAM, ESP API, OAuth) regardless of which top-level ESP the
  customer uses.
- ESP catalog covering 30+ SPF include hosts and 13 DKIM CNAME target patterns, including
  Salesforce (Sales Cloud, Service Cloud, Einstein Activity Capture), Marketing Cloud (ExactTarget),
  Pardot, Mailchimp, SendGrid, Postmark, Mailgun, HubSpot, Google Workspace, Microsoft 365,
  Zoho, Fastmail, iCloud.

## [0.1.0] - 2026-04-09

### Widget
- Embeddable JS widget (`@dns-entree/widget`) for drop-in DNS setup flows
- Three auto-selected tiers: Domain Connect (zero credentials) > API credentials > copy-paste fallback
- Shadow DOM isolation with light/dark theming, under 20 KB gzipped
- Available via npm and CDN (`unpkg.com/@dns-entree/widget`)

### Docker
- Multi-stage Dockerfile with distroless runtime (`gcr.io/distroless/static-debian12`)
- Builds both `entree` CLI and `entree-api` server in a single image
- Cross-platform support via `TARGETOS`/`TARGETARCH` build args
- Published to GHCR (`ghcr.io/spoofcanary/dns-entree`)

### Deploy
- One-click deploy configs for Railway, Render, and Fly.io
- Health check endpoint (`/healthz`) configured in all platform templates
- Persistent volume mounts for stateful migration storage

## [0.1.0-alpha] - 2026-04-06

### Added
- Provider interface with implementations for Cloudflare, Route53, GoDaddy, and Google Cloud DNS
- `DetectProvider` with NS pattern matching and RDAP fallback
- `PushService` for idempotent record push with post-write verification
- SPF merge algorithm with 10-lookup warning
- Domain Connect discovery, signing, and apply URL generation
- Template engine with sync, resolve, and apply
- `entree` CLI with `detect`, `apply`, `verify`, `spf-merge`, `dc-discover`, and `templates` commands
