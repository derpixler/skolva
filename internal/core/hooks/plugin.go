package hooks

import "github.com/jackc/pgx/v5/pgxpool"

type Plugin interface {
	Name() string
	Version() string
	Description() string
	Register(hm *HookManager) error
	Activate(db *pgxpool.Pool) error
	Deactivate() error
}

type PluginRegistry struct {
	plugins []Plugin
}

func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{}
}

func (r *PluginRegistry) Add(plugin Plugin) {
	r.plugins = append(r.plugins, plugin)
}

func (r *PluginRegistry) All() []Plugin {
	return r.plugins
}

func (r *PluginRegistry) RegisterAll(hm *HookManager) error {
	for _, p := range r.plugins {
		if err := p.Register(hm); err != nil {
			return err
		}
	}
	return nil
}

func (r *PluginRegistry) ActivateAll(db *pgxpool.Pool) error {
	for _, p := range r.plugins {
		if err := p.Activate(db); err != nil {
			return err
		}
	}
	return nil
}

func (r *PluginRegistry) DeactivateAll() error {
	for _, p := range r.plugins {
		if err := p.Deactivate(); err != nil {
			return err
		}
	}
	return nil
}
