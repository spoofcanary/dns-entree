package domainconnect

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
)

// testKeyPEM is a 2048-bit RSA private key in PKCS8 PEM form, generated
// fresh on every test run. Test-only; never used in production.
//
// It is populated in init() rather than embedded as a const because the test
// environment cannot shell out to openssl. Plan 02 may reuse it via package-
// internal _test.go visibility.
var testKeyPEM []byte

// testKey is the parsed form of testKeyPEM.
var testKey *rsa.PrivateKey

func init() {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic("testkey: generate: " + err.Error())
	}
	der, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		panic("testkey: marshal: " + err.Error())
	}
	testKeyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	testKey = k
}
