package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

var (
	ErrEmptyKey       = errors.New("encryption key must not be empty")
	ErrInvalidPayload = errors.New("invalid encrypted payload")
)

// Cipher provides authenticated symmetric encryption (AES-256-GCM).
// The 32-byte AES key is derived from an arbitrary key string via SHA-256,
// so any passphrase length is accepted.
type Cipher struct {
	gcm cipher.AEAD
}

func NewCipher(key string) (*Cipher, error) {
	if key == "" {
		return nil, ErrEmptyKey
	}
	sum := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, fmt.Errorf("create aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	return &Cipher{gcm: gcm}, nil
}

// Encrypt returns base64(nonce || ciphertext) for storage in a TEXT column.
func (c *Cipher) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext := c.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (c *Cipher) Decrypt(encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("%w: invalid base64: %v", ErrInvalidPayload, err)
	}
	ns := c.gcm.NonceSize()
	if len(raw) < ns {
		return "", ErrInvalidPayload
	}
	nonce, ciphertext := raw[:ns], raw[ns:]
	plaintext, err := c.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidPayload, err)
	}
	return string(plaintext), nil
}
