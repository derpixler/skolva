package auth

import (
	"github.com/derpixler/skolva-core/middleware"
	"github.com/gin-gonic/gin"
)

// Provider is the identity/login seam: it mounts the login/credential
// endpoints that establish a user session. The default LocalProvider uses
// local password authentication (plus register, TOTP/email 2FA and password
// reset); a future provider (e.g. OIDC/Keycloak) would instead mount a
// redirect/callback flow.
//
// Two adjacent seams stay separate and provider-agnostic:
//   - per-request token verification — middleware.Verifier (auth.NewVerifier);
//   - authorization (RBAC) — middleware.RequirePermission.
//
// User-management endpoints (user CRUD, roles, search) are independent of the
// provider and are mounted by registerManagementRoutes.
type Provider interface {
	Name() string
	RegisterRoutes(rg *gin.RouterGroup, h *Handler)
}

// LocalProvider is the default identity provider: local password auth.
type LocalProvider struct{}

// Name identifies the provider (matches the AUTH_PROVIDER value).
func (LocalProvider) Name() string { return "local" }

// RegisterRoutes mounts the local login/credential endpoints.
func (LocalProvider) RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.POST("/auth/login", h.Login)
	rg.POST("/auth/register", middleware.RequirePermission("users.write"), h.Register)

	rg.POST("/auth/2fa/setup", middleware.RequireAuth(), h.Setup2FA)
	rg.POST("/auth/2fa/confirm", middleware.RequireAuth(), h.Confirm2FA)
	rg.POST("/auth/2fa/verify", h.Verify2FA)
	rg.POST("/auth/2fa/recovery", h.Recover2FA)
	rg.POST("/auth/2fa/disable", middleware.RequireAuth(), h.Disable2FA)

	rg.POST("/auth/2fa/email/setup", middleware.RequireAuth(), h.SetupEmail2FA)
	rg.POST("/auth/2fa/email/confirm", middleware.RequireAuth(), h.ConfirmEmail2FA)
	rg.POST("/auth/2fa/email/verify", h.VerifyEmail2FA)
	rg.POST("/auth/2fa/email/resend", h.ResendEmail2FA)
	rg.POST("/auth/2fa/email/disable", middleware.RequireAuth(), h.DisableEmail2FA)

	rg.POST("/auth/password/forgot", h.ForgotPassword)
	rg.POST("/auth/password/reset", h.ResetPassword)
}

// ProviderFor returns the identity provider for the configured name (e.g. the
// AUTH_PROVIDER env value). Unknown names fall back to the local provider.
// Selecting a non-local provider is wired through the composition root once a
// concrete adapter (e.g. OIDC) exists.
func ProviderFor(name string) Provider {
	// Future adapter: case "oidc" → OIDC provider, wired through AUTH_PROVIDER.
	return LocalProvider{}
}
