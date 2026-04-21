// Package namecheap implements the dns-entree Provider interface for
// Namecheap's XML API (https://www.namecheap.com/support/api/).
//
// Namecheap API peculiarities worth calling out up front:
//
//  1. Eligibility gate. Namecheap requires the account to have $50+ of
//     lifetime spend or an equivalent balance before API access can be
//     enabled. Verify() surfaces this as a descriptive error so callers
//     can route the user to a manual-paste path when ineligible.
//
//  2. IP allowlist. Every API call must come from an IP the account
//     owner has whitelisted in Namecheap Profile -> Tools -> API Access.
//     Callers typically supply their egress IP via Credentials.ClientIP.
//
//  3. XML request/response. Responses are parsed with encoding/xml.
//
//  4. setHosts replaces ALL records for a domain. To add or update a
//     single record we must getHosts, merge, then setHosts with the
//     complete list. Losing existing records is the main risk; tests
//     cover the merge path.
package namecheap

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	entree "github.com/spoofcanary/dns-entree"
)

const (
	defaultBaseURL = "https://api.namecheap.com/xml.response"
	sandboxBaseURL = "https://api.sandbox.namecheap.com/xml.response"
)

func init() {
	entree.RegisterProvider("namecheap", func(creds entree.Credentials) (entree.Provider, error) {
		return NewProvider(creds.APIUser, creds.APIKey, creds.Username, creds.ClientIP)
	})
}

// Provider is the Namecheap DNS provider.
type Provider struct {
	apiUser    string
	apiKey     string
	username   string
	clientIP   string
	httpClient *http.Client
	baseURL    string
}

// NewProvider returns a configured Namecheap provider. clientIP must be
// whitelisted in the customer's Namecheap API settings or every call
// will 401 with "Invalid IP".
func NewProvider(apiUser, apiKey, username, clientIP string) (*Provider, error) {
	if apiUser == "" || apiKey == "" || clientIP == "" {
		return nil, errors.New("namecheap: api_user, api_key, and client_ip are required")
	}
	if username == "" {
		username = apiUser
	}
	return &Provider{
		apiUser:    apiUser,
		apiKey:     apiKey,
		username:   username,
		clientIP:   clientIP,
		httpClient: &http.Client{Timeout: 20 * time.Second},
		baseURL:    defaultBaseURL,
	}, nil
}

// NewSandboxProvider is NewProvider pointed at Namecheap's sandbox API.
// Used only by tests and provider smoke-tests.
func NewSandboxProvider(apiUser, apiKey, username, clientIP string) (*Provider, error) {
	p, err := NewProvider(apiUser, apiKey, username, clientIP)
	if err != nil {
		return nil, err
	}
	p.baseURL = sandboxBaseURL
	return p, nil
}

// Name returns the customer-facing provider name.
func (p *Provider) Name() string { return "Namecheap" }

// Slug returns the registry slug.
func (p *Provider) Slug() string { return "namecheap" }

// commonParams returns the 4 params required on every Namecheap API
// call along with the per-command params merged on top.
func (p *Provider) commonParams(command string) url.Values {
	v := url.Values{}
	v.Set("ApiUser", p.apiUser)
	v.Set("ApiKey", p.apiKey)
	v.Set("UserName", p.username)
	v.Set("ClientIp", p.clientIP)
	v.Set("Command", command)
	return v
}

// doXML POSTs a Namecheap API call and decodes the XML envelope. Returns
// an error when the API reports Status="ERROR" or transport fails.
func (p *Provider) doXML(ctx context.Context, params url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, strings.NewReader(params.Encode()))
	if err != nil {
		return fmt.Errorf("namecheap: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("namecheap: http call: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("namecheap: read body: %w", err)
	}
	if err := xml.Unmarshal(body, out); err != nil {
		return fmt.Errorf("namecheap: decode xml: %w (body=%s)", err, snippet(body))
	}
	return nil
}

func snippet(b []byte) string {
	if len(b) > 300 {
		return string(b[:300]) + "..."
	}
	return string(b)
}

// apiStatus captures the common status + errors portion of every
// Namecheap response. Each command has its own struct that embeds this
// plus the command-specific result path. encoding/xml has no equivalent
// of json.RawMessage, so two-phase unmarshal would be awkward.
//
// Note: no XMLName here because embedding an XMLName field breaks
// decoding into embedder structs (reflect panics). The top-level
// commands each declare their own XMLName.
type apiStatus struct {
	Status string     `xml:"Status,attr"`
	Errors []apiError `xml:"Errors>Error"`
}

type apiError struct {
	Number  string `xml:"Number,attr"`
	Message string `xml:",chardata"`
}

