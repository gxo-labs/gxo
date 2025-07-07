package logger

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	// Import public error types used for structural logging
	gxoerrors "github.com/gxo-labs/gxo/pkg/gxo/v1/errors"
	// Import the public logger interface it implements
	gxolog "github.com/gxo-labs/gxo/pkg/gxo/v1/log"
	// Import OpenTelemetry trace package for context handling
	"go.opentelemetry.io/otel/trace"
)

// Default log level if not specified or invalid.
const defaultLevel = slog.LevelInfo

// parseLogLevel converts common log level strings (case-insensitive) to slog.Level values.
func parseLogLevel(levelStr string) slog.Level {
	switch strings.ToUpper(levelStr) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		// Log a warning if falling back? Or assume Info is acceptable.
		// fmt.Fprintf(os.Stderr, "Warning: Invalid log level '%s', using default 'INFO'\n", levelStr)
		return defaultLevel
	}
}

// defaultLogger implements the public gxolog.Logger interface
// using the standard Go slog library.
type defaultLogger struct {
	// Embed the slog.Logger to directly expose its methods like Log, LogAttrs.
	*slog.Logger
}

// Compile-time check to ensure defaultLogger implements the public Logger interface.
var _ gxolog.Logger = (*defaultLogger)(nil)

// NewLogger creates a new Logger instance configured with the specified level,
// output format ("text" or "json"), and writer (defaults to os.Stderr).
// It returns an instance satisfying the public gxolog.Logger interface.
func NewLogger(levelStr string, formatStr string, writer io.Writer) gxolog.Logger { // Return public interface
	level := parseLogLevel(levelStr)
	if writer == nil {
		writer = os.Stderr
	}

	// Configure handler options: set level and custom attribute replacer.
	opts := &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: replaceLevelAttribute, // Use custom replacer for level key.
	}

	// Select the base slog handler based on the requested format.
	var baseHandler slog.Handler
	switch strings.ToLower(formatStr) {
	case "json":
		baseHandler = slog.NewJSONHandler(writer, opts)
	case "text":
		fallthrough // Fallthrough for text format.
	default: // Default to text format if formatStr is invalid or empty.
		baseHandler = slog.NewTextHandler(writer, opts)
	}

	// Wrap the base handler with the OtelHandler to inject trace/span IDs.
	otelHandler := NewOtelHandler(baseHandler)

	// Create the slog.Logger using the wrapped handler and return the custom logger struct.
	return &defaultLogger{
		Logger: slog.New(otelHandler),
	}
}

// Mapping from slog levels to desired uppercase string representation in logs.
var levelStringMap = map[slog.Level]string{
	slog.LevelDebug: "DEBUG",
	slog.LevelInfo:  "INFO",
	slog.LevelWarn:  "WARN",
	slog.LevelError: "ERROR",
}

// replaceLevelAttribute is used in HandlerOptions to customize the output
// of the standard slog level attribute to be an uppercase string (e.g., "INFO").
func replaceLevelAttribute(groups []string, a slog.Attr) slog.Attr {
	// Check if the attribute key is the standard level key.
	if a.Key == slog.LevelKey {
		// Attempt to get the level value.
		level, ok := a.Value.Any().(slog.Level)
		if !ok {
			// If not a slog.Level, return the original attribute.
			return a
		}
		// Look up the desired string representation.
		levelStr, exists := levelStringMap[level]
		if !exists {
			// Fallback to default string representation if not in our map.
			levelStr = level.String()
		}
		// Replace the attribute value with the string representation.
		a.Value = slog.StringValue(levelStr)
	}
	// Return the (potentially modified) attribute.
	return a
}

// NewDefaultLogger provides a basic text logger instance writing to Stderr at Info level.
// Useful for simple cases or when configuration is unavailable.
// Returns an instance satisfying the public gxolog.Logger interface.
func NewDefaultLogger(levelStr string) gxolog.Logger { // Return public interface
	// For simplicity, always use "text" and os.Stderr for this default logger.
	return NewLogger(levelStr, "text", os.Stderr)
}

// Debugf logs a formatted message at the DEBUG level.
// Implements the gxolog.Logger interface.
func (l *defaultLogger) Debugf(format string, args ...interface{}) {
	// Check if DEBUG level is enabled before formatting and logging.
	if l.Logger.Enabled(context.Background(), slog.LevelDebug) {
		// Format the message using Sprintf.
		msg := fmt.Sprintf(format, args...)
		// Log using the embedded slog.Logger's Log method.
		l.Logger.Log(context.Background(), slog.LevelDebug, msg)
	}
}

// Infof logs a formatted message at the INFO level.
// Implements the gxolog.Logger interface.
func (l *defaultLogger) Infof(format string, args ...interface{}) {
	if l.Logger.Enabled(context.Background(), slog.LevelInfo) {
		msg := fmt.Sprintf(format, args...)
		l.Logger.Log(context.Background(), slog.LevelInfo, msg)
	}
}

// Warnf logs a formatted message at the WARN level.
// Implements the gxolog.Logger interface.
func (l *defaultLogger) Warnf(format string, args ...interface{}) {
	if l.Logger.Enabled(context.Background(), slog.LevelWarn) {
		msg := fmt.Sprintf(format, args...)
		l.Logger.Log(context.Background(), slog.LevelWarn, msg)
	}
}

