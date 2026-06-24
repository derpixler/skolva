package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	apperrors "github.com/derpixler/skolva/internal/core/errors"
)

type Actor struct {
	UserID string
	Email  string
	Roles  []string
}

const actorKey = "actor"

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

func RequirePermission(permission string) gin.HandlerFunc {
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

func SetActor(c *gin.Context, actor *Actor) {
	c.Set(actorKey, actor)
}

func GetActor(c *gin.Context) *Actor {
	if actor, exists := c.Get(actorKey); exists {
		if a, ok := actor.(*Actor); ok {
			return a
		}
	}
	return nil
}
