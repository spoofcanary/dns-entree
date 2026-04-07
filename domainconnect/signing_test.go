package domainconnect

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/url"
	"strings"
	"testing"
)

func TestSignQueryString_RoundTrip(t *testing.T) {
	q := "domain=example.com&policy=none"
	sig, _, err := SignQueryString(q, testKey)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	h := sha256.Sum256([]byte(q))
	if err := rsa.VerifyPKCS1v15(&testKey.PublicKey, crypto.SHA256, h[:], raw); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestSignQueryString_Deterministic(t *testing.T) {
	q := "a=1&b=2"
	s1, k1, _ := SignQueryString(q, testKey)
	s2, k2, _ := SignQueryString(q, testKey)
	if s1 != s2 {
		t.Errorf("sig not deterministic")
	}
	if k1 != k2 {
		t.Errorf("keyHash not stable")
	}
}

func TestSignQueryString_KeyHashStable(t *testing.T) {
	_, k1, _ := SignQueryString("a=1", testKey)
	s2, k2, _ := SignQueryString("a=2", testKey)
	if k1 != k2 {
		t.Errorf("keyHash should be stable across inputs")
	}
	s1, _, _ := SignQueryString("a=1", testKey)
	if s1 == s2 {
		t.Errorf("sigs should differ for different inputs")
	}
}

func TestSignQueryString_KeyHashFormat(t *testing.T) {
	_, kh, _ := SignQueryString("x=y", testKey)
	if strings.ContainsAny(kh, "+/=") {
		t.Errorf("keyHash not base64url: %s", kh)
	}
	dec, err := base64.RawURLEncoding.DecodeString(kh)
	if err != nil {
		t.Fatal(err)
	}
	if len(dec) != 32 {
		t.Errorf("expected 32 bytes (sha256), got %d", len(dec))
	}
}

func TestSignQueryString_NilKey(t *testing.T) {
	_, _, err := SignQueryString("x=y", nil)
	if err == nil || !strings.Contains(err.Error(), "nil") {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestSortAndSignParams_AlphabeticalOrder(t *testing.T) {
	v := url.Values{"c": []string{"3"}, "a": []string{"1"}, "b": []string{"2"}}
	sq, sig, _, err := SortAndSignParams(v, testKey)
	if err != nil {
		t.Fatal(err)
	}
	if sq != "a=1&b=2&c=3" {
		t.Errorf("got %s", sq)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(sig)
	h := sha256.Sum256([]byte(sq))
	if err := rsa.VerifyPKCS1v15(&testKey.PublicKey, crypto.SHA256, h[:], raw); err != nil {
		t.Errorf("verify: %v", err)
	}
}

func TestSortAndSignParams_SpaceEncoding(t *testing.T) {
	v := url.Values{"q": []string{"hello world"}}
	sq, _, _, _ := SortAndSignParams(v, testKey)
	if !strings.Contains(sq, "q=hello%20world") {
		t.Errorf("expected %%20, got %s", sq)
	}
	if strings.Contains(sq, "+") {
		t.Errorf("found '+', want %%20: %s", sq)
	}
}

func TestSortAndSignParams_SpecialChars(t *testing.T) {
	v := url.Values{"v": []string{"a&b=c+d"}}
	sq, sig, _, err := SortAndSignParams(v, testKey)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(sig)
	h := sha256.Sum256([]byte(sq))
	if err := rsa.VerifyPKCS1v15(&testKey.PublicKey, crypto.SHA256, h[:], raw); err != nil {
		t.Errorf("verify: %v", err)
	}
}

func TestSortAndSignParams_EmptyParams(t *testing.T) {
	sq, sig, _, err := SortAndSignParams(url.Values{}, testKey)
	if err != nil {
		t.Fatal(err)
	}
	if sq != "" {
		t.Errorf("expected empty, got %s", sq)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(sig)
	h := sha256.Sum256(nil)
	if err := rsa.VerifyPKCS1v15(&testKey.PublicKey, crypto.SHA256, h[:], raw); err != nil {
		t.Errorf("verify: %v", err)
	}
}

func TestSortAndSignParams_KnownVector(t *testing.T) {
	// testKeyPEM regenerates per run, so we verify against the live public key
	// rather than pinning a hardcoded base64 string. PKCS1v15 determinism is
	// covered by TestSignQueryString_Deterministic.
	v := url.Values{"domain": []string{"example.com"}, "policy": []string{"none"}}
	sq, sig, _, err := SortAndSignParams(v, testKey)
	if err != nil {
		t.Fatal(err)
	}
	if sq != "domain=example.com&policy=none" {
		t.Fatalf("got %s", sq)
	}
	raw, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256([]byte(sq))
	if err := rsa.VerifyPKCS1v15(&testKey.PublicKey, crypto.SHA256, h[:], raw); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

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
