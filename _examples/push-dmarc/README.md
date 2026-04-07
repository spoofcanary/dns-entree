# push-dmarc

Pushes a DMARC TXT record at `_dmarc.<domain>` via Cloudflare.

## Prereqs

- A Cloudflare API token with `Zone:Read` + `DNS:Edit` on the target zone
- The domain must already exist as a Cloudflare zone

```
export DNSENTREE_CLOUDFLARE_TOKEN=cf_...
```

## Run

```
go run . example.com dmarc@example.com
```

## Expected Output

```
status=created verified=true value="v=DMARC1; p=none; rua=mailto:dmarc@example.com"
```

Re-running is idempotent: subsequent runs print `status=already_configured`.

## CI Note

This example is compile-tested in CI but never executed (no credentials in CI).
