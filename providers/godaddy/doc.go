// Package godaddy implements the [entree.Provider] interface for GoDaddy DNS
// using the GoDaddy v1 REST API.
//
// Authentication uses an API key/secret pair
// ([entree.Credentials.APIKey] + [entree.Credentials.APISecret]) sent via the
// sso-key Authorization header. Note that as of 2024 GoDaddy restricts
// production API access to accounts with 10+ domains; smaller accounts will
// receive 403s. The provider is auto-registered under the slug "godaddy" via
// init():
//
//	import _ "github.com/spoofcanary/dns-entree/providers/godaddy"
//
// # Stability
//
// Stable. Public API covered by semver from v1.0.0 forward.
package godaddy
