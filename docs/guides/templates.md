# Templates Guide

The `template` package loads Domain Connect template JSON, resolves `%variable%` tokens, and applies the resulting records through a `PushService`.

Use templates when you want to ship a reusable, version-controlled definition of "what DNS records does my service need?" - DMARC, DKIM, MX, verification tokens, etc.

## Loading Templates

### From Bytes or File

```go
import "github.com/spoofcanary/dns-entree/template"

tmpl, err := template.LoadTemplateJSON(jsonBytes)
// or
tmpl, err := template.LoadTemplateFile("./templates/dmarc.json")
```

Unknown JSON fields are silently dropped. Unknown record types are logged at warn level and skipped during resolve.

### From the Official Repo

`SyncTemplates` clones (or refreshes) the [Domain Connect Templates](https://github.com/Domain-Connect/Templates) repo into an XDG cache directory. `LoadTemplate` then reads a specific template from that cache.

```go
ctx := context.Background()
if err := template.SyncTemplates(ctx); err != nil {
    log.Fatal(err)
}

tmpl, err := template.LoadTemplate(ctx, "exampleservice.com", "dmarc")
if err != nil {
    log.Fatal(err)
}

// Or list everything:
summaries, err := template.ListTemplates()
```

Sync options:

```go
template.SyncTemplates(ctx,
    template.WithCacheDir("/tmp/dc-cache"),
    template.WithCacheTTL(24*time.Hour), // 0 = always refresh, <0 = never auto-refresh
    template.WithRepoURL("file:///path/to/local/fork"),
)
```

## Resolving Variables

Templates use `%varname%` tokens. `ResolveDetailed` substitutes them and returns one `ResolvedRecord` per output row.

```go
vars := map[string]string{
    "rua":  "mailto:rua@example.com",
    "ruf":  "mailto:ruf@example.com",
}
resolved, err := tmpl.ResolveDetailed(vars)
```

Each `ResolvedRecord` carries the concrete `entree.Record` plus the template's conflict-matching metadata (`Mode`, `Prefix`).

## TXT Conflict Modes

Domain Connect templates declare how to deal with pre-existing TXT records at the same name. dns-entree honors the spec's modes:

| Mode | Behavior |
| --- | --- |
| `None` (or empty) | Skip conflict resolution; write without touching other records. |
| `Prefix` | Delete any TXT at this name whose content starts with `txtConflictMatchingPrefix`. |
| `Exact` | Delete any TXT at this name whose content exactly equals the resolved value. |
| `All` | Delete every TXT at this name before writing. |

Conflict resolution only touches TXT at the exact same `Name`. Other record types are never deleted.

## Applying a Template

`template.ApplyTemplate` orchestrates resolve + conflict handling + dispatch through `PushService`:

```go
svc := entree.NewPushService(prov)
results, err := template.ApplyTemplate(ctx, svc, "example.com", tmpl, vars)
for _, r := range results {
    log.Printf("%s %s -> %s", r.RecordName, r.Status, r.RecordValue)
}
if err != nil {
    // errors are joined; processing continues so len(results) == len(tmpl records)
    log.Printf("partial failure: %v", err)
}
```

The dispatcher routes each record to the correct PushService method based on `Type`:

- `TXT` -> `PushTXTRecord`
- `CNAME` -> `PushCNAMERecord`
- `SPFM` -> `PushSPFRecord` (include extracted from the template data)
- `A`, `AAAA`, `MX`, `NS`, `SRV` -> `PushGenericRecord`

`ApplyTemplate` has partial-failure semantics: a failure on one record does not abort the rest. You always get a `PushResult` per template record, and the returned error is `errors.Join` of everything that went wrong.

## See Also

- [domain-connect.md](domain-connect.md) - when to apply via signed URL instead
- [../recipes/detect-and-apply.md](../recipes/detect-and-apply.md)
- [push-service.md](push-service.md)
