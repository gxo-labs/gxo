package errors

import (
	"errors"
	"fmt"
)

// --- GXO Core Error Types ---

// ConfigError represents an error encountered during the loading, parsing,
// or validation of the playbook configuration or engine options.
type ConfigError struct {
	Message string
	Cause   error
}

func NewConfigError(message string, cause error) *ConfigError {
	return &ConfigError{Message: message, Cause: cause}
}
func (e *ConfigError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("configuration error: %s: %v", e.Message, e.Cause)
	}
	return fmt.Sprintf("configuration error: %s", e.Message)
}
func (e *ConfigError) Unwrap() error { return e.Cause }

// ValidationError indicates that some input (e.g., playbook structure,
// schema version, parameters) failed validation checks.
type ValidationError struct {
	Message string
	Cause   error
}

func NewValidationError(message string, cause error) *ValidationError {
	return &ValidationError{Message: message, Cause: cause}
}
func (e *ValidationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("validation error: %s: %v", e.Message, e.Cause)
	}
	return fmt.Sprintf("validation error: %s", e.Message)
}
func (e *ValidationError) Unwrap() error { return e.Cause }

// PolicyViolationError signifies that an operation could not proceed because
// it would violate a configured policy (e.g., channel overflow strategy 'error').
type PolicyViolationError struct {
	PolicyType string // e.g., "ChannelPolicy", "TaskPolicy"
	Reason     string
	Cause      error
}

func NewPolicyViolationError(policyType, reason string, cause error) *PolicyViolationError {
	return &PolicyViolationError{PolicyType: policyType, Reason: reason, Cause: cause}
}
func (e *PolicyViolationError) Error() string {
	msg := fmt.Sprintf("policy violation (%s): %s", e.PolicyType, e.Reason)
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", msg, e.Cause)
	}
	return msg
}
func (e *PolicyViolationError) Unwrap() error { return e.Cause }

// TaskExecutionError represents a fatal error that occurred during the execution
// of a specific task, often returned directly by a Plugin's Perform method.
type TaskExecutionError struct {
	TaskName string // User-defined name or internal ID
	Cause    error
}

func NewTaskExecutionError(taskName string, cause error) *TaskExecutionError {
	return &TaskExecutionError{TaskName: taskName, Cause: cause}
}
func (e *TaskExecutionError) Error() string {
	if e.TaskName == "" {
		return fmt.Sprintf("task execution failed: %v", e.Cause)
	}
	return fmt.Sprintf("task '%s' execution failed: %v", e.TaskName, e.Cause)
}
func (e *TaskExecutionError) Unwrap() error { return e.Cause }

// ModuleNotFoundError indicates that a plugin specified in a task's 'type'
// field could not be found in the plugin registry.
type ModuleNotFoundError struct {
	ModuleName string
}

func NewModuleNotFoundError(moduleName string) *ModuleNotFoundError {
	return &ModuleNotFoundError{ModuleName: moduleName}
}
func (e *ModuleNotFoundError) Error() string {
	return fmt.Sprintf("plugin module not found: %s", e.ModuleName)
}

// SkippedError indicates a task was intentionally skipped (e.g., 'when' condition false).
// It implements the error interface but signifies non-failure. Used internally.
type SkippedError struct {
	Reason string
}

func NewSkippedError(reason string) *SkippedError {
	return &SkippedError{Reason: reason}
}
func (e *SkippedError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("task skipped: %s", e.Reason)
	}
	return "task skipped"
}

// IsSkipped checks if an error is a SkippedError using errors.As.
func IsSkipped(err error) bool {
	var skipErr *SkippedError
	return errors.As(err, &skipErr)
}

// RecordProcessingError represents an error that occurred while a plugin
// was processing a specific data record from its input channel. These errors
// are typically considered non-fatal to the task itself. Plugins report these
// via the dedicated error channel provided to Perform.
type RecordProcessingError struct {
	TaskName string      // User-defined name or internal ID
	ItemID   interface{} // Context about the specific record, can be nil
	Cause    error
}

func NewRecordProcessingError(taskName string, itemID interface{}, cause error) *RecordProcessingError {
	return &RecordProcessingError{TaskName: taskName, ItemID: itemID, Cause: cause}
}
func (e *RecordProcessingError) Error() string {
	itemName := "unknown item"
	if e.ItemID != nil {
		itemName = fmt.Sprintf("item '%v'", e.ItemID)
	}
	taskCtx := ""
	if e.TaskName != "" {
		taskCtx = fmt.Sprintf(" in task '%s'", e.TaskName)
	}
	return fmt.Sprintf("record processing error%s for %s: %v", taskCtx, itemName, e.Cause)
}
func (e *RecordProcessingError) Unwrap() error { return e.Cause }
