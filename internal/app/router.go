// Package app wires the Gin HTTP router, middleware stack, and health endpoint.
package app

import (
	"github.com/derpixler/skolva/internal/core/database"
	"github.com/derpixler/skolva/internal/core/hooks"
	"github.com/derpixler/skolva/internal/core/jobs"
	"github.com/derpixler/skolva/internal/core/middleware"
	"github.com/gin-gonic/gin"
)

// NewRouter returns a Gin engine with the standard middleware stack and
// a single /api/health endpoint. The health endpoint pings both database
// pools and returns 200 {"status":"healthy"} or 503 on failure.
func NewRouter(pools *database.Pools, hm *hooks.HookManager, worker *jobs.Worker) *gin.Engine {
	router := gin.New()

	router.Use(gin.Recovery())
	router.Use(middleware.RequestID())
	router.Use(middleware.CORS())
	router.Use(middleware.AuthSkeleton())
	router.Use(middleware.ActorMiddleware())

	api := router.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) {
			if err := pools.Health(c.Request.Context()); err != nil {
				c.JSON(503, gin.H{"status": "unhealthy", "error": err.Error()})
				return
			}
			c.JSON(200, gin.H{"status": "healthy"})
		})
	}

	return router
}
