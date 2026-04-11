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

## Stateful Migration Workflow

The one-shot `POST /v1/migrate` above is synchronous: scrape + create zone +
push records + verify all happen inside a single HTTP call. That breaks down
for real migrations, where the customer has to update nameservers at the
registrar and it takes minutes (or hours) for authoritative NS to propagate.
The stateful workflow decomposes the flow into preview + apply + poll-verify,
backed by a pluggable `MigrationStore` (default: JSON files on disk).

### Flow

```
POST /v1/migrate/preview       -> scrape source zone, store row, issue access_token
         |
         v
POST /v1/migrate/{id}/apply    -> create target zone, push records
         |
         v
      awaiting_ns_change       (customer updates NS at registrar)
         |
         v
POST /v1/migrate/{id}/verify   -> one VerifyAgainstNS round (poll until matched == total)
         |
         v
       complete
```

### Endpoints

All id-scoped routes require `Authorization: Bearer <access_token>`, where
`<access_token>` is the token returned by `POST /v1/migrate/preview`. The
token is compared in constant time and is never echoed back by `GET`/`LIST`.

| Route | Auth | Purpose |
|-------|------|---------|
| `POST /v1/migrate/preview` | none (front with proxy) | Scrape source zone, encrypt target credentials at rest, return `id` + `access_token` + `preview` zone + `expires_at`. Does NOT touch the target provider. |
| `POST /v1/migrate/{id}/apply` | bearer | Create the target zone and push the previewed records. Returns target nameservers and per-record apply results. Status moves to `awaiting_ns_change`. |
| `POST /v1/migrate/{id}/verify` | bearer | Run a single `VerifyAgainstNS` round. Returns `{matched, total, details}`. Status becomes `complete` when `matched == total`. Designed to be polled. |
| `GET /v1/migrate/{id}` | bearer | Fetch the stored migration row. `credential_blob` and `access_token` are always redacted on the wire. |
| `GET /v1/migrate` | none (front with proxy) | List migrations. Query: `tenant_id`, `status`, `limit`, `offset`. The library does not enforce multi-tenancy itself - operators MUST front this endpoint with a reverse proxy that enforces tenant/IP gating. |
| `DELETE /v1/migrate/{id}` | bearer | Delete the migration row. |

### curl walkthrough (happy path)

```sh
# 1. Preview
curl -sS -X POST http://localhost:8080/v1/migrate/preview \
  -H 'Content-Type: application/json' \
  -d '{
    "domain": "example.com",
    "target": "cloudflare",
    "target_credentials": {"cloudflare": {"token": "cf_TOKEN"}}
  }'
# -> {"ok":true,"data":{"id":"019364a2-...","access_token":"xyz","preview":{...},"expires_at":"..."}}

# 2. Apply
curl -sS -X POST http://localhost:8080/v1/migrate/019364a2-.../apply \
  -H 'Authorization: Bearer xyz'
# -> {"ok":true,"data":{"status":"awaiting_ns_change","target_nameservers":["ns1.cloudflare.com","ns2.cloudflare.com"]}}

# 3. Verify (poll)
curl -sS -X POST http://localhost:8080/v1/migrate/019364a2-.../verify \
  -H 'Authorization: Bearer xyz'
# -> {"ok":true,"data":{"matched":25,"total":27,"details":[...]}}

# Keep polling step 3 until matched == total; status becomes "complete".

# Fetch the row
curl -sS http://localhost:8080/v1/migrate/019364a2-... \
  -H 'Authorization: Bearer xyz'

# List (front this with a proxy!)
curl -sS 'http://localhost:8080/v1/migrate?status=complete&limit=20'

# Delete
curl -sS -X DELETE http://localhost:8080/v1/migrate/019364a2-... \
  -H 'Authorization: Bearer xyz'
```

### Access token lifecycle

- Generated at `preview` time: 32 random bytes, base64url-encoded.
- Stored on the row in `StoredMigration.AccessToken` and returned ONCE in the
  preview response body. There is no "fetch my token" endpoint.
- Compared in constant time on every id-scoped request.
- Redacted on `GET /v1/migrate/{id}` and `GET /v1/migrate` responses.
- Expires with the row (see TTL below). After expiry, every id-scoped call
  returns `410 Gone`.

### Storage configuration

