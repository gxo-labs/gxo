package command

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"syscall"
)

// CommandResult holds the outcome of executing an external command.
type CommandResult struct {
	// Stdout contains the standard output captured from the command.
	Stdout string
	// Stderr contains the standard error captured from the command.
	Stderr string
	// ExitCode is the exit status code returned by the command.
	// A value of -1 typically indicates an error occurred before the command
	// could be started or completed (e.g., command not found, context cancelled).
	ExitCode int
	// Error is any error encountered during the setup or execution of the command
	// (e.g., command not found, context cancellation). It does not necessarily
	// indicate a non-zero exit code, but rather issues with running the command itself.
	Error error
}

// Runner defines the interface for running external commands.
type Runner interface {
	// Run executes the specified command with given arguments, working directory,
	// and environment variables. It respects the provided context for cancellation.
	Run(ctx context.Context, command string, args []string, workingDir string, environment []string) (*CommandResult, error)
}

// defaultRunner implements the Runner interface using Go's os/exec package.
type defaultRunner struct{}

// NewRunner creates a new instance of the default command runner.
func NewRunner() Runner {
	return &defaultRunner{}
}

// Run executes the command using os/exec.
func (r *defaultRunner) Run(ctx context.Context, command string, args []string, workingDir string, environment []string) (*CommandResult, error) {
	cmd := exec.CommandContext(ctx, command, args...)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if workingDir != "" {
		cmd.Dir = workingDir
	}

	if len(environment) > 0 {
		// exec.CommandContext copies the current process's environment by default.
		// Setting cmd.Env replaces it entirely. If specific vars are needed,
		// they should be appended to the existing environment or handled carefully.
		// This implementation replaces the environment as per common usage in automation tools.
		cmd.Env = environment
	}

	result := &CommandResult{
		ExitCode: -1, // Default to -1 indicating potential execution failure
	}

	err := cmd.Run()

	result.Stdout = stdoutBuf.String()
	result.Stderr = stderrBuf.String()

	if err != nil {
		// Check if the error is due to context cancellation
		if ctx.Err() != nil {
			result.Error = ctx.Err()
			// Exit code remains -1 as the command was likely terminated prematurely
			return result, ctx.Err() // Return context error specifically
		}

		// Check if it's an ExitError to retrieve the status code
		var exitErr *exec.ExitError
		// Use errors.As from the standard 'errors' package
		if ok := errors.As(err, &exitErr); ok {
			// Command ran but exited with a non-zero status
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				result.ExitCode = status.ExitStatus()
			}
			// Store the underlying ExitError details, but don't return it as the primary error
			// unless it wasn't context cancellation. The caller usually checks ExitCode.
			result.Error = err
			// We return nil error here because the command *ran* successfully,
			// even if its exit code was non-zero. The caller should check ExitCode.
			return result, nil
		}

		// Other errors (e.g., command not found, permission issues)
		result.Error = err
		result.ExitCode = -1 // Indicate failure to start/run properly
		return result, err  // Return the execution error itself
	}

	// Command executed successfully (exit code 0)
	result.ExitCode = 0
	return result, nil
}