// Package entree is a provider-agnostic DNS automation library for pushing
// email-auth and verification records (DMARC, DKIM, SPF, BIMI, TXT/CNAME
// proofs) across Cloudflare, Route 53, GoDaddy, and Google Cloud DNS with
// idempotent writes, post-push verification, SPF merging, Domain Connect
// discovery/signing, and a template engine.
//
// The primary entry point for most callers is [PushService], which wraps any
// [Provider] with idempotent upsert + post-push DNS verification:
//
//	p, err := cloudflare.NewProvider(os.Getenv("CF_API_TOKEN"))
//	if err != nil { log.Fatal(err) }
//	svc := entree.NewPushService(p)
//	res, err := svc.PushTXTRecord(ctx, "example.com", "_dmarc.example.com",
//	    "v=DMARC1; p=none; rua=mailto:dmarc@example.com")
//	if err != nil { log.Fatal(err) }
//	fmt.Println(res.Status, res.Verified)
//
// Other top-level helpers: [DetectProvider] (NS pattern + RDAP fallback),
// [MergeSPF] (idempotent SPF include merge), [Verify] (authoritative-first
// DNS verification), and [NewProvider] / [RegisterProvider] for plugging in
// additional providers by slug.
//
// # Stability
//
// Stable. This package is part of the public API and will be covered by
// semver from v1.0.0 forward. During the v0.x line, breaking changes may land
// in any release; see docs/stability.md for details.
package entree
