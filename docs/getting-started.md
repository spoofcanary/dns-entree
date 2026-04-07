# Getting Started

Install dns-entree, construct a provider, and push your first DNS record in a few lines of Go.

## Install

```
go get github.com/spoofcanary/dns-entree
```

Provider integrations live under subpackages; import the ones you use. Each provider's `init()` registers a factory so `entree.NewProvider("cloudflare", creds)` also works.

```go
import (
    entree "github.com/spoofcanary/dns-entree"
    _ "github.com/spoofcanary/dns-entree/providers/cloudflare"
)
```

## Construct a Provider

Credentials live in `entree.Credentials`. Each provider uses only the fields it needs.

```go
creds := entree.Credentials{
    APIToken: os.Getenv("CLOUDFLARE_API_TOKEN"),
}
prov, err := entree.NewProvider("cloudflare", creds)
if err != nil {
    log.Fatal(err)
}
```

You can also call the package constructor directly:

```go
import "github.com/spoofcanary/dns-entree/providers/cloudflare"

prov, err := cloudflare.NewProvider(os.Getenv("CLOUDFLARE_API_TOKEN"))
```

## First Push

`PushService` wraps any `entree.Provider` with idempotent upsert + post-push DNS verification.

```go
ctx := context.Background()
svc := entree.NewPushService(prov)

res, err := svc.PushTXTRecord(ctx, "example.com", "_dmarc.example.com",
    "v=DMARC1; p=none; rua=mailto:dmarc@example.com")
if err != nil {
    log.Fatalf("push: %v", err)
}

log.Printf("status=%s verified=%v", res.Status, res.Verified)
```

`res.Status` is one of `created`, `updated`, `already_configured`, or `failed`. Re-running the same call yields `already_configured`.

## Verify a Record Independently

`entree.Verify` queries authoritative nameservers (falling back to recursive) and returns what the zone actually serves.

```go
vr, err := entree.Verify(ctx, "example.com", entree.VerifyOpts{
    RecordType: "TXT",
    Name:       "_dmarc.example.com",
    Contains:   "v=DMARC1",
})
log.Printf("verified=%v current=%q method=%s", vr.Verified, vr.CurrentValue, vr.Method)
```

## Detect a Provider

`entree.DetectProvider` resolves NS records and, when NS is ambiguous, falls back to RDAP registrar lookup.

```go
d, err := entree.DetectProvider(ctx, "example.com")
if err != nil {
    log.Fatal(err)
}
log.Printf("%s (%s) supported=%v via=%s", d.Label, d.Provider, d.Supported, d.Method)
```

## Next Steps

- [guides/push-service.md](guides/push-service.md) - idempotent push + verify
- [guides/spf-merge.md](guides/spf-merge.md) - merging SPF includes safely
- [guides/templates.md](guides/templates.md) - Domain Connect templates
- [providers/cloudflare.md](providers/cloudflare.md) - credential setup per provider
