# Adding a DNS Provider

dns-entree supports DNS providers at two levels. Most providers already work via Domain Connect with zero code. API-level providers need a small plugin.

## Do you even need to write code?

**If the provider supports Domain Connect** (Cloudflare, Name.com, OVH, and [dozens more](https://www.domainconnect.org/ecosystem)), it works out of the box. No plugin needed. dns-entree discovers DC support automatically, builds signed apply URLs, and the customer approves record changes through their provider's consent UI.

Check if a domain's provider supports DC:

```sh
entree dc-discover example.com
```

If `supported: true`, you're done. The widget, CLI, and HTTP API all handle DC automatically.

### Providers that gate Domain Connect behind Entri

Some providers technically support Domain Connect but route it through [Entri](https://www.entri.com), a paid third-party aggregator. This means DC discovery returns `supported: false` for their domains unless the calling application has an Entri subscription ($249/month). dns-entree does not integrate with Entri.

Known Entri-gated providers:

- **GoDaddy** -- DC routed through Entri. API also gated (requires 10+ domains or Discount Domain Club). See [GoDaddy provider docs](../providers/godaddy.md).
- **IONOS (1&1)** -- DC routed through Entri for some domain configurations.

For customers on these providers, the options are:

1. **API plugin** -- if the provider has a usable API (GoDaddy does, behind the paywall), write or use the existing provider plugin
2. **Copy-paste fallback** -- the widget shows exact records to add manually
3. **Zone migration** -- `entree migrate example.com --to cloudflare` moves the entire zone to a provider where DC works for free

**You only need a provider plugin when you want API-level access** -- programmatic list/set/delete of DNS records using the provider's REST API. This enables:

- Direct record pushing without customer interaction
- Zone migration (scrape + bulk apply)
- Automated verification
- SPF merge

## Writing a provider plugin

A provider is a Go package that implements one interface and registers itself.

### 1. Create the package

```
providers/
  namecheap/
    namecheap.go
    namecheap_test.go
    doc.go
```

### 2. Implement the Provider interface

```go
package namecheap

import (
    "context"

    entree "github.com/spoofcanary/dns-entree"
)

// Register on import via init().
func init() {
    entree.RegisterProvider("namecheap", func(creds entree.Credentials) (entree.Provider, error) {
        return NewProvider(creds.APIToken, creds.Extra["username"])
    })
}

type provider struct {
    apiKey   string
    username string
    // ... your HTTP client
}

func NewProvider(apiKey, username string) (entree.Provider, error) {
    if apiKey == "" {
        return nil, fmt.Errorf("namecheap: API key required")
    }
    return &provider{apiKey: apiKey, username: username}, nil
}

func (p *provider) ListRecords(ctx context.Context, domain string) ([]entree.Record, error) {
    // Call the provider's API to list DNS records for the domain.
    // Return them as []entree.Record with Type, Name, Content, TTL fields.
}

func (p *provider) SetRecord(ctx context.Context, domain string, rec entree.Record) error {
    // Create or update a DNS record.
    // If a record with the same type+name exists, update it.
    // If not, create it.
}

func (p *provider) DeleteRecord(ctx context.Context, domain string, rec entree.Record) error {
    // Delete a DNS record matching type+name+content.
}
```

That's the entire interface: `ListRecords`, `SetRecord`, `DeleteRecord`.

### 3. Add NS detection (optional)

If you want `entree detect example.com` to identify this provider from its nameservers, add the NS pattern to `detect.go`:

```go
// In the nsPatterns map:
"namecheap": {"registrar-servers.com"},
```

This is optional. Without it, detection returns "unknown" for domains on this provider, but all other functionality (push, verify, migrate) still works when the user provides credentials.

### 4. Add credential headers for the HTTP API (optional)

If you want the HTTP API to accept credentials for this provider via headers, add the mapping to `api/credentials.go`:

```go
"namecheap": {
    {"X-Entree-Namecheap-Api-Key", "api_key", true},
    {"X-Entree-Namecheap-Username", "username", false},
},
```

### 5. Write tests

Test against the real API if you have credentials, or mock the HTTP calls:

```go
func TestSetRecord(t *testing.T) {
    p, err := NewProvider("test-key", "testuser")
    if err != nil {
        t.Fatal(err)
    }
    // Test against a sandbox or mock server
}
```

### 6. Use it

Callers opt in with a blank import:

```go
import _ "github.com/spoofcanary/dns-entree/providers/namecheap"

// Now NewProvider("namecheap", creds) works
p, err := entree.NewProvider("namecheap", entree.Credentials{
    APIToken: os.Getenv("NAMECHEAP_API_KEY"),
    Extra:    map[string]string{"username": "myuser"},
})
```

Everything else works automatically: `PushService` pushes records, `Verify` confirms propagation, `MergeSPF` handles SPF includes, `Migrate` scrapes and bulk-applies zones.

## Provider conventions

- **Constructor returns `(Provider, error)`** -- never panic on bad credentials
- **All methods accept `context.Context`** -- respect cancellation and timeouts
- **SetRecord is idempotent** -- updating an existing record with the same content is a no-op, not an error
- **DeleteRecord is idempotent** -- deleting a non-existent record is a no-op
- **Quote handling** -- TXT record values should not be double-wrapped in quotes. If the provider API adds quotes, strip them in `ListRecords` and don't add them in `SetRecord`. See the Route53 provider for an example.
- **Sibling preservation** -- `SetRecord` for a TXT record must not delete other TXT records at the same name. GoDaddy's API replaces all records at a name+type on PUT, so the GoDaddy provider does a read-modify-write. Check your provider's API behavior.

## Checklist

- [ ] Package at `providers/<slug>/`
- [ ] `init()` calls `entree.RegisterProvider`
- [ ] `ListRecords`, `SetRecord`, `DeleteRecord` implemented
- [ ] Constructor validates credentials and returns error (no panics)
- [ ] Context respected in all methods
- [ ] SetRecord idempotent (update-or-create, not fail-on-exists)
- [ ] TXT quoting handled correctly
- [ ] Sibling records preserved on write
- [ ] NS pattern added to `detect.go` (optional)
- [ ] Credential headers added to `api/credentials.go` (optional)
- [ ] Tests pass
- [ ] `doc.go` with package-level comment
