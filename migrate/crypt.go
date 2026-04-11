package migrate

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ErrNoStateKey indicates no state key was configured (env empty, derivation
// not attempted). Callers decide whether to fall back to deriveStateKey.
var ErrNoStateKey = errors.New("migrate: no state key configured")

// LoadStateKey reads ENTREE_STATE_KEY from env (D-09). Empty env returns
// (nil, false, nil) - not an error; caller decides fallback. A valid key is
// 64 hex chars (32 bytes). env is injected for testability; pass os.Getenv in
// production.
func LoadStateKey(env func(string) string) ([]byte, bool, error) {
	raw := env("ENTREE_STATE_KEY")
	if raw == "" {
		return nil, false, nil
	}
	key, err := hex.DecodeString(raw)
	if err != nil {
		return nil, false, fmt.Errorf("migrate: ENTREE_STATE_KEY not valid hex: %w", err)
	}
	if len(key) != 32 {
		return nil, false, fmt.Errorf("migrate: ENTREE_STATE_KEY must decode to 32 bytes, got %d", len(key))
	}
	return key, true, nil
}

// DeriveStateKey returns a deterministic 32-byte key derived from the host
// name and a random salt stored at <stateDir>/.key-salt (mode 0600). The salt
// is created on first call and reused thereafter, so repeated calls in the
// same dir return the same key. Callers should emit a startup WARNING when
// falling back to this path (D-09).
func DeriveStateKey(stateDir string) ([]byte, error) {
	if stateDir == "" {
		return nil, errors.New("migrate: state dir required")
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return nil, fmt.Errorf("migrate: mkdir state dir: %w", err)
	}
	saltPath := filepath.Join(stateDir, ".key-salt")

	salt, err := os.ReadFile(saltPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("migrate: read salt: %w", err)
		}
		salt = make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, salt); err != nil {
			return nil, fmt.Errorf("migrate: generate salt: %w", err)
		}
		// O_EXCL guards against races; first writer wins (T-07-03).
		f, err := os.OpenFile(saltPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			if os.IsExist(err) {
				// Another process beat us; re-read.
				salt, err = os.ReadFile(saltPath)
				if err != nil {
					return nil, fmt.Errorf("migrate: re-read salt: %w", err)
				}
			} else {
				return nil, fmt.Errorf("migrate: create salt: %w", err)
			}
		} else {
			if _, werr := f.Write(salt); werr != nil {
				f.Close()
				return nil, fmt.Errorf("migrate: write salt: %w", werr)
			}
			if cerr := f.Close(); cerr != nil {
				return nil, fmt.Errorf("migrate: close salt: %w", cerr)
			}
		}
	}

	hostname, _ := os.Hostname()
	h := sha256.New()
	h.Write([]byte(hostname))
	h.Write([]byte{0})
	h.Write(salt)
	return h.Sum(nil), nil
}

// EncryptCreds seals plaintext with AES-256-GCM. The returned blob is
// nonce || ciphertext || tag. Key must be exactly 32 bytes.
func EncryptCreds(key, plaintext []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("migrate: key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("migrate: new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("migrate: new gcm: %w", err)
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("migrate: nonce: %w", err)
	}
	return aead.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptCreds opens a blob produced by EncryptCreds. Any tampering is
// detected via the GCM auth tag (T-07-02).
func DecryptCreds(key, blob []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("migrate: key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("migrate: new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("migrate: new gcm: %w", err)
	}
	ns := aead.NonceSize()
	if len(blob) < ns {
		return nil, errors.New("migrate: ciphertext too short")
	}
	nonce, ct := blob[:ns], blob[ns:]
	pt, err := aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("migrate: decrypt: %w", err)
	}
	return pt, nil
}
