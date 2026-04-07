// Package gcdns implements the [entree.Provider] interface for Google Cloud
// DNS using the v1 REST API.
//
// Authentication uses an OAuth2 bearer token ([entree.Credentials.Token]) and
// a GCP project ID ([entree.Credentials.ProjectID]). The caller is responsible
// for obtaining and refreshing the token (for example, via the google.golang.org/api
// auth libraries or a workload-identity source). The provider is auto-registered
// under the slug "google_cloud_dns" via init():
//
//	import _ "github.com/spoofcanary/dns-entree/providers/gcdns"
//
// # Stability
//
// Stable. Public API covered by semver from v1.0.0 forward.
package gcdns
