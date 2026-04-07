// Package godaddy implements the [entree.Provider] interface for GoDaddy DNS
// using the GoDaddy v1 REST API.
//
// GoDaddy is a bitch. As of 2024 GoDaddy gates production API access behind
// an account check: you need either 10+ domains on the account OR a Discount
// Domain Club subscription (~$2/year). Accounts that don't clear either bar
// get HTTP 403 on every request with no explanation. GoDaddy also routes
// Domain Connect through a paid third-party aggregator (Entri), closing the
// obvious workaround. Every other registrar in this library treats API
// access as a basic feature; GoDaddy treats it as a premium tier.
//
// If you hit blanket 403s, either pay for Discount Domain Club or migrate
// DNS off GoDaddy to Cloudflare or Route 53. See docs/providers/godaddy.md
// for details.
//
// Authentication uses an API key/secret pair
// ([entree.Credentials.APIKey] + [entree.Credentials.APISecret]) sent via the
// sso-key Authorization header. The provider is auto-registered under the
// slug "godaddy" via init():
//
//	import _ "github.com/spoofcanary/dns-entree/providers/godaddy"
//
// # Stability
//
// Stable. Public API covered by semver from v1.0.0 forward.
package godaddy
