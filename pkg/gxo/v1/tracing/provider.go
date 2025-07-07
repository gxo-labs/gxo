// --- START OF FILE pkg/gxo/v1/tracing/provider.go ---
package tracing

import (
	"context"
	// Import only the necessary OpenTelemetry trace package for interface definition.
	"go.opentelemetry.io/otel/trace"
)

// TracerProvider defines the interface for accessing the engine's tracer provider.
// This allows consumers of the GXO library to integrate GXO's tracing with their
// existing OpenTelemetry setup or provide custom implementations.
type TracerProvider interface {
	// GetTracer returns a Tracer instance with the specified name and options.
	// This aligns with OTel's TracerProvider interface concept.
	GetTracer(name string, opts ...trace.TracerOption) trace.Tracer

	// Shutdown gracefully shuts down the tracer provider, flushing any buffered spans.
	// The context should have a deadline for the shutdown process. Implementations
	// should handle cases where shutdown is not applicable (e.g., NoOp provider).
	Shutdown(ctx context.Context) error
}
