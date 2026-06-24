package middleware

import (
	"strings"

	apperrors "github.com/derpixler/skolva/internal/core/errors"
	"github.com/gin-gonic/gin"
)

// Actor represents the currently authenticated user.
type Actor struct {
	UserID string   // UUID of the user
	Email  string   // login email
	Roles  []string // assigned role slugs
}

const actorKey = "actor"

// AuthSkeleton is a placeholder JWT middleware. During Phase 1 it accepts
// only the literal token "test-token", which injects a hardcoded admin
// actor. Full JWT validation will be implemented in Phase 2.
func AuthSkeleton() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.Next()
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")

		if token == "test-token" {
			actor := &Actor{
				UserID: "00000000-0000-0000-0000-000000000001",
				Email:  "test@example.com",
				Roles:  []string{"admin"},
			}
			c.Set(actorKey, actor)
		}

		c.Next()
	}
}

// RequireAuth aborts with 401 if no actor is present in the context.
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor := GetActor(c)
		if actor == nil {
			c.AbortWithStatusJSON(401, apperrors.NewUnauthorized("authentication required"))
			return
		}
		c.Next()
	}
}

// RequirePermission aborts with 403 if the actor does not have admin role.
// TODO: replace admin-role bypass with actual permission check in Phase 2.
func RequirePermission(permission string) gin.HandlerFunc {
	_ = permission // TODO: use in Phase 2 when RBAC is implemented
	return func(c *gin.Context) {
		actor := GetActor(c)
		if actor == nil {
			c.AbortWithStatusJSON(401, apperrors.NewUnauthorized("authentication required"))
			return
		}

		for _, role := range actor.Roles {
			if role == "admin" {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(403, apperrors.NewForbidden("insufficient permissions"))
	}
}

// SetActor stores an Actor in the Gin context.
func SetActor(c *gin.Context, actor *Actor) { c.Set(actorKey, actor) }

// GetActor retrieves the Actor from the Gin context, or nil if absent.
func GetActor(c *gin.Context) *Actor {
	if actor, exists := c.Get(actorKey); exists {
		if a, ok := actor.(*Actor); ok {
			return a
		}
	}
	return nil
}
