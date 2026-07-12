package plugin

import (
	"fmt"
	"sort"
	"sync"
)

// Registry is a thread-safe store of registered NASIJ plugins.
// Plugins are keyed by their Name() return value.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
}

// NewRegistry returns an initialised, empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]Plugin),
	}
}

// Register adds a plugin to the registry.
//
// Returns an error if a plugin with the same name is already registered.
// Callers should not modify the plugin after registration.
func (r *Registry) Register(p Plugin) error {
	if p == nil {
		return fmt.Errorf("plugin registry: cannot register nil plugin")
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	name := p.Name()
	if name == "" {
		return fmt.Errorf("plugin registry: plugin Name() must not be empty")
	}
	if _, exists := r.plugins[name]; exists {
		return fmt.Errorf("plugin registry: %q is already registered", name)
	}

	r.plugins[name] = p
	return nil
}

// Get retrieves a plugin by name.
// Returns (plugin, true) if found, (nil, false) otherwise.
func (r *Registry) Get(name string) (Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	return p, ok
}

// List returns all registered plugins sorted alphabetically by name.
// The returned slice is a snapshot — mutations to the registry after this
// call are not reflected.
func (r *Registry) List() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Plugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name() < result[j].Name()
	})
	return result
}

// Count returns the number of currently registered plugins.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.plugins)
}

// Unregister removes a plugin by name.
// Returns an error if the plugin is not registered.
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[name]; !exists {
		return fmt.Errorf("plugin registry: %q is not registered", name)
	}
	delete(r.plugins, name)
	return nil
}

// ByKind returns all plugins of the specified kind, sorted by name.
func (r *Registry) ByKind(kind Kind) []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Plugin
	for _, p := range r.plugins {
		if p.Kind() == kind {
			result = append(result, p)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name() < result[j].Name()
	})
	return result
}
