package auth

import (
	"context"

	"github.com/derpixler/skolva/internal/core/hooks"
	"github.com/derpixler/skolva/internal/core/module"
	"github.com/gin-gonic/gin"
)

type identityModule struct {
	tm *TokenManager
}

// NewModule returns the identity (auth) feature as a module.Module. The token
// manager is injected by the product assembly; the rest of the dependencies
// (DB, cipher, mailer) arrive via module.Deps.
func NewModule(tm *TokenManager) module.Module { return &identityModule{tm: tm} }

func (m *identityModule) Name() string    { return "identity" }
func (m *identityModule) Version() string { return "0.1.0" }

// Permissions and Migrations are still centralized in schema.sql; they move
// here in the per-module migration phase (1d). The identity provider seam
// (re-scoped #135) lands as a later checkpoint.
func (m *identityModule) Permissions() []module.Permission { return nil }
func (m *identityModule) Migrations() []module.Migration   { return nil }

func (m *identityModule) RegisterHooks(*hooks.HookManager) error { return nil }

func (m *identityModule) RegisterRoutes(api *gin.RouterGroup, d module.Deps) {
	RegisterRoutes(api, d.DB, m.tm, d.Cipher, d.Mailer)
}

func (m *identityModule) OpenAPISpec() []byte { return nil }

func (m *identityModule) Activate(context.Context, module.Deps) error { return nil }
func (m *identityModule) Deactivate(context.Context) error            { return nil }
