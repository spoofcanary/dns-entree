# dns-entree HTTP API

A thin, stateless HTTP server that wraps the dns-entree library so callers in
any language can use it via plain JSON. Every request carries its own
credentials in headers; the server holds nothing between requests.

The OpenAPI 3.1 spec ships with the binary and is served at
`/v1/openapi.yaml`. This document is the human-readable companion.

## Deployment Model

Run `entree-api` (or `entree serve`) behind a reverse proxy that handles TLS
termination and authentication. The server itself speaks HTTP only and has
no built-in auth. Examples:

- Caddy with `basicauth` or `forward_auth`
- nginx with `auth_request` or mTLS
- Cloudflare Tunnel with Cloudflare Access policies
- Tailscale Funnel with tailnet ACLs
- AWS ALB with OIDC authenticator
- Traefik with the `BasicAuth` or `ForwardAuth` middleware

The "auth lives in the proxy" model lets operators reuse their existing
identity stack instead of learning another one.

## Install

```sh
# Standalone server binary
go install github.com/spoofcanary/dns-entree/cmd/entree-api@latest

# Or use the same server through the CLI binary
go install github.com/spoofcanary/dns-entree/cmd/entree@latest
entree serve --listen :8080
```

Both entry points share the same flag set and environment variables.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--listen` | `:8080` | Bind address (host:port). |
| `--log-level` | `info` | `debug`, `info`, `warn`, `error`. |
| `--log-format` | `json` | `json` or `text`. |
| `--cors-origin` | (none) | Allowlist origin for CORS. Repeatable. Pass `*` for any (dev only). |
| `--request-timeout` | `15m` | Hard `WriteTimeout` on each request. |
| `--template-cache-dir` | XDG default | Where to cache Domain Connect templates. |

## Environment Variables

Flags take precedence over environment variables.

| Variable | Maps to |
|----------|---------|
| `ENTREE_API_LISTEN` | `--listen` |
| `ENTREE_API_LOG_LEVEL` | `--log-level` |
| `ENTREE_API_LOG_FORMAT` | `--log-format` |
| `ENTREE_API_CORS_ORIGIN` | `--cors-origin` (comma-separated) |
| `ENTREE_API_REQUEST_TIMEOUT` | `--request-timeout` |
| `ENTREE_API_TEMPLATE_CACHE_DIR` | `--template-cache-dir` |

## Credential Headers

Every endpoint that needs to talk to a DNS provider expects credentials in
per-request headers. The server never persists or logs them.

| Header | Provider |
|--------|----------|
| `X-Entree-Provider` | `cloudflare`, `route53`, `godaddy`, `google_cloud_dns` |
| `X-Entree-Cloudflare-Token` | Cloudflare API token (zone-scoped) |
| `X-Entree-AWS-Access-Key-Id` | Route 53 access key id |
| `X-Entree-AWS-Secret-Access-Key` | Route 53 secret access key |
| `X-Entree-GoDaddy-Key` | GoDaddy API key |
| `X-Entree-GoDaddy-Secret` | GoDaddy API secret |
| `X-Entree-GCDNS-Service-Account-JSON` | Base64-encoded GCP service account JSON |

Two endpoints accept credentials in the JSON body instead of headers:

- `POST /v1/migrate` - needs source AND target credentials in one call.
- `POST /v1/zone/import` - target credentials live with the import payload.
- `POST /v1/dc/apply-url` - takes an RSA private key as
  `opts.private_key_pem` so it never appears in upstream proxy access logs.

## Envelope

Success responses:

```json
{ "ok": true, "schema_version": 1, "data": { ... } }
```

Error responses:

```json
{ "ok": false, "schema_version": 1, "error": { "code": "BAD_REQUEST", "message": "...", "details": {} } }
```

### Error codes

| Code | HTTP | Meaning |
|------|------|---------|
| `MISSING_CREDENTIALS` | 400 | Required X-Entree-* header(s) absent or malformed. |
| `BAD_REQUEST` | 400 | Invalid JSON, missing field, or validation failure. |
| `PROVIDER_ERROR` | 500 | Upstream DNS provider returned an error. |
| `VERIFY_MISMATCH` | 409 | `/v1/verify` could not confirm the expected record. |
| `TEMPLATE_NOT_FOUND` | 404 | Template lookup failed for the given provider+service id. |
| `RATE_LIMIT_EXCEEDED` | 429 | Reserved for future rate limiting. |
| `INTERNAL` | 500 | Unclassified internal failure. |

## Endpoints (curl examples)

All header values below are placeholders; substitute your own.

### POST /v1/detect

```sh
curl -sS http://localhost:8080/v1/detect \
  -H 'Content-Type: application/json' \
  -d '{"domain":"example.com"}'
```

### POST /v1/verify

```sh
curl -sS http://localhost:8080/v1/verify \
  -H 'Content-Type: application/json' \
  -d '{"domain":"example.com","type":"TXT","name":"_dmarc.example.com","contains":"v=DMARC1"}'
