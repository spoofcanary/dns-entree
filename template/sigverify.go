package template

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
)

// InvalidSignatureError is returned when signature verification fails.
type InvalidSignatureError struct{ Msg string }

func (e *InvalidSignatureError) Error() string { return "InvalidSignature: " + e.Msg }

// PubKeyLookupFunc resolves a DNS TXT record. The default implementation uses
// net.LookupTXT but callers can inject a stub for testing.
type PubKeyLookupFunc func(name string) ([]string, error)

// defaultPubKeyLookup uses the system DNS resolver.
func defaultPubKeyLookup(name string) ([]string, error) {
	return net.LookupTXT(name)
}

// VerifySignature checks an RSA-SHA256 PKCS1v15 signature over a query string.
//
// keyHost is the key= parameter value (e.g. "_dck1").
// syncPubKeyDomain is from the template JSON (e.g. "exampleservice.domainconnect.org").
// The public key is looked up as a TXT record at <keyHost>.<syncPubKeyDomain>.
//
// sig is the base64-encoded (standard alphabet) RSA signature.
// qs is the raw query string that was signed.
func VerifySignature(qs, sig, keyHost, syncPubKeyDomain string, lookup PubKeyLookupFunc) error {
	if sig == "" {
		return &InvalidSignatureError{Msg: "empty signature"}
	}
	if keyHost == "" {
		return &InvalidSignatureError{Msg: "empty key host"}
	}
	if syncPubKeyDomain == "" {
		return &InvalidSignatureError{Msg: "empty syncPubKeyDomain"}
	}

	// Look up TXT record for the public key.
	fqdn := keyHost + "." + syncPubKeyDomain
	txts, err := lookup(fqdn)
	if err != nil {
		return &InvalidSignatureError{Msg: fmt.Sprintf("DNS lookup %q: %v", fqdn, err)}
	}

	// Parse DC-format TXT records: "p=N,a=RS256,d=<base64chunk>"
	// Concatenate chunks ordered by p= index.
	pubKeyB64 := parseDCPubKeyTXT(txts)
	if pubKeyB64 == "" {
		return &InvalidSignatureError{Msg: fmt.Sprintf("no public key at %q", fqdn)}
	}

	// Decode the base64 DER public key.
	pubKeyDER, err := base64.StdEncoding.DecodeString(pubKeyB64)
	if err != nil {
		return &InvalidSignatureError{Msg: fmt.Sprintf("base64 decode public key: %v", err)}
	}

	// Parse the DER-encoded public key.
	pubKeyIface, err := x509.ParsePKIXPublicKey(pubKeyDER)
	if err != nil {
		return &InvalidSignatureError{Msg: fmt.Sprintf("parse public key: %v", err)}
	}
	rsaPubKey, ok := pubKeyIface.(*rsa.PublicKey)
	if !ok {
		return &InvalidSignatureError{Msg: "public key is not RSA"}
	}

	// Decode the signature.
	sigBytes, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		return &InvalidSignatureError{Msg: fmt.Sprintf("base64 decode signature: %v", err)}
	}

	// Verify: SHA256 hash of the query string, PKCS1v15.
	h := sha256.Sum256([]byte(qs))
	if err := rsa.VerifyPKCS1v15(rsaPubKey, crypto.SHA256, h[:], sigBytes); err != nil {
		return &InvalidSignatureError{Msg: fmt.Sprintf("signature verification failed: %v", err)}
	}

	return nil
}

// parseDCPubKeyTXT parses Domain Connect public key TXT records.
// Format: "p=N,a=RS256,d=<base64chunk>" where N is the part number.
// If records don't match this format, they're concatenated as-is (plain base64).
func parseDCPubKeyTXT(txts []string) string {
	type part struct {
		order int
		data  string
	}
	var parts []part
	plain := true

	for _, txt := range txts {
		txt = strings.TrimSpace(txt)
		if txt == "" {
			continue
		}
		// Try DC format: p=N,a=RS256,d=<data>
		if strings.HasPrefix(txt, "p=") {
			fields := strings.SplitN(txt, ",", 3)
			if len(fields) == 3 {
				pStr := strings.TrimPrefix(fields[0], "p=")
				p, err := strconv.Atoi(pStr)
				if err == nil {
					dField := fields[2]
					if strings.HasPrefix(dField, "d=") {
						parts = append(parts, part{order: p, data: strings.TrimPrefix(dField, "d=")})
						plain = false
						continue
					}
				}
			}
		}
		// Fallback: plain base64
		parts = append(parts, part{order: len(parts), data: txt})
	}

	if !plain && len(parts) > 0 {
		sort.Slice(parts, func(i, j int) bool { return parts[i].order < parts[j].order })
	}

	var b strings.Builder
	for _, p := range parts {
		b.WriteString(p.data)
	}
	return b.String()
}
