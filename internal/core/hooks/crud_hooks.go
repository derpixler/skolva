package hooks

import (
	"context"
	"fmt"
)

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

func NewCRUDHooks[T any](hm *HookManager, entityName string) *CRUDHooks[T] {
	return &CRUDHooks[T]{
		hm:         hm,
		entityName: entityName,
	}
}

func (h *CRUDHooks[T]) OnBeforeCreate(fn func(ctx context.Context, entity *T) error) {
	h.beforeCreate = fn
}

func (h *CRUDHooks[T]) OnAfterCreate(fn func(ctx context.Context, entity *T) error) {
	h.afterCreate = fn
}

func (h *CRUDHooks[T]) OnBeforeUpdate(fn func(ctx context.Context, entity *T) error) {
	h.beforeUpdate = fn
}

func (h *CRUDHooks[T]) OnAfterUpdate(fn func(ctx context.Context, entity *T) error) {
	h.afterUpdate = fn
}

func (h *CRUDHooks[T]) OnBeforeDelete(fn func(ctx context.Context, id string) error) {
	h.beforeDelete = fn
}

func (h *CRUDHooks[T]) OnAfterDelete(fn func(ctx context.Context, id string) error) {
	h.afterDelete = fn
}

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

func (h *CRUDHooks[T]) BeforeUpdate(ctx context.Context, entity *T) error {
	if h.beforeUpdate != nil {
		if err := h.beforeUpdate(ctx, entity); err != nil {
			return fmt.Errorf("%s.before_update: %w", h.entityName, err)
		}
	}
	return h.hm.DoActions(ctx, h.entityName+".before_update", HookContext{"entity": entity})
}

func (h *CRUDHooks[T]) AfterUpdate(ctx context.Context, entity *T) error {
	if h.afterUpdate != nil {
		if err := h.afterUpdate(ctx, entity); err != nil {
			return fmt.Errorf("%s.after_update: %w", h.entityName, err)
		}
	}
	return h.hm.DoActions(ctx, h.entityName+".after_update", HookContext{"entity": entity})
}

func (h *CRUDHooks[T]) BeforeDelete(ctx context.Context, id string) error {
	if h.beforeDelete != nil {
		if err := h.beforeDelete(ctx, id); err != nil {
			return fmt.Errorf("%s.before_delete: %w", h.entityName, err)
		}
	}
	return h.hm.DoActions(ctx, h.entityName+".before_delete", HookContext{"id": id})
}

func (h *CRUDHooks[T]) AfterDelete(ctx context.Context, id string) error {
	if h.afterDelete != nil {
		if err := h.afterDelete(ctx, id); err != nil {
			return fmt.Errorf("%s.after_delete: %w", h.entityName, err)
		}
	}
	return h.hm.DoActions(ctx, h.entityName+".after_delete", HookContext{"id": id})
}
