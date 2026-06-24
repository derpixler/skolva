package hooks

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

type HookContext map[string]any

type ActionFunc func(ctx context.Context, hc HookContext) error

type FilterFunc func(ctx context.Context, hc HookContext) (HookContext, error)

type ActionHandler struct {
	Name     string
	Priority int
	Plugin   string
	Handler  ActionFunc
}

type FilterHandler struct {
	Name     string
	Priority int
	Plugin   string
	Handler  FilterFunc
}

type HookManager struct {
	mu      sync.RWMutex
	actions map[string][]ActionHandler
	filters map[string][]FilterHandler
}

func NewHookManager() *HookManager {
	return &HookManager{
		actions: make(map[string][]ActionHandler),
		filters: make(map[string][]FilterHandler),
	}
}

func (hm *HookManager) AddAction(hook string, priority int, plugin string, handler ActionFunc) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hm.actions[hook] = append(hm.actions[hook], ActionHandler{
		Name:     hook,
		Priority: priority,
		Plugin:   plugin,
		Handler:  handler,
	})

	sort.Slice(hm.actions[hook], func(i, j int) bool {
		return hm.actions[hook][i].Priority < hm.actions[hook][j].Priority
	})
}

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

func (hm *HookManager) AddFilter(hook string, priority int, plugin string, handler FilterFunc) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hm.filters[hook] = append(hm.filters[hook], FilterHandler{
		Name:     hook,
		Priority: priority,
		Plugin:   plugin,
		Handler:  handler,
	})

	sort.Slice(hm.filters[hook], func(i, j int) bool {
		return hm.filters[hook][i].Priority < hm.filters[hook][j].Priority
	})
}

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