// Errorf logs a formatted message at the ERROR level.
// It checks if the last argument is an error and attempts to log structured
// details if it's a known GXO error type (like RecordProcessingError).
// Implements the gxolog.Logger interface.
func (l *defaultLogger) Errorf(format string, args ...interface{}) {
	if l.Logger.Enabled(context.Background(), slog.LevelError) {
		// Format the base message first.
		msg := fmt.Sprintf(format, args...)
		// Pass the message and original args to the helper for structured error handling.
		l.logHelper(context.Background(), slog.LevelError, msg, args...)
	}
}

// logHelper is an internal helper to add structured error details to log entries.
// It checks the last argument for an error type and adds specific attributes if it's
// a RecordProcessingError, otherwise logs the standard error string.
func (l *defaultLogger) logHelper(ctx context.Context, level slog.Level, msg string, args ...interface{}) {
	logArgs := []any{} // Use any for slog arguments.
	processedArgs := args // Assume we log all original args unless last is error.

	// Check if the last argument is an error.
	if len(args) > 0 {
		lastArg := args[len(args)-1]
		if err, ok := lastArg.(error); ok {
			// Last arg is an error, remove it from main args for Sprintf style message.
			processedArgs = args[:len(args)-1]
			// Check if it's a RecordProcessingError for structured logging.
			var rpe *gxoerrors.RecordProcessingError
			if errors.As(err, &rpe) {
				// Add specific attributes for RecordProcessingError.
				logArgs = append(logArgs, slog.String("error_type", "RecordProcessingError"))
				if rpe.TaskName != "" { logArgs = append(logArgs, slog.String("task_name", rpe.TaskName)) }
				if rpe.ItemID != nil { logArgs = append(logArgs, slog.Any("item_id", rpe.ItemID)) }
				// Include cause if available, otherwise the main error message.
				if rpe.Cause != nil {
					logArgs = append(logArgs, slog.String("error", rpe.Cause.Error()))
				} else {
					logArgs = append(logArgs, slog.String("error", rpe.Error())) // Use base error msg if no cause
				}
			} else {
				// For other errors, just add the standard error string.
				logArgs = append(logArgs, slog.String("error", err.Error()))
			}
		}
	}
	// Combine message args and structured error args.
	finalArgs := append(processedArgs, logArgs...)
	// Log using the embedded slog logger.
	l.Logger.Log(ctx, level, msg, finalArgs...)
}

// Log logs a message at the specified level with explicit key-value pairs.
// Implements the gxolog.Logger interface.
func (l *defaultLogger) Log(level slog.Level, msg string, args ...interface{}) {
	// Directly use the embedded slog.Logger's Log method.
	l.Logger.Log(context.Background(), level, msg, args...)
}

// LogCtx logs a message at the specified level, potentially including
// trace/span IDs from the context via the OtelHandler.
// Implements the gxolog.Logger interface.
func (l *defaultLogger) LogCtx(ctx context.Context, level slog.Level, msg string, args ...interface{}) {
	// Directly use the embedded slog.Logger's Log method, passing the context.
	l.Logger.Log(ctx, level, msg, args...)
}

// With returns a new Logger instance with added attributes.
// It returns the public interface type gxolog.Logger for consistency.
// Implements the gxolog.Logger interface.
func (l *defaultLogger) With(args ...interface{}) gxolog.Logger { // Return public interface
	// Create a new slog.Logger with the added attributes.
	newSlogger := l.Logger.With(args...)
	// Wrap the new slog.Logger in our defaultLogger struct and return as the interface.
	return &defaultLogger{Logger: newSlogger}
}

// IsEnabled checks if logging is enabled for the specified level.
// Implements the gxolog.Logger interface.
func (l *defaultLogger) IsEnabled(level slog.Level) bool {
	// Use the embedded slog.Logger's Enabled method.
	return l.Logger.Enabled(context.Background(), level)
}

// --- OtelHandler for Trace/Span ID Injection ---

// OtelHandler is a slog.Handler middleware that automatically injects
// OpenTelemetry trace_id and span_id attributes into log records if a valid
// span context exists in the logging context.
type OtelHandler struct {
	// next is the underlying slog.Handler that this handler wraps.
	next slog.Handler
}

// NewOtelHandler creates a new OtelHandler wrapping the provided handler.
func NewOtelHandler(next slog.Handler) *OtelHandler {
	return &OtelHandler{next: next}
}

// Enabled forwards the check to the wrapped handler.
func (h *OtelHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

// Handle processes the log record. It extracts span context from the context.Context,
// adds trace_id and span_id attributes if available, and then forwards the
// modified record to the wrapped handler.
func (h *OtelHandler) Handle(ctx context.Context, record slog.Record) error {
	// Extract the current span from the context.
	span := trace.SpanFromContext(ctx)
	// Check if the span context is valid (i.e., tracing is active for this request).
	if span.SpanContext().IsValid() {
		// Add trace and span IDs as attributes to the log record.
		record.AddAttrs(
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().SpanID().String()),
		)
	}
	// Pass the potentially modified record to the next handler in the chain.
	return h.next.Handle(ctx, record)
}

// WithAttrs returns a new OtelHandler wrapping the result of calling WithAttrs
// on the next handler.
func (h *OtelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return NewOtelHandler(h.next.WithAttrs(attrs))
}

// WithGroup returns a new OtelHandler wrapping the result of calling WithGroup
// on the next handler.
func (h *OtelHandler) WithGroup(name string) slog.Handler {
	return NewOtelHandler(h.next.WithGroup(name))
}