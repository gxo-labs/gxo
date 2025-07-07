package template

import "github.com/gxo-labs/gxo/internal/secrets"

// RedactedSecretValue is the placeholder string used to replace a tracked secret
// value that was attempted to be registered into the state.
const RedactedSecretValue = "[REDACTED_SECRET]"

// RedactTrackedSecrets recursively walks a data structure and replaces any
// string value that is a tracked secret, or contains a tracked secret,
// with a redacted placeholder. It returns the (potentially) new data structure
// and a boolean indicating if any redaction occurred.
func RedactTrackedSecrets(data interface{}, tracker *secrets.SecretTracker) (interface{}, bool) {
	if data == nil || tracker == nil {
		return data, false
	}

	// Use a helper function to manage the recursion and the 'redacted' flag state.
	return redactRecursive(data, tracker)
}

// redactRecursive is the internal worker for the redaction process.
func redactRecursive(data interface{}, tracker *secrets.SecretTracker) (interface{}, bool) {
	if data == nil {
		return nil, false
	}

	switch v := data.(type) {
	case string:
		// Check for both exact match and substring containment.
		if tracker.ContainsTrackedSecret(v) {
			return RedactedSecretValue, true
		}
		// If no secret is found, return the original string and false.
		return v, false

	case map[string]interface{}:
		// Handle nil map gracefully.
		if v == nil {
			return nil, false
		}

		redactedInMap := false
		// Create a new map to hold the results, ensuring the original is not modified.
		newMap := make(map[string]interface{}, len(v))
		for key, val := range v {
			// Recursively call the redaction function on the value.
			newVal, wasRedacted := redactRecursive(val, tracker)
			newMap[key] = newVal
			// If any value in the map was redacted, the whole map is considered to have been redacted.
			if wasRedacted {
				redactedInMap = true
			}
		}
		return newMap, redactedInMap

	case []interface{}:
		// Handle nil slice gracefully.
		if v == nil {
			return nil, false
		}
		redactedInSlice := false
		// Create a new slice to hold the results.
		newSlice := make([]interface{}, len(v))
		for i, val := range v {
			// Recursively call the redaction function on the element.
			newVal, wasRedacted := redactRecursive(val, tracker)
			newSlice[i] = newVal
			// If any element in the slice was redacted, the whole slice is considered to have been redacted.
			if wasRedacted {
				redactedInSlice = true
			}
		}
		return newSlice, redactedInSlice

	default:
		// For any other type (e.g., int, bool), no redaction is possible.
		return data, false
	}
}