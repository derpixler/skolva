package app

import (
	"github.com/derpixler/skolva/internal/auth"
	"github.com/derpixler/skolva/internal/core/module"
	"github.com/derpixler/skolva/internal/crm"
	"github.com/derpixler/skolva/internal/groups"
)

// DefaultRegistry returns the built-in module assembly for the Skolva product —
// identity (auth), groups and crm, in mount order. It is the single source of
// truth for which modules a build includes. The token manager is injected
// because identity needs it before the module.Deps bundle is available.
func DefaultRegistry(tm *auth.TokenManager) *module.Registry {
	return module.NewRegistry(
		auth.NewModule(tm),
		groups.Module(),
		crm.Module(),
	)
}
