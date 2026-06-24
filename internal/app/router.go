package app

import (
	"github.com/gin-gonic/gin"
	"github.com/derpixler/skolva/internal/core/database"
	"github.com/derpixler/skolva/internal/core/hooks"
	"github.com/derpixler/skolva/internal/core/jobs"
	"github.com/derpixler/skolva/internal/core/middleware"
)

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
