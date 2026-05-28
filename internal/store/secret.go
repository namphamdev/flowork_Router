// Secret-at-Rest (AES-256-GCM).

package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const encPrefix = "enc:v1:"

var (
	secretKeyOnce  sync.Once
	secretKeyBytes []byte
)

// secretKey loads (or generates once) the 32-byte machine key.
func secretKey() []byte {
	secretKeyOnce.Do(func() {
		path := filepath.Join(dataDir(), "secret.key")
		if b, err := os.ReadFile(path); err == nil && len(b) >= 32 {
			secretKeyBytes = b[:32]
			return
		}
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			secretKeyBytes = make([]byte, 32) // last resort; still functional
			return
		}
		_ = os.MkdirAll(dataDir(), 0o700)
		_ = os.WriteFile(path, key, 0o600)
		secretKeyBytes = key
	})
	return secretKeyBytes
}

func gcm() (cipher.AEAD, error) {
	block, err := aes.NewCipher(secretKey())
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// EncryptSecret returns "enc:v1:<base64(nonce|ciphertext)>". Empty input and
// already-encrypted input are returned unchanged (idempotent).
func EncryptSecret(plain string) string {
	if plain == "" || strings.HasPrefix(plain, encPrefix) {
		return plain
	}
	g, err := gcm()
	if err != nil {
		return plain // never lose the value
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return plain
	}
	ct := g.Seal(nonce, nonce, []byte(plain), nil)
	return encPrefix + base64.RawStdEncoding.EncodeToString(ct)
}

// DecryptSecret reverses EncryptSecret. Non-prefixed (legacy plaintext) values
// and any decryption failure are returned as-is.
func DecryptSecret(stored string) string {
	if !strings.HasPrefix(stored, encPrefix) {
		return stored
	}
	raw, err := base64.RawStdEncoding.DecodeString(stored[len(encPrefix):])
	if err != nil {
		return stored
	}
	g, err := gcm()
	if err != nil || len(raw) < g.NonceSize() {
		return stored
	}
	nonce, ct := raw[:g.NonceSize()], raw[g.NonceSize():]
	pt, err := g.Open(nil, nonce, ct, nil)
	if err != nil {
		return stored
	}
	return string(pt)
}
