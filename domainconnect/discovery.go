package domainconnect

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	settingsTimeout = 5 * time.Second
	maxBodyBytes    = 64 * 1024
)

// DiscoveryResult is the outcome of a Domain Connect discovery probe.
type DiscoveryResult struct {
	Supported    bool
	ProviderID   string
	ProviderName string
	URLSyncUX    string
	URLAsyncUX   string
	URLAPI       string
	Width        int
	Height       int
	Nameservers  []string
}

type discoverOptions struct {
	httpClient  *http.Client
	ipResolver  func(ctx context.Context, host string) ([]net.IP, error)
	txtResolver func(ctx context.Context, name string) ([]string, error)
}

// DiscoverOption configures Discover.
type DiscoverOption func(*discoverOptions)

// WithHTTPClient overrides the HTTP client used for the settings fetch.
func WithHTTPClient(c *http.Client) DiscoverOption {
	return func(o *discoverOptions) {
		if c != nil {
			o.httpClient = c
		}
	}
}

// WithResolver overrides the IP resolver used for SSRF screening.
func WithResolver(fn func(ctx context.Context, host string) ([]net.IP, error)) DiscoverOption {
	return func(o *discoverOptions) {
		if fn != nil {
			o.ipResolver = fn
		}
	}
}

// WithTXTResolver overrides the TXT resolver used to find the DC settings host.
func WithTXTResolver(fn func(ctx context.Context, name string) ([]string, error)) DiscoverOption {
	return func(o *discoverOptions) {
		if fn != nil {
			o.txtResolver = fn
		}
	}
}

func ipBlocked(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}

func anyIPBlocked(ips []net.IP) bool {
	if len(ips) == 0 {
		return true
	}
	for _, ip := range ips {
		if ipBlocked(ip) {
			return true
		}
	}
	return false
}

// Discover probes a domain for Domain Connect v2 support. It never returns
// network or parse errors; failures yield (DiscoveryResult{Supported:false}, nil).
// Errors are reserved for programmer mistakes (empty domain).
func Discover(ctx context.Context, domain string, opts ...DiscoverOption) (DiscoveryResult, error) {
	if domain == "" {
		return DiscoveryResult{}, errors.New("domainconnect: empty domain")
	}

	o := &discoverOptions{
		ipResolver: func(ctx context.Context, host string) ([]net.IP, error) {
			return net.DefaultResolver.LookupIP(ctx, "ip", host)
		},
		txtResolver: func(ctx context.Context, name string) ([]string, error) {
			return net.DefaultResolver.LookupTXT(ctx, name)
		},
	}
	for _, opt := range opts {
		opt(o)
	}
	if o.httpClient == nil {
		o.httpClient = &http.Client{
			Timeout:       settingsTimeout,
			CheckRedirect: makeCheckRedirect(o),
		}
	}

	txt, err := o.txtResolver(ctx, "_domainconnect."+domain)
	if err != nil || len(txt) == 0 {
		return DiscoveryResult{Supported: false}, nil
	}

	target := strings.TrimSpace(txt[0])
	if target == "" {
		return DiscoveryResult{Supported: false}, nil
	}
	host := target
	if i := strings.IndexByte(host, '/'); i >= 0 {
		host = host[:i]
	}
	host = strings.ToLower(host)

	ips, err := o.ipResolver(ctx, host)
	if err != nil || anyIPBlocked(ips) {
		return DiscoveryResult{Supported: false}, nil
	}

	settingsURL := "https://" + target + "/v2/" + domain + "/settings"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, settingsURL, nil)
	if err != nil {
		return DiscoveryResult{Supported: false}, nil
	}
	resp, err := o.httpClient.Do(req)
	if err != nil {
		return DiscoveryResult{Supported: false}, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return DiscoveryResult{Supported: false}, nil
	}

	var s struct {
		ProviderID   string `json:"providerId"`
		ProviderName string `json:"providerName"`
		URLSyncUX    string `json:"urlSyncUX"`
		URLAsyncUX   string `json:"urlAsyncUX"`
		URLAPI       string `json:"urlAPI"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, maxBodyBytes))
	if err := dec.Decode(&s); err != nil {
		return DiscoveryResult{Supported: false}, nil
	}

	ns := make([]string, 0, len(ips))
	for _, ip := range ips {
		ns = append(ns, ip.String())
	}

	return DiscoveryResult{
		Supported:    true,
		ProviderID:   s.ProviderID,
		ProviderName: s.ProviderName,
		URLSyncUX:    s.URLSyncUX,
		URLAsyncUX:   s.URLAsyncUX,
		URLAPI:       s.URLAPI,
		Width:        s.Width,
		Height:       s.Height,
		Nameservers:  ns,
	}, nil
}

func makeCheckRedirect(o *discoverOptions) func(req *http.Request, via []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("domainconnect: too many redirects")
		}
		if req.URL.Scheme != "https" {
			return errors.New("domainconnect: non-https redirect")
		}
		ips, err := o.ipResolver(req.Context(), req.URL.Hostname())
		if err != nil || anyIPBlocked(ips) {
			return errors.New("domainconnect: redirect host blocked")
		}
		return nil
	}
}
