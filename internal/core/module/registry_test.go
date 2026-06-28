package module_test

import (
	"context"
	"testing"

	"github.com/derpixler/skolva/internal/core/hooks"
	"github.com/derpixler/skolva/internal/core/module"
	"github.com/gin-gonic/gin"
)

// fakeModule is a minimal Module implementation for exercising the Registry.
type fakeModule struct {
	name        string
	perms       []module.Permission
	migs        []module.Migration
	hooked      bool
	routed      bool
	activated   bool
	deactivated bool
}

func (f *fakeModule) Name() string                                 { return f.name }
func (f *fakeModule) Version() string                              { return "0.0.0" }
func (f *fakeModule) Permissions() []module.Permission             { return f.perms }
func (f *fakeModule) Migrations() []module.Migration               { return f.migs }
func (f *fakeModule) RegisterHooks(*hooks.HookManager) error       { f.hooked = true; return nil }
func (f *fakeModule) RegisterRoutes(*gin.RouterGroup, module.Deps) { f.routed = true }
func (f *fakeModule) OpenAPISpec() []byte                          { return nil }
func (f *fakeModule) Activate(context.Context, module.Deps) error  { f.activated = true; return nil }
func (f *fakeModule) Deactivate(context.Context) error             { f.deactivated = true; return nil }

func TestRegistryAggregatesAndDrivesLifecycle(t *testing.T) {
	a := &fakeModule{name: "a", perms: []module.Permission{{Slug: "a.read"}}, migs: []module.Migration{{Version: 1, Name: "a1"}}}
	b := &fakeModule{name: "b", perms: []module.Permission{{Slug: "b.read"}, {Slug: "b.write"}}, migs: []module.Migration{{Version: 1, Name: "b1"}}}

	r := module.NewRegistry(a)
	r.Add(b)

	if got := len(r.Modules()); got != 2 {
		t.Fatalf("modules: want 2, got %d", got)
	}
	if got := len(r.Permissions()); got != 3 {
		t.Errorf("permissions: want 3 aggregated, got %d", got)
	}
	if got := len(r.Migrations()); got != 2 {
		t.Errorf("migrations: want 2 aggregated, got %d", got)
	}

	hm := hooks.NewHookManager()
	if err := r.RegisterHooks(hm); err != nil {
		t.Fatalf("register hooks: %v", err)
	}
	if !a.hooked || !b.hooked {
		t.Error("hooks not registered on all modules")
	}

	gin.SetMode(gin.TestMode)
	api := gin.New().Group("/api")
	r.MountRoutes(api, module.Deps{})
	if !a.routed || !b.routed {
		t.Error("routes not mounted on all modules")
	}

	if err := r.ActivateAll(context.Background(), module.Deps{}); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if !a.activated || !b.activated {
		t.Error("not all modules activated")
	}

	if err := r.DeactivateAll(context.Background()); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if !a.deactivated || !b.deactivated {
		t.Error("not all modules deactivated")
	}
}
