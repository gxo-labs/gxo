package module

import (
	"fmt"
	"sync"

	// Import public interfaces the internal registry deals with.
	gxoerrors "github.com/gxo-labs/gxo/pkg/gxo/v1/errors"
	"github.com/gxo-labs/gxo/pkg/gxo/v1/plugin" // Import the public plugin package
)

// ModuleFactory is a function type that creates instances of a specific Module.
// DEPRECATED: This local alias is maintained temporarily for compatibility during refactoring.
// Code should prefer using plugin.ModuleFactory directly from the public package.
// Used during registration. Now uses the public plugin.Module interface.
type ModuleFactory func() plugin.Module

// StaticRegistry implements the plugin.Registry interface using a compile-time map.
// It provides thread-safe registration and retrieval of module factories.
// This is the default registry implementation used by GXO if no other registry is provided.
type StaticRegistry struct {
	// factories maps the registered module name (string) to its factory function.
	// The factory function type used here MUST match the public interface definition.
	factories map[string]plugin.ModuleFactory // Use public type directly
	// mu provides read/write locking to ensure thread-safe access to the factories map.
	mu sync.RWMutex
}

// NewStaticRegistry creates a new, empty static registry.
// Modules must be registered using the Register method before they can be retrieved.
func NewStaticRegistry() *StaticRegistry {
	return &StaticRegistry{
		// Initialize the map to store factories of the public type.
		factories: make(map[string]plugin.ModuleFactory),
	}
}

// Register associates a module type name with its factory function.
// This function is typically called from the init() function of a module package
// or explicitly by the application wiring the registry. It enforces that module names
// and factories are valid and prevents duplicate registrations.
// The factory parameter type MUST match the public plugin.Registry interface definition.
func (r *StaticRegistry) Register(name string, factory plugin.ModuleFactory) error { // Use public type
	r.mu.Lock()
	defer r.mu.Unlock()

	// Validate input parameters.
	if name == "" {
		return gxoerrors.NewConfigError("module registration error: name cannot be empty", nil)
	}
	if factory == nil {
		return gxoerrors.NewConfigError(fmt.Sprintf("module registration error for '%s': factory cannot be nil", name), nil)
	}
	// Prevent duplicate registrations.
	if _, exists := r.factories[name]; exists {
		return gxoerrors.NewConfigError(fmt.Sprintf("module registration error: duplicate module name '%s'", name), nil)
	}

	// Store the factory function (which is of the public type).
	r.factories[name] = factory
	return nil
}

// Get retrieves the factory function for a given module name.
// It returns the factory and a nil error if found.
// If the module name is not registered, it returns nil and a ModuleNotFoundError.
// The return type MUST match the public plugin.Registry interface definition.
func (r *StaticRegistry) Get(name string) (plugin.ModuleFactory, error) { // Use public type
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Retrieve the factory (which is already of the public type).
	factory, exists := r.factories[name]
	if !exists {
		// Return a specific error type indicating the module was not found.
		return nil, gxoerrors.NewModuleNotFoundError(name)
	}
	// Return the factory directly.
	return factory, nil
}

// List returns a slice containing the names of all registered modules.
// The order of names in the returned slice is not guaranteed.
func (r *StaticRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Allocate a slice with the exact capacity for efficiency.
	names := make([]string, 0, len(r.factories))
	// Iterate over the map keys (module names) and append them to the slice.
	for name := range r.factories {
		names = append(names, name)
	}
	return names
}

// --- Default Global Registry (for compile-time registration via init) ---

var (
	// globalRegistry holds the default registry instance used for package-level
	// registration via the global Register function.
	globalRegistry = NewStaticRegistry()
	// Compile-time check to ensure StaticRegistry correctly implements the public
	// plugin.Registry interface. This fails the build if the implementation drifts.
	_ plugin.Registry = (*StaticRegistry)(nil)
)

// Register globally associates a module type name with its factory function
// in the default global registry instance. This is the intended mechanism for
// modules to self-register during program initialization via their init() functions.
// It panics on registration errors (e.g., duplicate name) because init() functions
// run early, and such errors indicate a programming mistake that must be fixed.
// The factory parameter type MUST match the public plugin.Registry interface definition.
func Register(name string, factory plugin.ModuleFactory) { // Use public type
	if err := globalRegistry.Register(name, factory); err != nil {
		// Panic provides immediate feedback during development about registration issues.
		panic(fmt.Errorf("failed to register module '%s' globally: %w", name, err))
	}
}

// DefaultStaticRegistryGetter provides convenient access to the global static registry instance.
// This allows the main application (`cmd/gxo`) or library consumers to easily retrieve
// the default registry containing compile-time registered modules. It exposes the global
// registry as the public plugin.Registry interface type.
var DefaultStaticRegistryGetter plugin.Registry = globalRegistry