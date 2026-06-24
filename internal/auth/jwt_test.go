package auth

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestNewTokenManagerEmptySecret(t *testing.T) {
	if _, err := NewTokenManager("", time.Hour); !errors.Is(err, ErrEmptySecret) {
		t.Errorf("expected ErrEmptySecret, got %v", err)
	}
}

func TestIssueAccessAndVerifyRoundTrip(t *testing.T) {
	m, err := NewTokenManager("super-secret", time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	roles := []string{"admin", "mitglied"}
	perms := []string{"users.read", "users.write"}
	tok, err := m.IssueAccess("user-1", "a@example.com", roles, perms)
	if err != nil {
		t.Fatalf("issue failed: %v", err)
	}

	claims, err := m.Verify(tok)
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if claims.Subject != "user-1" {
		t.Errorf("unexpected subject: %s", claims.Subject)
	}
	if claims.Email != "a@example.com" {
		t.Errorf("unexpected email: %s", claims.Email)
	}
	if claims.Kind != TokenKindAccess {
		t.Errorf("expected kind %s, got %s", TokenKindAccess, claims.Kind)
	}
	if !slices.Equal(claims.Roles, roles) {
		t.Errorf("unexpected roles: %v", claims.Roles)
	}
	if !slices.Equal(claims.Permissions, perms) {
		t.Errorf("unexpected permissions: %v", claims.Permissions)
	}
}

func TestIssuePending2FA(t *testing.T) {
	m, err := NewTokenManager("super-secret", time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tok, err := m.IssuePending2FA("user-2", 5*time.Minute)
	if err != nil {
		t.Fatalf("issue failed: %v", err)
	}

	claims, err := m.Verify(tok)
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if claims.Subject != "user-2" {
		t.Errorf("unexpected subject: %s", claims.Subject)
	}
	if claims.Kind != TokenKindPending2FA {
		t.Errorf("expected kind %s, got %s", TokenKindPending2FA, claims.Kind)
	}
}

func TestIssueEmptySubject(t *testing.T) {
	m, _ := NewTokenManager("super-secret", time.Hour)
	if _, err := m.IssueAccess("", "a@example.com", nil, nil); !errors.Is(err, ErrEmptySubject) {
		t.Errorf("expected ErrEmptySubject, got %v", err)
	}
}

func TestVerifyWrongSecret(t *testing.T) {
	issuer, _ := NewTokenManager("secret-a", time.Hour)
	verifier, _ := NewTokenManager("secret-b", time.Hour)

	tok, err := issuer.IssueAccess("user-1", "a@example.com", nil, nil)
	if err != nil {
		t.Fatalf("issue failed: %v", err)
	}
	if _, err := verifier.Verify(tok); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestVerifyExpiredToken(t *testing.T) {
	m, _ := NewTokenManager("super-secret", time.Hour)
	base := time.Now()
	m.now = func() time.Time { return base }

	tok, err := m.IssueAccess("user-1", "a@example.com", nil, nil)
	if err != nil {
		t.Fatalf("issue failed: %v", err)
	}

	m.now = func() time.Time { return base.Add(2 * time.Hour) }
	if _, err := m.Verify(tok); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken for expired token, got %v", err)
	}
}

func TestVerifyRejectsNoneAlgorithm(t *testing.T) {
	m, _ := NewTokenManager("super-secret", time.Hour)

	none := jwt.NewWithClaims(jwt.SigningMethodNone, Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			Issuer:    tokenIssuer,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Kind: TokenKindAccess,
	})
	s, err := none.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("failed to sign none token: %v", err)
	}
	if _, err := m.Verify(s); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken for none algorithm, got %v", err)
	}
}

func TestVerifyGarbageToken(t *testing.T) {
	m, _ := NewTokenManager("super-secret", time.Hour)
	if _, err := m.Verify("not.a.jwt"); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken for garbage token, got %v", err)
	}
}
