package hooks_test

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/derpixler/skolva/internal/core/hooks"
)

type testPlugin struct {
	name        string
	registered  bool
	activated   bool
	deactivated bool
}

func (p *testPlugin) Name() string        { return p.name }
func (p *testPlugin) Version() string      { return "1.0.0" }
func (p *testPlugin) Description() string  { return "test plugin" }
func (p *testPlugin) Register(hm *hooks.HookManager) error {
	p.registered = true
	return nil
}
func (p *testPlugin) Activate(db *pgxpool.Pool) error {
	p.activated = true
	return nil
}
func (p *testPlugin) Deactivate() error {
	p.deactivated = true
	return nil
}

func TestPluginRegistryRegisterAll(t *testing.T) {
	registry := hooks.NewPluginRegistry()
	plugin := &testPlugin{name: "test"}

	registry.Add(plugin)

	hm := hooks.NewHookManager()
	err := registry.RegisterAll(hm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !plugin.registered {
		t.Error("expected plugin to be registered")
	}
}

func TestPluginRegistryActivateAll(t *testing.T) {
	registry := hooks.NewPluginRegistry()
	plugin := &testPlugin{name: "test"}

	registry.Add(plugin)

	err := registry.ActivateAll(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !plugin.activated {
		t.Error("expected plugin to be activated")
	}
}

func TestPluginRegistryDeactivateAll(t *testing.T) {
	registry := hooks.NewPluginRegistry()
	plugin := &testPlugin{name: "test"}

	registry.Add(plugin)

	err := registry.DeactivateAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !plugin.deactivated {
		t.Error("expected plugin to be deactivated")
	}
}

func TestPluginRegistryAll(t *testing.T) {
	registry := hooks.NewPluginRegistry()
	plugin := &testPlugin{name: "test"}

	registry.Add(plugin)

	if len(registry.All()) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(registry.All()))
	}
	if registry.All()[0].Name() != "test" {
		t.Errorf("expected 'test', got '%s'", registry.All()[0].Name())
	}
}

func TestPluginLifecycle(t *testing.T) {
	registry := hooks.NewPluginRegistry()
	plugin := &testPlugin{name: "lifecycle-test"}

	registry.Add(plugin)

	hm := hooks.NewHookManager()

	if err := registry.RegisterAll(hm); err != nil {
		t.Fatalf("register: %v", err)
	}
	if !plugin.registered {
		t.Error("expected registered")
	}

	if err := registry.ActivateAll(nil); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if !plugin.activated {
		t.Error("expected activated")
	}

	if err := registry.DeactivateAll(); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if !plugin.deactivated {
		t.Error("expected deactivated")
	}
}
