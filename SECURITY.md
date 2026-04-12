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

---

## Security Audit

**Audit date:** 2026-04-11
**Scope:** Full codebase (D-01): library, CLI, HTTP API, Domain Connect, migration, crypto
**Auditor:** Automated (Claude) + static analysis patterns

### Severity Classification

| Severity | Definition |
|----------|------------|
| Critical | Remote code execution, credential exposure, authentication bypass |
| High | SSRF, injection, crypto weakness, authorization bypass |
| Medium | Input validation gaps, information leakage, DoS vectors |
| Low | Best-practice deviations, hardening opportunities |
| Info | Observations, positive findings, documentation |

### STRIDE Threat Register

| ID | Category | Component | Severity | Disposition | Description | Mitigation |
|----|----------|-----------|----------|-------------|-------------|------------|
| T-01 | Spoofing | HTTP API | Medium | accept | API has no built-in authentication; designed to run behind authenticating reverse proxy | Documented in Server Deployment section; proxy handles auth (mTLS, OIDC, basic-auth, Cloudflare Access) |
| T-02 | Spoofing | Migration API | Low | mitigate | Per-migration access tokens could be brute-forced | 256-bit random tokens via `crypto/rand`; constant-time comparison via `crypto/subtle.ConstantTimeCompare` (`api/handlers_migrate_stateful.go:68`) |
| T-03 | Spoofing | Migration API | Low | mitigate | Bearer token could leak in logs | `redactCreds()` strips AccessToken from all GET responses; credential headers scrubbed before slog sees request |
| T-04 | Tampering | HTTP API | Low | mitigate | Malformed DNS names could cause unexpected provider behavior | `ValidateDNSName()` enforces RFC 1035 (253-char limit, 63-char labels, no control chars, require TLD) with underscore/wildcard tolerance (`validate.go`) |
| T-05 | Tampering | HTTP API | Low | mitigate | Invalid record values (e.g., non-IP for A records) could cause provider-side errors or injection | `ValidateRecordValue()` type-checks A (IPv4), AAAA (IPv6), CNAME/NS (DNS name), MX (priority + target), TXT (length limit) (`validate.go:92-172`) |
| T-06 | Tampering | HTTP API | Low | mitigate | Extra/unknown JSON fields in request bodies could be ignored silently, masking client errors | `json.Decoder.DisallowUnknownFields()` rejects unknown keys (`api/handlers_core.go:146`) |
| T-07 | Tampering | HTTP API | Low | mitigate | Oversized request bodies could exhaust memory | `http.MaxBytesReader` enforced on every endpoint: 1 MiB default, 10 MiB for zone/migrate (`api/options.go:71-72`, `api/handlers_core.go:144`) |
| T-08 | Tampering | Template Engine | Low | mitigate | Template variable injection could produce malicious DNS records | `validateHost()` and `validatePointsTo()` reject control chars, validate FQDN/IP structure (`template/resolve.go`) |
| T-09 | Tampering | Migration Store | Low | mitigate | Concurrent apply requests could double-write records | Optimistic locking with Version increment BEFORE provider work; concurrent requests get `ErrVersionMismatch` / HTTP 409 (`api/handlers_migrate_stateful.go:299-309`) |
| T-10 | Tampering | Migration Store (SQLite) | Low | mitigate | SQL injection via filter parameters | All queries use `?` parameterized placeholders; no string interpolation of user input (`migrate/sqlstore/store.go`) |
| T-11 | Tampering | Migration Store (JSON) | Low | mitigate | Race condition on salt file creation | `os.O_EXCL` flag prevents TOCTOU races on `.key-salt` creation (`migrate/crypt.go:63`) |
| T-12 | Repudiation | HTTP API | Low | mitigate | Insufficient audit trail for DNS changes | Structured slog with request IDs on every request; route + method + status + duration logged (`api/middleware.go:117-126`) |
| T-13 | Repudiation | Library | Info | accept | No audit trail for DNS changes in library mode | Library callers own their own logging; dns-entree provides the operation, caller provides observability |
| T-14 | Information Disclosure | HTTP API | Low | mitigate | Credential headers leaked in logs | 4-layer defense-in-depth: (1) `credentialRedactMW` strips X-Entree-* headers before slog middleware (`api/middleware.go:180-198`), (2) slog middleware never inspects request headers (D-25), (3) `scrubDetails()` strips sensitive keys from error payloads (`api/errors.go:57-72`), (4) `credentialError` never includes header values in message/details |
| T-15 | Information Disclosure | HTTP API | Low | mitigate | Provider errors could expose internal details | `scrubDetails()` strips keys named `value`, `token`, `secret`, `password`, `key` from error detail maps (`api/errors.go:49-55`); generic messages returned to callers |
| T-16 | Information Disclosure | HTTP API | Info | accept | CORS `*` origin allowed if configured by operator | CORS is opt-in via `Options.CORSOrigins`; default is empty (no CORS headers emitted). Wildcard (`*`) is a valid operator choice for public APIs (`api/middleware.go:136-172`) |
| T-17 | Information Disclosure | Migration API | Low | mitigate | Migration GET endpoint could expose encrypted credentials | `redactCreds()` clears `CredentialBlob` and `AccessToken` from all responses including list (`api/handlers_migrate_stateful.go:73-81`) |
| T-18 | Information Disclosure | Migration API | Medium | accept | `GET /v1/migrate` list endpoint has no built-in auth | By design (D-19): operators must front this with a reverse proxy. Documented in Server Deployment section. All credential fields redacted on list responses |
| T-19 | Information Disclosure | Dependencies | Info | accept | `go.mod` dependency versions visible in public repo | Standard for open-source Go projects. No secret information. Versions aid reproducibility |
| T-20 | Denial of Service | HTTP API | Medium | accept | No built-in rate limiting on stateless endpoints | Designed for reverse proxy deployment; proxy handles rate limiting. Server timeouts provide baseline protection |
| T-21 | Denial of Service | HTTP API | Low | mitigate | Slow clients could hold connections indefinitely | Server timeouts configured: ReadHeaderTimeout=10s, ReadTimeout=30s, WriteTimeout=15m (configurable), IdleTimeout=120s (`api/server.go:192-197`) |
| T-22 | Denial of Service | HTTP API | Low | mitigate | Large request bodies could exhaust memory | `MaxBytesReader` on all endpoints caps body size at 1 MiB / 10 MiB (`api/handlers_core.go:144`) |
| T-23 | Denial of Service | SPF Merge | Low | mitigate | Pathological SPF input could cause excessive processing | Fuzz-tested with edge cases including 100+ includes, null bytes, unicode (`spf_fuzz_test.go`, `security_test.go:79-106`) |
| T-24 | Denial of Service | Domain Connect | Low | mitigate | Discovery response body could exhaust memory | Response body limited to 64 KiB via `io.LimitReader` (`domainconnect/discovery.go:157`) |
| T-25 | Denial of Service | Migration Sweeper | Low | mitigate | Expired migrations accumulate on disk | Background sweeper runs every 5 minutes (configurable) to delete expired rows (`api/server.go:229-263`) |
| T-26 | Elevation of Privilege | HTTP API | Info | accept | No roles/permissions in API | Single-tenant by design. Multi-tenant isolation is the reverse proxy's responsibility, documented in Server Deployment |
| T-27 | Elevation of Privilege | Template Engine | Low | mitigate | Malicious template repo URL could clone attacker-controlled content | `SyncTemplates` defaults to hardcoded official repo URL (`template/sync.go:21`); `WithRepoURL` override exists only for tests. `validateID()` rejects path traversal (`../`, `/`, `\`) in provider/service IDs (`template/sync.go:254-262`) |
| T-28 | Elevation of Privilege | Migration Store (SQLite) | Low | mitigate | SQL injection could escalate to arbitrary data access | All SQL uses parameterized queries with `?` placeholders. No string concatenation of user input in query construction (`migrate/sqlstore/store.go`) |
| T-29 | Spoofing | Domain Connect | High | mitigate | SSRF via discovery DNS TXT record pointing to internal IP | IP blocklist rejects loopback, private, link-local, multicast, and unspecified addresses; redirect validation re-checks IPs and blocks non-HTTPS redirects; max 5 redirects (`domainconnect/discovery.go:68-83, 203-217`) |
| T-30 | Spoofing | Domain Connect | Medium | mitigate | Discovery domain could contain injection characters | `validateDomain()` rejects control chars, spaces, consecutive dots, characters `/?#[]@!$&'()+,;=`, requires TLD, max 253 chars (`domainconnect/discovery.go:183-201`) |
| T-31 | Tampering | Crypto | Low | mitigate | Credential blob tampering could go undetected | AES-256-GCM provides authenticated encryption; any tampering detected by GCM auth tag on decrypt (`migrate/crypt.go:115-138`) |
| T-32 | Tampering | Crypto | Low | mitigate | Nonce reuse could weaken AES-GCM | Random 12-byte nonce from `crypto/rand` per encryption; no nonce counter or reuse (`migrate/crypt.go:107-109`) |
| T-33 | Information Disclosure | Crypto | Medium | accept | Hostname-derived key fallback is weaker than explicit key | Warning emitted at startup when using derived key. `ENTREE_STATE_KEY` env var is documented as recommended for production (`migrate/crypt.go:44`, `api/server.go:95`) |
| T-34 | Tampering | Domain Connect Signing | Low | mitigate | Incorrect URL encoding could allow signature bypass | `SortAndSignParams` uses `url.QueryEscape` with targeted `+` to `%20` replacement per DC spec; standard `url.Values.Encode()` intentionally avoided (`domainconnect/signing.go:89-113`) |
| T-35 | Information Disclosure | HTTP API | Low | mitigate | Panic stack traces could leak internal paths | `recoverMW` catches panics, logs stack to server log only, returns generic `internal server error` to client (`api/middleware.go:56-73`) |
| T-36 | Denial of Service | Migration Apply | Low | mitigate | Rapid record writes could overwhelm target provider | `WriteLimiter` rate-limits provider writes during migration apply (`api/handlers_migrate_stateful.go:337`) |

### Component Analysis

#### Credential Handling

**Current state:** Defense-in-depth at 4 layers prevents credential leakage.

**Architecture:**
1. **Layer 1 - Header stripping:** `credentialRedactMW` clones the request, saves original credential headers to context, strips them from the clone. All downstream middleware (including slog) sees only the scrubbed request (`api/middleware.go:180-198`).
2. **Layer 2 - Context-only access:** Handlers retrieve credentials via `originalCredentialHeader()` which reads from context, not request headers (`api/middleware.go:204-213`).
3. **Layer 3 - Structured logging:** `slogMW` never inspects request headers (D-25). Logs only method, path, route, status, duration, request ID, remote addr (`api/middleware.go:117-126`).
4. **Layer 4 - Error scrubbing:** `scrubDetails()` strips keys named `value`, `token`, `secret`, `password`, `key` from error detail maps before serialization (`api/errors.go:49-72`).

**Test coverage:** `api/security_test.go` provides exhaustive verification including edge-case values with special characters (quotes, slashes, base64 padding).

**References:**
- `api/credentials.go` - Credential extraction, `credentialHeaderNames` set
- `api/middleware.go:180-213` - Redaction middleware + context helper
- `api/errors.go:49-72` - Error detail scrubbing
- `api/security_test.go:17-74` - Exhaustive redaction test

#### SSRF Protection

**Current state:** Multi-layer SSRF defense for Domain Connect discovery.

**Controls:**
1. **IP blocklist:** `ipBlocked()` rejects loopback, private (RFC 1918), link-local unicast/multicast, unspecified, and multicast addresses (`domainconnect/discovery.go:68-71`).
2. **All-IP check:** `anyIPBlocked()` rejects if ANY resolved IP is blocked, or if resolution returns zero IPs (`domainconnect/discovery.go:73-83`).
3. **Redirect validation:** `makeCheckRedirect()` re-resolves and re-checks IPs on every redirect, blocks non-HTTPS redirects, caps at 5 redirects (`domainconnect/discovery.go:203-217`).
4. **Response size limit:** `io.LimitReader(resp.Body, 64*1024)` caps response body at 64 KiB (`domainconnect/discovery.go:157`).
5. **Domain validation:** `validateDomain()` rejects injection characters, consecutive dots, missing TLD, and domains exceeding 253 chars (`domainconnect/discovery.go:183-201`).

**Fuzz coverage:** `domainconnect/discovery_fuzz_test.go` fuzzes domain validation.

**References:**
- `domainconnect/discovery.go:68-83` - IP blocklist
- `domainconnect/discovery.go:203-217` - Redirect validation
- `domainconnect/discovery_fuzz_test.go` - Fuzz target

#### Cryptographic Operations

**Current state:** Industry-standard crypto primitives properly applied.

**AES-256-GCM credential encryption:**
- 32-byte key required (validated at encrypt/decrypt boundaries)
- Random 12-byte nonce per encryption via `crypto/rand`
- GCM authentication tag detects tampering on decrypt
- Blob format: `nonce || ciphertext || tag`
- References: `migrate/crypt.go:95-138`

**RSA-SHA256 Domain Connect signing:**
- PKCS1v15 signing with SHA-256 digest
- Supports both PKCS8 and PKCS1 PEM formats
- Key hash uses SHA-256 of SubjectPublicKeyInfo DER for key identification
- URL encoding uses `QueryEscape` with `+` to `%20` replacement per DC spec
- References: `domainconnect/signing.go`

**Key management:**
- Production: `ENTREE_STATE_KEY` env var (64 hex chars = 32 bytes)
- Development fallback: `SHA-256(hostname || random-salt)` with O_EXCL salt file creation
- Warning log emitted when using derived key
- References: `migrate/crypt.go:24-91`

**Positive findings:**
- No custom crypto algorithms; all standard library primitives
- No ECB mode, no static IVs, no hardcoded keys
- Proper nonce generation from `crypto/rand`
- Key length validation at both encrypt and decrypt boundaries

#### Input Validation

**Current state:** Comprehensive validation added in Phase 09-01.

**DNS name validation (`validate.go`):**
- Rejects control characters (0x00-0x20, 0x7F)
- Enforces max 253 characters total, 63 per label
- Requires at least two labels (must have TLD)
- Rejects leading/trailing hyphens in labels
- Allows underscores (DKIM/SRV), wildcards as first label, trailing dots (FQDN)
- Fuzz-tested via `validate_fuzz_test.go`

**Record value validation (`validate.go`):**
- A records: must be valid IPv4 (`net.ParseIP` + `To4()` check)
- AAAA records: must be valid IPv6 (not IPv4)
- CNAME/NS records: must be valid DNS name
- MX records: non-negative integer priority + target
- TXT records: max 4096 characters
- SRV records: non-empty (basic check)
- Unknown types: pass through (lenient policy)

**API integration:** All domain-accepting endpoints call `ValidateDNSName()` before any network I/O. Record apply endpoints validate each record's value via `ValidateRecordValue()`.

**References:**
- `validate.go` - Validation functions
- `validate_fuzz_test.go` - Fuzz targets
- `api/handlers_core.go:182, 218, 298` - API integration points

#### HTTP API Security

**Current state:** Hardened server configuration with proper middleware ordering.

**Server configuration (`api/server.go:191-197`):**
- `ReadHeaderTimeout`: 10 seconds (prevents slowloris)
- `ReadTimeout`: 30 seconds
- `WriteTimeout`: Configurable, default 15 minutes (long for DNS operations)
- `IdleTimeout`: 120 seconds

**Middleware chain (`api/server.go:178-186`):**
Order (outermost first): `recover` -> `requestID` -> `credentialRedact` -> `slog` -> `cors` -> `mux`

Key invariant: `credentialRedact` runs BEFORE `slog` so logging cannot observe raw credential headers.

**Request handling:**
- Content-Type enforcement: `requireJSON()` rejects non-JSON requests
- Method enforcement: `requireMethod()` returns 405 with Allow header
- Request ID: Generated via 16 bytes from `crypto/rand` or echoed from `X-Request-ID`
- Panic recovery: `recoverMW` catches panics, logs stack, returns generic 500

**References:**
- `api/server.go` - Server configuration and middleware chain
- `api/middleware.go` - All middleware implementations
- `api/handlers_core.go:114-157` - Request helpers

#### Template Engine Security

**Current state:** Secure template loading with path traversal prevention.

**Controls:**
- Provider/service IDs validated: rejects `/`, `\`, and `..` path traversal (`template/sync.go:254-262`)
- Git clone uses hardcoded official repo URL by default (`template/sync.go:21`)
- `WithRepoURL` override exists only for testing
- `validateHost()` and `validatePointsTo()` reject control characters and validate DNS structure in resolved templates (`template/resolve.go`)
- Depth-1 shallow clone minimizes attack surface

**References:**
- `template/sync.go` - Clone/sync logic, ID validation
- `template/resolve.go` - Template resolution with host/target validation

#### Migration Store

**Current state:** Two backends, both with proper access controls.

**JSON store (`migrate/store_json.go`):**
- Directory mode 0700, file mode 0600
- Per-row file locking via sidecar `.lock` files
- Atomic writes via temp file + rename
- Write probe on initialization to verify directory permissions
- Optimistic locking via Version field

**SQLite store (`migrate/sqlstore/store.go`):**
- WAL journal mode with single writer (`MaxOpenConns(1)`)
- 5-second busy timeout prevents lock contention deadlocks
- All queries parameterized with `?` placeholders
- Optimistic locking via `UPDATE ... WHERE version = ?`
- Build-tag gated (`sqlite`) so default builds do not link SQLite

**References:**
- `migrate/store_json.go` - JSON file-based store
- `migrate/sqlstore/store.go` - SQLite store

### Dependencies

| Dependency | Version | Purpose | Security Notes |
|------------|---------|---------|----------------|
| `github.com/aws/aws-sdk-go-v2` | v1.41.4 | AWS Route53 provider | Official AWS SDK; handles STS, signing |
| `github.com/cloudflare/cloudflare-go` | v0.116.0 | Cloudflare provider | Official Cloudflare SDK |
| `github.com/go-git/go-git/v5` | v5.17.2 | Template repo clone/sync | Pure Go git; handles HTTPS cloning |
| `github.com/google/uuid` | v1.6.0 | Migration ID generation | UUID v4 from crypto/rand |
| `github.com/miekg/dns` | v1.1.72 | DNS resolution, verification | Widely-used DNS library |
| `github.com/spf13/cobra` | v1.8.1 | CLI framework | No security surface |
| `golang.org/x/sys` | v0.43.0 | System calls (file locking) | Official Go extended library |
| `golang.org/x/time` | v0.9.0 | Rate limiter for migration writes | Official Go extended library |
| `modernc.org/sqlite` | v1.48.1 | Pure Go SQLite (optional, build-tag gated) | No CGO dependency; compiled to Go |
| `github.com/ProtonMail/go-crypto` | v1.1.6 | Crypto primitives (indirect, via go-git) | Fork of golang.org/x/crypto with ed25519 |

**Dependency audit notes:**
- No known CVEs in current versions as of audit date
- All crypto operations use Go standard library (`crypto/aes`, `crypto/rsa`, `crypto/sha256`, `crypto/subtle`), not third-party crypto libraries
- `modernc.org/sqlite` is build-tag gated; default builds have zero SQLite surface
- `go-git` is the only dependency that performs network I/O at import time (template sync), constrained to a single hardcoded HTTPS URL

### Recommendations

**Priority-ordered future improvements:**

1. **Key rotation tooling** (Medium): The `rotate-state-key` subcommand is planned but not shipped. Currently rotation requires manual re-encryption or migration recreation.

2. **Rate limiting middleware** (Low): Built-in rate limiting on stateless endpoints would provide defense-in-depth beyond reverse proxy reliance. The `golang.org/x/time/rate` dependency is already present (used for migration writes).

3. **Credential struct String() method** (Low): Adding a `String()` method to `Credentials` that returns `[REDACTED]` would prevent accidental leakage via `fmt.Printf("%v", creds)`. Currently documented in `security_test.go` that callers must not log the struct directly.

4. **CSP headers** (Info): If the API ever serves HTML (currently JSON-only), add Content-Security-Policy headers.

5. **Dependency update automation** (Info): Consider Dependabot/Renovate for automated security updates of Go module dependencies.

### Positive Findings

The following security-positive patterns were verified during this audit:

1. **Defense-in-depth credential handling**: 4-layer redaction architecture prevents credential leakage even if one layer fails. Empirically tested in `api/security_test.go`.

2. **SSRF blocklist with redirect validation**: Every DNS resolution and HTTP redirect re-checks against the IP blocklist. Non-HTTPS redirects are rejected. Response size is capped.

3. **AES-256-GCM with proper nonce handling**: Random nonces from `crypto/rand`, no nonce reuse patterns, authentication tag validates integrity.

4. **O_EXCL salt file creation**: Prevents race conditions in key derivation fallback path (`migrate/crypt.go:63`).

5. **Constant-time token comparison**: `crypto/subtle.ConstantTimeCompare` for migration bearer tokens prevents timing attacks (`api/handlers_migrate_stateful.go:68`).

6. **Parameterized SQL throughout**: Zero string concatenation in SQL queries across the entire SQLite store implementation.

7. **DisallowUnknownFields on all JSON decoders**: Rejects unexpected fields, preventing silent data loss and potential injection vectors.

8. **MaxBytesReader on all endpoints**: Consistent body size limits prevent memory exhaustion attacks.

9. **Structured logging with request IDs**: Every request gets a unique 128-bit ID from `crypto/rand`, enabling audit trail correlation.

10. **Middleware ordering enforces security invariants**: `credentialRedact` before `slog` is an architectural guarantee, not just a convention.

11. **Template ID path traversal prevention**: `validateID()` rejects `/`, `\`, and `..` in provider/service IDs before filesystem access.

12. **Atomic file operations**: JSON store uses temp file + rename for crash-safe writes; file modes enforce 0600/0700 access control.

### Phase 09 Fixes Applied

The following issues were identified and fixed during this security audit phase:

| Issue | Severity | Fix | Reference |
|-------|----------|-----|-----------|
| No DNS name validation | Medium | Added `ValidateDNSName()` with RFC 1035 compliance | `validate.go` (Plan 09-01) |
| No record value validation | Medium | Added `ValidateRecordValue()` with type-specific checks | `validate.go` (Plan 09-01) |
| API endpoints accepted arbitrary domain input | Medium | Wired `ValidateDNSName()` into all domain-accepting handlers | `api/handlers_core.go` (Plan 09-01) |
| No Domain Connect domain validation | Medium | Added `validateDomain()` in discovery package | `domainconnect/discovery.go` (Plan 09-01) |
| No fuzz testing for parsers | Low | Added 3 fuzz targets: SPF merge, DC discovery, DNS validation | `spf_fuzz_test.go`, `domainconnect/discovery_fuzz_test.go`, `validate_fuzz_test.go` (Plan 09-02) |
| No security-focused test suite | Low | Added credential redaction, SSRF bypass, error scrubbing, timing-safe tests | `security_test.go`, `api/security_test.go` (Plan 09-02) |
