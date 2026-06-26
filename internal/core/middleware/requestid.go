package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const requestIDHeader = "X-Request-ID"

// RequestID reads the X-Request-ID header from the incoming request.
// If absent, a new UUID is generated. The value is stored in the Gin
// context under the key "request_id" and set on the response header.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(requestIDHeader)
		if id == "" {
			id = uuid.New().String()
		}

		c.Set("request_id", id)
		c.Header(requestIDHeader, id)
		c.Next()
	}
}
