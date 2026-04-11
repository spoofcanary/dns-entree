# Domain Connect Guide

The `domainconnect` package implements [Domain Connect v2](https://github.com/Domain-Connect/spec) discovery and async apply URL signing. It is the bridge between a service (you) and DNS providers that let end users self-apply templates.

## Discovery

```go
import "github.com/spoofcanary/dns-entree/domainconnect"

dr, err := domainconnect.Discover(ctx, "example.com")
if err != nil {
    log.Fatal(err) // only returned for programmer errors (empty domain)
}
if !dr.Supported {
    // provider does not expose DC, or discovery failed
    return
}
log.Printf("provider=%s asyncUX=%s", dr.ProviderName, dr.URLAsyncUX)
```

Discover:

1. Looks up `_domainconnect.<domain>` TXT to find the settings host.
2. Screens the resolved IPs against loopback, private, link-local, multicast, and unspecified ranges (SSRF defense).
3. Fetches `https://<host>/v2/<domain>/settings` with a 5s timeout and a 64 KiB body cap.
4. Decodes `providerId`, `providerName`, `urlSyncUX`, `urlAsyncUX`, `urlAPI`, `width`, `height`.

Network or parse failures return `(DiscoveryResult{Supported: false}, nil)`. Discovery is best-effort, never a blocking error.

### SSRF Protection

All outbound hosts are resolved before the HTTP dial and rejected if any IP is in a blocked range. HTTP redirects re-validate the target before following, up to 5 hops. Non-HTTPS redirects are rejected.

### Test Hooks

```go
domainconnect.Discover(ctx, "example.com",
    domainconnect.WithHTTPClient(myClient),
    domainconnect.WithResolver(myIPResolver),
    domainconnect.WithTXTResolver(myTXTResolver),
)
```

## Signing

Async apply URLs are signed with RSA-SHA256 PKCS1v15. The signature is base64url (no padding), URL-safe, and deterministic.

```go
key, err := domainconnect.LoadPrivateKey(pemBytes) // PKCS1 or PKCS8 PEM
if err != nil {
    log.Fatal(err)
}

sig, keyHash, err := domainconnect.SignQueryString("a=1&b=two", key)
```

`keyHash` is informational (`base64url(sha256(SPKI DER))`) - useful for log correlation and rotation. It is **not** the value of the `key=` URL parameter.

## Apply URL Construction

```go
url, err := domainconnect.BuildApplyURL(domainconnect.ApplyURLOpts{
    URLAsyncUX:  dr.URLAsyncUX,
    ProviderID:  "exampleservice.com",
    ServiceID:   "dmarc",
    Domain:      "example.com",
    Host:        "",                      // optional subdomain
    Params:      map[string]string{"rua": "mailto:rua@example.com"},
    PrivateKey:  key,
    KeyHost:     "_dcsig.example.net",    // where the public key TXT lives
    RedirectURI: "https://app.example.net/done",
    State:       "opaque-session-token",
})
```

The builder:

1. Merges implicit fields (`domain`, `host`, `redirect_uri`, `state`) and `Params`.
2. Sorts keys alphabetically.
3. URL-encodes values with `%20` for spaces (not `+`; providers reject `+`).
4. Signs the sorted query string.
5. Appends `&key=<KeyHost>&sig=<base64url-sig>` after signing.

Reserved keys (do not pass in `Params`): `domain`, `host`, `redirect_uri`, `state`, `key`, `sig`.

## Key Hosting

Publish the public key as a TXT record at `KeyHost`, containing the base64 DER of the `SubjectPublicKeyInfo`. Providers fetch this to verify your signature.

## Providers with Domain Connect Support

- **Cloudflare** - domains on Cloudflare nameservers advertise DC via `_domainconnect` TXT record. See [Cloudflare provider docs](../providers/cloudflare.md#domain-connect-support).
- **GoDaddy** - original DC sponsor; most GoDaddy-hosted domains support DC out of the box.

Discovery (`domainconnect.Discover`) works against any DC-compliant provider, not just these two.

## Testing with dctest

The `domainconnect/dctest` package provides a mock DC provider for integration tests. No real DNS or provider needed.

```go
import "github.com/spoofcanary/dns-entree/domainconnect/dctest"

srv := dctest.NewServer()
defer srv.Close()

// srv.URL        - base URL of the mock provider (TLS)
// srv.TXTRecord() - value for _domainconnect TXT record
// srv.PrivateKey() - RSA key for signing apply URLs
// srv.HTTPClient() - *http.Client trusting the mock cert

// Full round-trip: discover -> sign -> apply -> verify
result, _ := domainconnect.Discover(ctx, "example.com",
    domainconnect.WithHTTPClient(srv.HTTPClient()),
    domainconnect.WithTXTResolver(func(_ context.Context, _ string) ([]string, error) {
        return []string{srv.TXTRecord()}, nil
    }),
    domainconnect.WithResolver(func(_ context.Context, _ string) ([]net.IP, error) {
        return []net.IP{net.ParseIP("93.184.216.34")}, nil
    }),
)

applyURL, _ := domainconnect.BuildApplyURL(domainconnect.ApplyURLOpts{
    URLAsyncUX: result.URLAsyncUX,
    ProviderID: result.ProviderID,
    ServiceID:  "my-service",
    Domain:     "example.com",
    PrivateKey: srv.PrivateKey(),
    KeyHost:    "keys.example.com",
})

srv.HTTPClient().Get(applyURL)

// Inspect captured requests
reqs := srv.ApplyRequests()
fmt.Println(reqs[0].SigValid) // true
```

Options: `WithKey(key)` to supply your own RSA key, `WithProviderID(id)` to customize the provider ID, `WithApplyHandler(fn)` to return errors for specific domains.

## See Also

- [templates.md](templates.md) - applying DC templates directly via the provider API
- [../recipes/detect-and-apply.md](../recipes/detect-and-apply.md)
- [Domain Connect spec](https://github.com/Domain-Connect/spec)
