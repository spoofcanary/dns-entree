package migrate

import (
	"bytes"
	"testing"
)

// TestSecurityAESGCMRoundTrip tests encrypt-then-decrypt for various payload sizes.
func TestSecurityAESGCMRoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 32)

	cases := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"one_byte", []byte{0xFF}},
		{"small", []byte("hello world")},
		{"1KB", bytes.Repeat([]byte("A"), 1024)},
		{"1MB", bytes.Repeat([]byte("B"), 1024*1024)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			blob, err := EncryptCreds(key, tc.data)
			if err != nil {
				t.Fatalf("encrypt: %v", err)
			}
			got, err := DecryptCreds(key, blob)
			if err != nil {
				t.Fatalf("decrypt: %v", err)
			}
			if !bytes.Equal(got, tc.data) {
				t.Fatalf("round-trip mismatch: got %d bytes, want %d", len(got), len(tc.data))
			}
		})
	}
}

// TestSecurityAESGCMTamperDetection verifies that flipping a single bit in
// the ciphertext causes decryption to fail.
func TestSecurityAESGCMTamperDetection(t *testing.T) {
	key := bytes.Repeat([]byte{0x17}, 32)
	plaintext := []byte("sensitive credentials payload")

	blob, err := EncryptCreds(key, plaintext)
	if err != nil {
		t.Fatal(err)
	}

	// Flip one bit in the middle of the ciphertext.
	tampered := make([]byte, len(blob))
	copy(tampered, blob)
	tampered[len(tampered)/2] ^= 0x01

	if _, err := DecryptCreds(key, tampered); err == nil {
		t.Fatal("tampered ciphertext: expected decrypt error")
	}
}

// TestSecurityAESGCMWrongKey verifies that using a different key fails.
func TestSecurityAESGCMWrongKey(t *testing.T) {
	key1 := bytes.Repeat([]byte{0xAA}, 32)
	key2 := bytes.Repeat([]byte{0xBB}, 32)

	blob, err := EncryptCreds(key1, []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := DecryptCreds(key2, blob); err == nil {
		t.Fatal("wrong key: expected decrypt error")
	}
}

// TestSecurityAESGCMNonceUniqueness verifies that encrypting the same
// plaintext twice produces different ciphertexts (because nonces differ).
func TestSecurityAESGCMNonceUniqueness(t *testing.T) {
	key := bytes.Repeat([]byte{0x55}, 32)
	plaintext := []byte("same plaintext")

	blob1, err := EncryptCreds(key, plaintext)
	if err != nil {
		t.Fatal(err)
	}
	blob2, err := EncryptCreds(key, plaintext)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Equal(blob1, blob2) {
		t.Fatal("two encryptions of same plaintext produced identical ciphertext; nonces should differ")
	}
}
