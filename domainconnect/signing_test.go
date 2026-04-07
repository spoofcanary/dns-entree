package domainconnect

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
)

func signAndVerify(t *testing.T, key *rsa.PrivateKey) {
	t.Helper()
	h := sha256.Sum256([]byte("test payload"))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, h[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, h[:], sig); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestLoadPrivateKey_PKCS8(t *testing.T) {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKCS8PrivateKey(k)
	p := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	loaded, err := LoadPrivateKey(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	signAndVerify(t, loaded)
}

func TestLoadPrivateKey_PKCS1(t *testing.T) {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(k),
	})
	loaded, err := LoadPrivateKey(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	signAndVerify(t, loaded)
}

func TestLoadPrivateKey_EmbeddedFixture(t *testing.T) {
	loaded, err := LoadPrivateKey(testKeyPEM)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded == nil {
		t.Fatal("nil key")
	}
	if loaded.PublicKey.N.BitLen() != 2048 {
		t.Errorf("expected 2048 bits, got %d", loaded.PublicKey.N.BitLen())
	}
}

func TestLoadPrivateKey_NoPEMBlock(t *testing.T) {
	_, err := LoadPrivateKey([]byte("garbage"))
	if err == nil || !strings.Contains(err.Error(), "PEM") {
		t.Fatalf("expected PEM error, got %v", err)
	}
}

func TestLoadPrivateKey_UnknownBlockType(t *testing.T) {
	p := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: []byte{0x01, 0x02}})
	_, err := LoadPrivateKey(p)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadPrivateKey_NotRSA(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	p := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	_, err = LoadPrivateKey(p)
	if err == nil || !strings.Contains(err.Error(), "not RSA") {
		t.Fatalf("expected not RSA error, got %v", err)
	}
}

func TestLoadPrivateKey_CorruptDER(t *testing.T) {
	p := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte{0xde, 0xad, 0xbe, 0xef}})
	_, err := LoadPrivateKey(p)
	if err == nil {
		t.Fatal("expected error")
	}
}
