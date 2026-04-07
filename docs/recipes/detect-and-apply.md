# Recipe: Detect + Apply a Domain Connect Template

End-to-end: given a domain and a template, detect the hosting provider, sync the official Domain Connect template repo, load the requested template, and apply it.

## Prerequisites

- API credentials for at least one supported provider (so apply can write)
- Network access for RDAP fallback and the Domain Connect template repo

## Full Program

```go
package main

import (
    "context"
    "log"
    "os"

    entree "github.com/spoofcanary/dns-entree"
    _ "github.com/spoofcanary/dns-entree/providers/cloudflare"
    _ "github.com/spoofcanary/dns-entree/providers/route53"
    "github.com/spoofcanary/dns-entree/template"
)

func main() {
    ctx := context.Background()
    domain := "example.com"

    // 1. Detect the hosting provider.
    d, err := entree.DetectProvider(ctx, domain)
    if err != nil {
        log.Fatalf("detect: %v", err)
    }
    log.Printf("provider=%s label=%q supported=%v via=%s",
        d.Provider, d.Label, d.Supported, d.Method)

    if !d.Supported {
        log.Fatalf("no direct integration for %s; use Domain Connect signed URLs instead", d.Label)
    }

    // 2. Build credentials based on the detected provider.
    var creds entree.Credentials
    switch d.Provider {
    case entree.ProviderCloudflare:
        creds.APIToken = os.Getenv("CLOUDFLARE_API_TOKEN")
    case entree.ProviderRoute53:
        creds.AccessKey = os.Getenv("AWS_ACCESS_KEY_ID")
        creds.SecretKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
        creds.Region = "us-east-1"
    }

    prov, err := entree.NewProvider(string(d.Provider), creds)
    if err != nil {
        log.Fatalf("provider: %v", err)
    }
    svc := entree.NewPushService(prov)

    // 3. Sync the official template repo and load a specific template.
    if err := template.SyncTemplates(ctx); err != nil {
        log.Fatalf("sync: %v", err)
    }
    tmpl, err := template.LoadTemplate(ctx, "exampleservice.com", "dmarc")
    if err != nil {
        log.Fatalf("load: %v", err)
    }

    // 4. Apply with variables.
    vars := map[string]string{
        "rua": "mailto:rua@example.com",
    }
    results, err := template.ApplyTemplate(ctx, svc, domain, tmpl, vars)
    for _, r := range results {
        log.Printf("%s %s -> %s", r.RecordName, r.Status, r.RecordValue)
    }
    if err != nil {
        log.Fatalf("apply had errors: %v", err)
    }
}
```

## What Happens

1. `DetectProvider` resolves NS, matches patterns, falls back to RDAP if ambiguous.
2. `template.SyncTemplates` clones or refreshes `github.com/Domain-Connect/Templates` into the XDG cache.
3. `LoadTemplate` reads the JSON for `exampleservice.com/dmarc`.
4. `ApplyTemplate` resolves `%variables%`, applies TXT conflict modes, dispatches through `PushService`, and returns one `PushResult` per record.

Partial failure is expected: if one record fails, the rest still run and you get errors joined.

## See Also

- [../guides/templates.md](../guides/templates.md)
- [../guides/provider-detection.md](../guides/provider-detection.md)
- [../guides/domain-connect.md](../guides/domain-connect.md)
