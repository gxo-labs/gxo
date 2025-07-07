package template

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"text/template"
	"time"

	"github.com/gxo-labs/gxo/internal/secrets"
	"github.com/gxo-labs/gxo/pkg/gxo/v1/events"
	pkgsecrets "github.com/gxo-labs/gxo/pkg/gxo/v1/secrets"
)

// GetFuncMap creates and returns the standard function map for GXO templates.
// It accepts a task-specific SecretTracker to correctly "taint" secrets
// resolved during a specific render operation.
func GetFuncMap(secretsProvider pkgsecrets.Provider, bus events.Bus, tracker *secrets.SecretTracker) template.FuncMap {
	fm := template.FuncMap{
		"env": funcEnv,
		// Add the standard 'eq' function for equality checks inside templates.
		"eq": func(a, b interface{}) bool {
			return reflect.DeepEqual(a, b)
		},
	}

	if secretsProvider != nil {
		fm["secret"] = createSecretFunc(secretsProvider, bus, tracker)
	}

	return fm
}

// funcEnv retrieves an environment variable.
func funcEnv(key string) string {
	return os.Getenv(key)
}

// createSecretFunc is a closure that builds the 'secret' template function,
// capturing the necessary providers and the task-local tracker.
func createSecretFunc(provider pkgsecrets.Provider, bus events.Bus, tracker *secrets.SecretTracker) func(string) (string, error) {
	return func(key string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		value, found, err := provider.GetSecret(ctx, key)
		if err != nil {
			return "", fmt.Errorf("failed to retrieve secret '%s': %w", key, err)
		}
		if !found {
			return "", fmt.Errorf("secret '%s' not found", key)
		}

		if bus != nil {
			bus.Emit(events.Event{
				Type:      events.SecretAccessed,
				Timestamp: time.Now(),
				Payload:   map[string]interface{}{"secret_key": key},
			})
		}

		// If a tracker was provided for this render call, add the resolved secret to it.
		// This "taints" the value for the duration of this task's execution.
		if tracker != nil {
			tracker.Add(value)
		}

		return value, nil
	}
}

// RedactSecretsInString performs a simple keyword-based redaction on a string.
// This function is intended for general output like logs or errors where
// a SecretTracker is not available.
func RedactSecretsInString(input string, keywords map[string]struct{}) string {
	if len(keywords) == 0 || input == "" {
		return input
	}

	redacted := false
	lines := strings.Split(input, "\n")
	outputLines := make([]string, len(lines))

	for i, line := range lines {
		outputLine := line
		lowerLine := strings.ToLower(line)
		for keyword := range keywords {
			if idx := strings.Index(lowerLine, keyword); idx != -1 {
				redactStart := idx + len(keyword)
				for redactStart < len(line) && strings.ContainsAny(string(line[redactStart]), ":= '\"") {
					redactStart++
				}

				if redactStart < len(line) {
					outputLine = line[:redactStart] + "[REDACTED]"
					redacted = true
					break
				}
			}
		}
		outputLines[i] = outputLine
	}

	if !redacted {
		return input
	}
	return strings.Join(outputLines, "\n")
}

// RedactSecretsInError redacts sensitive keywords within an error's message.
func RedactSecretsInError(err error, keywords map[string]struct{}) error {
	if err == nil || len(keywords) == 0 {
		return err
	}
	errMsg := err.Error()
	redactedMsg := RedactSecretsInString(errMsg, keywords)
	if errMsg != redactedMsg {
		return errors.New(redactedMsg)
	}
	return err
}