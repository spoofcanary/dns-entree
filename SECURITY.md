# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in dns-entree, please report it
privately. Do **not** open a public GitHub issue.

**Contact:** security@spoofcanary.com

Include:
- A description of the issue
- Steps to reproduce
- Affected version(s)
- Any proof-of-concept code or output

## Response SLA

- **Acknowledgement:** within 72 hours of receipt
- **Initial assessment:** within 7 days
- **Fix for high-severity issues:** within 30 days of confirmation
- **Public disclosure:** coordinated with the reporter after a fix ships

## Supported Versions

During the v0.x line, only the latest minor release receives security fixes.
Once v1.0.0 ships, the previous minor will also receive backports for one
release cycle.

| Version       | Supported          |
| ------------- | ------------------ |
| 0.1.x         | Yes (latest only)  |
| < 0.1.0       | No                 |

## Credential Handling

dns-entree never persists provider credentials to disk in plaintext.

**Library mode:** Credentials live in memory only. The caller constructs a
`Provider` with credentials in Go code; dns-entree holds them for the
duration of the call and does not write them anywhere.

**HTTP API (stateless mode):** Credentials are passed per-request via
`X-Entree-*` headers. The server holds nothing between requests. Header
values are redacted from all log output (empirically tested).

**HTTP API (stateful migration mode):** Target provider credentials
submitted during `POST /v1/migrate/preview` are encrypted at rest with
AES-256-GCM before being written to the migration store.

Key management (in priority order):

1. **`ENTREE_STATE_KEY` env var** (recommended): 64 hex characters (32
   bytes). Set this in production. Example:
   `openssl rand -hex 32`
2. **Hostname-derived fallback** (development only): On first start, a
   random 32-byte salt is generated at `<state-dir>/.key-salt` (mode 0600).
   The key is SHA-256(hostname || salt). A WARN log is emitted at boot.
   Do not use this across multiple hosts - each derives a different key.

Credentials are decrypted only when `apply` is called, held in memory for
the provider API call, then discarded. The ciphertext blob includes a GCM
authentication tag - any tampering is detected on decrypt.

Key rotation (`rotate-state-key` subcommand) is planned but not yet
shipped. To rotate manually: re-encrypt all stored migrations or delete
and re-create them.

See [docs/http-api.md](docs/http-api.md#encryption-at-rest) for the full
reference.

## Server Deployment

The `entree-api` HTTP server has **no built-in authentication or TLS**.
It is designed to run behind a reverse proxy that handles both.

**Required for production:**

- **TLS termination** at the proxy layer (Caddy, nginx, ALB, Cloudflare
  Tunnel, etc.). The server speaks plain HTTP only.
- **Authentication** at the proxy layer. Options: basicauth, forward_auth,
  mTLS, OIDC (AWS ALB), or Cloudflare Access. The server trusts any
  request that reaches it.
- **Network isolation**: bind to localhost or a private network interface.
  Never expose the server port directly to the internet.
- **Multi-tenancy gating**: the server does not enforce tenant isolation.
  If multiple tenants share one server, the proxy MUST enforce per-tenant
  access control on stateful migration endpoints (`/v1/migrate/*`).

**Do not run `entree-api` on a public port without a proxy in front.**
The server will happily accept unauthenticated requests to push DNS
records using whatever credentials the caller provides in headers.

See [docs/http-api.md](docs/http-api.md) for proxy configuration examples.

## Scope

In scope:
- The `dns-entree` library and `entree` CLI
- Provider integrations under `providers/`
- Domain Connect handling under `domainconnect/`

Out of scope:
- Vulnerabilities in upstream provider APIs (report to the provider directly)
- Issues in example code under `_examples/`
