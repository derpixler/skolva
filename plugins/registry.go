// Package plugins is the central registry for Go-level plugins.
//
// In Phase 1 the registry is empty. As modules are implemented,
// each one registers its hooks here via plugins.All().
// The registry is consumed by cmd/api/main.go at startup.
package plugins

import "github.com/derpixler/skolva-core/hooks"

// All returns the list of registered plugins. Currently empty — plugins
// will be added as modules are implemented in subsequent phases.
func All() []hooks.Plugin {
	return []hooks.Plugin{}
}
