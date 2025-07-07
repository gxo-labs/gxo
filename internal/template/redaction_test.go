package template_test

import (
	"testing"

	"github.com/gxo-labs/gxo/internal/secrets"
	"github.com/gxo-labs/gxo/internal/template"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTracker creates a new SecretTracker and populates it with a standard set of secrets for testing.
func setupTracker() *secrets.SecretTracker {
	tracker := secrets.NewSecretTracker()
	tracker.Add("s3cr3t_p@ssw0rd")
	tracker.Add("another-key-456")
	return tracker
}

func TestRedactTrackedSecrets_SimpleString(t *testing.T) {
	tracker := setupTracker()

	// Test case 1: The string is an exact match for a tracked secret.
	input1 := "s3cr3t_p@ssw0rd"
	redacted1, wasRedacted1 := template.RedactTrackedSecrets(input1, tracker)
	assert.True(t, wasRedacted1, "Should report that redaction occurred for exact match")
	assert.Equal(t, template.RedactedSecretValue, redacted1, "Exact match secret should be redacted")

	// Test case 2: The string contains a tracked secret as a substring.
	input2 := "The API key is s3cr3t_p@ssw0rd and should not be logged."
	redacted2, wasRedacted2 := template.RedactTrackedSecrets(input2, tracker)
	assert.True(t, wasRedacted2, "Should report that redaction occurred for substring match")
	assert.Equal(t, template.RedactedSecretValue, redacted2, "String containing a secret should be redacted")

	// Test case 3: The string does not contain any tracked secret.
	input3 := "This is a perfectly safe string."
	redacted3, wasRedacted3 := template.RedactTrackedSecrets(input3, tracker)
	assert.False(t, wasRedacted3, "Should report no redaction for a safe string")
	assert.Equal(t, input3, redacted3, "Safe string should remain unchanged")
}

func TestRedactTrackedSecrets_NilAndEmpty(t *testing.T) {
	tracker := setupTracker()

	// Test nil input data
	redacted, wasRedacted := template.RedactTrackedSecrets(nil, tracker)
	assert.False(t, wasRedacted, "Should not report redaction for nil input")
	assert.Nil(t, redacted, "Should return nil for nil input")

	// Test nil tracker
	redacted, wasRedacted = template.RedactTrackedSecrets("some data", nil)
	assert.False(t, wasRedacted, "Should not report redaction for nil tracker")
	assert.Equal(t, "some data", redacted, "Should return original data for nil tracker")
}

func TestRedactTrackedSecrets_InSlice(t *testing.T) {
	tracker := setupTracker()

	input := []interface{}{
		"safe string 1",
		"another-key-456", // This should be redacted
		12345,
		"safe string 2",
		"postgres://user:s3cr3t_p@ssw0rd@host/db", // This should be redacted
	}

	redacted, wasRedacted := template.RedactTrackedSecrets(input, tracker)
	require.True(t, wasRedacted, "Should report that redaction occurred in the slice")
	require.IsType(t, []interface{}{}, redacted, "Redacted result should still be a slice")

	redactedSlice := redacted.([]interface{})
	require.Len(t, redactedSlice, 5)

	assert.Equal(t, "safe string 1", redactedSlice[0])
	assert.Equal(t, template.RedactedSecretValue, redactedSlice[1], "Secret at index 1 should be redacted")
	assert.Equal(t, 12345, redactedSlice[2], "Non-string value should be unchanged")
	assert.Equal(t, "safe string 2", redactedSlice[3])
	assert.Equal(t, template.RedactedSecretValue, redactedSlice[4], "Connection string at index 4 should be redacted")
}

func TestRedactTrackedSecrets_InMap(t *testing.T) {
	tracker := setupTracker()

	input := map[string]interface{}{
		"key1":      "some safe value",
		"apiKey":    "another-key-456", // This should be redacted
		"port":      8080,
		"nestedMap": map[string]interface{}{
			"connectionString": "user=admin;password=s3cr3t_p@ssw0rd;", // This should be redacted
		},
	}

	redacted, wasRedacted := template.RedactTrackedSecrets(input, tracker)
	require.True(t, wasRedacted, "Should report that redaction occurred in the map")
	require.IsType(t, map[string]interface{}{}, redacted, "Redacted result should still be a map")

	redactedMap := redacted.(map[string]interface{})

	assert.Equal(t, "some safe value", redactedMap["key1"])
	assert.Equal(t, template.RedactedSecretValue, redactedMap["apiKey"], "Secret at key 'apiKey' should be redacted")
	assert.Equal(t, 8080, redactedMap["port"], "Non-string value should be unchanged")

	nestedRedacted, ok := redactedMap["nestedMap"].(map[string]interface{})
	require.True(t, ok, "Nested map should still exist and be a map")
	assert.Equal(t, template.RedactedSecretValue, nestedRedacted["connectionString"], "Secret in nested map should be redacted")
}

func TestRedactTrackedSecrets_NoRedaction(t *testing.T) {
	tracker := setupTracker()

	input := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
		"key3": []interface{}{"a", "b", 456},
		"key4": map[string]interface{}{
			"nestedKey": "nestedValue",
		},
	}

	redacted, wasRedacted := template.RedactTrackedSecrets(input, tracker)
	assert.False(t, wasRedacted, "Should report no redaction occurred")
	assert.Equal(t, input, redacted, "The data structure should be unchanged")
}