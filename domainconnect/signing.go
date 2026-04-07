package domainconnect

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// LoadPrivateKey parses an RSA private key from a PEM block. Both PKCS8
// ("PRIVATE KEY") and PKCS1 ("RSA PRIVATE KEY") encodings are supported.
// Encrypted PEM is out of scope.
func LoadPrivateKey(pemData []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, errors.New("domainconnect: no PEM block found in key data")
	}
	switch block.Type {
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("domainconnect: parse PKCS8 key: %w", err)
		}
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("domainconnect: key is not RSA")
		}
		return rsaKey, nil
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("domainconnect: parse PKCS1 key: %w", err)
		}
		return key, nil
	default:
		return nil, fmt.Errorf("domainconnect: unsupported PEM block type %q", block.Type)
	}
}

// SignQueryString signs the given query string with RSA-SHA256 PKCS1v15 and
// returns the base64url (no padding) encoded signature plus a keyHash.
//
// PKCS1v15 is deterministic: the same (key, query) pair always produces the
// same signature bytes. base64url uses '-' and '_' (URL-safe) so the signature
// can be appended to a URL without further escaping.
//
// keyHash is base64url(sha256(SubjectPublicKeyInfo DER)) and is informational
// only — useful for logging and key rotation. It is NOT the value to put in
// the `key=` URL parameter; that parameter holds the DNS host where the public
// key TXT record lives (see ApplyURLOpts.KeyHost).
func SignQueryString(query string, key *rsa.PrivateKey) (sig string, keyHash string, err error) {
	if key == nil {
		return "", "", errors.New("domainconnect: nil private key")
	}
	h := sha256.Sum256([]byte(query))
	raw, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, h[:])
	if err != nil {
		return "", "", fmt.Errorf("domainconnect: sign: %w", err)
	}
	sig = base64.RawURLEncoding.EncodeToString(raw)

	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return "", "", fmt.Errorf("domainconnect: marshal public key: %w", err)
	}
	pubHash := sha256.Sum256(pubDER)
	keyHash = base64.RawURLEncoding.EncodeToString(pubHash[:])
	return sig, keyHash, nil
}

// SortAndSignParams alphabetically sorts the keys in params, URL-encodes each
// value with url.QueryEscape (which uses %20 for spaces, NOT '+'), joins them
// as k=v&k=v, and signs the resulting string. Returns the sorted query string
// alongside the signature and keyHash.
//
// url.Values.Encode() is intentionally NOT used: it form-encodes spaces as
// '+', which would cause Domain Connect providers to reject the signature.
func SortAndSignParams(params url.Values, key *rsa.PrivateKey) (sortedQuery, sig, keyHash string, err error) {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(k)
		b.WriteByte('=')
		// url.QueryEscape uses '+' for spaces (form-encoding); DC spec needs
		// %20. '+' literals are escaped to %2B by QueryEscape, so this
		// targeted replacement is safe.
		b.WriteString(strings.ReplaceAll(url.QueryEscape(params.Get(k)), "+", "%20"))
	}
	sortedQuery = b.String()
	sig, keyHash, err = SignQueryString(sortedQuery, key)
	if err != nil {
		return "", "", "", err
	}
	return sortedQuery, sig, keyHash, nil
}
