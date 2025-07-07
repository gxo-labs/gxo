package engine_test

import (
	"context"
	goerrors "errors"
	"fmt"
	"sync"
	"time"

	gxoerrors "github.com/gxo-labs/gxo/pkg/gxo/v1/errors"
	"github.com/gxo-labs/gxo/pkg/gxo/v1/plugin"
	gxov1state "github.com/gxo-labs/gxo/pkg/gxo/v1/state"

	"github.com/gxo-labs/gxo/internal/module"
	"github.com/gxo-labs/gxo/internal/paramutil"
)

// InMemoryRegistry provides a simple, in-memory implementation of the plugin.Registry
// interface for use in tests.
type InMemoryRegistry struct {
	factories map[string]plugin.ModuleFactory
	mu        sync.RWMutex
}

// NewInMemoryRegistry creates a new, empty registry.
func NewInMemoryRegistry() *InMemoryRegistry {
	return &InMemoryRegistry{
		factories: make(map[string]plugin.ModuleFactory),
	}
}

// Register adds a module factory to the registry.
func (r *InMemoryRegistry) Register(name string, factory plugin.ModuleFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if name == "" {
		return gxoerrors.NewConfigError("mock registry: name cannot be empty", nil)
	}
	if factory == nil {
		return gxoerrors.NewConfigError(fmt.Sprintf("mock registry: factory cannot be nil for '%s'", name), nil)
	}
	r.factories[name] = factory
	return nil
}

// Get retrieves a module factory from the registry.
func (r *InMemoryRegistry) Get(name string) (plugin.ModuleFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	factory, exists := r.factories[name]
	if !exists {
		return nil, gxoerrors.NewModuleNotFoundError(name)
	}
	return factory, nil
}

// List returns the names of all registered modules.
func (r *InMemoryRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	return names
}

var _ plugin.Registry = (*InMemoryRegistry)(nil)

// MockModule is a flexible module for testing engine behavior.
type MockModule struct{}

// NewMockModule is the factory function for MockModule.
func NewMockModule() plugin.Module {
	return &MockModule{}
}

// Perform implements the mock module's logic.
// It can be configured via parameters to fail, delay, or check state.
func (m *MockModule) Perform(
	ctx context.Context,
	params map[string]interface{},
	stateReader gxov1state.StateReader,
	inputs map[string]<-chan map[string]interface{},
	outputChans []chan<- map[string]interface{},
	errChan chan<- error,
) (interface{}, error) {
	isDryRun := ctx.Value(module.DryRunKey{}) == true

	// If 'fail_message' is present, fail with that message.
	if failMsg, exists, _ := paramutil.GetOptionalString(params, "fail_message"); exists {
		if isDryRun {
			return map[string]interface{}{"dry_run": true, "action": "would fail", "message": failMsg}, nil
		}
		return nil, goerrors.New(failMsg)
	}

	// If '_mock_delay' is present, wait for the specified duration.
	if delayStr, exists, _ := paramutil.GetOptionalString(params, "_mock_delay"); exists {
		delay, parseErr := time.ParseDuration(delayStr)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid _mock_delay format: %w", parseErr)
		}
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Drain any input channels to prevent deadlocks in streaming tests.
	var wg sync.WaitGroup
	if len(inputs) > 0 {
		wg.Add(len(inputs))
		for _, inputChan := range inputs {
			go func(ch <-chan map[string]interface{}) {
				defer wg.Done()
				for range ch {
				}
			}(inputChan)
		}
	}
	wg.Wait()

	if isDryRun {
		return map[string]interface{}{"dry_run": true, "action": "would succeed"}, nil
	}

	// Default behavior: return the received params map as the summary.
	return params, nil
}

// RegisterTestMockModule is a helper function to register the MockModule into a test registry.
func RegisterTestMockModule(registry *InMemoryRegistry) error {
	moduleName := "mock"
	if registry == nil {
		return goerrors.New("registry cannot be nil")
	}
	return registry.Register(moduleName, NewMockModule)
}