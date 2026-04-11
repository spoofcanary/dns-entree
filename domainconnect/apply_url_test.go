package domainconnect

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
)

func mustOpts(t *testing.T) ApplyURLOpts {
	t.Helper()
	return ApplyURLOpts{
		URLAsyncUX: "https://example.async",
		ProviderID: "p",
		ServiceID:  "s",
		Domain:     "example.com",
		PrivateKey: testKey,
		KeyHost:    "_dc-key.example.com",
	}
}

func TestBuildApplyURL_Structure(t *testing.T) {
	o := mustOpts(t)
	o.Host = "www"
	u, err := BuildApplyURL(o)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(u, "https://example.async/v2/domainTemplates/providers/p/services/s/apply?") {
		t.Fatalf("bad prefix: %s", u)
	}
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatal(err)
	}
	q := parsed.Query()
	for _, k := range []string{"domain", "host", "key", "sig"} {
		if q.Get(k) == "" {
			t.Errorf("missing %s", k)
		}
	}
}

func TestBuildApplyURL_AlphabeticalOrder(t *testing.T) {
	o := mustOpts(t)
	o.Params = map[string]string{"zeta": "z", "alpha": "a", "beta": "b"}
	u, err := BuildApplyURL(o)
	if err != nil {
		t.Fatal(err)
	}
	qStart := strings.Index(u, "?") + 1
	keyIdx := strings.Index(u, "&key=")
	signed := u[qStart:keyIdx]
	want := "alpha=a&beta=b&domain=example.com&zeta=z"
	if signed != want {
		t.Errorf("order mismatch:\ngot:  %s\nwant: %s", signed, want)
	}
}

func TestBuildApplyURL_HostOptional_Empty(t *testing.T) {
	o := mustOpts(t)
	u, err := BuildApplyURL(o)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(u, "host=") {
		t.Errorf("expected no host=, got %s", u)
	}
}

func TestBuildApplyURL_HostIncluded(t *testing.T) {
	o := mustOpts(t)
	o.Host = "www"
	u, _ := BuildApplyURL(o)
	if !strings.Contains(u, "host=www") {
		t.Errorf("missing host=www: %s", u)
	}
}

func TestBuildApplyURL_RedirectURIIncluded(t *testing.T) {
	o := mustOpts(t)
	o.RedirectURI = "https://app.example/cb"
	u, _ := BuildApplyURL(o)
	parsed, _ := url.Parse(u)
	if parsed.Query().Get("redirect_uri") != "https://app.example/cb" {
		t.Errorf("missing redirect_uri")
	}
}

func TestBuildApplyURL_StateIncluded(t *testing.T) {
	o := mustOpts(t)
	o.State = "xyz"
	u, _ := BuildApplyURL(o)
	parsed, _ := url.Parse(u)
	if parsed.Query().Get("state") != "xyz" {
		t.Errorf("missing state")
	}
}

func TestBuildApplyURL_KeyParamIsKeyHost(t *testing.T) {
	o := mustOpts(t)
	u, _ := BuildApplyURL(o)
	parsed, _ := url.Parse(u)
	if parsed.Query().Get("key") != "_dc-key.example.com" {
		t.Errorf("key param wrong: %s", parsed.Query().Get("key"))
	}
}

func TestBuildApplyURL_SignatureNotInSignable(t *testing.T) {
	o := mustOpts(t)
	o.Host = "www"
	o.Params = map[string]string{"foo": "bar baz", "zed": "1"}
	u, err := BuildApplyURL(o)
	if err != nil {
		t.Fatal(err)
	}

	// Extract signed substring (between '?' and '&key=')
	qStart := strings.Index(u, "?") + 1
	keyIdx := strings.Index(u, "&key=")
	signed := u[qStart:keyIdx]

	// Recompute via SortAndSignParams from the parsed query (minus key/sig)
	parsed, _ := url.Parse(u)
	q := parsed.Query()
	sigStr := q.Get("sig")
	q.Del("key")
	q.Del("sig")
	recomputed, _, _, err := SortAndSignParams(q, testKey)
	if err != nil {
		t.Fatal(err)
	}
	if recomputed != signed {
		t.Errorf("recomputed sorted query mismatch:\ngot:  %s\nwant: %s", recomputed, signed)
	}

	// Verify sig
	rawSig, err := base64.StdEncoding.DecodeString(sigStr)
	if err != nil {
		t.Fatalf("decode sig: %v", err)
	}
	h := sha256.Sum256([]byte(signed))
	if err := rsa.VerifyPKCS1v15(&testKey.PublicKey, crypto.SHA256, h[:], rawSig); err != nil {
		t.Errorf("verify: %v", err)
	}
}

func TestBuildApplyURL_SpaceEncoding(t *testing.T) {
	o := mustOpts(t)
	o.Params = map[string]string{"name": "hello world"}
	u, _ := BuildApplyURL(o)
	if !strings.Contains(u, "name=hello%20world") {
		t.Errorf("expected %%20 encoding, got %s", u)
	}
	if strings.Contains(u, "name=hello+world") {
		t.Errorf("found '+' encoding, want %%20: %s", u)
	}
}

func TestBuildApplyURL_SpecialChars(t *testing.T) {
	o := mustOpts(t)
	o.Params = map[string]string{"v": "a&b=c"}
	u, err := BuildApplyURL(o)
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(u)
	q := parsed.Query()
	if q.Get("v") != "a&b=c" {
		t.Errorf("round-trip failed: %q", q.Get("v"))
	}
	// Verify sig still good
	sigStr := q.Get("sig")
	q.Del("key")
	q.Del("sig")
	recomputed, _, _, _ := SortAndSignParams(q, testKey)
	rawSig, _ := base64.StdEncoding.DecodeString(sigStr)
	h := sha256.Sum256([]byte(recomputed))
	if err := rsa.VerifyPKCS1v15(&testKey.PublicKey, crypto.SHA256, h[:], rawSig); err != nil {
		t.Errorf("verify: %v", err)
	}
}

func TestBuildApplyURL_NilPrivateKey(t *testing.T) {
	o := mustOpts(t)
	o.PrivateKey = nil
	if _, err := BuildApplyURL(o); err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildApplyURL_EmptyURLAsyncUX(t *testing.T) {
	o := mustOpts(t)
	o.URLAsyncUX = ""
	if _, err := BuildApplyURL(o); err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildApplyURL_EmptyProviderOrService(t *testing.T) {
	o := mustOpts(t)
	o.ProviderID = ""
	if _, err := BuildApplyURL(o); err == nil {
		t.Fatal("expected error")
	}
	o = mustOpts(t)
	o.ServiceID = ""
	if _, err := BuildApplyURL(o); err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildApplyURL_EmptyDomain(t *testing.T) {
	o := mustOpts(t)
	o.Domain = ""
	if _, err := BuildApplyURL(o); err == nil {
		t.Fatal("expected error")
	}
}
