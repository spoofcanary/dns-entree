# SPF Merge Guide

`entree.MergeSPF` folds a list of `include:` includes into an existing SPF record without disturbing the rest. `PushService.PushSPFRecord` is the convenience wrapper that fetches the current apex TXT, calls MergeSPF, and writes only if the result changed.

## Why a Dedicated Merger

SPF records have painful constraints:

- There can be only one `v=spf1` record per domain.
- Order of mechanisms matters; `-all` / `~all` must stay at the end.
- RFC 7208 caps total DNS lookups at 10. Exceeding the cap causes `permerror` at receivers.
- Naive string concatenation breaks existing `include:`, `ip4:`, and `redirect=` terms.

Callers who want to add an include (a new ESP, a monitoring vendor) should never hand-edit the record.

## API

```go
type MergeResult struct {
    Value               string   // the merged record
    Changed             bool     // false if includes were already present
    BrokenInput         bool     // existing record was unparseable; replaced
    LookupCount         int      // total DNS-lookup mechanisms in Value
    LookupLimitExceeded bool     // LookupCount > 10
    Warnings            []string // human-readable notes
}

func MergeSPF(current string, includes []string) (MergeResult, error)
```

## Merge Rules

1. **Empty input** - returns a fresh `v=spf1 include:... ~all`.
2. **Non-SPF input** - logged as `BrokenInput`, replaced with a fresh record (warning appended).
3. **Already present** - includes already in the record are skipped; `Changed=false` if all were present.
4. **Insertion point** - new `include:` terms are inserted immediately before the first terminator (`-all`, `~all`, `?all`, `+all`) or before `redirect=`. If neither exists, `-all` is appended.
5. **Order preserving** - existing mechanisms are kept in their original order.

## The 10-Lookup Caveat

Each `include:`, `a`, `mx`, `a:`, `mx:`, `ptr:`, `exists:`, and `redirect=` term counts as one DNS lookup. The full chain (including nested includes at the receiver) has a hard cap of 10. dns-entree counts only the terms visible in the record itself and sets `LookupLimitExceeded` when that local count exceeds 10.

```go
res, _ := entree.MergeSPF(current, []string{"servers.mcsv.net"})
if res.LookupLimitExceeded {
    for _, w := range res.Warnings {
        log.Printf("spf warning: %s", w)
    }
}
```

The warning surfaces as:

    SPF record exceeds 10 DNS lookup limit (RFC 7208); mail receivers may return permerror

You should still write the record (the merge is correct), then schedule a cleanup: consolidate includes, remove unused ESPs, or switch to `ip4:` blocks.

## When to Use MergeSPF vs PushSPFRecord

- Call `MergeSPF` directly when you want to inspect the result or present a diff to a user.
- Call `PushService.PushSPFRecord(ctx, domain, includes)` when you want the merge + write + verify in one call; it sets `PushResult.PreviousValue` to the pre-merge record.

## See Also

- [push-service.md](push-service.md)
- RFC 7208 section 4.6.4
