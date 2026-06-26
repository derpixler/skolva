// Package middleware provides HTTP middleware for the Gin router.
//
// Included middleware:
//
//	CORS         — permissive cross-origin headers for development.
//	RequestID    — propagates or generates an X-Request-ID header.
//	AuthSkeleton — placeholder JWT parser (hardcoded "test-token").
//	Actor        — reads the actor from the Gin context.
package middleware

import (
	"github.com/gin-gonic/gin"
)

// CORS sets permissive cross-origin headers. In production this should be
// replaced with a stricter configuration that whitelists specific origins.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Request-ID")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Disposition")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
