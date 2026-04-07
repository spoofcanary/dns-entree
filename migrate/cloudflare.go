package migrate

import (
	"context"
	"errors"
	"fmt"

	cf "github.com/cloudflare/cloudflare-go"
)

// CloudflareAdapter implements Adapter for Cloudflare via cloudflare-go.
type CloudflareAdapter struct{}

func init() {
	RegisterAdapter("cloudflare", CloudflareAdapter{})
}

// EnsureZone creates the Cloudflare zone if absent and returns the zone ID
// and assigned authoritative nameservers.
func (CloudflareAdapter) EnsureZone(ctx context.Context, domain string, opts ProviderOpts) (ZoneInfo, error) {
	if err := validateDomain(domain); err != nil {
		return ZoneInfo{}, err
	}
	if err := validateEndpoint(opts.Endpoint); err != nil {
		return ZoneInfo{}, err
	}
	if opts.APIToken == "" {
		return ZoneInfo{}, errors.New("cloudflare: APIToken required")
	}
	if opts.AccountID == "" {
		return ZoneInfo{}, errors.New("cloudflare: AccountID required")
	}

	cfOpts := []cf.Option{}
	if opts.Endpoint != "" {
		cfOpts = append(cfOpts, cf.BaseURL(opts.Endpoint))
	}
	if opts.HTTPClient != nil {
		cfOpts = append(cfOpts, cf.HTTPClient(opts.HTTPClient))
	}
	api, err := cf.NewWithAPIToken(opts.APIToken, cfOpts...)
	if err != nil {
		return ZoneInfo{}, fmt.Errorf("cloudflare: client: %w", err)
	}

	existing, err := api.ListZones(ctx, domain)
	if err != nil {
		return ZoneInfo{}, fmt.Errorf("cloudflare: list zones: %w", err)
	}
	for _, z := range existing {
		if z.Name == domain {
			return ZoneInfo{ZoneID: z.ID, Nameservers: z.NameServers, Created: false}, nil
		}
	}

	created, err := api.CreateZone(ctx, domain, false, cf.Account{ID: opts.AccountID}, "full")
	if err != nil {
		return ZoneInfo{}, fmt.Errorf("cloudflare: create zone: %w", err)
	}
	return ZoneInfo{ZoneID: created.ID, Nameservers: created.NameServers, Created: true}, nil
}
