# Recipe: Push DKIM CNAMEs via Route 53

Walkthrough: install two DKIM CNAME records that delegate key publication to an ESP (e.g. Google Workspace, SendGrid, Mailgun).

## Prerequisites

- AWS credentials with the IAM policy in [../providers/route53.md](../providers/route53.md)
- Route 53 hosted zone for `example.com`
- DKIM hostnames provided by your ESP (e.g. `s1._domainkey` pointing at `s1.domainkey.u1234.wl.sendgrid.net`)

## Full Program

```go
package main

import (
    "context"
    "log"
    "os"

    entree "github.com/spoofcanary/dns-entree"
    _ "github.com/spoofcanary/dns-entree/providers/route53"
)

type dkim struct {
    Name   string
    Target string
}

func main() {
    ctx := context.Background()

    prov, err := entree.NewProvider("route53", entree.Credentials{
        AccessKey: os.Getenv("AWS_ACCESS_KEY_ID"),
        SecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
        Region:    "us-east-1",
    })
    if err != nil {
        log.Fatalf("provider: %v", err)
    }

    svc := entree.NewPushService(prov)

    records := []dkim{
        {"s1._domainkey.example.com", "s1.domainkey.u1234.wl.sendgrid.net"},
        {"s2._domainkey.example.com", "s2.domainkey.u1234.wl.sendgrid.net"},
    }

    for _, r := range records {
        res, err := svc.PushCNAMERecord(ctx, "example.com", r.Name, r.Target)
        if err != nil {
            log.Printf("%s: FAIL %v", r.Name, err)
            continue
        }
        log.Printf("%s: %s verified=%v", r.Name, res.Status, res.Verified)
    }
}
```

## Expected Output

```
s1._domainkey.example.com: created verified=true
s2._domainkey.example.com: created verified=true
```

Re-running yields `already_configured` for both. `PushCNAMERecord` normalizes trailing dots and case when comparing existing records, so `sendgrid.net` and `SendGrid.net.` are treated as equal.

## Notes

- DKIM selectors (`s1`, `s2`, `google`, etc.) are ESP-specific; check your ESP's setup page for the exact names.
- If the ESP gives you a public key directly (a long base64 blob) you want `PushTXTRecord` instead, at the selector name.

## See Also

- [../providers/route53.md](../providers/route53.md)
- [../guides/push-service.md](../guides/push-service.md)
