# push-dkim

Pushes a DKIM CNAME record (`<selector>._domainkey.<domain>`) via AWS Route 53.

## Prereqs

- IAM credentials with `route53:ChangeResourceRecordSets` + `route53:ListHostedZones` on the target zone
- Hosted zone for the domain must already exist

```
export DNSENTREE_AWS_ACCESS_KEY_ID=AKIA...
export DNSENTREE_AWS_SECRET_ACCESS_KEY=...
export DNSENTREE_AWS_REGION=us-east-1
```

## Run

```
go run . example.com selector1 selector1.dkim.provider.com
```

## Expected Output

```
status=created verified=true value="selector1.dkim.provider.com"
```

## CI Note

Compile-tested in CI, not executed.
