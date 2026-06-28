package auth_test

import (
	"strings"
	"testing"

	"github.com/derpixler/skolva/internal/auth"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := auth.HashPassword("s3cr3t-pw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if hash == "s3cr3t-pw" {
		t.Fatal("hash must not equal plaintext")
	}
	if !auth.VerifyPassword(hash, "s3cr3t-pw") {
		t.Error("expected correct password to verify")
	}
	if auth.VerifyPassword(hash, "wrong-pw") {
		t.Error("expected wrong password to fail verification")
	}
}

func TestHashPasswordUsesRandomSalt(t *testing.T) {
	h1, err := auth.HashPassword("samepw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	h2, err := auth.HashPassword("samepw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h1 == h2 {
		t.Error("expected different hashes for identical input (random salt)")
	}
}

func TestHashPasswordEmpty(t *testing.T) {
	if _, err := auth.HashPassword(""); err == nil {
		t.Error("expected error for empty password")
	}
}

func TestHashPasswordTooLong(t *testing.T) {
	long := strings.Repeat("a", 73)
	if _, err := auth.HashPassword(long); err == nil {
		t.Error("expected error for password longer than 72 bytes")
	}
}

func TestVerifyPasswordInvalidHash(t *testing.T) {
	if auth.VerifyPassword("not-a-bcrypt-hash", "whatever") {
		t.Error("expected verification to fail for an invalid hash")
	}
}
