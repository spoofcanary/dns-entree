# detect-and-apply-template

End-to-end library example: detect the DNS provider for a domain, sync the Domain-Connect/Templates git cache, load a specific template, resolve variables, and apply it via a Cloudflare `PushService`.

## Prereqs

- Cloudflare API token (token env var)
- Internet access to clone `github.com/Domain-Connect/Templates`
- Template cache is stored under `$XDG_CACHE_HOME/dns-entree/templates` (or `~/Library/Caches` on macOS)

```
export DNSENTREE_CLOUDFLARE_TOKEN=cf_...
```

## Run

```
go run . example.com exampleservice.com template1
```

## Expected Output

```
detected: provider=cloudflare label="Cloudflare" supported=true method=ns_pattern
  created _dmarc.example.com -> v=DMARC1; ... (verified=true)
  ...
```

Variables required by the chosen template beyond `domain` must be supplied by editing `vars` in `main.go`.

## CI Note

Compile-tested in CI only.
