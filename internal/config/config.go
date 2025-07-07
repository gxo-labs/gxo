package config

import (
	"time"
)

// Constants for Channel Policy Overflow Strategy.
const (
	OverflowBlock      = "block"
	OverflowDropNew    = "drop_new"
	OverflowDropOldest = "drop_oldest"
	OverflowError      = "error"
)

// Playbook represents the top-level structure of a GXO playbook YAML file.
type Playbook struct {
	Name          string                 `yaml:"name"`
	SchemaVersion string                 `yaml:"schemaVersion"`
	Vars          map[string]interface{} `yaml:"vars,omitempty"`
	Tasks         []Task                 `yaml:"tasks"`
	TaskPolicy    *TaskPolicy            `yaml:"task_policy,omitempty"`
	ChannelPolicy *ChannelPolicy         `yaml:"channel_policy,omitempty"`

	// StatePolicy defines a global default policy for state access. This policy
	// will apply to all tasks in the playbook unless overridden by a task's
	// own state_policy block. Optional.
	StatePolicy *StatePolicy `yaml:"state_policy,omitempty"`
	// FilePath is an internal field for storing the source file path for context
	// in logging and error messages. It is not parsed from the YAML.
	FilePath string `yaml:"-"`
}

// Task represents a single unit of work within a playbook.
type Task struct {
	Name           string                 `yaml:"name,omitempty"`
	Type           string                 `yaml:"type"`
	Params         map[string]interface{} `yaml:"params,omitempty"`
	Register       string                 `yaml:"register,omitempty"`
	StreamInputs   []string               `yaml:"stream_inputs,omitempty"`
	IgnoreErrors   bool                   `yaml:"ignore_errors,omitempty"`
	When           string                 `yaml:"when,omitempty"`
	Loop           interface{}            `yaml:"loop,omitempty"`
	LoopControl    *LoopControlConfig     `yaml:"loop_control,omitempty"`
	Retry          *RetryConfig           `yaml:"retry,omitempty"`
	Timeout        string                 `yaml:"timeout,omitempty"`
	PolicyRef      string                 `yaml:"policy_ref,omitempty"` // Reserved for future use.
	Policy         *TaskPolicy            `yaml:"policy,omitempty"`

	// StatePolicy defines a task-specific state access policy, overriding any
	// global state_policy defined at the playbook level. Optional.
	StatePolicy *StatePolicy `yaml:"state_policy,omitempty"`
	// InternalID is a unique identifier assigned by the engine during loading.
	// It is used for all internal referencing (e.g., in the DAG).
	InternalID string `yaml:"-"`
}

// LoopControlConfig specifies how loops defined by the 'loop' directive are executed.
type LoopControlConfig struct {
	Parallel int    `yaml:"parallel,omitempty"`
	LoopVar  string `yaml:"loop_var,omitempty"`
}

// RetryConfig defines the parameters for retrying a task upon failure.
type RetryConfig struct {
	Attempts      int      `yaml:"attempts,omitempty"`
	Delay         string   `yaml:"delay,omitempty"`
	MaxDelay      string   `yaml:"max_delay,omitempty"`
	BackoffFactor *float64 `yaml:"backoff_factor,omitempty"`
	Jitter        *float64 `yaml:"jitter,omitempty"`
	OnError       *bool    `yaml:"on_error,omitempty"`
}

// ChannelPolicy defines policies for data channels used for streaming between tasks.
type ChannelPolicy struct {
	Name             string `yaml:"-" json:"-"`
	BufferSize       *int   `yaml:"buffer_size,omitempty" json:"buffer_size,omitempty"`
	OverflowStrategy string `yaml:"overflow_strategy,omitempty" json:"overflow_strategy,omitempty"`
}

// TaskPolicy defines generic policies applicable to task execution behavior.
type TaskPolicy struct {
	Name          string `yaml:"-" json:"-"`
	SkipOnNoInput *bool  `yaml:"skip_on_no_input,omitempty" json:"skip_on_no_input,omitempty"`
}

// GetLoopParallel returns the configured loop parallelism or the default (1).
func (t *Task) GetLoopParallel() int {
	if t.LoopControl != nil && t.LoopControl.Parallel > 1 {
		return t.LoopControl.Parallel
	}
	return 1
}

// GetLoopVar returns the configured loop variable name or the default ("item").
func (t *Task) GetLoopVar() string {
	if t.LoopControl != nil && t.LoopControl.LoopVar != "" {
		return t.LoopControl.LoopVar
	}
	return "item"
}

// GetRetryAttempts returns the configured number of attempts or the default (1).
func (t *Task) GetRetryAttempts() int {
	if t.Retry != nil && t.Retry.Attempts >= 1 {
		return t.Retry.Attempts
	}
	return 1
}

// GetRetryDelay returns the configured base retry delay duration or the default (1 second).
func (t *Task) GetRetryDelay() time.Duration {
	delayStr := "1s"
	if t.Retry != nil && t.Retry.Delay != "" {
		delayStr = t.Retry.Delay
	}
	duration, err := time.ParseDuration(delayStr)
	if err != nil || duration <= 0 {
		return 1 * time.Second
	}
	return duration
}

// GetRetryMaxDelay returns the configured maximum retry delay duration, or 0 if unset/invalid.
func (t *Task) GetRetryMaxDelay() time.Duration {
	if t.Retry != nil && t.Retry.MaxDelay != "" {
		duration, err := time.ParseDuration(t.Retry.MaxDelay)
		if err != nil || duration < 0 {
			return 0
		}
		return duration
	}
	return 0
}

// GetRetryBackoffFactor returns the configured backoff factor, defaulting to 1.0.
func (t *Task) GetRetryBackoffFactor() float64 {
	if t.Retry != nil && t.Retry.BackoffFactor != nil {
		if *t.Retry.BackoffFactor >= 1.0 {
			return *t.Retry.BackoffFactor
		}
	}
	return 1.0
}

// GetRetryJitter returns the configured jitter factor (clamped between 0.0 and 1.0), defaulting to 0.0.
func (t *Task) GetRetryJitter() float64 {
	if t.Retry != nil && t.Retry.Jitter != nil {
		jitter := *t.Retry.Jitter
		if jitter < 0.0 {
			jitter = 0.0
		} else if jitter > 1.0 {
			jitter = 1.0
		}
		return jitter
	}
	return 0.0
}

// ShouldRetryOnError returns whether retries should occur only on errors, defaulting to true.
func (t *Task) ShouldRetryOnError() bool {
	if t.Retry != nil && t.Retry.OnError != nil {
		return *t.Retry.OnError
	}
	return true
}

// GetTimeout returns the configured task-specific timeout duration, or 0 if unset/invalid.
func (t *Task) GetTimeout() time.Duration {
	if t.Timeout == "" {
		return 0
	}
	duration, err := time.ParseDuration(t.Timeout)
	if err != nil || duration < 0 {
		return 0
	}
	return duration
}