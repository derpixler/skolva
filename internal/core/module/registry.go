package module

import (
	"context"
	"fmt"

	"github.com/derpixler/skolva/internal/core/hooks"
	"github.com/gin-gonic/gin"
)

// Registry collects modules and drives their wiring in registration order.
// It is the single place the product assembly uses to compose a build.
type Registry struct {
	modules []Module
}

// NewRegistry returns a registry seeded with the given modules.
func NewRegistry(modules ...Module) *Registry {
	return &Registry{modules: append([]Module(nil), modules...)}
}

// Add appends modules to the registry (registration order is preserved).
func (r *Registry) Add(modules ...Module) {
	r.modules = append(r.modules, modules...)
}

// Modules returns a copy of the registered modules.
func (r *Registry) Modules() []Module {
	out := make([]Module, len(r.modules))
	copy(out, r.modules)
	return out
}

// Permissions aggregates every module's permission contributions.
func (r *Registry) Permissions() []Permission {
	var out []Permission
	for _, m := range r.modules {
		out = append(out, m.Permissions()...)
	}
	return out
}

// Migrations aggregates module migrations in registration order; each module's
// own slice is kept in its given order.
func (r *Registry) Migrations() []Migration {
	var out []Migration
	for _, m := range r.modules {
		out = append(out, m.Migrations()...)
	}
	return out
}

// RegisterHooks wires every module's hook handlers.
func (r *Registry) RegisterHooks(hm *hooks.HookManager) error {
	for _, m := range r.modules {
		if err := m.RegisterHooks(hm); err != nil {
			return fmt.Errorf("module %s: register hooks: %w", m.Name(), err)
		}
	}
	return nil
}

// MountRoutes mounts every module's routes onto the API group.
func (r *Registry) MountRoutes(api *gin.RouterGroup, d Deps) {
	for _, m := range r.modules {
		m.RegisterRoutes(api, d)
	}
}

// ActivateAll activates modules in registration order. On the first failure it
// deactivates the already-activated modules in reverse order and returns.
func (r *Registry) ActivateAll(ctx context.Context, d Deps) error {
	for i, m := range r.modules {
		if err := m.Activate(ctx, d); err != nil {
			for j := i - 1; j >= 0; j-- {
				_ = r.modules[j].Deactivate(ctx)
			}
			return fmt.Errorf("module %s: activate: %w", m.Name(), err)
		}
	}
	return nil
}

// DeactivateAll deactivates modules in reverse registration order, returning
// the first error encountered (after attempting all).
func (r *Registry) DeactivateAll(ctx context.Context) error {
	var firstErr error
	for i := len(r.modules) - 1; i >= 0; i-- {
		if err := r.modules[i].Deactivate(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// OpenAPIFragments returns each module's non-empty OpenAPI fragment, in
// registration order, for composition into the product spec.
func (r *Registry) OpenAPIFragments() [][]byte {
	var out [][]byte
	for _, m := range r.modules {
		if frag := m.OpenAPISpec(); len(frag) > 0 {
			out = append(out, frag)
		}
	}
	return out
}
