package migrate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const godaddyDefaultBaseURL = "https://api.godaddy.com/v1"

// GoDaddyAdapter implements Adapter for GoDaddy. Zone create is a no-op:
// GoDaddy ties zones to domain registration, so the only thing we can do is
// confirm the domain is registered to the account and return its current
// authoritative nameservers.
type GoDaddyAdapter struct{}

func init() {
	RegisterAdapter("godaddy", GoDaddyAdapter{})
}

type godaddyDomain struct {
	Domain      string   `json:"domain"`
	Status      string   `json:"status"`
	NameServers []string `json:"nameServers"`
}

func (GoDaddyAdapter) EnsureZone(ctx context.Context, domain string, opts ProviderOpts) (ZoneInfo, error) {
	if err := validateDomain(domain); err != nil {
		return ZoneInfo{}, err
	}
	if err := validateEndpoint(opts.Endpoint); err != nil {
		return ZoneInfo{}, err
	}
	if opts.APIKey == "" || opts.APISecret == "" {
		return ZoneInfo{}, errors.New("godaddy: APIKey and APISecret required")
	}

	base := godaddyDefaultBaseURL
	if opts.Endpoint != "" {
		base = opts.Endpoint
	}
	httpc := opts.HTTPClient
	if httpc == nil {
		httpc = &http.Client{Timeout: 15 * time.Second}
	}

	url := fmt.Sprintf("%s/domains/%s", base, domain)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ZoneInfo{}, err
	}
	req.Header.Set("Authorization", "sso-key "+opts.APIKey+":"+opts.APISecret)
	resp, err := httpc.Do(req)
	if err != nil {
		return ZoneInfo{}, fmt.Errorf("godaddy: get domain: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusNotFound {
		return ZoneInfo{}, fmt.Errorf("godaddy: domain %s not registered to this account; GoDaddy does not support API zone creation (zones are tied to registration)", domain)
	}
	if resp.StatusCode >= 400 {
		return ZoneInfo{}, fmt.Errorf("godaddy: get domain status %d", resp.StatusCode)
	}

	var d godaddyDomain
	if err := json.Unmarshal(body, &d); err != nil {
		return ZoneInfo{}, fmt.Errorf("godaddy: decode: %w", err)
	}
	// Some godaddy responses omit nameServers from /domains/{domain}; that is
	// acceptable - caller can resolve via authoritative lookup separately.
	return ZoneInfo{
		ZoneID:      strings.ToLower(d.Domain),
		Nameservers: d.NameServers,
		Created:     false,
	}, nil
}
