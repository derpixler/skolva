package middleware

import (
	"github.com/gin-gonic/gin"
)

func ActorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if actor, exists := c.Get("actor"); exists {
			_ = actor
		}
		c.Next()
	}
}
