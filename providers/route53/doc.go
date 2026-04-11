// Package route53 implements the [entree.Provider] interface for AWS Route 53
// using aws-sdk-go-v2.
//
// Authentication uses IAM access/secret keys ([entree.Credentials.AccessKey]
// and [entree.Credentials.SecretKey]) with the route53:ListHostedZones,
// route53:ListResourceRecordSets, and route53:ChangeResourceRecordSets
// permissions. ApplyRecords uses a single ChangeBatch for efficiency. The
// provider is auto-registered under the slug "route53" via init():
//
//	import _ "github.com/spoofcanary/dns-entree/providers/route53"
//
// # Stability
//
// Stable. Public API covered by semver from v1.0.0 forward.
package route53
