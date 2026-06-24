package hooks

import "github.com/jackc/pgx/v5/pgxpool"

// Plugin is the interface that all Go-level plugins must implement.
//
// Register is called at startup to add hooks. Activate receives the
// database pool for any setup or migration work. Deactivate is called
// on shutdown and should release resources.
type Plugin interface {
	Name() string
	Version() string
	Description() string
	Register(hm *HookManager) error
	Activate(db *pgxpool.Pool) error
	Deactivate() error
}

// PluginRegistry collects plugins and provides batch lifecycle methods.
type PluginRegistry struct {
	plugins []Plugin
}

// NewPluginRegistry returns an empty registry.
func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{}
}

// Add appends a plugin to the registry.
func (r *PluginRegistry) Add(plugin Plugin) {
	r.plugins = append(r.plugins, plugin)
}

// All returns a copy of the registered plugins.
func (r *PluginRegistry) All() []Plugin {
	return r.plugins
}

// RegisterAll calls Register on every plugin. Fails on the first error.
func (r *PluginRegistry) RegisterAll(hm *HookManager) error {
	for _, p := range r.plugins {
		if err := p.Register(hm); err != nil {
			return err
		}
	}
	return nil
}

// ActivateAll calls Activate on every plugin. Fails on the first error.
func (r *PluginRegistry) ActivateAll(db *pgxpool.Pool) error {
	for _, p := range r.plugins {
		if err := p.Activate(db); err != nil {
			return err
		}
	}
	return nil
}

// DeactivateAll calls Deactivate on every plugin. Fails on the first error.
func (r *PluginRegistry) DeactivateAll() error {
	for _, p := range r.plugins {
		if err := p.Deactivate(); err != nil {
			return err
		}
	}
	return nil
}
