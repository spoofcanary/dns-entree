// Package domainconnect implements Domain Connect v2 discovery, RSA-SHA256
// query-string signing, and signed async apply URL construction.
//
// Typical flow:
//
//  1. [Discover] probes a domain for Domain Connect support (TXT settings
//     host lookup + HTTPS settings fetch with SSRF screening).
//  2. [LoadPrivateKey] parses a PEM-encoded RSA key.
//  3. [BuildApplyURL] assembles a signed async apply URL the user can be
//     redirected to in order to apply a template at their DNS provider.
//
// [SignQueryString] is exposed for callers that need to sign a query string
// directly (for example, to sign a custom URL format).
//
// # Stability
//
// Stable. Public API covered by semver from v1.0.0 forward.
package domainconnect