| Flag | Env var | Default | Purpose |
|------|---------|---------|---------|
| `--state-dir` | `ENTREE_API_STATE_DIR` | `$XDG_DATA_HOME/entree/migrations` (falls back to `~/.local/share/entree/migrations`) | Directory for JSON rows, lock files, and `.key-salt`. |
| `--migration-ttl` | `ENTREE_API_MIGRATION_TTL` | `1h` | TTL applied at `preview` time. `ExpiresAt = CreatedAt + TTL`. |
| `--gc-interval` | `ENTREE_API_GC_INTERVAL` | `5m` | Background sweeper period. The server also exposes `entree-api gc --once` for cron-based ops. |
| `--migration-rate` | `ENTREE_API_MIGRATION_RATE` | `10` | Records/sec rate limit during apply. |

File modes: the state dir is created with mode `0700`; every row file is
written with mode `0600`. Only the user running `entree-api` can read them.

**Multi-process on Windows: unsupported for the default JSON backend.** The
backend uses `flock()` on unix; Windows has an in-process mutex fallback. Run
exactly one `entree-api` process per state dir on Windows, or use the SQLite
backend below.

### Encryption at rest

The encrypted payload is `StoredMigration.CredentialBlob` - the target
provider credentials passed to `preview`. Everything is AES-256-GCM.

Key selection (in order):

1. `ENTREE_STATE_KEY` env var: 64 hex chars (32 bytes). **Recommended for
   production and any multi-host deployment.**
2. Derived at startup from `hostname` + `<state-dir>/.key-salt` (32 random
   bytes generated on first start). Emits a WARN log at boot:
   > `state key derived from hostname fallback; set ENTREE_STATE_KEY in production`

   Do not use the hostname fallback across multiple hosts - each host will
   derive a different key and rows become unreadable on the other host. Key
   rotation (`rotate-state-key` subcommand) is deferred to a later release.

### Backends

The default binary ships the JSON-files backend. The SQLite backend is an
opt-in build tag and pulls in `modernc.org/sqlite` (pure Go, no cgo, about 5
MB of binary bloat).

```sh
# Default: JSON-files backend, zero extra dependencies.
go install github.com/spoofcanary/dns-entree/cmd/entree-api@latest

# Optional: SQLite backend.
go build -tags=sqlite -o entree-api ./cmd/entree-api
```

Library users who embed `api.NewServer(...)` pass a `migrate.MigrationStore`
explicitly. The SQLite store is `migrate/sqlstore.NewStore(path)` and is only
compiled when you build with `-tags=sqlite`.

| | JSON files (default) | SQLite (`-tags=sqlite`) |
|---|---|---|
| Dependencies | none | `modernc.org/sqlite` (~5 MB) |
| Layout | one file per migration at `<state-dir>/<id>.json` + `.lock` + `.key-salt` | single `migrations.db` file |
| Concurrency | `flock()` on unix; in-process mutex fallback on Windows | SQL transactions with optimistic `version` check |
| Multi-process | unix only (flock); not supported on Windows | yes |
| List() cost | reads every file in the state dir | indexed query |
| Ops | `cp`, `rm`, `jq` | `sqlite3` CLI |
| Inspection | every row is a self-describing JSON file | `sqlite3 migrations.db '.schema'` |

### Deprecation of POST /v1/migrate

The original synchronous `POST /v1/migrate` endpoint is still wired up and
still works, but every response now carries:

```
Deprecation: true
Sunset: <RFC 1123 date one release out>
Link: </v1/migrate/preview>; rel="successor-version"
```

It will be removed in the release after v0.2.0. New integrations must use
`preview` + `apply` + `verify`.

### Error codes

In addition to the common codes above, the stateful routes return:

| HTTP | Code | Meaning |
|------|------|---------|
| 401 | `BAD_REQUEST` | Missing or invalid `Authorization: Bearer <access_token>` on an id-scoped route. Access token comparison is constant-time. |
| 404 | `BAD_REQUEST` | Migration id does not exist (or was deleted). |
| 409 | `BAD_REQUEST` | Optimistic-lock version mismatch: another concurrent writer updated the row between read and write. Retry by re-fetching via `GET /v1/migrate/{id}`. |
| 410 | `BAD_REQUEST` | Migration row has expired (`ExpiresAt < now`). Start a new `preview`. |

(The envelope `code` stays `BAD_REQUEST` for the 4xx family; the HTTP status
carries the precise meaning. `500` still maps to `INTERNAL` / `PROVIDER_ERROR`.)

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
