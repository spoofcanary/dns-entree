# Google Cloud DNS

dns-entree talks to the Cloud DNS REST API v1 using an OAuth2 bearer token. You supply the access token and the GCP project ID.

## Credential Setup

### 1. Create a Service Account

In the GCP Console:

1. IAM & Admin -> Service Accounts -> **Create Service Account**
2. Name: `dns-entree`
3. Grant role: **DNS Administrator** (`roles/dns.admin`) on the project

### 2. Create a Key

On the service account detail page -> **Keys** -> **Add Key** -> **Create new key** -> **JSON**. Download the JSON file and keep it secret.

### 3. Mint an Access Token

dns-entree accepts a raw OAuth2 bearer token, not the JSON key file. Mint tokens from the JSON credential using `google.golang.org/api/option` or the `gcloud` CLI:

```
gcloud auth activate-service-account --key-file=./sa.json
gcloud auth print-access-token
```

Or in code, use `golang.org/x/oauth2/google` to get a `TokenSource` and call `.Token()` when refreshing.

The token is scoped to `https://www.googleapis.com/auth/ndev.clouddns.readwrite` (inherited from `dns.admin`). Tokens last ~1 hour; your caller must refresh.

### 4. Managed Zone

Cloud DNS organizes records under **Managed Zones**. dns-entree resolves the managed zone for a given domain via the standard list API, so you only need to know the project ID. Ensure a managed zone exists for each domain you want to write to.

## Usage

```go
prov, err := entree.NewProvider("google_cloud_dns", entree.Credentials{
    Token:     oauthToken,             // OAuth2 bearer, not the JSON
    ProjectID: "my-gcp-project-123",
})
```

## Delete-Then-Create Atomicity Caveat

Cloud DNS's record write model is change-based: every mutation is a `Change` with `additions` and `deletions` arrays, and the API applies them as a single transaction. dns-entree's current implementation performs record updates as a delete followed by a create in **separate API calls** rather than as a single `Change`. There is a short window where the record does not exist in the zone.

Implications:

- For TXT / CNAME updates that your system depends on being always present (e.g. verification tokens for an active service), there is a small risk of a gap if the second call fails.
- `PushService` retries are safe because the whole operation is idempotent, but a resolver querying during the gap may see NXDOMAIN.
- Future versions of dns-entree will merge the delete+create into a single `Change` for atomicity. Track this in the project roadmap if you need a guarantee today.

For idempotent first writes (creating a record that did not exist) there is no gap; the caveat only applies to updates.

## Known Limits

- 300 API requests per minute per project by default.
- Managed zone record set limit: 10,000 per zone.
- DNSSEC-enabled zones behave the same from the API but signing is managed by GCP.

## Official Docs

- Cloud DNS API: <https://cloud.google.com/dns/docs/reference/v1>
- IAM roles: <https://cloud.google.com/dns/docs/access-control>
- Service accounts: <https://cloud.google.com/iam/docs/service-accounts>
