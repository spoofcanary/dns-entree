package entree

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// DetectionResult is returned by DetectProvider.
type DetectionResult struct {
	Provider    ProviderType
	Label       string
	Supported   bool
	Nameservers []string
	Method      string // "ns_pattern" | "rdap_fallback"
}

type nsPattern struct {
	pattern  string
	provider ProviderType
}

// nsPatterns: 16 NS substring -> provider mappings. First match wins.
var nsPatterns = []nsPattern{
	{"cloudflare.com", ProviderCloudflare},
	{"awsdns", ProviderRoute53},
	{"domaincontrol.com", ProviderGoDaddy},
	{"godaddy.com", ProviderGoDaddy},
	{"googledomains.com", ProviderGoogleCloudDNS},
	{"squarespace", ProviderType("squarespace")},
	{"namecheap", ProviderType("namecheap")},
	{"hover.com", ProviderType("hover")},
	{"digitalocean.com", ProviderType("digitalocean")},
	{"hetzner.com", ProviderType("hetzner")},
	{"registrar-servers.com", ProviderType("namecheap")},
	// Namecheap BasicDNS (the default free DNS for Namecheap-registered
	// domains) uses orbit/horizon.dns-parking.com nameservers.
	{"dns-parking.com", ProviderType("namecheap")},
	{"nsone.net", ProviderType("ns1")},
	{"dnsv.jp", ProviderType("dnsv")},
	{"linode.com", ProviderType("linode")},
	{"vultr.com", ProviderType("vultr")},
	{"dnsimple.com", ProviderType("dnsimple")},
}

var providerLabels = map[ProviderType]string{
	ProviderCloudflare:           "Cloudflare",
	ProviderRoute53:              "Amazon Route 53",
	ProviderGoDaddy:              "GoDaddy",
	ProviderGoogleCloudDNS:       "Google Cloud DNS",
	ProviderType("squarespace"):  "Squarespace",
	ProviderType("namecheap"):    "Namecheap",
	ProviderType("hover"):        "Hover",
	ProviderType("digitalocean"): "DigitalOcean",
	ProviderType("hetzner"):      "Hetzner",
	ProviderType("ns1"):          "NS1",
	ProviderType("dnsv"):         "DNSV",
	ProviderType("linode"):       "Linode",
	ProviderType("vultr"):        "Vultr",
	ProviderType("dnsimple"):     "DNSimple",
}

// tier1Providers is the set of providers with direct API support in dns-entree
// Phase 1. Used for the Supported flag (D-19). Hardcoded to avoid registry
// init-order coupling.
var tier1Providers = map[ProviderType]bool{
	ProviderCloudflare:         true,
	ProviderRoute53:            true,
	ProviderGoDaddy:            true,
	ProviderGoogleCloudDNS:     true,
	ProviderType("namecheap"):  true,
}

// DetectFromNS classifies a provider from a list of NS hostnames. Pure,
// deterministic, no I/O. First pattern match wins.
func DetectFromNS(hosts []string) DetectionResult {
	result := DetectionResult{
		Nameservers: hosts,
		Method:      "ns_pattern",
	}
	for _, host := range hosts {
		hl := strings.ToLower(host)
		for _, p := range nsPatterns {
			if strings.Contains(hl, p.pattern) {
				result.Provider = p.provider
				result.Label = providerLabels[p.provider]
				result.Supported = tier1Providers[p.provider]
				return result
			}
		}
	}
	return result
}

