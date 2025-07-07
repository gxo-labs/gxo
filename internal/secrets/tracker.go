package secrets

import (
	"strings"
	"sync"
)

// SecretTracker tracks resolved secret values during a task's execution
// to prevent them from being registered into the state. It is designed to be
// used for a single task instance and is not globally shared.
type SecretTracker struct {
	mu              sync.RWMutex
	resolvedSecrets map[string]struct{} // Stores the raw secret values
}

// NewSecretTracker creates a new, empty tracker.
func NewSecretTracker() *SecretTracker {
	return &SecretTracker{
		resolvedSecrets: make(map[string]struct{}),
	}
}

// Add marks a secret value as having been seen by this tracker instance.
// It is thread-safe. It ignores empty strings.
func (t *SecretTracker) Add(secretValue string) {
	if secretValue == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.resolvedSecrets[secretValue] = struct{}{}
}

// IsTracked checks if a given string value is a tracked secret.
// This performs an exact match and is thread-safe.
func (t *SecretTracker) IsTracked(value string) bool {
	if value == "" {
		return false
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	_, found := t.resolvedSecrets[value]
	return found
}

// ContainsTrackedSecret checks if the given input string contains any of the
// tracked secret values as a substring. This is the primary method used for
// redaction to catch secrets embedded in larger strings (e.g., connection strings).
// It returns true if a secret is found within the string. It is thread-safe.
func (t *SecretTracker) ContainsTrackedSecret(input string) bool {
	if input == "" {
		return false
	}
	t.mu.RLock()
	defer t.mu.RUnlock()

	// An empty tracker can't contain anything.
	if len(t.resolvedSecrets) == 0 {
		return false
	}

	// Iterate through all known secret values for this task.
	for secret := range t.resolvedSecrets {
		// A simple substring check is effective and reasonably performant.
		if strings.Contains(input, secret) {
			return true
		}
	}
	return false
}