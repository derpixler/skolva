// Package module defines the Skolva module SDK: the contract every feature
// module implements and a registry the product assembly uses to wire modules
// (routes, hooks, permissions, migrations, lifecycle) uniformly.
//
// A module owns its routes, hooks, permissions, migrations and OpenAPI
// fragment, and must never import another module — cross-module interaction
// goes through the hook/event bus carried in Deps. This is the in-place seam;
// on extraction it becomes github.com/derpixler/skolva-core/module.
package module

import (
	"context"
	"log/slog"

	"github.com/derpixler/skolva/internal/core/cache"
	"github.com/derpixler/skolva/internal/core/events"
	"github.com/derpixler/skolva/internal/core/hooks"
	"github.com/derpixler/skolva/internal/core/mail"
	"github.com/derpixler/skolva/internal/core/search"
	"github.com/derpixler/skolva/internal/core/secrets"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Permission is an RBAC permission a module contributes to the catalog.
type Permission struct {
	Slug        string
	Description string
}

// Migration is one ordered, versioned schema change owned by a module.
type Migration struct {
	Version int64
	Name    string
	SQL     string
}

// Deps is the typed dependency bundle handed to modules at activation and when
// mounting routes. It replaces passing a bare *pgxpool.Pool so modules depend
// on capabilities rather than concrete wiring. Swappable infra seams
// (Events, Cache, Search, Jobs) and auth services are added as they land.
type Deps struct {
	DB     *pgxpool.Pool
	Hooks  *hooks.HookManager
	Cipher *secrets.Cipher
	Mailer mail.Mailer
	Logger *slog.Logger

	// Swappable infra seams (defaults: in-proc events, in-memory cache,
	// Postgres-FTS search). A JobQueue seam is deferred until a module
	// actually enqueues background work.
	Events events.Bus
	Cache  cache.Cache
	Search search.Service
}

// Module is the contract every feature module implements.
type Module interface {
	Name() string
	Version() string

	// Permissions returns the RBAC permissions this module contributes.
	Permissions() []Permission
	// Migrations returns this module's ordered, versioned schema changes.
	Migrations() []Migration

	// RegisterHooks wires action/filter handlers onto the shared bus.
	RegisterHooks(hm *hooks.HookManager) error
	// RegisterRoutes mounts the module's HTTP routes onto the API group.
	RegisterRoutes(api *gin.RouterGroup, d Deps)
	// OpenAPISpec returns this module's OpenAPI fragment (nil if none).
	OpenAPISpec() []byte

	// Activate is called once at startup (after migrations); Deactivate on
	// shutdown.
	Activate(ctx context.Context, d Deps) error
	Deactivate(ctx context.Context) error
}
