package tracing

import (
	"errors" // Needed for creating new error in RecordErrorWithContext
	"strings"

	// Import required OpenTelemetry packages
	"go.opentelemetry.io/otel"              // For global TracerProvider access fallback (though direct injection preferred)
	"go.opentelemetry.io/otel/attribute" // For creating span attributes
	codes "go.opentelemetry.io/otel/codes" // For setting span status codes
	oteltrace "go.opentelemetry.io/otel/trace"
)

// tracerName is the default name used when acquiring a tracer instance.
// Consistent naming helps identify the source of spans.
const tracerName = "gxo"

// GetTracer returns a named tracer instance from the globally configured OpenTelemetry provider.
// If no global provider is configured (e.g., in tests or simple applications),
// it defaults to returning a NoOpTracer, which safely discards all tracing data.
// Note: It's generally preferred to inject the TracerProvider into components rather
// than relying on the global provider.
func GetTracer() oteltrace.Tracer {
	// otel.Tracer handles the fallback to NoOpTracer internally if no provider is set.
	return otel.Tracer(tracerName)
}

// RedactStringMap creates a *new* map with values redacted based on keyword matching in keys.
// It iterates through the input map, checks if the lowercase version of the key exists
// in the `keywords` map (which should contain lowercase keywords), and replaces the value
// with "[REDACTED]" if a match is found. Original map is unchanged.
// Returns the original map if keywords map is nil/empty or input map is nil.
func RedactStringMap(input map[string]string, keywords map[string]struct{}) map[string]string {
	// Return early if redaction is not possible or needed.
	if len(keywords) == 0 || input == nil {
		return input
	}
	// Create a new map to store the potentially redacted output.
	output := make(map[string]string, len(input))
	// Iterate through the input map.
	for k, v := range input {
		// Check if the lowercase key exists in the keywords map.
		if _, redact := keywords[strings.ToLower(k)]; redact {
			// If keyword found, assign the redacted placeholder.
			output[k] = "[REDACTED]"
		} else {
			// Otherwise, copy the original value.
			output[k] = v
		}
	}
	return output
}

// RedactAttributes creates a new slice of OpenTelemetry KeyValue attributes
// where the value of any attribute whose key (converted to lowercase) matches
// a key in the `keywords` map is replaced with "[REDACTED]".
// Returns the original slice if keywords map is nil/empty or input slice is empty.
func RedactAttributes(attrs []attribute.KeyValue, keywords map[string]struct{}) []attribute.KeyValue {
	// Return early if redaction is not possible or needed.
	if len(keywords) == 0 || len(attrs) == 0 {
		return attrs
	}
	// Create a new slice to hold the potentially redacted attributes.
	redactedAttrs := make([]attribute.KeyValue, 0, len(attrs))
	// Iterate through the input attributes.
	for _, kv := range attrs {
		// Convert the attribute key to lowercase for case-insensitive matching.
		keyLower := strings.ToLower(string(kv.Key))
		// Check if the lowercase key exists in the keywords map.
		if _, redact := keywords[keyLower]; redact {
			// If keyword found, create a new attribute with the same key but redacted value.
			redactedAttrs = append(redactedAttrs, attribute.String(string(kv.Key), "[REDACTED]"))
		} else {
			// Otherwise, append the original attribute to the result slice.
			redactedAttrs = append(redactedAttrs, kv)
		}
	}
	return redactedAttrs
}

// RedactSecretsInString searches a string for sensitive keywords (case-insensitive)
// and replaces potentially associated values with "[REDACTED]".
// This function uses simple heuristics (looking for keywords followed by common separators)
// and may not catch all possible formats. It operates line by line.
// `keywords` should be a map where keys are lowercase sensitive keywords.
// (Consolidated helper from template/funcs.go - keep only one canonical version, placing here for tracing context).
func RedactSecretsInString(input string, keywords map[string]struct{}) string {
	// Skip processing if no keywords are defined or input is empty.
	if len(keywords) == 0 || input == "" {
		return input
	}

	redacted := false
	lines := strings.Split(input, "\n")
	outputLines := make([]string, len(lines))

	// Process each line individually.
	for i, line := range lines {
		outputLine := line // Assume no redaction initially for this line.
		lowerLine := strings.ToLower(line)
		// Check against each registered keyword.
		for keyword := range keywords {
			// Find the first occurrence of the keyword (case-insensitive).
			if idx := strings.Index(lowerLine, keyword); idx != -1 {
				// Keyword found. Attempt to redact the likely value following it.
				redactStart := idx + len(keyword)
				// Skip over common separators immediately following the keyword.
				for redactStart < len(line) && strings.ContainsAny(string(line[redactStart]), ":= '\"") {
					redactStart++
				}

				// Simple strategy: redact from the first non-separator char to the end of the line.
				// Ensure we don't redact if redactStart is beyond the line length.
				if redactStart < len(line) {
					outputLine = line[:redactStart] + "[REDACTED]"
					redacted = true // Mark that redaction occurred somewhere.
					// Once a keyword triggers redaction on a line, stop checking other keywords.
					break
				}
			}
		}
		outputLines[i] = outputLine
	}

	// Return the original string if no redactions were made.
	if !redacted {
		return input
	}
	// Join the processed lines back together.
	return strings.Join(outputLines, "\n")
}

// RecordErrorWithContext records an error on an OpenTelemetry span, ensuring the
// error message stored in the span attributes and status is redacted based on keywords.
// It also attempts to record a stack trace. Does nothing if the error is nil or the span is nil/not recording.
func RecordErrorWithContext(span oteltrace.Span, err error, keywords map[string]struct{}) {
	// Ensure there's an error and a valid span to record on.
	if err == nil || span == nil || !span.IsRecording() {
		return
	}
	// Redact the error message before using it in span events/status.
	redactedErrMsg := RedactSecretsInString(err.Error(), keywords)
	// Create a new error containing only the redacted message for recording purposes.
	// Note: This loses the original error type information for the recording itself.
	redactedErrForRecording := errors.New(redactedErrMsg)

	// Record the error event on the span, using the redacted error and adding stack trace option.
	span.RecordError(redactedErrForRecording, oteltrace.WithStackTrace(true))
	// Set the span status to Error, using the redacted message as the description.
	span.SetStatus(codes.Error, redactedErrMsg)
}