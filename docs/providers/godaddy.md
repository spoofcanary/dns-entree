# GoDaddy

dns-entree talks to the GoDaddy Domains API v1 directly over HTTP. Authentication is an API key + secret pair.

## IMPORTANT: 10-Domain Restriction

**GoDaddy's production API is gatekept. Accounts with fewer than 10 domains on the account receive HTTP 403 on every request**, regardless of API key validity. This is a GoDaddy policy change (circa 2024) and cannot be worked around from dns-entree.

If you hit a blanket 403:

- Check domain count on the GoDaddy account. Fewer than 10 -> no API access.
- Consolidate domains onto an account that clears the threshold, or
- Fall back to Domain Connect (see [../guides/domain-connect.md](../guides/domain-connect.md)) - GoDaddy supports DC on many zones even when the raw API is blocked.

## Credential Setup

1. Sign in at <https://developer.godaddy.com/keys>.
2. Click **Create New API Key** -> environment **Production** (the OTE sandbox does not manage real zones).
3. Name it (e.g. `dns-entree`).
4. Copy the **Key** and **Secret**; GoDaddy shows the secret once.

Store as `GODADDY_API_KEY` / `GODADDY_API_SECRET`.

Auth header format (for reference):

```
Authorization: sso-key <key>:<secret>
```

dns-entree builds this automatically.

## Usage

```go
prov, err := entree.NewProvider("godaddy", entree.Credentials{
    APIKey:    os.Getenv("GODADDY_API_KEY"),
    APISecret: os.Getenv("GODADDY_API_SECRET"),
})
```

## Read-Modify-Write Semantics

The GoDaddy API is unusual: `PUT /v1/domains/{domain}/records/{type}/{name}` **replaces all records of that type at that name**, not just one. Naive callers that PUT a single record wipe out other records at the same name.

dns-entree's GoDaddy provider handles this correctly: `SetRecord` performs a read-modify-write, preserving other records at the same `(type, name)` tuple and only substituting the target value. Callers of `PushService` never see the distinction, but if you bypass PushService and call the provider directly, be aware that `SetRecord` does a GET first.

## Known Limits

- 60 requests per minute per API key (production).
- Record changes propagate within a few minutes but are not transactional.
- No batch endpoint; each record is a separate HTTP call.

## Gotchas

- The 10-domain restriction is the most common source of 403s; it is not an auth problem.
- TTL below 600 is silently clamped to 600.
- Some zones are "premium DNS only" and require a GoDaddy subscription before the API will accept writes.

## Official Docs

- API reference: <https://developer.godaddy.com/doc/endpoint/domains>
- Key management: <https://developer.godaddy.com/keys>
