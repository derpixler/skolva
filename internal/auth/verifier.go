package auth

import (
	"errors"

	"github.com/derpixler/skolva/internal/core/middleware"
)

// NewVerifier returns a middleware.Verifier that validates access tokens via
// the TokenManager and maps the claims onto an Actor. It bridges the auth and
// middleware packages without creating an import cycle (middleware never
// imports auth).
func NewVerifier(tm *TokenManager) middleware.Verifier {
	return func(token string) (*middleware.Actor, error) {
		claims, err := tm.Verify(token)
		if err != nil {
			return nil, err
		}
		if claims.Kind != TokenKindAccess {
			return nil, errors.New("not an access token")
		}
		return &middleware.Actor{
			UserID:      claims.Subject,
			Email:       claims.Email,
			Roles:       claims.Roles,
			Permissions: claims.Permissions,
		}, nil
	}
}
