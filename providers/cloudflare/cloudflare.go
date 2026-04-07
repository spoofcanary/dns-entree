package cloudflare

import (
	"context"
	"errors"
	"fmt"
	"strings"

	cf "github.com/cloudflare/cloudflare-go"
	entree "github.com/spoofcanary/dns-entree"
)

func init() {
	entree.RegisterProvider("cloudflare", func(creds entree.Credentials) (entree.Provider, error) {
		return NewProvider(creds.APIToken)
	})
}

// cloudflareClient is the subset of cloudflare-go used by Provider.
// Defined as an interface so tests can inject a mock.
type cloudflareClient interface {
	ListZones(ctx context.Context, z ...string) ([]cf.Zone, error)
	ZoneIDByName(name string) (string, error)
	ListDNSRecords(ctx context.Context, rc *cf.ResourceContainer, params cf.ListDNSRecordsParams) ([]cf.DNSRecord, *cf.ResultInfo, error)
	CreateDNSRecord(ctx context.Context, rc *cf.ResourceContainer, params cf.CreateDNSRecordParams) (cf.DNSRecord, error)
	UpdateDNSRecord(ctx context.Context, rc *cf.ResourceContainer, params cf.UpdateDNSRecordParams) (cf.DNSRecord, error)
	DeleteDNSRecord(ctx context.Context, rc *cf.ResourceContainer, id string) error
}

// Provider is the Cloudflare DNS provider.
type Provider struct {
	client cloudflareClient
}

var _ entree.Provider = (*Provider)(nil)

// NewProvider constructs a Cloudflare provider from an API token.
func NewProvider(apiToken string) (*Provider, error) {
	if apiToken == "" {
		return nil, errors.New("cloudflare: APIToken required")
	}
	api, err := cf.NewWithAPIToken(apiToken)
	if err != nil {
		return nil, fmt.Errorf("cloudflare api: %w", err)
	}
	return &Provider{client: api}, nil
}

// newProviderWithClient is for tests to inject a mock client.
func newProviderWithClient(c cloudflareClient) *Provider {
	return &Provider{client: c}
}

func (p *Provider) Name() string { return "Cloudflare" }
func (p *Provider) Slug() string { return "cloudflare" }

func (p *Provider) Verify(ctx context.Context) ([]entree.Zone, error) {
	cfZones, err := p.client.ListZones(ctx)
	if err != nil {
		return nil, fmt.Errorf("list zones: %w", err)
	}
	zones := make([]entree.Zone, 0, len(cfZones))
	for _, z := range cfZones {
		zones = append(zones, entree.Zone{ID: z.ID, Name: z.Name, Status: z.Status})
	}
	return zones, nil
}

// resolveZoneID looks up a zone ID, falling back to parent domains if the
// supplied name is a subdomain (Pitfall 6 from RESEARCH.md).
func (p *Provider) resolveZoneID(domain string) (string, error) {
	name := domain
	for {
		id, err := p.client.ZoneIDByName(name)
		if err == nil {
			return id, nil
		}
		idx := strings.Index(name, ".")
		if idx < 0 || idx == len(name)-1 {
			return "", fmt.Errorf("zone lookup %s: %w", domain, err)
		}
		name = name[idx+1:]
	}
}

func (p *Provider) GetRecords(ctx context.Context, domain, recordType string) ([]entree.Record, error) {
	zoneID, err := p.resolveZoneID(domain)
	if err != nil {
		return nil, err
	}
	cfRecords, _, err := p.client.ListDNSRecords(ctx, cf.ZoneIdentifier(zoneID), cf.ListDNSRecordsParams{Type: recordType})
	if err != nil {
		return nil, fmt.Errorf("list records: %w", err)
	}
	records := make([]entree.Record, 0, len(cfRecords))
	for _, r := range cfRecords {
		records = append(records, entree.Record{ID: r.ID, Type: r.Type, Name: r.Name, Content: r.Content, TTL: r.TTL})
	}
	return records, nil
}

func (p *Provider) SetRecord(ctx context.Context, domain string, record entree.Record) error {
	zoneID, err := p.resolveZoneID(domain)
	if err != nil {
		return err
	}
	existing, _, err := p.client.ListDNSRecords(ctx, cf.ZoneIdentifier(zoneID), cf.ListDNSRecordsParams{Type: record.Type, Name: record.Name})
	if err != nil {
		return fmt.Errorf("check existing: %w", err)
	}
	ttl := record.TTL
	if ttl == 0 {
		ttl = 1 // Cloudflare auto TTL
	}
	if len(existing) > 0 {
		_, err = p.client.UpdateDNSRecord(ctx, cf.ZoneIdentifier(zoneID), cf.UpdateDNSRecordParams{
			ID: existing[0].ID, Type: record.Type, Name: record.Name, Content: record.Content, TTL: ttl,
		})
		if err != nil {
			return fmt.Errorf("update record: %w", err)
		}
		return nil
	}
	_, err = p.client.CreateDNSRecord(ctx, cf.ZoneIdentifier(zoneID), cf.CreateDNSRecordParams{
		Type: record.Type, Name: record.Name, Content: record.Content, TTL: ttl,
	})
	if err != nil {
		return fmt.Errorf("create record: %w", err)
	}
	return nil
}

func (p *Provider) DeleteRecord(ctx context.Context, domain, recordID string) error {
	zoneID, err := p.resolveZoneID(domain)
	if err != nil {
		return err
	}
	if err := p.client.DeleteDNSRecord(ctx, cf.ZoneIdentifier(zoneID), recordID); err != nil {
		return fmt.Errorf("delete record: %w", err)
	}
	return nil
}

func (p *Provider) ApplyRecords(ctx context.Context, domain string, records []entree.Record) error {
	return entree.DefaultApplyRecords(p, ctx, domain, records)
}
