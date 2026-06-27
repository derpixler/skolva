package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestNewTokenManagerEmptySecret(t *testing.T) {
	tstep(t, "NewTokenManager with empty secret -> ErrEmptySecret")
	_, err := NewTokenManager("", time.Hour)
	assertErrIs(t, err, ErrEmptySecret, "empty secret rejected")
}

func TestIssueAccessAndVerifyRoundTrip(t *testing.T) {
	tstep(t, "issue access token + verify round-trip")
	m, err := NewTokenManager("super-secret", time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	roles := []string{"admin", "mitglied"}
	perms := []string{"users.read", "users.write"}
	tlog(t, "[val ] input subject=user-1 email=a@example.com roles=%v perms=%v", roles, perms)

	tok, err := m.IssueAccess("user-1", "a@example.com", roles, perms)
	if err != nil {
		t.Fatalf("issue failed: %v", err)
	}
	tlog(t, "[val ] issued access token (%d chars)", len(tok))

	claims, err := m.Verify(tok)
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	tlog(t, "[val ] claims subject=%s email=%s kind=%s roles=%v perms=%v",
		claims.Subject, claims.Email, claims.Kind, claims.Roles, claims.Permissions)

	assertEq(t, claims.Subject, "user-1", "claim subject")
	assertEq(t, claims.Email, "a@example.com", "claim email")
	assertEq(t, claims.Kind, TokenKindAccess, "claim kind")
	assertSliceEq(t, claims.Roles, roles, "claim roles")
	assertSliceEq(t, claims.Permissions, perms, "claim permissions")
}

func TestIssuePending2FA(t *testing.T) {
	tstep(t, "issue pending-2FA token + verify")
	m, err := NewTokenManager("super-secret", time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tok, err := m.IssuePending2FA("user-2", 5*time.Minute)
	if err != nil {
		t.Fatalf("issue failed: %v", err)
	}
	tlog(t, "[val ] issued pending-2FA token (%d chars), ttl=5m", len(tok))

	claims, err := m.Verify(tok)
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	tlog(t, "[val ] claims subject=%s kind=%s", claims.Subject, claims.Kind)

	assertEq(t, claims.Subject, "user-2", "claim subject")
	assertEq(t, claims.Kind, TokenKindPending2FA, "claim kind")
}

func TestIssueEmptySubject(t *testing.T) {
	tstep(t, "IssueAccess with empty subject -> ErrEmptySubject")
	m, _ := NewTokenManager("super-secret", time.Hour)
	_, err := m.IssueAccess("", "a@example.com", nil, nil)
	assertErrIs(t, err, ErrEmptySubject, "empty subject rejected")
}

func TestVerifyWrongSecret(t *testing.T) {
	tstep(t, "verify token signed with a different secret -> ErrInvalidToken")
	issuer, _ := NewTokenManager("secret-a", time.Hour)
	verifier, _ := NewTokenManager("secret-b", time.Hour)

	tok, err := issuer.IssueAccess("user-1", "a@example.com", nil, nil)
	if err != nil {
		t.Fatalf("issue failed: %v", err)
	}
	_, err = verifier.Verify(tok)
	assertErrIs(t, err, ErrInvalidToken, "wrong secret rejected")
}

func TestVerifyExpiredToken(t *testing.T) {
	tstep(t, "verify an expired token -> ErrInvalidToken (clock advanced +2h)")
	m, _ := NewTokenManager("super-secret", time.Hour)
	base := time.Now()
	m.now = func() time.Time { return base }

	tok, err := m.IssueAccess("user-1", "a@example.com", nil, nil)
	if err != nil {
		t.Fatalf("issue failed: %v", err)
	}

	m.now = func() time.Time { return base.Add(2 * time.Hour) }
	_, err = m.Verify(tok)
	assertErrIs(t, err, ErrInvalidToken, "expired token rejected")
}

func TestVerifyRejectsNoneAlgorithm(t *testing.T) {
	tstep(t, "verify a token with alg=none -> ErrInvalidToken (algorithm-confusion attack)")
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
	_, err = m.Verify(s)
	assertErrIs(t, err, ErrInvalidToken, "alg=none rejected")
}

func TestVerifyGarbageToken(t *testing.T) {
	tstep(t, "verify a malformed token -> ErrInvalidToken")
	m, _ := NewTokenManager("super-secret", time.Hour)
	_, err := m.Verify("not.a.jwt")
	assertErrIs(t, err, ErrInvalidToken, "garbage token rejected")
}