func (r *apiStatus) err() error {
	if strings.EqualFold(r.Status, "OK") {
		return nil
	}
	if len(r.Errors) == 0 {
		return errors.New("namecheap: unknown error")
	}
	msgs := make([]string, 0, len(r.Errors))
	for _, e := range r.Errors {
		msgs = append(msgs, fmt.Sprintf("%s (%s)", strings.TrimSpace(e.Message), e.Number))
	}
	return fmt.Errorf("namecheap: %s", strings.Join(msgs, "; "))
}

// getListResponse is the full envelope for namecheap.domains.getList.
type getListResponse struct {
	XMLName xml.Name `xml:"ApiResponse"`
	apiStatus
	Domains []struct {
		Name      string `xml:"Name,attr"`
		IsExpired string `xml:"IsExpired,attr"`
	} `xml:"CommandResponse>DomainGetListResult>Domain"`
}

// Verify returns the domains the API user owns. Also serves as a
// creds + IP allowlist check (fails early if either is wrong).
func (p *Provider) Verify(ctx context.Context) ([]entree.Zone, error) {
	params := p.commonParams("namecheap.domains.getList")
	params.Set("PageSize", "100")
	var resp getListResponse
	if err := p.doXML(ctx, params, &resp); err != nil {
		return nil, err
	}
	if err := resp.err(); err != nil {
		return nil, err
	}
	zones := make([]entree.Zone, 0, len(resp.Domains))
	for _, d := range resp.Domains {
		status := "active"
		if strings.EqualFold(d.IsExpired, "true") {
			status = "expired"
		}
		zones = append(zones, entree.Zone{Name: d.Name, Status: status})
	}
	return zones, nil
}

// GetRecords returns all current records for a domain, optionally filtered
// by type. The type filter is applied client-side since the Namecheap
// getHosts endpoint returns everything.
func (p *Provider) GetRecords(ctx context.Context, domain, recordType string) ([]entree.Record, error) {
	hosts, err := p.getHosts(ctx, domain)
	if err != nil {
		return nil, err
	}
	out := make([]entree.Record, 0, len(hosts))
	for _, h := range hosts {
		if recordType != "" && !strings.EqualFold(h.Type, recordType) {
			continue
		}
		out = append(out, hostToRecord(h, domain))
	}
	return out, nil
}

// SetRecord adds or replaces a single record. Because Namecheap's
// setHosts is all-or-nothing, we read the full host list, merge this
// record in (replacing any existing record at the same name+type), and
// write the full list back.
func (p *Provider) SetRecord(ctx context.Context, domain string, record entree.Record) error {
	hosts, err := p.getHosts(ctx, domain)
	if err != nil {
		return err
	}
	hosts = mergeRecord(hosts, record, domain)
	return p.setHosts(ctx, domain, hosts)
}

// DeleteRecord removes a record by its stable ID (HostId). If recordID
// is empty, this is a no-op. Namecheap does not expose per-record
// delete; we re-push the host list without the target record.
func (p *Provider) DeleteRecord(ctx context.Context, domain, recordID string) error {
	if recordID == "" {
		return errors.New("namecheap: DeleteRecord requires a record ID")
	}
	hosts, err := p.getHosts(ctx, domain)
	if err != nil {
		return err
	}
	kept := hosts[:0]
	for _, h := range hosts {
		if h.HostID != recordID {
			kept = append(kept, h)
		}
	}
	if len(kept) == len(hosts) {
		return fmt.Errorf("namecheap: record %s not found", recordID)
	}
	return p.setHosts(ctx, domain, kept)
}

// ApplyRecords writes the given records, merging each with the existing
// set (preserving any records not in the input). Namecheap limitation:
// this is still a single setHosts call so ordering matches the final
// merged list we build.
func (p *Provider) ApplyRecords(ctx context.Context, domain string, records []entree.Record) error {
	hosts, err := p.getHosts(ctx, domain)
	if err != nil {
		return err
	}
	for _, r := range records {
		hosts = mergeRecord(hosts, r, domain)
	}
	return p.setHosts(ctx, domain, hosts)
}

// host is the Namecheap per-record shape. Fields mirror the XML.
type host struct {
	HostID     string `xml:"HostId,attr"`
	Name       string `xml:"Name,attr"`
	Type       string `xml:"Type,attr"`
	Address    string `xml:"Address,attr"`
	MXPref     string `xml:"MXPref,attr"`
	TTL        string `xml:"TTL,attr"`
	AssociatedAppTitle string `xml:"AssociatedAppTitle,attr,omitempty"`
	IsActive   string `xml:"IsActive,attr,omitempty"`
}

