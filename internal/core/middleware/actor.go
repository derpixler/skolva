package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
)

type actorCtxKey struct{}

// ActorMiddleware copies the authenticated actor into the request's
// context.Context, so non-gin code (e.g. repositories using
// dbexec.WithActor) can read it via ActorFromContext.
func ActorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if actor := GetActor(c); actor != nil {
			ctx := context.WithValue(c.Request.Context(), actorCtxKey{}, actor)
			c.Request = c.Request.WithContext(ctx)
		}
		c.Next()
	}
}

// ActorFromContext returns the actor stored in ctx, or nil.
func ActorFromContext(ctx context.Context) *Actor {
	if a, ok := ctx.Value(actorCtxKey{}).(*Actor); ok {
		return a
	}
	return nil
}
