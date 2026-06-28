// Package app wires the Gin HTTP router, middleware stack, and health endpoint.
package app

import (
	"github.com/derpixler/skolva-core/database"
	"github.com/derpixler/skolva-core/middleware"
	"github.com/derpixler/skolva-core/module"
	apispec "github.com/derpixler/skolva/api"
	"github.com/gin-gonic/gin"
)

// NewRouter returns a Gin engine with the standard middleware stack, a health
// endpoint, the modules' routes (mounted from the registry), the OpenAPI spec
// and the API docs. The module assembly and its dependency bundle are built by
// the composition root (cmd/api) and passed in.
func NewRouter(pools *database.Pools, registry *module.Registry, deps module.Deps, verify middleware.Verifier) *gin.Engine {
	router := gin.New()

	router.Use(gin.Recovery())
	router.Use(middleware.RequestID())
	router.Use(middleware.CORS())
	router.Use(middleware.Authenticate(verify))
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

		registry.MountRoutes(api, deps)

		api.GET("/openapi.yaml", func(c *gin.Context) {
			c.Data(200, "application/yaml", apispec.Spec)
		})
		api.GET("/docs", func(c *gin.Context) {
			c.Data(200, "text/html; charset=utf-8", apispec.RedocHTML)
		})
		api.GET("/docs/redoc.js", func(c *gin.Context) {
			c.Data(200, "application/javascript", apispec.RedocJS)
		})
	}

	return router
}
