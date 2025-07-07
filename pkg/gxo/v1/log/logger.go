// Package log defines the public logging interface used across GXO packages.
package log

import (
	"context"
	// Use standard library's structured logging level type.
	"log/slog"
)

// Logger defines the public interface for logging operations within GXO.
// This interface allows consumers of the GXO library or internal components
// to use different logging implementations consistently. It mirrors common
// logging patterns found in libraries like slog.
type Logger interface {
	// Debugf logs a formatted message at the DEBUG level.
	// Arguments are handled in the manner of fmt.Sprintf.
	Debugf(format string, args ...interface{})
	// Infof logs a formatted message at the INFO level.
	// Arguments are handled in the manner of fmt.Sprintf.
	Infof(format string, args ...interface{})
	// Warnf logs a formatted message at the WARN level.
	// Arguments are handled in the manner of fmt.Sprintf.
	Warnf(format string, args ...interface{})
	// Errorf logs a formatted message at the ERROR level.
	// Arguments are handled in the manner of fmt.Sprintf. It's recommended
	// implementations also check if the last arg is an error and log it structurally.
	Errorf(format string, args ...interface{})

	// Log logs a message at the specified slog.Level with additional key-value attributes.
	// This is the primary method for structured logging.
	Log(level slog.Level, msg string, args ...interface{})
	// LogCtx logs a message at the specified slog.Level, potentially including
	// context information like trace IDs if supported by the implementation.
	LogCtx(ctx context.Context, level slog.Level, msg string, args ...interface{})

	// With returns a new Logger instance with the specified attributes added
	// to all subsequent log entries. Attributes are typically key-value pairs.
	With(args ...interface{}) Logger
	// IsEnabled checks if the logger is configured to output logs at the given level.
	// This can be used to avoid expensive computation for log messages that would be discarded.
	IsEnabled(level slog.Level) bool
}