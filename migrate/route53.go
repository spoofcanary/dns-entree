package migrate

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awscreds "github.com/aws/aws-sdk-go-v2/credentials"
	awsr53 "github.com/aws/aws-sdk-go-v2/service/route53"
)

// Route53Adapter implements Adapter for AWS Route 53.
type Route53Adapter struct{}

func init() {
	RegisterAdapter("route53", Route53Adapter{})
}

// EnsureZone creates the hosted zone if absent and returns the zone ID and
// delegation-set nameservers.
func (Route53Adapter) EnsureZone(ctx context.Context, domain string, opts ProviderOpts) (ZoneInfo, error) {
	if err := validateDomain(domain); err != nil {
		return ZoneInfo{}, err
	}
	if err := validateEndpoint(opts.Endpoint); err != nil {
		return ZoneInfo{}, err
	}
	if opts.AccessKey == "" || opts.SecretKey == "" {
		return ZoneInfo{}, errors.New("route53: AccessKey and SecretKey required")
	}
	region := opts.Region
	if region == "" {
		region = "us-east-1"
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(awscreds.NewStaticCredentialsProvider(opts.AccessKey, opts.SecretKey, "")),
	)
	if err != nil {
		return ZoneInfo{}, fmt.Errorf("route53: aws config: %w", err)
	}

	clientOpts := []func(*awsr53.Options){}
	if opts.Endpoint != "" {
		ep := opts.Endpoint
		clientOpts = append(clientOpts, func(o *awsr53.Options) {
			o.BaseEndpoint = aws.String(ep)
		})
	}
	if opts.HTTPClient != nil {
		hc := opts.HTTPClient
		clientOpts = append(clientOpts, func(o *awsr53.Options) {
			o.HTTPClient = hc
		})
	}
	client := awsr53.NewFromConfig(cfg, clientOpts...)

	// Look for existing hosted zone with exact name match.
	listOut, err := client.ListHostedZonesByName(ctx, &awsr53.ListHostedZonesByNameInput{
		DNSName: aws.String(domain),
	})
	if err != nil {
		return ZoneInfo{}, fmt.Errorf("route53: list zones: %w", err)
	}
	for _, hz := range listOut.HostedZones {
		name := strings.TrimSuffix(aws.ToString(hz.Name), ".")
		if name == domain {
			zoneID := strings.TrimPrefix(aws.ToString(hz.Id), "/hostedzone/")
			getOut, err := client.GetHostedZone(ctx, &awsr53.GetHostedZoneInput{Id: aws.String(zoneID)})
			if err != nil {
				return ZoneInfo{}, fmt.Errorf("route53: get zone: %w", err)
			}
			var ns []string
			if getOut.DelegationSet != nil {
				ns = getOut.DelegationSet.NameServers
			}
			return ZoneInfo{ZoneID: zoneID, Nameservers: ns, Created: false}, nil
		}
	}

	createOut, err := client.CreateHostedZone(ctx, &awsr53.CreateHostedZoneInput{
		Name:            aws.String(domain),
		CallerReference: aws.String(fmt.Sprintf("entree-migrate-%d", time.Now().UnixNano())),
	})
	if err != nil {
		return ZoneInfo{}, fmt.Errorf("route53: create zone: %w", err)
	}
	zoneID := ""
	if createOut.HostedZone != nil {
		zoneID = strings.TrimPrefix(aws.ToString(createOut.HostedZone.Id), "/hostedzone/")
	}
	var ns []string
	if createOut.DelegationSet != nil {
		ns = createOut.DelegationSet.NameServers
	}
	return ZoneInfo{ZoneID: zoneID, Nameservers: ns, Created: true}, nil
}
