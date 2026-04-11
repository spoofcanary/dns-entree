# Recipe: Push a DMARC Record via Cloudflare

End-to-end walkthrough: set a DMARC policy record at `_dmarc.example.com` on a Cloudflare-hosted zone.

## Prerequisites

- `CLOUDFLARE_API_TOKEN` with `Zone:Read` and `Zone:Edit` scoped to the zone (see [../providers/cloudflare.md](../providers/cloudflare.md))
- Go 1.21+ and dns-entree: `go get github.com/spoofcanary/dns-entree`

## Full Program

```go
package main

import (
    "context"
    "log"
    "os"

    entree "github.com/spoofcanary/dns-entree"
    _ "github.com/spoofcanary/dns-entree/providers/cloudflare"
)

func main() {
    ctx := context.Background()

    prov, err := entree.NewProvider("cloudflare", entree.Credentials{
        APIToken: os.Getenv("CLOUDFLARE_API_TOKEN"),
    })
    if err != nil {
        log.Fatalf("provider: %v", err)
    }

    svc := entree.NewPushService(prov)

    res, err := svc.PushTXTRecord(ctx,
        "example.com",
        "_dmarc.example.com",
        "v=DMARC1; p=none; rua=mailto:dmarc@example.com; ruf=mailto:dmarc@example.com; fo=1",
    )
    if err != nil {
        log.Fatalf("push: %v", err)
    }

    log.Printf("status=%s verified=%v prev=%q",
        res.Status, res.Verified, res.PreviousValue)
}
```

## Expected Output

First run:

```
status=created verified=true prev=""
```

Second run (idempotent):

```
status=already_configured verified=false prev=""
```

`verified=false` on the second run is expected: `already_configured` skips the post-push verify because no write occurred.

## Troubleshooting

- `cloudflare api: HTTP status 403` - token is missing `Zone:Edit` on this zone
- `status=failed` with context cancelled - check your ctx / timeout
- `verified=false` after `created` - propagation delay; query again in 30s

## See Also

- [../guides/push-service.md](../guides/push-service.md)
- [../providers/cloudflare.md](../providers/cloudflare.md)
