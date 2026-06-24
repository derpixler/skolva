package hooks

import (
	"context"
	"fmt"
)

// CRUDHooks[T] wraps the HookManager to provide typed lifecycle hooks for
// a specific entity type. Hooks fire in this order for each operation:
//
//	Create: validate filters → before_create actions → after_create actions → response filters
//	Update:  before_update actions → after_update actions
//	Delete:  before_delete actions → after_delete actions
type CRUDHooks[T any] struct {
	hm           *HookManager
	entityName   string
	beforeCreate func(ctx context.Context, entity *T) error
	afterCreate  func(ctx context.Context, entity *T) error
	beforeUpdate func(ctx context.Context, entity *T) error
	afterUpdate  func(ctx context.Context, entity *T) error
	beforeDelete func(ctx context.Context, id string) error
	afterDelete  func(ctx context.Context, id string) error
}

// NewCRUDHooks creates a CRUD hook wrapper for the given entity name.
func NewCRUDHooks[T any](hm *HookManager, entityName string) *CRUDHooks[T] {
	return &CRUDHooks[T]{hm: hm, entityName: entityName}
}

// OnBeforeCreate registers a callback that runs before entity creation.
func (h *CRUDHooks[T]) OnBeforeCreate(fn func(ctx context.Context, entity *T) error) {
	h.beforeCreate = fn
}

// OnAfterCreate registers a callback that runs after entity creation.
func (h *CRUDHooks[T]) OnAfterCreate(fn func(ctx context.Context, entity *T) error) {
	h.afterCreate = fn
}

// OnBeforeUpdate registers a callback that runs before entity update.
func (h *CRUDHooks[T]) OnBeforeUpdate(fn func(ctx context.Context, entity *T) error) {
	h.beforeUpdate = fn
}

// OnAfterUpdate registers a callback that runs after entity update.
func (h *CRUDHooks[T]) OnAfterUpdate(fn func(ctx context.Context, entity *T) error) {
	h.afterUpdate = fn
}

// OnBeforeDelete registers a callback that runs before entity deletion.
func (h *CRUDHooks[T]) OnBeforeDelete(fn func(ctx context.Context, id string) error) {
	h.beforeDelete = fn
}

// OnAfterDelete registers a callback that runs after entity deletion.
func (h *CRUDHooks[T]) OnAfterDelete(fn func(ctx context.Context, id string) error) {
	h.afterDelete = fn
}

// BeforeCreate runs the before-create callback, validate filters, and
// before_create actions.
func (h *CRUDHooks[T]) BeforeCreate(ctx context.Context, entity *T) error {
	if h.beforeCreate != nil {
		if err := h.beforeCreate(ctx, entity); err != nil {
			return fmt.Errorf("%s.before_create: %w", h.entityName, err)
		}
	}
	hc, err := h.hm.ApplyFilters(ctx, h.entityName+".create.validate", HookContext{"entity": entity})
	if err != nil {
		return err
	}
	return h.hm.DoActions(ctx, h.entityName+".before_create", hc)
}

// AfterCreate runs the after-create callback, response filters, and
// after_create actions.
func (h *CRUDHooks[T]) AfterCreate(ctx context.Context, entity *T) error {
	if h.afterCreate != nil {
		if err := h.afterCreate(ctx, entity); err != nil {
			return fmt.Errorf("%s.after_create: %w", h.entityName, err)
		}
	}
	hc, err := h.hm.ApplyFilters(ctx, h.entityName+".create.response", HookContext{"entity": entity})
	if err != nil {
		return err
	}
	return h.hm.DoActions(ctx, h.entityName+".after_create", hc)
}

// BeforeUpdate runs the before-update callback and actions.
func (h *CRUDHooks[T]) BeforeUpdate(ctx context.Context, entity *T) error {
	if h.beforeUpdate != nil {
		if err := h.beforeUpdate(ctx, entity); err != nil {
			return fmt.Errorf("%s.before_update: %w", h.entityName, err)
		}
	}
	return h.hm.DoActions(ctx, h.entityName+".before_update", HookContext{"entity": entity})
}

// AfterUpdate runs the after-update callback and actions.
func (h *CRUDHooks[T]) AfterUpdate(ctx context.Context, entity *T) error {
	if h.afterUpdate != nil {
		if err := h.afterUpdate(ctx, entity); err != nil {
			return fmt.Errorf("%s.after_update: %w", h.entityName, err)
		}
	}
	return h.hm.DoActions(ctx, h.entityName+".after_update", HookContext{"entity": entity})
}

// BeforeDelete runs the before-delete callback and actions.
func (h *CRUDHooks[T]) BeforeDelete(ctx context.Context, id string) error {
	if h.beforeDelete != nil {
		if err := h.beforeDelete(ctx, id); err != nil {
			return fmt.Errorf("%s.before_delete: %w", h.entityName, err)
		}
	}
	return h.hm.DoActions(ctx, h.entityName+".before_delete", HookContext{"id": id})
}

// AfterDelete runs the after-delete callback and actions.
func (h *CRUDHooks[T]) AfterDelete(ctx context.Context, id string) error {
	if h.afterDelete != nil {
		if err := h.afterDelete(ctx, id); err != nil {
			return fmt.Errorf("%s.after_delete: %w", h.entityName, err)
		}
	}
	return h.hm.DoActions(ctx, h.entityName+".after_delete", HookContext{"id": id})
}
