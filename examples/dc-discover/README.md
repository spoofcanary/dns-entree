# dc-discover

Performs Domain Connect v2 discovery for a domain via the library's SSRF-screened `domainconnect.Discover`.

## Prereqs

- None. Pure network probe, no credentials.

## Run

```
go run . example.com
```

## Expected Output

```
providerId:   exampleservice.com
providerName: Example Provider
urlSyncUX:    https://example.com/dc/v2/sync
urlAsyncUX:   https://example.com/dc/v2/async
urlAPI:       https://example.com/dc/v2/api
width/height: 750x750
```

If the domain does not publish a `_domainconnect` TXT record or the settings endpoint fails, prints `Domain Connect: not supported`.

## CI Note

Compile-tested in CI. Safe to run locally.
