// Package gcdns implements the entree.Provider interface for Google Cloud DNS.
package gcdns

import (
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

const defaultBaseURL = "https://dns.googleapis.com/dns/v1"
const defaultCRMURL = "https://cloudresourcemanager.googleapis.com/v1"

func init() {
	entree.RegisterProvider("google_cloud_dns", func(creds entree.Credentials) (entree.Provider, error) {
		return NewProvider(creds.Token, creds.ProjectID)
	})
}

// Provider is the Google Cloud DNS provider.
type Provider struct {
	accessToken string
	projectID   string
	baseURL     string
	client      *http.Client
}

var _ entree.Provider = (*Provider)(nil)

// NewProvider constructs a Google Cloud DNS provider from an OAuth2 access token
// and a GCP project ID.
func NewProvider(accessToken, projectID string) (*Provider, error) {
	if accessToken == "" {
		return nil, errors.New("gcdns: Token required")
	}
	if projectID == "" {
		return nil, errors.New("gcdns: ProjectID required")
	}
	return &Provider{
		accessToken: accessToken,
		projectID:   projectID,
		baseURL:     defaultBaseURL,
		client:      &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func (p *Provider) Name() string { return "Google Cloud DNS" }
func (p *Provider) Slug() string { return "google_cloud_dns" }

func (p *Provider) doRequest(ctx context.Context, method, url string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("gcdns: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gcdns: http request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gcdns: read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("gcdns api %d: %s", resp.StatusCode, string(data))
	}
	return data, nil
}

func (p *Provider) Verify(ctx context.Context) ([]entree.Zone, error) {
	url := fmt.Sprintf("%s/projects/%s/managedZones", p.baseURL, p.projectID)
	data, err := p.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		ManagedZones []struct {
			Name    string `json:"name"`
			DNSName string `json:"dnsName"`
			ID      string `json:"id"`
		} `json:"managedZones"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("gcdns: parse zones: %w", err)
	}

	zones := make([]entree.Zone, 0, len(result.ManagedZones))
	for _, mz := range result.ManagedZones {
		zones = append(zones, entree.Zone{
			ID:     mz.Name,
			Name:   strings.TrimSuffix(mz.DNSName, "."),
			Status: "active",
		})
	}
	return zones, nil
}

func (p *Provider) GetRecords(ctx context.Context, domain, recordType string) ([]entree.Record, error) {
	zoneID, err := p.findZoneID(ctx, domain)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/projects/%s/managedZones/%s/rrsets", p.baseURL, p.projectID, zoneID)
	if recordType != "" {
		url += "?type=" + recordType
	}

	data, err := p.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Rrsets []struct {
			Name    string   `json:"name"`
			Type    string   `json:"type"`
			TTL     int      `json:"ttl"`
			Rrdatas []string `json:"rrdatas"`
		} `json:"rrsets"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("gcdns: parse records: %w", err)
	}

	var records []entree.Record
	for _, rrs := range result.Rrsets {
		name := strings.TrimSuffix(rrs.Name, ".")
		for _, rdata := range rrs.Rrdatas {
			records = append(records, entree.Record{
				ID:      name + "|" + rrs.Type,
				Type:    rrs.Type,
				Name:    name,
				Content: strings.Trim(rdata, "\""),
				TTL:     rrs.TTL,
			})
		}
	}
	return records, nil
}

// SetRecord uses delete-then-create upsert (matches upstream production behavior).
func (p *Provider) SetRecord(ctx context.Context, domain string, record entree.Record) error {
	zoneID, err := p.findZoneID(ctx, domain)
	if err != nil {
		return err
	}

	ttl := record.TTL
	if ttl == 0 {
		ttl = 300
	}

	content := record.Content
	if record.Type == "TXT" {
		content = fmt.Sprintf(`"%s"`, record.Content)
	}

	name := record.Name
	if !strings.HasSuffix(name, ".") {
		name += "."
	}

	// Delete existing rrset (POST fails if rrset already exists).
	_ = p.deleteRRSet(ctx, zoneID, name, record.Type)

	body := fmt.Sprintf(`{"name":"%s","type":"%s","ttl":%d,"rrdatas":[%q]}`,
		name, record.Type, ttl, content)

	url := fmt.Sprintf("%s/projects/%s/managedZones/%s/rrsets", p.baseURL, p.projectID, zoneID)
	if _, err := p.doRequest(ctx, "POST", url, strings.NewReader(body)); err != nil {
		return fmt.Errorf("gcdns: create record: %w", err)
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
		return fmt.Errorf("gcdns: invalid record ID: %s", recordID)
	}
	name, rType := parts[0], parts[1]
	if !strings.HasSuffix(name, ".") {
		name += "."
	}
	return p.deleteRRSet(ctx, zoneID, name, rType)
}

func (p *Provider) deleteRRSet(ctx context.Context, zoneID, name, rType string) error {
	url := fmt.Sprintf("%s/projects/%s/managedZones/%s/rrsets/%s/%s",
		p.baseURL, p.projectID, zoneID, name, rType)
	_, err := p.doRequest(ctx, "DELETE", url, nil)
	return err
}

func (p *Provider) ApplyRecords(ctx context.Context, domain string, records []entree.Record) error {
	return entree.DefaultApplyRecords(p, ctx, domain, records)
}

// findZoneID returns the zone ID with the longest dnsName suffix matching domain.
func (p *Provider) findZoneID(ctx context.Context, domain string) (string, error) {
	zones, err := p.Verify(ctx)
	if err != nil {
		return "", err
	}
	var best *entree.Zone
	for i, z := range zones {
		if z.Name == domain || strings.HasSuffix(domain, "."+z.Name) {
			if best == nil || len(z.Name) > len(best.Name) {
				best = &zones[i]
			}
		}
	}
	if best == nil {
		return "", fmt.Errorf("gcdns: no managed zone found for %s", domain)
	}
	return best.ID, nil
}

// DiscoverGCPProject finds the GCP project that contains a DNS zone for the given domain.
// Lists projects via Cloud Resource Manager v1 and checks each for matching managed zones.
func DiscoverGCPProject(ctx context.Context, accessToken, domain string) (string, error) {
	return discoverGCPProject(ctx, accessToken, domain, defaultCRMURL, defaultBaseURL)
}

func discoverGCPProject(ctx context.Context, accessToken, domain, crmURL, dnsURL string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	req, err := http.NewRequestWithContext(ctx, "GET",
		crmURL+"/projects?filter=lifecycleState%3AACTIVE&pageSize=50", nil)
	if err != nil {
		return "", fmt.Errorf("gcdns: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gcdns: list projects: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gcdns: list projects %d: %s", resp.StatusCode, body)
	}

	var projectsResp struct {
		Projects []struct {
			ProjectID string `json:"projectId"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(body, &projectsResp); err != nil {
		return "", fmt.Errorf("gcdns: parse projects: %w", err)
	}

	targetDNS := domain + "."
	for _, proj := range projectsResp.Projects {
		zonesURL := fmt.Sprintf("%s/projects/%s/managedZones?dnsName=%s",
			dnsURL, proj.ProjectID, targetDNS)
		zReq, err := http.NewRequestWithContext(ctx, "GET", zonesURL, nil)
		if err != nil {
			continue
		}
		zReq.Header.Set("Authorization", "Bearer "+accessToken)
		zResp, err := client.Do(zReq)
		if err != nil {
			continue
		}
		zBody, _ := io.ReadAll(zResp.Body)
		zResp.Body.Close()
		if zResp.StatusCode != http.StatusOK {
			continue
		}
		var zonesResp struct {
			ManagedZones []struct {
				Name string `json:"name"`
			} `json:"managedZones"`
		}
		if err := json.Unmarshal(zBody, &zonesResp); err != nil {
			continue
		}
		if len(zonesResp.ManagedZones) > 0 {
			return proj.ProjectID, nil
		}
	}
	return "", fmt.Errorf("gcdns: no GCP project found with DNS zone for %s", domain)
}
