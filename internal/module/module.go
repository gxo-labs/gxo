package module

import (
	"github.com/gxo-labs/gxo/internal/config"
	gxolog "github.com/gxo-labs/gxo/pkg/gxo/v1/log"
	gxov1state "github.com/gxo-labs/gxo/pkg/gxo/v1/state"
)

// DryRunKey is the context key used to signal dry-run mode to modules.
// Modules should check for the presence of this key's value in the context
// provided to their Perform method to determine if they should simulate actions.
type DryRunKey struct{}

// ProducerIDMapKey is the key used in a module's parameters to pass the
// mapping of producer task names to their internal IDs. This allows streaming
// modules to correctly identify their input channels.
const ProducerIDMapKey = "_gxo_producer_id_map_"

// ExecutionContext provides context information (like task config, state access, logger)
// to internal ExecutionHooks. This interface is not intended for direct use by external plugins.
// It allows hooks to interact with the engine's state during specific lifecycle points.
type ExecutionContext interface {
	// Task returns the internal configuration definition of the task being executed.
	// Hooks can use this to access task parameters or metadata.
	Task() *config.Task

	// State returns a read-only view of the current playbook state.
	// Hooks can inspect variables, registered results, or task statuses.
	// The returned reader uses the public state interface definition.
	State() gxov1state.StateReader

	// Logger returns a logger instance satisfying the public interface (gxolog.Logger).
	// Hooks should use this logger for any output.
	Logger() gxolog.Logger

	// Get retrieves a value previously stored by a hook using Set within the same
	// task execution lifecycle (e.g., passing data from BeforeExecute to AfterExecute).
	// This provides a mechanism for hooks to maintain state across execution points.
	Get(key interface{}) interface{}

	// Set stores a value associated with a key within the context's internal storage.
	// This is intended for hooks to communicate data between their own execution points
	// (e.g., storing a start time in BeforeExecute to calculate duration in AfterExecute).
	Set(key interface{}, value interface{})
}

// ExecutionHook defines an interface for injecting custom logic before and after
// the core execution (Module.Perform) of a task instance.
// This remains an internal engine detail for potential future features like
// enhanced metrics, tracing, or policy enforcement that operate outside the module itself.
type ExecutionHook interface {
	// BeforeExecute is called just before the TaskRunner attempts to execute
	// the Module.Perform method (including retries).
	// An error returned from this hook will prevent the module execution.
	BeforeExecute(execCtx ExecutionContext) error

	// AfterExecute is called after the Module.Perform method (including all retries)
	// has completed, regardless of whether it succeeded, failed, or was skipped.
	// It receives the final summary and error returned by the task execution logic.
	// Errors returned from this hook are logged but generally do not alter the
	// overall playbook flow, although they might influence final reporting.
	AfterExecute(execCtx ExecutionContext, summary interface{}, taskErr error) error
}