```

### POST /v1/spf-merge

```sh
curl -sS http://localhost:8080/v1/spf-merge \
  -H 'Content-Type: application/json' \
  -d '{"current":"v=spf1 include:_spf.google.com ~all","includes":["sendgrid.net","amazonses.com"]}'
```

### POST /v1/apply

```sh
curl -sS http://localhost:8080/v1/apply \
  -H 'Content-Type: application/json' \
  -H 'X-Entree-Provider: cloudflare' \
  -H 'X-Entree-Cloudflare-Token: cf_TOKEN' \
  -d '{
    "domain": "example.com",
    "records": [
      {"type":"TXT","name":"_dmarc.example.com","content":"v=DMARC1; p=none"}
    ],
    "dry_run": false
  }'
```

### POST /v1/apply/template

```sh
curl -sS http://localhost:8080/v1/apply/template \
  -H 'Content-Type: application/json' \
  -H 'X-Entree-Provider: cloudflare' \
  -H 'X-Entree-Cloudflare-Token: cf_TOKEN' \
  -d '{
    "domain": "example.com",
    "provider_id": "google",
    "service_id": "workspace",
    "vars": {"verification": "google-site-verification=abc"}
  }'
```

### POST /v1/dc/discover

```sh
curl -sS http://localhost:8080/v1/dc/discover \
  -H 'Content-Type: application/json' \
  -d '{"domain":"example.com"}'
```

### POST /v1/dc/apply-url

```sh
curl -sS http://localhost:8080/v1/dc/apply-url \
  -H 'Content-Type: application/json' \
  -d '{
    "opts": {
      "provider_id": "google",
      "service_id": "workspace",
      "domain": "example.com",
      "params": {"verification": "google-site-verification=abc"}
    }
  }'
```

### POST /v1/templates/sync

```sh
curl -sS -X POST http://localhost:8080/v1/templates/sync \
  -H 'Content-Type: application/json' -d '{}'
```

### GET /v1/templates

```sh
curl -sS http://localhost:8080/v1/templates
```

### GET /v1/templates/{provider}/{service}

```sh
curl -sS http://localhost:8080/v1/templates/google/workspace
```

### POST /v1/templates/{provider}/{service}/resolve

```sh
curl -sS http://localhost:8080/v1/templates/google/workspace/resolve \
  -H 'Content-Type: application/json' \
  -d '{"vars":{"verification":"google-site-verification=abc"}}'
```

### POST /v1/migrate

```sh
curl -sS http://localhost:8080/v1/migrate \
  -H 'Content-Type: application/json' \
  -d '{
    "domain": "example.com",
    "source_provider": "godaddy",
    "target_provider": "cloudflare",
    "source_credentials": {"godaddy": {"key": "k", "secret": "s"}},
    "target_credentials": {"cloudflare": {"token": "cf_TOKEN"}},
    "dry_run": true
  }'
```

### POST /v1/zone/export

```sh
curl -sS http://localhost:8080/v1/zone/export \
  -H 'Content-Type: application/json' \
  -H 'X-Entree-Provider: cloudflare' \
  -H 'X-Entree-Cloudflare-Token: cf_TOKEN' \
  -d '{"domain":"example.com","no_axfr":true}'
```

### POST /v1/zone/import

```sh
curl -sS http://localhost:8080/v1/zone/import \
  -H 'Content-Type: application/json' \
  -d '{
    "to": "cloudflare",
    "target_credentials": {"cloudflare": {"token": "cf_TOKEN"}},
    "zone": {"domain": "example.com", "records": []},
    "dry_run": true
  }'
```

### GET /healthz

```sh
curl -sf http://localhost:8080/healthz
```

Always returns `200 {"ok": true}` while the process is up.

### GET /readyz

```sh
curl -sf http://localhost:8080/readyz
```

Returns `200 {"ok": true}` when the server can serve traffic. Today it
matches `/healthz`; reserved for future readiness checks (template cache
warm-up, downstream connectivity).

### GET /metrics

```sh
curl -s http://localhost:8080/metrics
```

Prometheus text exposition. Three metric families:

- `entree_http_requests_total{method,path,status}`
- `entree_http_request_duration_seconds_bucket{...}`
- `entree_provider_operations_total{provider,op,status}`

### GET /v1/openapi.yaml

```sh
curl -s http://localhost:8080/v1/openapi.yaml | head
```

Returns the OpenAPI 3.1 document that ships with the running binary.

## Security Notes

- Headers are stripped from log context by a credential-redact middleware
  that runs BEFORE the request logger. Handlers do not log raw credential
  values either.
- Error responses never echo credential header values, even in `details`.
- Request bodies on `POST /v1/apply`, `/v1/apply/template`, `/v1/detect`,
  `/v1/verify`, `/v1/spf-merge`, `/v1/dc/*`, and `/v1/templates/*` are
  capped at 1 MiB. `/v1/migrate`, `/v1/zone/export`, and `/v1/zone/import`
  allow up to 10 MiB.
- The default request timeout is 15 minutes (configurable via
  `--request-timeout`); long-running migrations are synchronous by design.
- CORS is off by default. Enable it explicitly with `--cors-origin`.
- The server is HTTP-only. Always front it with a reverse proxy that does
  TLS termination and auth.
