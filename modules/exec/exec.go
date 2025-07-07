package exec

import (
	"context"
	"fmt"
	"sync"

	"github.com/gxo-labs/gxo/internal/command"
	"github.com/gxo-labs/gxo/internal/module"
	"github.com/gxo-labs/gxo/internal/paramutil"
	gxoerrors "github.com/gxo-labs/gxo/pkg/gxo/v1/errors"
	"github.com/gxo-labs/gxo/pkg/gxo/v1/plugin"
	"github.com/gxo-labs/gxo/pkg/gxo/v1/state"
)

// The init function is a Go mechanism that runs automatically when the package
// is imported. Here, it's used to self-register the ExecModule with the
// global default module registry.
func init() {
	// The name "exec" is the type users will specify in their playbook YAML.
	module.Register("exec", NewExecModule)
}

// ExecModule is a GXO module for executing local shell commands.
type ExecModule struct{}

// NewExecModule is the factory function that creates new instances of ExecModule.
// It is required by the module registration system.
func NewExecModule() plugin.Module {
	return &ExecModule{}
}

// Perform is the core logic for the exec module. It validates parameters,
// executes the command, and returns the result.
func (m *ExecModule) Perform(
	ctx context.Context,
	params map[string]interface{},
	stateReader state.StateReader,
	inputs map[string]<-chan map[string]interface{}, // Accepts the standard map-based input.
	outputChans []chan<- map[string]interface{},
	errChan chan<- error,
) (interface{}, error) {
	// CRITICAL: Even though this module is not a streaming consumer, it must
	// drain any input channels it receives. If it doesn't, a producer task
	// that is streaming to it will block forever, causing a playbook deadlock.
	var drainWg sync.WaitGroup
	drainWg.Add(len(inputs))
	for _, inputChan := range inputs {
		go func(ch <-chan map[string]interface{}) {
			defer drainWg.Done()
			for range ch {
				// Read from the channel and discard the record.
			}
		}(inputChan)
	}
	// Wait for all drain goroutines to complete before exiting. This ensures
	// proper cleanup in all execution paths.
	defer drainWg.Wait()

	// Use the paramutil helpers for robust and clear parameter validation.
	cmd, err := paramutil.GetRequiredString(params, "command")
	if err != nil {
		return nil, err
	}
	args, _, err := paramutil.GetOptionalStringSlice(params, "args")
	if err != nil {
		return nil, err
	}
	workingDir, _, err := paramutil.GetOptionalString(params, "working_dir")
	if err != nil {
		return nil, err
	}
	environment, _, err := paramutil.GetOptionalStringSlice(params, "environment")
	if err != nil {
		return nil, err
	}

	// Check the context for the DryRunKey to determine if we are in dry-run mode.
	if isDryRun := ctx.Value(module.DryRunKey{}) == true; isDryRun {
		// In dry-run mode, do not execute the command. Instead, return a
		// summary indicating what would have been done.
		return map[string]interface{}{
			"dry_run":   true,
			"command":   cmd,
			"args":      args,
			"stdout":    "",
			"stderr":    "",
			"exit_code": 0,
		}, nil
	}

	// Use the command runner abstraction for execution. This makes the module
	// easier to test by allowing the runner to be mocked.
	cmdRunner := command.NewRunner()
	result, runErr := cmdRunner.Run(ctx, cmd, args, workingDir, environment)

	// If runErr is not nil, it indicates a problem with starting or managing
	// the command process itself (e.g., command not found, context cancelled).
	if runErr != nil {
		// Return the command result (which may have partial stdout/stderr)
		// along with the fatal execution error.
		return result, fmt.Errorf("failed to execute command: %w", runErr)
	}

	// The command ran to completion. Create a summary map to be registered.
	summaryMap := map[string]interface{}{
		"stdout":    result.Stdout,
		"stderr":    result.Stderr,
		"exit_code": result.ExitCode,
	}

	// Check the command's exit code. A non-zero exit code is considered a
	// task failure.
	if result.ExitCode != 0 {
		// Wrap the error in a structured TaskExecutionError for better
		// diagnostics by the engine and observability tools.
		return summaryMap, gxoerrors.NewTaskExecutionError(
			"exec", // The name of this module type.
			fmt.Errorf("command exited with non-zero status: %d", result.ExitCode),
		)
	}

	// Command executed successfully (exit code 0).
	return summaryMap, nil
}