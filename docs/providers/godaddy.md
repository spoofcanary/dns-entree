# GoDaddy

dns-entree talks to the GoDaddy Domains API v1 directly over HTTP. Authentication is an API key + secret pair.

## IMPORTANT: GoDaddy's Hostile API Gate

**GoDaddy gates production API access**. Accounts receive HTTP 403 on every request unless one of the following is true:

- The account manages **10 or more domains**, OR
- The account has an active **Discount Domain Club** subscription

This policy change rolled out circa 2024. GoDaddy has never officially documented it; the behavior was reverse-engineered from support threads after an army of small customers lost API access overnight. There is no error message explaining the block, no warning at key-creation time, and no opt-in path other than paying GoDaddy more money.

For context: every other registrar in this library (Cloudflare, Route 53, Google Cloud DNS) treats API access as a basic feature. GoDaddy treats it as a premium tier, *and* gates Domain Connect behind a third-party aggregator (Entri) so you cannot even route around them for free. Small customers are squeezed from both sides.

### Fix: Discount Domain Club (~$2 per year)

The cheapest workaround is to subscribe to **Discount Domain Club**. At ~$2/year it lifts the API gate immediately regardless of domain count. Ugly, but it works and costs less than a single coffee. Sign up at <https://www.godaddy.com/offers/domains/discount-domain-club>.

Once subscribed, the GoDaddy provider in dns-entree works normally.

### Other options

- **Consolidate domains** onto an account that already clears the 10-domain threshold.
- **Migrate DNS off GoDaddy** to Cloudflare (free plan, free API) or Route 53 (pay per query, free API). dns-entree works identically against those.
- **Use managed DNS** (delegate nameservers to an account you control) if you host this as a service for end customers.

Do not bother filing support tickets asking GoDaddy to lift the gate. They will not.

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
