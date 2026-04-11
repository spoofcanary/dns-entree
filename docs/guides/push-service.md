# PushService Guide

`PushService` is the idempotent write surface on top of any `entree.Provider`. It implements read-before-write so repeated calls converge on the same zone state.

## Construction

```go
svc := entree.NewPushService(prov)
```

One PushService per Provider; they are safe to reuse across goroutines provided the underlying provider client is.

## Typed Methods

| Method | Record type | Comparison |
| --- | --- | --- |
| `PushTXTRecord(ctx, domain, name, content)` | TXT | exact string match |
| `PushCNAMERecord(ctx, domain, name, target)` | CNAME | case- and trailing-dot-insensitive |
| `PushSPFRecord(ctx, domain, includes)` | SPF (TXT at apex) | include-set aware, see [spf-merge.md](spf-merge.md) |
| `PushGenericRecord(ctx, domain, record)` | A/AAAA/MX/NS/SRV | exact; MX also compares priority |

TXT and CNAME must use the typed methods. Passing them to `PushGenericRecord` returns an error.

## PushResult

```go
type PushResult struct {
    Status        PushStatus // created | updated | already_configured | failed
    RecordName    string
    RecordValue   string
    PreviousValue string // set when Status == updated
    Verified      bool
    VerifyError   error
}
```

`Status` tells you what happened at the provider:

- `created` - no prior record existed
- `updated` - a record at the same name existed with different content; overwritten
- `already_configured` - a record at the same name already had the desired content; nothing written
- `failed` - the provider call errored; the returned error has details

## Verify Semantics

After a successful write, PushService issues a DNS lookup against the authoritative nameservers (with recursive fallback) looking for the record. The result lands in `Verified` and `VerifyError`.

- `Verified == true` - authoritative DNS observed the record
- `Verified == false && VerifyError == nil` - look again later; propagation delay is common, not a hard failure
- `VerifyError != nil` - verification itself failed (network, NXDOMAIN, wrong type); the push itself already succeeded

Verification never aborts a write. The idempotent write is the source of truth; verification is an observability signal.

## Error Model

Non-nil `error` from a Push* call always accompanies `Status == failed`. Wrap with `errors.Is` / `errors.As` to inspect provider-specific errors. The returned `*PushResult` is still populated with the attempted name/value for logging.

## Example: Idempotent Run Loop

```go
for _, domain := range domains {
    res, err := svc.PushTXTRecord(ctx, domain, "_dmarc."+domain,
        "v=DMARC1; p=none; rua=mailto:rua@example.com")
    if err != nil {
        log.Printf("%s: %v", domain, err)
        continue
    }
    switch res.Status {
    case entree.StatusCreated, entree.StatusUpdated:
        log.Printf("%s: %s (verified=%v)", domain, res.Status, res.Verified)
    case entree.StatusAlreadyConfigured:
        // no-op
    }
}
```

## See Also

- [spf-merge.md](spf-merge.md)
- [templates.md](templates.md)
- [../recipes/push-dmarc.md](../recipes/push-dmarc.md)
