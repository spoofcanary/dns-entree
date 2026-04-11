// Package cloudflare implements the [entree.Provider] interface for
// Cloudflare DNS using cloudflare-go.
//
// Authentication uses a scoped API token ([entree.Credentials.APIToken]) with
// Zone:Read and DNS:Edit permissions on the target zones. The provider is
// auto-registered under the slug "cloudflare" via an init() hook, so callers
// using [entree.NewProvider] only need to import this package for its side
// effects:
//
//	import _ "github.com/spoofcanary/dns-entree/providers/cloudflare"
//
// # Stability
//
// Stable. Public API covered by semver from v1.0.0 forward.
package cloudflare
