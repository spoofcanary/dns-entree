package api

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net/http"
	"strings"

	entree "github.com/spoofcanary/dns-entree"
	"github.com/spoofcanary/dns-entree/domainconnect"
)

// handleDCDiscover handles POST /v1/dc/discover. Body: {"domain": "..."}.
// Returns DiscoveryResult on Supported==true, 404 TEMPLATE_NOT_FOUND otherwise.
func (s *Server) handleDCDiscover(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Domain string `json:"domain"`
	}
	if !decodeJSON(w, r, BodyLimitDefault, &req) {
		return
	}
	if strings.TrimSpace(req.Domain) == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "domain required", nil)
		return
	}
	if err := entree.ValidateDNSName(req.Domain); err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "invalid domain: "+err.Error(), nil)
		return
	}
	result, err := domainconnect.Discover(r.Context(), req.Domain)
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "invalid domain", nil)
		return
	}
	if !result.Supported {
		writeError(w, http.StatusNotFound, CodeTemplateNotFound, "domain does not support Domain Connect", nil)
		return
	}
	writeJSON(w, result)
}

// dcApplyURLRequest carries ApplyURLOpts for the handler. The PEM private key
// is accepted in the body only, never in headers, and is never logged. The
// slog middleware in this package only sees method/path/status/duration, so
// body contents cannot reach logs via the request log line.
type dcApplyURLRequest struct {
	Opts dcApplyURLOpts `json:"opts"`
}

type dcApplyURLOpts struct {
	URLAsyncUX    string            `json:"url_async_ux"`
	ProviderID    string            `json:"provider_id"`
	ServiceID     string            `json:"service_id"`
	Domain        string            `json:"domain"`
	Host          string            `json:"host"`
	Params        map[string]string `json:"params"`
	PrivateKeyPEM string            `json:"private_key_pem"`
	KeyHost       string            `json:"key_host"`
	RedirectURI   string            `json:"redirect_uri"`
	State         string            `json:"state"`
}

// handleDCApplyURL handles POST /v1/dc/apply-url. Pure URL construction — no
// outbound HTTP is performed. The PEM key is parsed in-process and zeroed
// from memory as soon as BuildApplyURL returns.
func (s *Server) handleDCApplyURL(w http.ResponseWriter, r *http.Request) {
	var req dcApplyURLRequest
	if !decodeJSON(w, r, BodyLimitDefault, &req) {
		return
	}
	if req.Opts.Domain != "" {
		if err := entree.ValidateDNSName(req.Opts.Domain); err != nil {
			writeError(w, http.StatusBadRequest, CodeBadRequest, "invalid domain: "+err.Error(), nil)
			return
		}
	}
	if req.Opts.PrivateKeyPEM == "" {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "private_key_pem required", nil)
		return
	}
	key, err := parseRSAPrivateKeyPEM(req.Opts.PrivateKeyPEM)
	// Clear the PEM bytes from the request struct so a later marshal / reflect
	// cannot observe them. Defense in depth against accidental logging.
	req.Opts.PrivateKeyPEM = ""
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "invalid PEM private key", nil)
		return
	}
	url, err := domainconnect.BuildApplyURL(domainconnect.ApplyURLOpts{
		URLAsyncUX:  req.Opts.URLAsyncUX,
		ProviderID:  req.Opts.ProviderID,
		ServiceID:   req.Opts.ServiceID,
		Domain:      req.Opts.Domain,
		Host:        req.Opts.Host,
		Params:      req.Opts.Params,
		PrivateKey:  key,
		KeyHost:     req.Opts.KeyHost,
		RedirectURI: req.Opts.RedirectURI,
		State:       req.Opts.State,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	writeJSON(w, map[string]any{"url": url})
}

func parseRSAPrivateKeyPEM(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("no PEM block found")
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaKey, ok := k.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("PEM key is not RSA")
	}
	return rsaKey, nil
}
