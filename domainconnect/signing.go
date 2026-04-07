package domainconnect

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
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
