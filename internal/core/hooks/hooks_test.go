package hooks_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/derpixler/skolva/internal/core/hooks"
)

func TestHookManagerAddAction(t *testing.T) {
	hm := hooks.NewHookManager()

	called := false
	hm.AddAction("test.hook", 10, "test-plugin", func(ctx context.Context, hc hooks.HookContext) error {
		called = true
		return nil
	})

	err := hm.DoActions(context.Background(), "test.hook", hooks.HookContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected action to be called")
	}
}

func TestHookManagerActionPriority(t *testing.T) {
	hm := hooks.NewHookManager()

	var order []int

	hm.AddAction("test.hook", 20, "plugin-a", func(ctx context.Context, hc hooks.HookContext) error {
		order = append(order, 20)
		return nil
	})
	hm.AddAction("test.hook", 10, "plugin-b", func(ctx context.Context, hc hooks.HookContext) error {
		order = append(order, 10)
		return nil
	})
	hm.AddAction("test.hook", 30, "plugin-c", func(ctx context.Context, hc hooks.HookContext) error {
		order = append(order, 30)
		return nil
	})

	err := hm.DoActions(context.Background(), "test.hook", hooks.HookContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(order) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(order))
	}
	if order[0] != 10 || order[1] != 20 || order[2] != 30 {
		t.Errorf("expected priority order [10, 20, 30], got %v", order)
	}
}

func TestHookManagerMultipleHandlers(t *testing.T) {
	hm := hooks.NewHookManager()

	count := 0
	for i := 0; i < 5; i++ {
		hm.AddAction("test.hook", i, "plugin", func(ctx context.Context, hc hooks.HookContext) error {
			count++
			return nil
		})
	}

	err := hm.DoActions(context.Background(), "test.hook", hooks.HookContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 5 {
		t.Errorf("expected 5 calls, got %d", count)
	}
}

func TestHookManagerFilter(t *testing.T) {
	hm := hooks.NewHookManager()

	hm.AddFilter("test.filter", 10, "plugin", func(ctx context.Context, hc hooks.HookContext) (hooks.HookContext, error) {
		hc["transformed"] = true
		return hc, nil
	})

	result, err := hm.ApplyFilters(context.Background(), "test.filter", hooks.HookContext{"original": true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["transformed"] != true {
		t.Error("expected transformed=true")
	}
	if result["original"] != true {
		t.Error("expected original=true")
	}
}

func TestHookManagerFilterChain(t *testing.T) {
	hm := hooks.NewHookManager()

	hm.AddFilter("test.chain", 10, "p1", func(ctx context.Context, hc hooks.HookContext) (hooks.HookContext, error) {
		hc["step1"] = true
		return hc, nil
	})
	hm.AddFilter("test.chain", 20, "p2", func(ctx context.Context, hc hooks.HookContext) (hooks.HookContext, error) {
		hc["step2"] = true
		return hc, nil
	})

	result, err := hm.ApplyFilters(context.Background(), "test.chain", hooks.HookContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["step1"] != true || result["step2"] != true {
		t.Error("expected both filter steps to be applied")
	}
}

func TestHookManagerRemovePlugin(t *testing.T) {
	hm := hooks.NewHookManager()

	called := false
	hm.AddAction("test.hook", 10, "plugin-a", func(ctx context.Context, hc hooks.HookContext) error {
		called = true
		return nil
	})

	hm.RemovePlugin("plugin-a")

	err := hm.DoActions(context.Background(), "test.hook", hooks.HookContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("expected action to be removed")
	}
}

func TestHookManagerNoHandler(t *testing.T) {
	hm := hooks.NewHookManager()

	err := hm.DoActions(context.Background(), "nonexistent.hook", hooks.HookContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHookManagerNoFilter(t *testing.T) {
	hm := hooks.NewHookManager()

	hc, err := hm.ApplyFilters(context.Background(), "nonexistent.filter", hooks.HookContext{"key": "value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hc["key"] != "value" {
		t.Error("expected unchanged hook context")
	}
}

func TestHookManagerActionError(t *testing.T) {
	hm := hooks.NewHookManager()

	hm.AddAction("test.hook", 10, "plugin", func(ctx context.Context, hc hooks.HookContext) error {
		return fmt.Errorf("action failed")
	})

	err := hm.DoActions(context.Background(), "test.hook", hooks.HookContext{})
	if err == nil {
		t.Error("expected error")
	}
}

func TestHookManagerFilterError(t *testing.T) {
	hm := hooks.NewHookManager()

	hm.AddFilter("test.filter", 10, "plugin", func(ctx context.Context, hc hooks.HookContext) (hooks.HookContext, error) {
		return nil, fmt.Errorf("filter failed")
	})

	_, err := hm.ApplyFilters(context.Background(), "test.filter", hooks.HookContext{})
	if err == nil {
		t.Error("expected error")
	}
}

func TestHookManagerRemovePluginFilters(t *testing.T) {
	hm := hooks.NewHookManager()

	transformed := false
	hm.AddFilter("test.hook", 10, "plugin-a", func(ctx context.Context, hc hooks.HookContext) (hooks.HookContext, error) {
		hc["transformed"] = true
		return hc, nil
	})

	hm.RemovePlugin("plugin-a")

	hc, err := hm.ApplyFilters(context.Background(), "test.hook", hooks.HookContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if transformed {
		t.Error("expected filter to be removed")
	}
	if hc["transformed"] != nil {
		t.Error("expected no transformation")
	}
}
