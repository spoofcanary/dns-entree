package migrate

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
)

const gcdnsDefaultBaseURL = "https://dns.googleapis.com/dns/v1"

// GCDNSAdapter implements Adapter for Google Cloud DNS via REST.
type GCDNSAdapter struct{}

func init() {
	RegisterAdapter("gcdns", GCDNSAdapter{})
	RegisterAdapter("google_cloud_dns", GCDNSAdapter{})
}

type gcdnsManagedZone struct {
	Name        string   `json:"name"`
	DNSName     string   `json:"dnsName"`
	NameServers []string `json:"nameServers"`
}

type gcdnsListResp struct {
	ManagedZones []gcdnsManagedZone `json:"managedZones"`
}

// EnsureZone creates the managed zone if absent and returns the zone name and
// assigned nameServers from the create response (D-15: read directly, do not
// rely on Verify which strips the field).
func (GCDNSAdapter) EnsureZone(ctx context.Context, domain string, opts ProviderOpts) (ZoneInfo, error) {
	if err := validateDomain(domain); err != nil {
		return ZoneInfo{}, err
	}
	if err := validateEndpoint(opts.Endpoint); err != nil {
		return ZoneInfo{}, err
	}
	if opts.Token == "" {
		return ZoneInfo{}, errors.New("gcdns: Token required")
	}
	if opts.ProjectID == "" {
		return ZoneInfo{}, errors.New("gcdns: ProjectID required")
	}

	base := gcdnsDefaultBaseURL
	if opts.Endpoint != "" {
		base = opts.Endpoint
	}
	httpc := opts.HTTPClient
	if httpc == nil {
		httpc = &http.Client{Timeout: 15 * time.Second}
	}

	dnsName := domain + "."
	listURL := fmt.Sprintf("%s/projects/%s/managedZones?dnsName=%s", base, opts.ProjectID, dnsName)
	body, err := gcdnsDo(ctx, httpc, http.MethodGet, listURL, opts.Token, nil)
	if err != nil {
		return ZoneInfo{}, fmt.Errorf("gcdns: list zones: %w", err)
	}
	var listed gcdnsListResp
	if err := json.Unmarshal(body, &listed); err != nil {
		return ZoneInfo{}, fmt.Errorf("gcdns: parse list: %w", err)
	}
	for _, mz := range listed.ManagedZones {
		if strings.TrimSuffix(mz.DNSName, ".") == domain {
			return ZoneInfo{ZoneID: mz.Name, Nameservers: mz.NameServers, Created: false}, nil
		}
	}

	// Create.
	zoneName := fmt.Sprintf("entree-%d", time.Now().UnixNano())
	createBody, _ := json.Marshal(map[string]string{
		"name":        zoneName,
		"dnsName":     dnsName,
		"description": "Created by dns-entree migrate",
	})
	createURL := fmt.Sprintf("%s/projects/%s/managedZones", base, opts.ProjectID)
	respBody, err := gcdnsDo(ctx, httpc, http.MethodPost, createURL, opts.Token, createBody)
	if err != nil {
		return ZoneInfo{}, fmt.Errorf("gcdns: create zone: %w", err)
	}
	var created gcdnsManagedZone
	if err := json.Unmarshal(respBody, &created); err != nil {
		return ZoneInfo{}, fmt.Errorf("gcdns: parse create: %w", err)
	}
	return ZoneInfo{ZoneID: created.Name, Nameservers: created.NameServers, Created: true}, nil
}

func gcdnsDo(ctx context.Context, c *http.Client, method, url, token string, body []byte) ([]byte, error) {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		// Do not include token; only status + body (which from a stub never
		// echoes secrets).
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return data, nil
}
