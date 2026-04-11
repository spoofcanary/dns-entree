# Recipe: Migrate from Bash Scripts

If you currently manage DNS with `dig` + `curl` + provider-specific bash, this recipe shows the one-to-one replacement in dns-entree.

## Before: Bash

```bash
# Check existing record
dig +short TXT _dmarc.example.com @1.1.1.1

# Write via Cloudflare API
ZONE_ID=$(curl -s -H "Authorization: Bearer $CF_TOKEN" \
  "https://api.cloudflare.com/client/v4/zones?name=example.com" \
  | jq -r '.result[0].id')

REC_ID=$(curl -s -H "Authorization: Bearer $CF_TOKEN" \
  "https://api.cloudflare.com/client/v4/zones/$ZONE_ID/dns_records?type=TXT&name=_dmarc.example.com" \
  | jq -r '.result[0].id')

if [ "$REC_ID" = "null" ]; then
  curl -X POST -H "Authorization: Bearer $CF_TOKEN" \
    -d '{"type":"TXT","name":"_dmarc.example.com","content":"v=DMARC1; p=none","ttl":300}' \
    "https://api.cloudflare.com/client/v4/zones/$ZONE_ID/dns_records"
else
  curl -X PUT -H "Authorization: Bearer $CF_TOKEN" \
    -d '{"type":"TXT","name":"_dmarc.example.com","content":"v=DMARC1; p=none","ttl":300}' \
    "https://api.cloudflare.com/client/v4/zones/$ZONE_ID/dns_records/$REC_ID"
fi

# Verify
dig +short TXT _dmarc.example.com @1.1.1.1
```

Problems:

- Separate create/update code paths
- No idempotency check (rewrites identical records)
- Bespoke per provider (rewrite for Route 53, GoDaddy, etc.)
- Verification relies on whatever resolver `dig` happens to use
- Errors in JSON responses are not inspected

## After: dns-entree

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
    prov, err := entree.NewProvider("cloudflare", entree.Credentials{
        APIToken: os.Getenv("CF_TOKEN"),
    })
    if err != nil {
        log.Fatal(err)
    }
    svc := entree.NewPushService(prov)

    res, err := svc.PushTXTRecord(context.Background(),
        "example.com", "_dmarc.example.com", "v=DMARC1; p=none")
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("%s verified=%v", res.Status, res.Verified)
}
```

Same behavior, minus the sharp edges:

| Bash concern | dns-entree answer |
| --- | --- |
| Separate create/update | One call, status tells you which happened |
| Idempotency | Read-before-write built in |
| Per-provider rewrites | Swap the import; same API |
| Verification | Authoritative NS lookup with recursive fallback |
| Error inspection | Typed `error` + `PushResult.VerifyError` |

## Mapping Common Operations

| Bash | dns-entree |
| --- | --- |
| `dig +short TXT _dmarc.example.com` | `entree.Verify(ctx, domain, entree.VerifyOpts{RecordType: "TXT", Name: "_dmarc.example.com"})` |
| `curl -X POST .../dns_records` (create) | `svc.PushTXTRecord(...)` |
| `curl -X PUT .../dns_records/<id>` (update) | `svc.PushTXTRecord(...)` (same call) |
| `curl -X DELETE .../dns_records/<id>` | `prov.DeleteRecord(ctx, domain, recordID)` |
| Provider-specific SPF append | `svc.PushSPFRecord(ctx, domain, []string{"include:...")` |

## See Also

- [../guides/push-service.md](../guides/push-service.md)
- [../guides/spf-merge.md](../guides/spf-merge.md)
