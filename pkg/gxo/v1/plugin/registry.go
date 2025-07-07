package plugin

import (
	"context"

	"github.com/gxo-labs/gxo/pkg/gxo/v1/state"
)

// Module defines the public interface that all GXO plugins (modules) must implement.
// It is the fundamental unit of execution logic in GXO.
type Module interface {
	// Perform executes the plugin's core logic. It is the heart of every module.
	//
	// Parameters:
	// - ctx: The context for the execution, carrying deadlines, cancellation signals,
	//   and request-scoped values like the DryRunKey. Modules MUST respect
	//   context cancellation, especially on long-running or blocking operations.
	//
	// - params: A map of parameters defined in the playbook task. String values
	//   will have already been rendered by the template engine. Modules should
	//   use helpers from internal/paramutil for robust validation.
	//
	// - stateReader: A read-only interface to the playbook's state store. Modules
	//   can use this to access playbook variables (`vars`) or the registered
	//   results of other tasks. By default, all data read is a deep copy to
	//   guarantee immutability.
	//
	// - inputs: A map of read-only data channels for streaming input. The map key is
	//   the internal ID of the producer task, allowing the module to identify
	//   the source of each stream. Modules that act as consumers MUST read from
	//   all provided channels until they are closed to prevent deadlocking the
	//   producers.
	//
	// - outputChans: A slice of write-only channels for fanning-out streaming data.
	//   Modules that act as producers MUST write each record they generate to every
	//   channel in this slice. The engine will close these channels automatically
	//   after Perform returns successfully.
	//
	// - errChan: A write-only channel for reporting non-fatal, record-specific
	//   processing errors. These errors are logged by the engine but do not
	//   halt the task. Use gxoerrors.NewRecordProcessingError for this purpose.
	//
	// Returns:
	// - summary (interface{}): A value to be stored in the state if the task's
	//   `register` directive is set. This can be any serializable type.
	//
	// - err (error): A fatal error that will cause the task to fail and, unless
	//   `ignore_errors` is true, will halt the entire playbook. A return value of
	//   `nil` indicates successful completion.
	Perform(ctx context.Context, params map[string]interface{}, stateReader state.StateReader, inputs map[string]<-chan map[string]interface{}, outputChans []chan<- map[string]interface{}, errChan chan<- error) (summary interface{}, err error)
}

// ModuleFactory is a function type that creates new instances of a specific Module.
// Each module registers a factory function of this type.
type ModuleFactory func() Module

// Registry defines the public interface for the engine's plugin registry.
// It provides a mechanism for registering and retrieving module factories by name.
type Registry interface {
	// Get retrieves the factory function for a given module name.
	// It returns a gxoerrors.ModuleNotFoundError if the name is not registered.
	Get(name string) (ModuleFactory, error)

	// Register associates a module type name with its factory function.
	// This should be concurrency-safe. It returns an error if the name is
	// empty, the factory is nil, or the name is already registered.
	Register(name string, factory ModuleFactory) error

	// List returns a slice containing the names of all registered modules.
	// The order of names in the returned slice is not guaranteed.
	List() []string
}