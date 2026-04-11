// Package template implements a Domain Connect template loader, variable
// resolver, and end-to-end applier that pushes resolved records through a
// dns-entree [entree.PushService].
//
// The typical flow is:
//
//	tmpl, err := template.LoadFile("dmarc.json")
//	if err != nil { log.Fatal(err) }
//	results, err := template.ApplyTemplate(ctx, pushSvc, "example.com", tmpl,
//	    map[string]string{"domain": "example.com"})
//
// [Sync] / [SyncAndLoad] fetch and cache the upstream Domain-Connect/Templates
// repository so callers can pick a template by (providerID, serviceID) without
// shipping the JSON in their own binary.
//
// Supported record types: TXT, CNAME, A, AAAA, MX, NS, SRV, SPFM. SPFM entries
// are auto-routed through [entree.PushService.PushSPFRecord] with merge
// semantics; TXT entries honor txtConflictMatchingMode per the Domain Connect
// spec.
//
// # Stability
//
// Stable. Public API covered by semver from v1.0.0 forward.
package template
