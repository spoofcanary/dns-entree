package domainconnect

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"net/url"
)

// ApplyURLOpts configures BuildApplyURL.
//
// KeyHost is the DNS host where the public key TXT record lives (placed in
// the &key= URL parameter per the Domain Connect spec). It is NOT the same as
// the keyHash returned by SignQueryString — that value is informational only,
// for caller logging and key rotation.
//
// Reserved keys: callers must not pass "domain", "host", "redirect_uri",
// "state", "key", or "sig" inside Params. Behavior is undefined if they do
// (last write wins, but key/sig will still be appended after signing).
type ApplyURLOpts struct {
	URLAsyncUX  string
	ProviderID  string
	ServiceID   string
	Domain      string
	Host        string            // optional subdomain; "" omits &host=
	Params      map[string]string // template variables
	PrivateKey  *rsa.PrivateKey
	KeyHost     string // DNS host where _domainconnect public key TXT lives
	RedirectURI string // optional
	State       string // optional
}

// BuildApplyURL produces a single signed Domain Connect async apply URL of
// the form:
//
//	{URLAsyncUX}/v2/domainTemplates/providers/{ProviderID}/services/{ServiceID}/apply?{sorted-params}&key={KeyHost}&sig={base64url-sig}
//
// All template variables (Params) plus the implicit fields (domain, host,
// redirect_uri, state) are sorted alphabetically and URL-encoded with %20 for
// spaces (NOT '+') before signing. The key and sig parameters are appended
// AFTER signing and are not part of the signed string.
func BuildApplyURL(opts ApplyURLOpts) (string, error) {
	if opts.URLAsyncUX == "" {
		return "", errors.New("domainconnect: BuildApplyURL: URLAsyncUX required")
	}
	if opts.ProviderID == "" {
		return "", errors.New("domainconnect: BuildApplyURL: ProviderID required")
	}
	if opts.ServiceID == "" {
		return "", errors.New("domainconnect: BuildApplyURL: ServiceID required")
	}
	if opts.Domain == "" {
		return "", errors.New("domainconnect: BuildApplyURL: Domain required")
	}
	if opts.PrivateKey == nil {
		return "", errors.New("domainconnect: BuildApplyURL: PrivateKey required")
	}

	v := url.Values{}
	v.Set("domain", opts.Domain)
	if opts.Host != "" {
		v.Set("host", opts.Host)
	}
	if opts.RedirectURI != "" {
		v.Set("redirect_uri", opts.RedirectURI)
	}
	if opts.State != "" {
		v.Set("state", opts.State)
	}
	for k, val := range opts.Params {
		v.Set(k, val)
	}

	sortedQuery, sig, _, err := SortAndSignParams(v, opts.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("domainconnect: BuildApplyURL: %w", err)
	}

	fullQuery := sortedQuery + "&key=" + url.QueryEscape(opts.KeyHost) + "&sig=" + url.QueryEscape(sig)
	path := "/v2/domainTemplates/providers/" + opts.ProviderID + "/services/" + opts.ServiceID + "/apply"
	return opts.URLAsyncUX + path + "?" + fullQuery, nil
}
