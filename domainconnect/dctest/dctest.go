// Package dctest provides a mock Domain Connect provider for testing.
//
// The mock server implements the full DC async flow: discovery via TXT +
// settings JSON, signature verification on apply, and configurable responses.
// Use it in tests that need to exercise the round-trip without hitting a real
// DNS provider.
//
//	srv := dctest.NewServer(dctest.WithKey(privateKey))
//	defer srv.Close()
//	// srv.URL is the DC provider base URL
//	// srv.TXTRecord() returns the _domainconnect TXT value
//	// srv.ApplyRequests() returns all received apply requests
package dctest

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
)

// ApplyRequest captures a single apply request received by the mock server.
type ApplyRequest struct {
	ProviderID string
	ServiceID  string
	Domain     string
	Host       string
	Params     url.Values
	Signature  string
	KeyHost    string
	SigValid   bool
}

// Option configures a mock DC server.
type Option func(*Server)

// WithKey sets the RSA key pair used for signature verification. The server
// publishes the public key at the TXT record host. If not set, a 2048-bit
// key is generated on startup.
func WithKey(key *rsa.PrivateKey) Option {
	return func(s *Server) { s.key = key }
}

// WithProviderID sets the provider ID returned in discovery. Default: "dctest".
func WithProviderID(id string) Option {
	return func(s *Server) { s.providerID = id }
}

// WithProviderName sets the provider display name. Default: "DC Test Provider".
func WithProviderName(name string) Option {
	return func(s *Server) { s.providerName = name }
}

// WithApplyHandler sets a custom handler for apply requests. The default
// handler returns 200 with {"status":"applied"}. Return a non-nil error
// to make the mock return 400.
func WithApplyHandler(fn func(req ApplyRequest) error) Option {
	return func(s *Server) { s.applyHandler = fn }
}

// Server is a mock Domain Connect provider backed by httptest.Server.
type Server struct {
	// URL is the base URL of the mock server (e.g. http://127.0.0.1:PORT).
	URL string

	ts           *httptest.Server
	key          *rsa.PrivateKey
	providerID   string
	providerName string
	applyHandler func(ApplyRequest) error

	mu       sync.Mutex
	requests []ApplyRequest
}

// NewServer creates and starts a mock DC provider. Call Close() when done.
func NewServer(opts ...Option) *Server {
	s := &Server{
		providerID:   "dctest",
		providerName: "DC Test Provider",
	}
	for _, o := range opts {
		o(s)
	}
	if s.key == nil {
		k, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			panic("dctest: generate key: " + err.Error())
		}
		s.key = k
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/domain-connect", s.handleSettings)
	// DC protocol: settings are at /v2/{domain}/settings
	mux.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/settings") {
			s.handleSettings(w, r)
			return
		}
		if strings.Contains(r.URL.Path, "/domainTemplates/providers/") {
			s.handleApply(w, r)
			return
		}
		http.NotFound(w, r)
	})

	s.ts = httptest.NewTLSServer(mux)
	s.URL = s.ts.URL
	return s
}

// Close shuts down the mock server.
func (s *Server) Close() {
	s.ts.Close()
}

// TXTRecord returns the value that should be in the _domainconnect TXT record
// for domains pointing at this mock provider.
func (s *Server) TXTRecord() string {
	u, _ := url.Parse(s.URL)
	return u.Host
}

// PublicKeyPEM returns the PEM-encoded public key for TXT record publication.
func (s *Server) PublicKeyPEM() string {
	der, err := x509.MarshalPKIXPublicKey(&s.key.PublicKey)
	if err != nil {
		panic("dctest: marshal public key: " + err.Error())
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

// PublicKeyBase64 returns the base64-encoded DER public key (the format used
// in DNS TXT records for DC signature verification).
func (s *Server) PublicKeyBase64() string {
	der, err := x509.MarshalPKIXPublicKey(&s.key.PublicKey)
	if err != nil {
		panic("dctest: marshal public key: " + err.Error())
	}
	return base64.StdEncoding.EncodeToString(der)
}

// PrivateKey returns the server's private key for use in signing apply URLs.
func (s *Server) PrivateKey() *rsa.PrivateKey {
	return s.key
}

// ApplyRequests returns all apply requests received by the mock server.
func (s *Server) ApplyRequests() []ApplyRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ApplyRequest, len(s.requests))
	copy(out, s.requests)
	return out
}

// Reset clears recorded requests.
func (s *Server) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = nil
}

// HTTPClient returns an *http.Client configured to talk to the mock server.
func (s *Server) HTTPClient() *http.Client {
	return s.ts.Client()
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{
		"providerName": s.providerName,
		"providerID":   s.providerID,
		"urlAsyncUX":   s.URL,
		"urlSyncUX":    s.URL,
		"urlAPI":       s.URL,
		"width":        750,
		"height":       750,
		"nameservers":  []string{"ns1.dctest.example", "ns2.dctest.example"},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleApply(w http.ResponseWriter, r *http.Request) {
	// Parse path: /v2/domainTemplates/providers/{pid}/services/{sid}/apply
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v2/domainTemplates/providers/"), "/")
	if len(parts) < 4 || parts[1] != "services" || parts[3] != "apply" {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	q := r.URL.Query()
	req := ApplyRequest{
		ProviderID: parts[0],
		ServiceID:  parts[2],
		Domain:     q.Get("domain"),
		Host:       q.Get("host"),
		Signature:  q.Get("sig"),
		KeyHost:    q.Get("key"),
		Params:     q,
	}

	// Verify signature: reconstruct the signed query string (all params
	// except key and sig, sorted alphabetically).
	req.SigValid = s.verifySignature(q)

	s.mu.Lock()
	s.requests = append(s.requests, req)
	s.mu.Unlock()

	if s.applyHandler != nil {
		if err := s.applyHandler(req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "applied"})
}

func (s *Server) verifySignature(q url.Values) bool {
	sig := q.Get("sig")
	if sig == "" {
		return false
	}

	// Reconstruct the signed portion: all params except "key" and "sig", sorted.
	signed := url.Values{}
	for k, vs := range q {
		if k == "key" || k == "sig" {
			continue
		}
		for _, v := range vs {
			signed.Add(k, v)
		}
	}

	// Sort and encode with %20 (not +)
	keys := make([]string, 0, len(signed))
	for k := range signed {
		keys = append(keys, k)
	}
	sortStrings(keys)

	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(strings.ReplaceAll(url.QueryEscape(signed.Get(k)), "+", "%20"))
	}

	sigBytes, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		return false
	}

	h := sha256.Sum256([]byte(b.String()))
	return rsa.VerifyPKCS1v15(&s.key.PublicKey, crypto.SHA256, h[:], sigBytes) == nil
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
