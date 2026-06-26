// Package app wires the Gin HTTP router, middleware stack, and health endpoint.
package app

import (
	apispec "github.com/derpixler/skolva/api"
	"github.com/derpixler/skolva/internal/auth"
	"github.com/derpixler/skolva/internal/core/database"
	"github.com/derpixler/skolva/internal/core/hooks"
	"github.com/derpixler/skolva/internal/core/jobs"
	"github.com/derpixler/skolva/internal/core/mail"
	"github.com/derpixler/skolva/internal/core/middleware"
	"github.com/derpixler/skolva/internal/core/secrets"
	"github.com/derpixler/skolva/internal/crm"
	"github.com/derpixler/skolva/internal/groups"
	"github.com/gin-gonic/gin"
)

// NewRouter returns a Gin engine with the standard middleware stack, health
// endpoint, module routes (auth, crm, groups), OpenAPI spec serve, and API docs.
func NewRouter(pools *database.Pools, hm *hooks.HookManager, worker *jobs.Worker, verify middleware.Verifier, tm *auth.TokenManager, cipher *secrets.Cipher, mailer mail.Mailer) *gin.Engine {
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

		auth.RegisterRoutes(api, pools.Web, tm, cipher, mailer)
		groups.RegisterRoutes(api, pools.Web)
		crm.RegisterRoutes(api, pools.Web)

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
