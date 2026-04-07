# Amazon Route 53

dns-entree uses `aws-sdk-go-v2` with static credentials. The default region is `us-east-1` (Route 53 is global; region only affects the SDK signer).

## Credential Setup

### 1. Create an IAM Policy

In the AWS Console -> IAM -> Policies -> Create Policy -> JSON, paste:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "Route53Read",
      "Effect": "Allow",
      "Action": [
        "route53:ListHostedZones",
        "route53:ListHostedZonesByName",
        "route53:ListResourceRecordSets",
        "route53:GetChange"
      ],
      "Resource": "*"
    },
    {
      "Sid": "Route53Write",
      "Effect": "Allow",
      "Action": "route53:ChangeResourceRecordSets",
      "Resource": "arn:aws:route53:::hostedzone/*"
    }
  ]
}
```

Name it `dns-entree-route53` and save.

### 2. Create an IAM User

- IAM -> Users -> Create User (`dns-entree`)
- Attach the policy above directly (no group needed for a service user)
- On the user, Security Credentials -> Create access key -> "Application running outside AWS"
- Copy `Access key ID` and `Secret access key`

Store as `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY`.

### 3. Hosted Zone Lookup

dns-entree resolves zone IDs by name via `ListHostedZonesByName`; you do not need to hardcode zone IDs. If you want to verify ahead of time:

```
aws route53 list-hosted-zones-by-name --dns-name example.com
```

The ID returned looks like `/hostedzone/Z1234567890ABC`; dns-entree handles the full form.

## Usage

```go
prov, err := entree.NewProvider("route53", entree.Credentials{
    AccessKey: os.Getenv("AWS_ACCESS_KEY_ID"),
    SecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
    Region:    "us-east-1",
})
```

## TXT Quoting

Route 53 requires TXT record values to be wrapped in double quotes on the wire, and values longer than 255 characters must be split into multiple quoted strings. dns-entree's `entree.Record.Content` is always **unquoted**; the Route 53 provider adds and removes the quoting for you on write and read.

Do not pre-quote TXT content. Pass `v=DMARC1; p=none`, not `"v=DMARC1; p=none"`.

## Known Limits

- Changes are eventually consistent via `GetChange`; post-push verification may briefly return `false`.
- Rate limit: 5 API requests per second per AWS account across Route 53.
- 10,000 hosted zones per account by default.

## Gotchas

- The IAM policy above is intentionally broad on resource for read operations; narrow it if you manage many zones with a single account.
- Route 53 charges per hosted zone and per million queries; dns-entree reads are cheap but not free.

## Official Docs

- Route 53 API reference: <https://docs.aws.amazon.com/Route53/latest/APIReference/>
- IAM policy reference: <https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/r53-api-permissions-ref.html>
- aws-sdk-go-v2: <https://aws.github.io/aws-sdk-go-v2/docs/>
