package hooks_test

import (
	"context"
	"testing"

	"github.com/derpixler/skolva/internal/core/hooks"
)

type testEntity struct {
	ID   string
	Name string
}

func TestCRUDHooksCreateLifecycle(t *testing.T) {
	hm := hooks.NewHookManager()
	crud := hooks.NewCRUDHooks[testEntity](hm, "test_entity")

	beforeCalled := false
	afterCalled := false

	crud.OnBeforeCreate(func(ctx context.Context, entity *testEntity) error {
		beforeCalled = true
		return nil
	})
	crud.OnAfterCreate(func(ctx context.Context, entity *testEntity) error {
		afterCalled = true
		return nil
	})

	entity := &testEntity{ID: "1", Name: "test"}
	err := crud.BeforeCreate(context.Background(), entity)
	if err != nil {
		t.Fatalf("BeforeCreate: unexpected error: %v", err)
	}
	if !beforeCalled {
		t.Error("expected beforeCreate to be called")
	}

	err = crud.AfterCreate(context.Background(), entity)
	if err != nil {
		t.Fatalf("AfterCreate: unexpected error: %v", err)
	}
	if !afterCalled {
		t.Error("expected afterCreate to be called")
	}
}

func TestCRUDHooksUpdateLifecycle(t *testing.T) {
	hm := hooks.NewHookManager()
	crud := hooks.NewCRUDHooks[testEntity](hm, "test_entity")

	beforeCalled := false
	afterCalled := false

	crud.OnBeforeUpdate(func(ctx context.Context, entity *testEntity) error {
		beforeCalled = true
		return nil
	})
	crud.OnAfterUpdate(func(ctx context.Context, entity *testEntity) error {
		afterCalled = true
		return nil
	})

	entity := &testEntity{ID: "1", Name: "updated"}
	err := crud.BeforeUpdate(context.Background(), entity)
	if err != nil {
		t.Fatalf("BeforeUpdate: unexpected error: %v", err)
	}
	if !beforeCalled {
		t.Error("expected beforeUpdate to be called")
	}

	err = crud.AfterUpdate(context.Background(), entity)
	if err != nil {
		t.Fatalf("AfterUpdate: unexpected error: %v", err)
	}
	if !afterCalled {
		t.Error("expected afterUpdate to be called")
	}
}

func TestCRUDHooksDeleteLifecycle(t *testing.T) {
	hm := hooks.NewHookManager()
	crud := hooks.NewCRUDHooks[testEntity](hm, "test_entity")

	beforeCalled := false
	afterCalled := false

	crud.OnBeforeDelete(func(ctx context.Context, id string) error {
		beforeCalled = true
		return nil
	})
	crud.OnAfterDelete(func(ctx context.Context, id string) error {
		afterCalled = true
		return nil
	})

	err := crud.BeforeDelete(context.Background(), "1")
	if err != nil {
		t.Fatalf("BeforeDelete: unexpected error: %v", err)
	}
	if !beforeCalled {
		t.Error("expected beforeDelete to be called")
	}

	err = crud.AfterDelete(context.Background(), "1")
	if err != nil {
		t.Fatalf("AfterDelete: unexpected error: %v", err)
	}
	if !afterCalled {
		t.Error("expected afterDelete to be called")
	}
}

func TestCRUDHooksWithFilters(t *testing.T) {
	hm := hooks.NewHookManager()
	crud := hooks.NewCRUDHooks[testEntity](hm, "test_entity")

	hm.AddFilter("test_entity.create.validate", 10, "test", func(ctx context.Context, hc hooks.HookContext) (hooks.HookContext, error) {
		if e, ok := hc["entity"].(*testEntity); ok {
			e.Name = "filtered"
		}
		return hc, nil
	})

	var capturedName string
	_ = capturedName
	crud.OnBeforeCreate(func(ctx context.Context, entity *testEntity) error {
		capturedName = entity.Name
		return nil
	})

	entity := &testEntity{ID: "1", Name: "original"}
	err := crud.BeforeCreate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if entity.Name != "filtered" {
		t.Errorf("expected filtered, got %s", entity.Name)
	}
}

func TestCRUDHooksWithoutCallbacks(t *testing.T) {
	hm := hooks.NewHookManager()
	crud := hooks.NewCRUDHooks[testEntity](hm, "test_entity")

	err := crud.BeforeCreate(context.Background(), &testEntity{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = crud.AfterCreate(context.Background(), &testEntity{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = crud.BeforeUpdate(context.Background(), &testEntity{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = crud.AfterUpdate(context.Background(), &testEntity{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = crud.BeforeDelete(context.Background(), "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = crud.AfterDelete(context.Background(), "1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
