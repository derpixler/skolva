// Package hooks provides an extensibility system for module interaction.
//
// Two primitives are supported:
//
//	Actions — fire-and-forget callbacks executed in priority order.
//	Filters — transform chains that pass a HookContext through handlers.
//
// Plugins register handlers at startup and can be removed at shutdown.
package hooks

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// HookContext is a key-value bag passed between hook handlers.
type HookContext map[string]any

// ActionFunc is a fire-and-forget hook handler.
type ActionFunc func(ctx context.Context, hc HookContext) error

// FilterFunc transforms a HookContext, returning a new one or an error.
type FilterFunc func(ctx context.Context, hc HookContext) (HookContext, error)

// ActionHandler bundles an action with metadata for priority sorting.
type ActionHandler struct {
	Name     string
	Priority int
	Plugin   string
	Handler  ActionFunc
}

// FilterHandler bundles a filter with metadata for priority sorting.
type FilterHandler struct {
	Name     string
	Priority int
	Plugin   string
	Handler  FilterFunc
}

// HookManager is the central registry for action and filter hooks.
// It is safe for concurrent use.
type HookManager struct {
	mu      sync.RWMutex
	actions map[string][]ActionHandler
	filters map[string][]FilterHandler
}

// NewHookManager returns an empty HookManager.
func NewHookManager() *HookManager {
	return &HookManager{
		actions: make(map[string][]ActionHandler),
		filters: make(map[string][]FilterHandler),
	}
}

// AddAction registers an action handler for the given hook name.
// Handlers are sorted by priority (ascending) on insertion.
func (hm *HookManager) AddAction(hook string, priority int, plugin string, handler ActionFunc) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hm.actions[hook] = append(hm.actions[hook], ActionHandler{
		Name: hook, Priority: priority, Plugin: plugin, Handler: handler,
	})

	sort.Slice(hm.actions[hook], func(i, j int) bool {
		return hm.actions[hook][i].Priority < hm.actions[hook][j].Priority
	})
}

// DoActions executes all action handlers for the given hook in priority order.
// The first error stops the chain and is returned.
func (hm *HookManager) DoActions(ctx context.Context, hook string, hc HookContext) error {
	hm.mu.RLock()
	handlers := hm.actions[hook]
	hm.mu.RUnlock()

	for _, h := range handlers {
		if err := h.Handler(ctx, hc); err != nil {
			return fmt.Errorf("action %s/%s: %w", hook, h.Plugin, err)
		}
	}
	return nil
}

// AddFilter registers a filter handler for the given hook name.
// Handlers are sorted by priority (ascending) on insertion.
func (hm *HookManager) AddFilter(hook string, priority int, plugin string, handler FilterFunc) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hm.filters[hook] = append(hm.filters[hook], FilterHandler{
		Name: hook, Priority: priority, Plugin: plugin, Handler: handler,
	})

	sort.Slice(hm.filters[hook], func(i, j int) bool {
		return hm.filters[hook][i].Priority < hm.filters[hook][j].Priority
	})
}

// ApplyFilters runs all filter handlers for the given hook in priority order.
// Each handler receives the HookContext returned by the previous handler.
// The first error stops the chain and is returned.
func (hm *HookManager) ApplyFilters(ctx context.Context, hook string, hc HookContext) (HookContext, error) {
	hm.mu.RLock()
	handlers := hm.filters[hook]
	hm.mu.RUnlock()

	var err error
	for _, h := range handlers {
		hc, err = h.Handler(ctx, hc)
		if err != nil {
			return nil, fmt.Errorf("filter %s/%s: %w", hook, h.Plugin, err)
		}
	}
	return hc, nil
}

// RemovePlugin purges all action and filter handlers registered by the given plugin.
func (hm *HookManager) RemovePlugin(plugin string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	for hook, handlers := range hm.actions {
		filtered := handlers[:0]
		for _, h := range handlers {
			if h.Plugin != plugin {
				filtered = append(filtered, h)
			}
		}
		hm.actions[hook] = filtered
	}

	for hook, handlers := range hm.filters {
		filtered := handlers[:0]
		for _, h := range handlers {
			if h.Plugin != plugin {
				filtered = append(filtered, h)
			}
		}
		hm.filters[hook] = filtered
	}
}