// DetectProvider resolves NS records for a domain and identifies the DNS
// hosting provider. Falls back to RDAP registrar lookup when NS detection is
// ambiguous (googledomains.com) or unknown.
func DetectProvider(ctx context.Context, domain string) (*DetectionResult, error) {
	nsRecords, err := net.LookupNS(domain)
	if err != nil {
		return &DetectionResult{Method: "ns_pattern"}, err
	}
	hosts := make([]string, 0, len(nsRecords))
	for _, ns := range nsRecords {
		hosts = append(hosts, strings.ToLower(strings.TrimSuffix(ns.Host, ".")))
	}

	result := DetectFromNS(hosts)

	ambiguous := result.Provider == "" || result.Provider == ProviderGoogleCloudDNS
	if ambiguous {
		registrar := lookupRegistrar(ctx, domain)
		if registrar != "" {
			override := registrarToProvider(registrar)
			if override != "" && override != result.Provider {
				result.Provider = override
				result.Label = providerLabels[override]
				result.Supported = tier1Providers[override]
				result.Method = "rdap_fallback"
			}
		}
	}

	return &result, nil
}

var registrarPatterns = []struct {
	pattern  string
	provider ProviderType
}{
	{"squarespace", ProviderType("squarespace")},
	{"godaddy", ProviderGoDaddy},
	{"cloudflare", ProviderCloudflare},
	{"namecheap", ProviderType("namecheap")},
	{"amazon", ProviderRoute53},
	{"route 53", ProviderRoute53},
	{"google", ProviderGoogleCloudDNS},
	{"hover", ProviderType("hover")},
	{"digitalocean", ProviderType("digitalocean")},
	{"hetzner", ProviderType("hetzner")},
}

func registrarToProvider(registrar string) ProviderType {
	lower := strings.ToLower(registrar)
	for _, p := range registrarPatterns {
		if strings.Contains(lower, p.pattern) {
			return p.provider
		}
	}
	return ""
}

// rdapEndpoints returns the list of RDAP URLs to try for a domain.
// Overridable in tests.
var rdapEndpoints = func(domain string) []string {
	tld := domain
	if idx := strings.LastIndex(domain, "."); idx >= 0 {
		tld = domain[idx+1:]
	}
	switch tld {
	case "com", "net":
		return []string{fmt.Sprintf("https://rdap.verisign.com/%s/v1/domain/%s", tld, domain)}
	case "org":
		return []string{fmt.Sprintf("https://rdap.org/domain/%s", domain)}
	case "io":
		return []string{fmt.Sprintf("https://rdap.nic.io/domain/%s", domain)}
	}
	return []string{
		fmt.Sprintf("https://rdap.identitydigital.services/rdap/domain/%s", domain),
		fmt.Sprintf("https://rdap.verisign.com/com/v1/domain/%s", domain),
		fmt.Sprintf("https://rdap.org/domain/%s", domain),
	}
}

// LookupRegistrar queries RDAP for the domain registrar name. Returns empty
// string on any failure (best-effort, never blocks detection).
func LookupRegistrar(ctx context.Context, domain string) string {
	return lookupRegistrar(ctx, domain)
}

func lookupRegistrar(ctx context.Context, domain string) string {
	client := &http.Client{Timeout: 5 * time.Second}

	for _, rdapURL := range rdapEndpoints(domain) {
		req, err := http.NewRequestWithContext(ctx, "GET", rdapURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Accept", "application/rdap+json")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			continue
		}

		var rdap struct {
			Entities []struct {
				Roles      []string          `json:"roles"`
				VCardArray []json.RawMessage `json:"vcardArray"`
			} `json:"entities"`
		}
		if err := json.Unmarshal(body, &rdap); err != nil {
			continue
		}

		for _, entity := range rdap.Entities {
			for _, role := range entity.Roles {
				if role != "registrar" {
					continue
				}
				if len(entity.VCardArray) <= 1 {
					continue
				}
				var vcard [][]interface{}
				if err := json.Unmarshal(entity.VCardArray[1], &vcard); err != nil {
					continue
				}
				for _, field := range vcard {
					if len(field) < 4 {
						continue
					}
					name, ok := field[0].(string)
					if !ok || name != "fn" {
						continue
					}
					if val, ok := field[3].(string); ok && val != "" {
						return val
					}
				}
			}
		}
	}

	return ""
}