func hostToRecord(h host, domain string) entree.Record {
	name := h.Name
	if name == "@" {
		name = domain
	} else if !strings.HasSuffix(name, "."+domain) {
		name = name + "." + domain
	}
	ttl, _ := strconv.Atoi(h.TTL)
	prio, _ := strconv.Atoi(h.MXPref)
	return entree.Record{
		ID:       h.HostID,
		Type:     strings.ToUpper(h.Type),
		Name:     name,
		Content:  h.Address,
		TTL:      ttl,
		Priority: prio,
	}
}

// splitDomain turns example.com into (sld="example", tld="com").
// Namecheap's dns.* commands require them as separate params.
func splitDomain(domain string) (string, string, error) {
	d := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")
	if d == "" {
		return "", "", errors.New("namecheap: empty domain")
	}
	idx := strings.Index(d, ".")
	if idx < 0 {
		return "", "", fmt.Errorf("namecheap: %q is not a full domain", domain)
	}
	return d[:idx], d[idx+1:], nil
}

// hostNameFor returns the Namecheap "HostName" for a record. Namecheap
// uses "@" for the root of the zone and the bare subdomain label for
// subdomain records ("_dmarc", "mail", etc.). Record.Name is the full
// FQDN in our model; we strip the trailing ".<domain>" here.
func hostNameFor(recordName, domain string) string {
	recordName = strings.TrimSuffix(strings.ToLower(recordName), ".")
	domain = strings.TrimSuffix(strings.ToLower(domain), ".")
	if recordName == "" || recordName == domain {
		return "@"
	}
	suffix := "." + domain
	if strings.HasSuffix(recordName, suffix) {
		return strings.TrimSuffix(recordName, suffix)
	}
	return recordName
}

// mergeRecord returns a new host slice with record either appended (if
// no match exists) or replacing a host of the same Name+Type.
func mergeRecord(hosts []host, record entree.Record, domain string) []host {
	targetName := hostNameFor(record.Name, domain)
	targetType := strings.ToUpper(record.Type)
	newHost := host{
		Name:    targetName,
		Type:    targetType,
		Address: record.Content,
		TTL:     strconv.Itoa(record.TTL),
	}
	if record.TTL == 0 {
		newHost.TTL = "1800"
	}
	if record.Priority > 0 {
		newHost.MXPref = strconv.Itoa(record.Priority)
	} else {
		newHost.MXPref = "10"
	}
	for i, h := range hosts {
		if strings.EqualFold(h.Name, targetName) && strings.EqualFold(h.Type, targetType) {
			// Replace in place.
			hosts[i] = newHost
			return hosts
		}
	}
	return append(hosts, newHost)
}

type getHostsResponse struct {
	XMLName xml.Name `xml:"ApiResponse"`
	apiStatus
	Hosts []host `xml:"CommandResponse>DomainDNSGetHostsResult>host"`
}

// setHostsResponse is used for the write call. It only needs the status.
type setHostsResponse struct {
	XMLName xml.Name `xml:"ApiResponse"`
	apiStatus
}

// getHosts reads the full host list for a domain.
func (p *Provider) getHosts(ctx context.Context, domain string) ([]host, error) {
	sld, tld, err := splitDomain(domain)
	if err != nil {
		return nil, err
	}
	params := p.commonParams("namecheap.domains.dns.getHosts")
	params.Set("SLD", sld)
	params.Set("TLD", tld)
	var resp getHostsResponse
	if err := p.doXML(ctx, params, &resp); err != nil {
		return nil, err
	}
	if err := resp.err(); err != nil {
		return nil, err
	}
	return resp.Hosts, nil
}

// setHosts writes the given host list, replacing all records.
func (p *Provider) setHosts(ctx context.Context, domain string, hosts []host) error {
	sld, tld, err := splitDomain(domain)
	if err != nil {
		return err
	}
	params := p.commonParams("namecheap.domains.dns.setHosts")
	params.Set("SLD", sld)
	params.Set("TLD", tld)
	for i, h := range hosts {
		idx := strconv.Itoa(i + 1)
		params.Set("HostName"+idx, h.Name)
		params.Set("RecordType"+idx, h.Type)
		params.Set("Address"+idx, h.Address)
		if h.MXPref != "" {
			params.Set("MXPref"+idx, h.MXPref)
		}
		if h.TTL != "" {
			params.Set("TTL"+idx, h.TTL)
		}
	}
	var resp setHostsResponse
	if err := p.doXML(ctx, params, &resp); err != nil {
		return err
	}
	return resp.err()
}
