package godaddy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	entree "github.com/spoofcanary/dns-entree"
)

const defaultBaseURL = "https://api.godaddy.com/v1"

func init() {
	entree.RegisterProvider("godaddy", func(creds entree.Credentials) (entree.Provider, error) {
		return NewProvider(creds.APIKey, creds.APISecret)
	})
}

// Provider is the GoDaddy DNS provider.
type Provider struct {
	apiKey    string
	apiSecret string
	baseURL   string
	client    *http.Client
}

var _ entree.Provider = (*Provider)(nil)

// NewProvider constructs a GoDaddy provider from API key and secret.
func NewProvider(apiKey, apiSecret string) (*Provider, error) {
	if apiKey == "" {
		return nil, errors.New("godaddy: APIKey required")
	}
	if apiSecret == "" {
		return nil, errors.New("godaddy: APISecret required")
	}
	return &Provider{
		apiKey:    apiKey,
		apiSecret: apiSecret,
		baseURL:   defaultBaseURL,
		client:    &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func (p *Provider) Name() string { return "GoDaddy" }
func (p *Provider) Slug() string { return "godaddy" }

func (p *Provider) authHeader() string {
	return fmt.Sprintf("sso-key %s:%s", p.apiKey, p.apiSecret)
}

type godaddyRecord struct {
	Type string `json:"type"`
	Name string `json:"name"`
	Data string `json:"data"`
	TTL  int    `json:"ttl"`
}

func (p *Provider) Verify(ctx context.Context) ([]entree.Zone, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/domains?statuses=ACTIVE", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", p.authHeader())

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("godaddy: list domains: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("godaddy api %d: %s", resp.StatusCode, body)
	}

	var domains []struct {
		Domain string `json:"domain"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&domains); err != nil {
		return nil, fmt.Errorf("godaddy: decode: %w", err)
	}

	zones := make([]entree.Zone, 0, len(domains))
	for _, d := range domains {
		zones = append(zones, entree.Zone{
			ID:     d.Domain,
			Name:   d.Domain,
			Status: strings.ToLower(d.Status),
		})
	}
	return zones, nil
}

func (p *Provider) GetRecords(ctx context.Context, domain, recordType string) ([]entree.Record, error) {
	url := fmt.Sprintf("%s/domains/%s/records", p.baseURL, domain)
	if recordType != "" {
		url += "/" + recordType
	}

	gdRecs, err := p.fetchRecords(ctx, url)
	if err != nil {
		return nil, err
	}

	records := make([]entree.Record, 0, len(gdRecs))
	for _, r := range gdRecs {
		var fullName string
		if r.Name != "@" {
			fullName = r.Name + "." + domain
		} else {
			fullName = domain
		}
		records = append(records, entree.Record{
			ID:      r.Name + "|" + r.Type,
			Type:    r.Type,
			Name:    fullName,
			Content: r.Data,
			TTL:     r.TTL,
		})
	}
	return records, nil
}

// getRecordsAtNameType fetches existing records at a specific name+type for read-modify-write.
func (p *Provider) getRecordsAtNameType(ctx context.Context, domain, recordType, name string) ([]godaddyRecord, error) {
	url := fmt.Sprintf("%s/domains/%s/records/%s/%s", p.baseURL, domain, recordType, name)
	return p.fetchRecords(ctx, url)
}

func (p *Provider) fetchRecords(ctx context.Context, url string) ([]godaddyRecord, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", p.authHeader())

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("godaddy: get records: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("godaddy api %d: %s", resp.StatusCode, body)
	}

	var gdRecords []godaddyRecord
	if err := json.NewDecoder(resp.Body).Decode(&gdRecords); err != nil {
		return nil, fmt.Errorf("godaddy: decode: %w", err)
	}
	return gdRecords, nil
}

// mergeGoDaddyRecords replaces a record with matching Data, or appends it if new.
// This preserves sibling records at the same name+type (D-05 / QUAL-04 fix).
func mergeGoDaddyRecords(existing []godaddyRecord, newRec godaddyRecord) []godaddyRecord {
	for i, r := range existing {
		if r.Data == newRec.Data {
			existing[i] = newRec
			return existing
		}
	}
	return append(existing, newRec)
}

// normalizeGoDaddyName converts an FQDN to GoDaddy's relative name form.
// "example.com" + domain "example.com" -> "@"
// "_dmarc.example.com" + domain "example.com" -> "_dmarc"
func normalizeGoDaddyName(name, domain string) string {
	if strings.HasSuffix(name, "."+domain) {
		name = strings.TrimSuffix(name, "."+domain)
	}
	if name == domain {
		name = "@"
	}
	return name
}

// SetRecord creates or updates a DNS record using read-modify-write to preserve siblings.
// GoDaddy's PUT /records/{type}/{name} REPLACES all records at that name+type, so we must
// fetch existing records first, merge in the new one, and PUT the full set back.
func (p *Provider) SetRecord(ctx context.Context, domain string, record entree.Record) error {
	name := normalizeGoDaddyName(record.Name, domain)
	ttl := record.TTL
	if ttl < 600 {
		ttl = 600 // GoDaddy minimum TTL
	}

	// Step 1: GET existing records at this name+type (read)
	existing, err := p.getRecordsAtNameType(ctx, domain, record.Type, name)
	if err != nil {
		// Treat fetch failure (e.g. 404 no records) as empty - we'll create fresh.
		existing = nil
	}

	// Step 2: Merge new record into existing set (modify)
	newRec := godaddyRecord{
		Type: record.Type,
		Name: name,
		Data: record.Content,
		TTL:  ttl,
	}
	merged := mergeGoDaddyRecords(existing, newRec)

	// Step 3: PUT the full merged array (write)
	body, err := json.Marshal(merged)
	if err != nil {
		return fmt.Errorf("godaddy: marshal: %w", err)
	}
	url := fmt.Sprintf("%s/domains/%s/records/%s/%s", p.baseURL, domain, record.Type, name)
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", p.authHeader())
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("godaddy: set record: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("godaddy api %d: %s", resp.StatusCode, respBody)
	}
	return nil
}

func (p *Provider) DeleteRecord(ctx context.Context, domain, recordID string) error {
	parts := strings.SplitN(recordID, "|", 2)
	if len(parts) != 2 {
		return fmt.Errorf("godaddy: invalid record ID: %s", recordID)
	}
	name, rType := parts[0], parts[1]

	url := fmt.Sprintf("%s/domains/%s/records/%s/%s", p.baseURL, domain, rType, name)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", p.authHeader())

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("godaddy: delete record: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("godaddy api %d: %s", resp.StatusCode, body)
	}
	return nil
}

func (p *Provider) ApplyRecords(ctx context.Context, domain string, records []entree.Record) error {
	return entree.DefaultApplyRecords(p, ctx, domain, records)
}
