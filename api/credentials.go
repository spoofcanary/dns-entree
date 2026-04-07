package api

import (
	"encoding/base64"
	"fmt"
	"net/http"

	entree "github.com/spoofcanary/dns-entree"
)

// providerHeaderSpecs lists the required X-Entree-* headers per provider slug
// (D-05). Optional headers (region, project id) are looked up directly inside
// parseCredentialHeaders rather than enforced here.
var providerHeaderSpecs = map[string][]string{
	"cloudflare":       {"X-Entree-Cloudflare-Token"},
	"route53":          {"X-Entree-AWS-Access-Key-Id", "X-Entree-AWS-Secret-Access-Key"},
	"godaddy":          {"X-Entree-GoDaddy-Key", "X-Entree-GoDaddy-Secret"},
	"google_cloud_dns": {"X-Entree-GCDNS-Service-Account-JSON"},
	"fake":             {},
}

// credentialError is returned by parseCredentialHeaders. It carries the stable
// error code, a safe message, and structured details that NEVER include header
// values (D-26).
type credentialError struct {
	status  int
	code    string
	message string
	details map[string]any
}

func (e *credentialError) Error() string { return e.message }

// writeTo writes this credential error onto the response.
func (e *credentialError) writeTo(w http.ResponseWriter) {
	writeError(w, e.status, e.code, e.message, e.details)
}

// parseCredentialHeaders extracts the provider slug and credential set from
// X-Entree-* headers. It never echoes a header value into the returned error.
// When credentialRedactMW has scrubbed the request headers, this function
// transparently reads from the saved-originals context bag.
func parseCredentialHeaders(r *http.Request) (string, entree.Credentials, *credentialError) {
	get := func(name string) string {
		if v := originalCredentialHeader(r, name); v != "" {
			return v
		}
		return r.Header.Get(name)
	}
	slug := get("X-Entree-Provider")
	if slug == "" {
		return "", entree.Credentials{}, &credentialError{
			status:  http.StatusBadRequest,
			code:    CodeMissingCredentials,
			message: "missing X-Entree-Provider header",
			details: map[string]any{"expected_header": "X-Entree-Provider"},
		}
	}
	expected, known := providerHeaderSpecs[slug]
	if !known {
		return "", entree.Credentials{}, &credentialError{
			status:  http.StatusBadRequest,
			code:    CodeBadRequest,
			message: fmt.Sprintf("unknown provider %q", slug),
			details: map[string]any{"provider": slug},
		}
	}
	var missing []string
	for _, h := range expected {
		if get(h) == "" {
			missing = append(missing, h)
		}
	}
	if len(missing) > 0 {
		return "", entree.Credentials{}, &credentialError{
			status:  http.StatusBadRequest,
			code:    CodeMissingCredentials,
			message: "missing credential headers",
			details: map[string]any{
				"provider":         slug,
				"expected_headers": expected,
				"missing_headers":  missing,
			},
		}
	}

	var c entree.Credentials
	switch slug {
	case "cloudflare":
		c.APIToken = get("X-Entree-Cloudflare-Token")
	case "route53":
		c.AccessKey = get("X-Entree-AWS-Access-Key-Id")
		c.SecretKey = get("X-Entree-AWS-Secret-Access-Key")
		c.Region = get("X-Entree-AWS-Region")
	case "godaddy":
		c.APIKey = get("X-Entree-GoDaddy-Key")
		c.APISecret = get("X-Entree-GoDaddy-Secret")
	case "google_cloud_dns":
		raw := get("X-Entree-GCDNS-Service-Account-JSON")
		decoded, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return "", entree.Credentials{}, &credentialError{
				status:  http.StatusBadRequest,
				code:    CodeMissingCredentials,
				message: "X-Entree-GCDNS-Service-Account-JSON is not valid base64",
				details: map[string]any{"provider": slug},
			}
		}
		c.Token = string(decoded)
		c.ProjectID = get("X-Entree-GCDNS-Project-Id")
	case "fake":
		// no credentials
	}
	return slug, c, nil
}

// credentialHeaderNames is the set of header keys (canonicalised) that must
// never be logged or echoed. Used by middleware to enforce defense in depth.
var credentialHeaderNames = func() map[string]struct{} {
	m := map[string]struct{}{
		http.CanonicalHeaderKey("X-Entree-Provider"):                   {},
		http.CanonicalHeaderKey("X-Entree-Cloudflare-Token"):           {},
		http.CanonicalHeaderKey("X-Entree-AWS-Access-Key-Id"):          {},
		http.CanonicalHeaderKey("X-Entree-AWS-Secret-Access-Key"):      {},
		http.CanonicalHeaderKey("X-Entree-AWS-Region"):                 {},
		http.CanonicalHeaderKey("X-Entree-GoDaddy-Key"):                {},
		http.CanonicalHeaderKey("X-Entree-GoDaddy-Secret"):             {},
		http.CanonicalHeaderKey("X-Entree-GCDNS-Service-Account-JSON"): {},
		http.CanonicalHeaderKey("X-Entree-GCDNS-Project-Id"):           {},
	}
	return m
}()
