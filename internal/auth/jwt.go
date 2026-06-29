package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	TokenKindAccess     = "access"
	TokenKindPending2FA = "2fa_pending"

	tokenIssuer = "skolva"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrEmptySecret  = errors.New("jwt secret must not be empty")
	ErrEmptySubject = errors.New("token subject must not be empty")
)

type Claims struct {
	jwt.RegisteredClaims
	Email       string   `json:"email,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	Kind        string   `json:"kind"`
}

type TokenManager struct {
	secret    []byte
	accessTTL time.Duration
	now       func() time.Time
}

func NewTokenManager(secret string, accessTTL time.Duration) (*TokenManager, error) {
	if secret == "" {
		return nil, ErrEmptySecret
	}
	return &TokenManager{
		secret:    []byte(secret),
		accessTTL: accessTTL,
		now:       time.Now,
	}, nil
}

func (m *TokenManager) IssueAccess(userID, email string, roles, permissions []string) (string, error) {
	return m.issue(userID, email, roles, permissions, TokenKindAccess, m.accessTTL)
}

func (m *TokenManager) IssuePending2FA(userID string, ttl time.Duration) (string, error) {
	return m.issue(userID, "", nil, nil, TokenKindPending2FA, ttl)
}

func (m *TokenManager) issue(userID, email string, roles, permissions []string, kind string, ttl time.Duration) (string, error) {
	if userID == "" {
		return "", ErrEmptySubject
	}
	now := m.now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    tokenIssuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			ID:        uuid.NewString(),
		},
		Email:       email,
		Roles:       roles,
		Permissions: permissions,
		Kind:        kind,
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

func (m *TokenManager) Verify(tokenString string) (*Claims, error) {
	claims := &Claims{}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuer(tokenIssuer),
		jwt.WithTimeFunc(m.now),
	)
	token, err := parser.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: unexpected signing method", ErrInvalidToken)
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	if !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
