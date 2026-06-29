package middleware

import (
	"strings"

	apperrors "github.com/derpixler/skolva/internal/core/errors"
	"github.com/gin-gonic/gin"
)

const actorKey = "actor"

// Actor is the authenticated principal for a request.
type Actor struct {
	UserID      string
	Email       string
	Roles       []string
	Permissions []string
}

// HasRole reports whether the actor has the given role.
func (a *Actor) HasRole(role string) bool {
	for _, r := range a.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasPermission reports whether the actor holds the given permission.
// The "admin" role is treated as a wildcard.
func (a *Actor) HasPermission(permission string) bool {
	if a.HasRole("admin") {
		return true
	}
	for _, p := range a.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

// Verifier turns a bearer token into an Actor, or returns an error when the
// token is invalid. It decouples the middleware from the auth module (no
// import cycle): the concrete verifier is injected at wiring time.
type Verifier func(token string) (*Actor, error)

// Authenticate verifies a Bearer token (when present) and stores the Actor.
//   - No Authorization header: the request continues unauthenticated; route
//     guards (RequireAuth/RequirePermission) decide access.
//   - Header present but invalid: 401.
func Authenticate(verify Verifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.Next()
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		actor, err := verify(token)
		if err != nil || actor == nil {
			c.AbortWithStatusJSON(401, apperrors.NewUnauthorized("invalid or expired token"))
			return
		}

		SetActor(c, actor)
		c.Next()
	}
}

// RequireAuth aborts with 401 when no authenticated actor is present.
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if GetActor(c) == nil {
			c.AbortWithStatusJSON(401, apperrors.NewUnauthorized("authentication required"))
			return
		}
		c.Next()
	}
}

// RequirePermission aborts with 403 when the actor lacks the given permission.
func RequirePermission(permission string) gin.HandlerFunc {
	_ = permission // TODO: use in Phase 2 when RBAC is implemented
	return func(c *gin.Context) {
		actor := GetActor(c)
		if actor == nil {
			c.AbortWithStatusJSON(401, apperrors.NewUnauthorized("authentication required"))
			return
		}
		if !actor.HasPermission(permission) {
			c.AbortWithStatusJSON(403, apperrors.NewForbidden("insufficient permissions"))
			return
		}
		c.Next()
	}
}

// SetActor stores the actor on the gin context.
func SetActor(c *gin.Context, actor *Actor) {
	c.Set(actorKey, actor)
}

// GetActor returns the actor stored on the gin context, or nil.
func GetActor(c *gin.Context) *Actor {
	if v, exists := c.Get(actorKey); exists {
		if a, ok := v.(*Actor); ok {
			return a
		}
	}
	return nil
}
