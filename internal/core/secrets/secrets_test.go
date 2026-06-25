package secrets_test

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/derpixler/skolva/internal/core/secrets"
)

func TestNewCipherEmptyKey(t *testing.T) {
	if _, err := secrets.NewCipher(""); !errors.Is(err, secrets.ErrEmptyKey) {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	c, err := secrets.NewCipher("a-passphrase")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, plain := range []string{"", "totp-secret", "äöü-special-✓"} {
		enc, err := c.Encrypt(plain)
		if err != nil {
			t.Fatalf("encrypt failed: %v", err)
		}
		if enc == plain && plain != "" {
			t.Errorf("ciphertext must not equal plaintext for %q", plain)
		}
		dec, err := c.Decrypt(enc)
		if err != nil {
			t.Fatalf("decrypt failed: %v", err)
		}
		if dec != plain {
			t.Errorf("round-trip mismatch: got %q, want %q", dec, plain)
		}
	}
}

func TestEncryptUsesRandomNonce(t *testing.T) {
	c, _ := secrets.NewCipher("a-passphrase")
	e1, _ := c.Encrypt("same")
	e2, _ := c.Encrypt("same")
	if e1 == e2 {
		t.Error("expected different ciphertext for identical input (random nonce)")
	}
}

func TestDecryptTampered(t *testing.T) {
	c, _ := secrets.NewCipher("a-passphrase")
	enc, _ := c.Encrypt("totp-secret")

	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	raw[len(raw)-1] ^= 0xFF
	tampered := base64.StdEncoding.EncodeToString(raw)

	if _, err := c.Decrypt(tampered); !errors.Is(err, secrets.ErrInvalidPayload) {
		t.Errorf("expected ErrInvalidPayload for tampered ciphertext, got %v", err)
	}
}

func TestDecryptInvalidBase64(t *testing.T) {
	c, _ := secrets.NewCipher("a-passphrase")
	if _, err := c.Decrypt("not base64!!!"); !errors.Is(err, secrets.ErrInvalidPayload) {
		t.Errorf("expected ErrInvalidPayload for invalid base64, got %v", err)
	}
}

func TestDecryptTooShort(t *testing.T) {
	c, _ := secrets.NewCipher("a-passphrase")
	short := base64.StdEncoding.EncodeToString([]byte{0x01, 0x02})
	if _, err := c.Decrypt(short); !errors.Is(err, secrets.ErrInvalidPayload) {
		t.Errorf("expected ErrInvalidPayload for too-short payload, got %v", err)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	enc, _ := mustCipher(t, "key-a").Encrypt("totp-secret")
	if _, err := mustCipher(t, "key-b").Decrypt(enc); !errors.Is(err, secrets.ErrInvalidPayload) {
		t.Errorf("expected ErrInvalidPayload when decrypting with wrong key, got %v", err)
	}
}

func mustCipher(t *testing.T, key string) *secrets.Cipher {
	t.Helper()
	c, err := secrets.NewCipher(key)
	if err != nil {
		t.Fatalf("NewCipher(%q) failed: %v", key, err)
	}
	return c
}
