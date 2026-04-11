package migrate

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validHexKey() string {
	return strings.Repeat("ab", 32) // 64 hex chars = 32 bytes
}

func TestLoadStateKey(t *testing.T) {
	// Empty env -> no key, no error.
	key, fromEnv, err := LoadStateKey(func(string) string { return "" })
	if err != nil || fromEnv || key != nil {
		t.Fatalf("empty env: got (%v,%v,%v)", key, fromEnv, err)
	}

	// Valid hex.
	key, fromEnv, err = LoadStateKey(func(k string) string {
		if k == "ENTREE_STATE_KEY" {
			return validHexKey()
		}
		return ""
	})
	if err != nil {
		t.Fatalf("valid hex: %v", err)
	}
	if !fromEnv || len(key) != 32 {
		t.Fatalf("want 32-byte key fromEnv, got len=%d fromEnv=%v", len(key), fromEnv)
	}

	// Invalid hex.
	if _, _, err := LoadStateKey(func(string) string { return "zzzz" }); err == nil {
		t.Fatal("invalid hex: want error")
	}

	// Wrong length (valid hex but 16 bytes).
	if _, _, err := LoadStateKey(func(string) string { return hex.EncodeToString(make([]byte, 16)) }); err == nil {
		t.Fatal("wrong length: want error")
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 32)
	cases := [][]byte{
		{},
		[]byte("x"),
		bytes.Repeat([]byte("A"), 1024),
	}
	for i, pt := range cases {
		blob, err := EncryptCreds(key, pt)
		if err != nil {
			t.Fatalf("case %d encrypt: %v", i, err)
		}
		got, err := DecryptCreds(key, blob)
		if err != nil {
			t.Fatalf("case %d decrypt: %v", i, err)
		}
		if !bytes.Equal(got, pt) {
			t.Fatalf("case %d round-trip mismatch", i)
		}
	}

	// Nil key rejected.
	if _, err := EncryptCreds(nil, []byte("x")); err == nil {
		t.Fatal("nil key: want error")
	}
	if _, err := DecryptCreds(nil, []byte("x")); err == nil {
		t.Fatal("nil key decrypt: want error")
	}
}

func TestDecryptTampered(t *testing.T) {
	key := bytes.Repeat([]byte{0x17}, 32)
	blob, err := EncryptCreds(key, []byte("secret payload"))
	if err != nil {
		t.Fatal(err)
	}
	// Flip a byte in the ciphertext region (after nonce).
	blob[len(blob)-1] ^= 0x01
	if _, err := DecryptCreds(key, blob); err == nil {
		t.Fatal("tampered blob: want decrypt error")
	}
}

func TestDeriveStateKey(t *testing.T) {
	dir := t.TempDir()
	k1, err := DeriveStateKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(k1) != 32 {
		t.Fatalf("want 32-byte key, got %d", len(k1))
	}
	k2, err := DeriveStateKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(k1, k2) {
		t.Fatal("derived key not stable across calls")
	}
	info, err := os.Stat(filepath.Join(dir, ".key-salt"))
	if err != nil {
		t.Fatalf("salt missing: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("salt mode = %v, want 0600", info.Mode().Perm())
	}
}
