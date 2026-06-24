package middleware

import "github.com/gin-gonic/gin"

// ActorMiddleware is a noop placeholder that reads the actor from the
// Gin context. In future phases this will initialise request-scoped
// logging fields or similar per-request configuration.
func ActorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if actor, exists := c.Get("actor"); exists {
			_ = actor
		}
		c.Next()
	}
}
