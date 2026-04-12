# spf-merge

Library equivalent of `entree spf-merge`. Pure function, no network or provider I/O, no credentials needed.

## Run

```
go run . "v=spf1 include:_spf.google.com ~all" servers.mcsv.net
```

## Expected Output

```
value: v=spf1 include:_spf.google.com include:servers.mcsv.net ~all
changed: true
lookup_count: 2
lookup_limit_exceeded: false
```

Re-running with the same inputs prints `changed: false` (idempotent).

## CI Note

Compile-tested in CI. Safe to run locally with no credentials.
