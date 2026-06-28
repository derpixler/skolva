package crm

import (
	"context"

	"github.com/derpixler/skolva/internal/core/hooks"
	"github.com/derpixler/skolva/internal/core/module"
	"github.com/gin-gonic/gin"
)

type mod struct{}

// Module returns the CRM feature as a module.Module.
func Module() module.Module { return &mod{} }

func (m *mod) Name() string    { return "crm" }
func (m *mod) Version() string { return "0.1.0" }

// Permissions and Migrations are still centralized in schema.sql; they move
// here in the per-module migration phase (1d).
func (m *mod) Permissions() []module.Permission { return nil }
func (m *mod) Migrations() []module.Migration   { return nil }

func (m *mod) RegisterHooks(*hooks.HookManager) error { return nil }

func (m *mod) RegisterRoutes(api *gin.RouterGroup, d module.Deps) {
	RegisterRoutes(api, d.DB)
}

func (m *mod) OpenAPISpec() []byte { return nil }

func (m *mod) Activate(context.Context, module.Deps) error { return nil }
func (m *mod) Deactivate(context.Context) error            { return nil }
