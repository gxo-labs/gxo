package secrets_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/gxo-labs/gxo/internal/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewSecretTracker ensures the constructor returns a non-nil, properly initialized tracker.
func TestNewSecretTracker(t *testing.T) {
	tracker := secrets.NewSecretTracker()
	require.NotNil(t, tracker, "NewSecretTracker should not return nil")
}

// TestAddAndIsTracked verifies the basic functionality of adding a secret and
// checking for its exact presence.
func TestAddAndIsTracked(t *testing.T) {
	tracker := secrets.NewSecretTracker()
	secretValue := "my-secret-password-123"

	// Assert initial state
	assert.False(t, tracker.IsTracked(secretValue), "Tracker should be empty initially")
	assert.False(t, tracker.IsTracked(""), "IsTracked should be false for empty string")

	// Add the secret
	tracker.Add(secretValue)

	// Assert after adding
	assert.True(t, tracker.IsTracked(secretValue), "Tracker should find the exact secret value")
	assert.False(t, tracker.IsTracked("not-the-secret"), "Tracker should not find a different value")
}

// TestContainsTrackedSecret verifies the substring matching capability, which is
// crucial for redacting connection strings and other composite secrets.
func TestContainsTrackedSecret(t *testing.T) {
	tracker := secrets.NewSecretTracker()
	secretValue := "s3cr3t_t0k3n"
	tracker.Add(secretValue)

	testCases := []struct {
		name          string
		input         string
		expectFound   bool
		shouldBeEmpty bool
	}{
		{
			name:        "Exact Match",
			input:       "s3cr3t_t0k3n",
			expectFound: true,
		},
		{
			name:        "Contained in Connection String",
			input:       "postgres://user:s3cr3t_t0k3n@host:5432/db",
			expectFound: true,
		},
		{
			name:        "Contained in Authorization Header",
			input:       "Authorization: Bearer s3cr3t_t0k3n",
			expectFound: true,
		},
		{
			name:        "Not Contained",
			input:       "this is a normal string",
			expectFound: false,
		},
		{
			name:        "Partial Match (should not be found)",
			input:       "s3cr3t_t0k",
			expectFound: false,
		},
		{
			name:          "Empty Input String",
			input:         "",
			expectFound:   false,
			shouldBeEmpty: true,
		},
		{
			name:          "Empty Tracker",
			input:         "some value",
			expectFound:   false,
			shouldBeEmpty: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			localTracker := secrets.NewSecretTracker()
			if !tc.shouldBeEmpty {
				localTracker.Add(secretValue)
			}
			assert.Equal(t, tc.expectFound, localTracker.ContainsTrackedSecret(tc.input))
		})
	}
}

// TestAddEmptyAndNil does nothing and does not panic.
func TestAddEmptyAndNil(t *testing.T) {
	tracker := secrets.NewSecretTracker()
	assert.NotPanics(t, func() {
		tracker.Add("")
	}, "Adding an empty string should not panic")
	assert.False(t, tracker.IsTracked(""), "Tracker should not track empty strings")
}

// TestConcurrency validates that the SecretTracker is thread-safe by
// concurrently reading and writing to a shared instance. This test will fail
// if run with the `-race` flag if the RWMutex is not implemented correctly.
func TestConcurrency(t *testing.T) {
	tracker := secrets.NewSecretTracker()
	const numGoroutines = 100
	const numSecretsPerRoutine = 50

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Spawn multiple goroutines that all write and read concurrently.
	for i := 0; i < numGoroutines; i++ {
		go func(routineID int) {
			defer wg.Done()
			for j := 0; j < numSecretsPerRoutine; j++ {
				// Each routine adds a unique secret.
				secretToAdd := fmt.Sprintf("secret_from_routine_%d_item_%d", routineID, j)
				tracker.Add(secretToAdd)

				// Each routine also reads a known secret from another routine.
				// This creates a mix of read and write operations.
				secretToRead := "secret_from_routine_0_item_0"
				if routineID > 0 {
					_ = tracker.ContainsTrackedSecret(secretToRead)
				}
			}
		}(i)
	}

	wg.Wait()

	// Final verification: Check if all secrets were added correctly.
	for i := 0; i < numGoroutines; i++ {
		for j := 0; j < numSecretsPerRoutine; j++ {
			secretToCheck := fmt.Sprintf("secret_from_routine_%d_item_%d", i, j)
			assert.True(t, tracker.IsTracked(secretToCheck), "Secret from routine %d item %d should be tracked", i, j)
		}
	}
}