package v1

import (
	"context"
	"runtime"
	"time"

	"github.com/gxo-labs/gxo/internal/config"
	"github.com/gxo-labs/gxo/pkg/gxo/v1/events"
	gxoerrors "github.com/gxo-labs/gxo/pkg/gxo/v1/errors"
	"github.com/gxo-labs/gxo/pkg/gxo/v1/metrics"
	"github.com/gxo-labs/gxo/pkg/gxo/v1/plugin"
	"github.com/gxo-labs/gxo/pkg/gxo/v1/secrets"
	"github.com/gxo-labs/gxo/pkg/gxo/v1/state"
	"github.com/gxo-labs/gxo/pkg/gxo/v1/tracing"
)

// EngineV1 defines the public interface for the GXO automation engine.
type EngineV1 interface {
	// RunPlaybook executes a playbook from its raw YAML content.
	RunPlaybook(ctx context.Context, playbookYAML []byte) (*ExecutionReport, error)

	// MetricsRegistryProvider returns the underlying metrics provider.
	MetricsRegistryProvider() metrics.RegistryProvider
	// TracerProvider returns the underlying tracing provider.
	TracerProvider() tracing.TracerProvider

	// Setter methods for configuring engine components programmatically.
	SetStateStore(store state.Store) error
	SetSecretsProvider(provider secrets.Provider) error
	SetEventBus(bus events.Bus) error
	SetPluginRegistry(registry plugin.Registry) error
	SetMetricsRegistryProvider(provider metrics.RegistryProvider) error
	SetTracerProvider(provider tracing.TracerProvider) error
	SetDefaultTimeout(timeout time.Duration) error
	SetWorkerPoolSize(size int) error
	SetDefaultChannelPolicy(policy ChannelPolicy) error
	SetRedactedKeywords(keywords []string) error
	SetStallPolicy(policy *config.StallPolicy) error
}

// EngineOption is a function type used to configure the GXO engine at creation.
type EngineOption func(EngineV1) error

// TaskResult holds the final outcome of a single task execution.
type TaskResult struct {
	Status    string        `json:"status"`
	Error     string        `json:"error,omitempty"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Duration  time.Duration `json:"duration"`
}

// ExecutionReport provides a comprehensive summary of a completed playbook run.
type ExecutionReport struct {
	PlaybookName   string                `json:"playbook_name"`
	OverallStatus  string                `json:"overall_status"`
	StartTime      time.Time             `json:"start_time"`
	EndTime        time.Time             `json:"end_time"`
	Duration       time.Duration         `json:"duration"`
	TotalTasks     int                   `json:"total_tasks"`
	CompletedTasks int                   `json:"completed_tasks"`
	FailedTasks    int                   `json:"failed_tasks"`
	SkippedTasks   int                   `json:"skipped_tasks"`
	Error          string                `json:"error,omitempty"`
	TaskResults    map[string]TaskResult `json:"task_results"`
}

// ChannelPolicy defines the public configuration for streaming channels.
type ChannelPolicy struct {
	BufferSize       *int   `yaml:"buffer_size,omitempty" json:"buffer_size,omitempty"`
	OverflowStrategy string `yaml:"overflow_strategy,omitempty" json:"overflow_strategy,omitempty"`
}

// WithStateStore is an engine option to provide a custom state store.
func WithStateStore(store state.Store) EngineOption {
	return func(e EngineV1) error {
		if store == nil {
			return gxoerrors.NewConfigError("state store cannot be nil", nil)
		}
		return e.SetStateStore(store)
	}
}

// WithSecretsProvider is an engine option to provide a custom secrets provider.
func WithSecretsProvider(provider secrets.Provider) EngineOption {
	return func(e EngineV1) error {
		if provider == nil {
			return gxoerrors.NewConfigError("secrets provider cannot be nil", nil)
		}
		return e.SetSecretsProvider(provider)
	}
}

// WithEventBus is an engine option to provide a custom event bus.
func WithEventBus(bus events.Bus) EngineOption {
	return func(e EngineV1) error {
		if bus == nil {
			return gxoerrors.NewConfigError("event bus cannot be nil", nil)
		}
		return e.SetEventBus(bus)
	}
}

// WithPluginRegistry is an engine option to provide a custom plugin registry.
func WithPluginRegistry(registry plugin.Registry) EngineOption {
	return func(e EngineV1) error {
		if registry == nil {
			return gxoerrors.NewConfigError("plugin registry cannot be nil", nil)
		}
		return e.SetPluginRegistry(registry)
	}
}

// WithMetricsRegistryProvider is an engine option to provide a custom metrics provider.
func WithMetricsRegistryProvider(provider metrics.RegistryProvider) EngineOption {
	return func(e EngineV1) error {
		if provider == nil {
			return gxoerrors.NewConfigError("metrics registry provider cannot be nil", nil)
		}
		return e.SetMetricsRegistryProvider(provider)
	}
}

// WithTracerProvider is an engine option to provide a custom tracing provider.
func WithTracerProvider(provider tracing.TracerProvider) EngineOption {
	return func(e EngineV1) error {
		if provider == nil {
			return gxoerrors.NewConfigError("tracer provider cannot be nil", nil)
		}
		return e.SetTracerProvider(provider)
	}
}

// WithWorkerPoolSize is an engine option to configure the number of concurrent task workers.
func WithWorkerPoolSize(size int) EngineOption {
	return func(e EngineV1) error {
		effectiveSize := size
		if effectiveSize <= 0 {
			effectiveSize = runtime.NumCPU()
		}
		return e.SetWorkerPoolSize(effectiveSize)
	}
}

// WithDefaultChannelPolicy is an engine option to set the default streaming channel policy.
func WithDefaultChannelPolicy(policy ChannelPolicy) EngineOption {
	return func(e EngineV1) error {
		return e.SetDefaultChannelPolicy(policy)
	}
}

// WithDefaultTimeout is an engine option to set the default timeout for tasks.
func WithDefaultTimeout(timeout time.Duration) EngineOption {
	return func(e EngineV1) error {
		if timeout < 0 {
			return gxoerrors.NewConfigError("default timeout cannot be negative", nil)
		}
		return e.SetDefaultTimeout(timeout)
	}
}

// WithRedactedKeywords is an engine option to configure the list of keywords for secret redaction.
func WithRedactedKeywords(keywords []string) EngineOption {
	return func(e EngineV1) error {
		return e.SetRedactedKeywords(keywords)
	}
}

// WithStallPolicy is an engine option to configure the engine's stall detection mechanism.
func WithStallPolicy(interval time.Duration, tolerance int) EngineOption {
	return func(e EngineV1) error {
		if interval <= 0 || tolerance <= 0 {
			return gxoerrors.NewConfigError("stall policy interval and tolerance must be positive", nil)
		}
		policy := &config.StallPolicy{
			Interval:  interval,
			Tolerance: tolerance,
		}
		return e.SetStallPolicy(policy)
	}
}