package route53

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awscreds "github.com/aws/aws-sdk-go-v2/credentials"
	awsr53 "github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	entree "github.com/spoofcanary/dns-entree"
)

func init() {
	entree.RegisterProvider("route53", func(creds entree.Credentials) (entree.Provider, error) {
		return NewProvider(creds.AccessKey, creds.SecretKey, creds.Region)
	})
}

// route53Client is the subset of the Route 53 SDK used by Provider.
type route53Client interface {
	ListHostedZones(ctx context.Context, input *awsr53.ListHostedZonesInput, opts ...func(*awsr53.Options)) (*awsr53.ListHostedZonesOutput, error)
	ListHostedZonesByName(ctx context.Context, input *awsr53.ListHostedZonesByNameInput, opts ...func(*awsr53.Options)) (*awsr53.ListHostedZonesByNameOutput, error)
	ListResourceRecordSets(ctx context.Context, input *awsr53.ListResourceRecordSetsInput, opts ...func(*awsr53.Options)) (*awsr53.ListResourceRecordSetsOutput, error)
	ChangeResourceRecordSets(ctx context.Context, input *awsr53.ChangeResourceRecordSetsInput, opts ...func(*awsr53.Options)) (*awsr53.ChangeResourceRecordSetsOutput, error)
}

// Provider is the Route 53 DNS provider.
type Provider struct {
	client route53Client
}

var _ entree.Provider = (*Provider)(nil)

// NewProvider constructs a Route 53 provider from static AWS credentials.
func NewProvider(accessKey, secretKey, region string) (*Provider, error) {
	if accessKey == "" {
		return nil, errors.New("route53: AccessKey required")
	}
	if secretKey == "" {
		return nil, errors.New("route53: SecretKey required")
	}
	if region == "" {
		region = "us-east-1"
	}
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(awscreds.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	return &Provider{client: awsr53.NewFromConfig(cfg)}, nil
}

// newProviderWithClient is for tests to inject a mock.
func newProviderWithClient(c route53Client) *Provider {
	return &Provider{client: c}
}

func (p *Provider) Name() string { return "Route 53" }
func (p *Provider) Slug() string { return "route53" }

func (p *Provider) Verify(ctx context.Context) ([]entree.Zone, error) {
	out, err := p.client.ListHostedZones(ctx, &awsr53.ListHostedZonesInput{})
	if err != nil {
		return nil, fmt.Errorf("list hosted zones: %w", err)
	}
	zones := make([]entree.Zone, 0, len(out.HostedZones))
	for _, hz := range out.HostedZones {
		zones = append(zones, entree.Zone{
			ID:     strings.TrimPrefix(aws.ToString(hz.Id), "/hostedzone/"),
			Name:   strings.TrimSuffix(aws.ToString(hz.Name), "."),
			Status: "active",
		})
	}
	return zones, nil
}

func (p *Provider) findZoneID(ctx context.Context, domain string) (string, error) {
	out, err := p.client.ListHostedZonesByName(ctx, &awsr53.ListHostedZonesByNameInput{
		DNSName: aws.String(domain),
	})
	if err != nil {
		return "", fmt.Errorf("find zone: %w", err)
	}
	// Longest-suffix match.
	bestID := ""
	bestLen := -1
	for _, hz := range out.HostedZones {
		name := strings.TrimSuffix(aws.ToString(hz.Name), ".")
		if name == domain || strings.HasSuffix(domain, "."+name) {
			if len(name) > bestLen {
				bestLen = len(name)
				bestID = strings.TrimPrefix(aws.ToString(hz.Id), "/hostedzone/")
			}
		}
	}
	if bestID == "" {
		return "", fmt.Errorf("no hosted zone found for %s", domain)
	}
	return bestID, nil
}

func (p *Provider) GetRecords(ctx context.Context, domain, recordType string) ([]entree.Record, error) {
	zoneID, err := p.findZoneID(ctx, domain)
	if err != nil {
		return nil, err
	}
	out, err := p.client.ListResourceRecordSets(ctx, &awsr53.ListResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneID),
	})
	if err != nil {
		return nil, fmt.Errorf("list records: %w", err)
	}
	var records []entree.Record
	for _, rrs := range out.ResourceRecordSets {
		rType := string(rrs.Type)
		if recordType != "" && rType != recordType {
			continue
		}
		name := strings.TrimSuffix(aws.ToString(rrs.Name), ".")
		for _, rr := range rrs.ResourceRecords {
			records = append(records, entree.Record{
				ID:      name + "|" + rType,
				Type:    rType,
				Name:    name,
				Content: strings.Trim(aws.ToString(rr.Value), "\""),
				TTL:     int(aws.ToInt64(rrs.TTL)),
			})
		}
	}
	return records, nil
}

func (p *Provider) SetRecord(ctx context.Context, domain string, record entree.Record) error {
	zoneID, err := p.findZoneID(ctx, domain)
	if err != nil {
		return err
	}
	ttl := int64(record.TTL)
	if ttl == 0 {
		ttl = 300
	}
	content := record.Content
	if record.Type == "TXT" {
		content = fmt.Sprintf(`"%s"`, record.Content)
	}
	_, err = p.client.ChangeResourceRecordSets(ctx, &awsr53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneID),
		ChangeBatch: &r53types.ChangeBatch{
			Changes: []r53types.Change{{
				Action: r53types.ChangeActionUpsert,
				ResourceRecordSet: &r53types.ResourceRecordSet{
					Name: aws.String(record.Name),
					Type: r53types.RRType(record.Type),
					TTL:  aws.Int64(ttl),
					ResourceRecords: []r53types.ResourceRecord{
						{Value: aws.String(content)},
					},
				},
			}},
		},
	})
	if err != nil {
		return fmt.Errorf("upsert record: %w", err)
	}
	return nil
}

func (p *Provider) DeleteRecord(ctx context.Context, domain, recordID string) error {
	zoneID, err := p.findZoneID(ctx, domain)
	if err != nil {
		return err
	}
	parts := strings.SplitN(recordID, "|", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid record ID format: %s", recordID)
	}
	name, rType := parts[0], parts[1]
	records, err := p.GetRecords(ctx, domain, rType)
	if err != nil {
		return err
	}
	for _, r := range records {
		if r.Name == name {
			content := r.Content
			if rType == "TXT" {
				content = fmt.Sprintf(`"%s"`, r.Content)
			}
			_, err = p.client.ChangeResourceRecordSets(ctx, &awsr53.ChangeResourceRecordSetsInput{
				HostedZoneId: aws.String(zoneID),
				ChangeBatch: &r53types.ChangeBatch{
					Changes: []r53types.Change{{
						Action: r53types.ChangeActionDelete,
						ResourceRecordSet: &r53types.ResourceRecordSet{
							Name: aws.String(name),
							Type: r53types.RRType(rType),
							TTL:  aws.Int64(int64(r.TTL)),
							ResourceRecords: []r53types.ResourceRecord{
								{Value: aws.String(content)},
							},
						},
					}},
				},
			})
			return err
		}
	}
	return fmt.Errorf("record not found: %s", recordID)
}

func (p *Provider) ApplyRecords(ctx context.Context, domain string, records []entree.Record) error {
	return entree.DefaultApplyRecords(p, ctx, domain, records)
}
