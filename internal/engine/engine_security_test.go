package engine_test

import (
	"context"
	"testing"
	"time"

	"github.com/gxo-labs/gxo/internal/engine"
	"github.com/gxo-labs/gxo/internal/events"
	"github.com/gxo-labs/gxo/internal/logger"
	"github.com/gxo-labs/gxo/internal/state"
	intTracing "github.com/gxo-labs/gxo/internal/tracing"

	gxo "github.com/gxo-labs/gxo/pkg/gxo/v1"
	pkgsecrets "github.com/gxo-labs/gxo/pkg/gxo/v1/secrets"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockSecretsProvider is a simple, in-memory secrets provider for testing.
type MockSecretsProvider struct {
	secrets map[string]string
}

// NewMockSecretsProvider creates a new mock secrets provider.
func NewMockSecretsProvider() *MockSecretsProvider {
	return &MockSecretsProvider{
		secrets: make(map[string]string),
	}
}

// AddSecret adds a secret to the mock provider.
func (p *MockSecretsProvider) AddSecret(key, value string) {
	p.secrets[key] = value
}

// GetSecret implements the secrets.Provider interface for the mock.
func (p *MockSecretsProvider) GetSecret(_ context.Context, key string) (string, bool, error) {
	value, found := p.secrets[key]
	return value, found, nil
}

// Ensure it implements the interface.
var _ pkgsecrets.Provider = (*MockSecretsProvider)(nil)

// setupSecurityTestEngine creates a GXO engine instance specifically configured for security tests.
// It uses a mock secrets provider to inject known secret values.
func setupSecurityTestEngine(t *testing.T) (*engine.Engine, *state.MemoryStateStore, *MockSecretsProvider) {
	t.Helper()

	log := logger.NewLogger("debug", "text", nil)
	stateStore := state.NewMemoryStateStore()
	eventBus := events.NewNoOpEventBus()
	reg := NewInMemoryRegistry()
	require.NoError(t, RegisterTestMockModule(reg))

	mockSecrets := NewMockSecretsProvider()

	noOpTracerProvider, err := intTracing.NewNoOpProvider()
	require.NoError(t, err)

	opts := []gxo.EngineOption{
		gxo.WithStateStore(stateStore),
		gxo.WithSecretsProvider(mockSecrets),
		gxo.WithEventBus(eventBus),
		gxo.WithPluginRegistry(reg),
		gxo.WithTracerProvider(noOpTracerProvider),
	}

	engineInstance, err := engine.NewEngine(log, opts...)
	require.NoError(t, err)

	return engineInstance, stateStore, mockSecrets
}

// TestSecretRedactionOnRegister is an integration test that verifies the full
// "Taint and Redact" lifecycle.
func TestSecretRedactionOnRegister(t *testing.T) {
	engineInstance, stateStore, mockSecrets := setupSecurityTestEngine(t)

	const secretKey = "TEST_API_KEY"
	const secretValue = "my-super-secret-api-key-12345"
	mockSecrets.AddSecret(secretKey, secretValue)

	// The simplified MockModule returns its params map by default.
	// The invalid "_mock_register" key has been removed.
	playbookYAML := `
schemaVersion: "v1.0.0"
name: secret_redaction_test
tasks:
  - name: task_using_secret
    type: mock
    params:
      connection_string: "postgres://user:{{ secret \"TEST_API_KEY\" }}@host/db"
    register: task_output
`
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	report, err := engineInstance.RunPlaybook(ctx, []byte(playbookYAML))

	require.NoError(t, err, "Playbook should execute successfully")
	require.NotNil(t, report)
	assert.Equal(t, "Completed", report.OverallStatus, "Playbook should complete successfully")

	registeredOutput, found := stateStore.Get("task_output")
	require.True(t, found, "Expected 'task_output' to be registered in the state")

	outputMap, ok := registeredOutput.(map[string]interface{})
	require.True(t, ok, "Registered output should be a map")

	connectionString, ok := outputMap["connection_string"].(string)
	require.True(t, ok, "connection_string field should exist and be a string")

	assert.NotContains(t, connectionString, secretValue, "The raw secret value should not be present in the registered state")
	assert.Contains(t, connectionString, "[REDACTED_SECRET]", "The secret value should be replaced with the redacted placeholder")
}

// TestErrorRedaction ensures that if a secret is part of an error message,
// the final error reported by the engine is properly redacted.
func TestErrorRedaction(t *testing.T) {
	engineInstance, _, mockSecrets := setupSecurityTestEngine(t)
	require.NoError(t, engineInstance.SetRedactedKeywords([]string{"password", "apikey"}))

	const secretKey = "PROD_API_KEY"
	const secretValue = "prod_api_key_xyz987"
	mockSecrets.AddSecret(secretKey, secretValue)

	// The simplified MockModule fails if a "fail_message" param is provided.
	playbookYAML := `
schemaVersion: "v1.0.0"
name: error_redaction_test
tasks:
  - name: task_fails_with_secret
    type: mock
    params:
      fail_message: "Authentication failed for apikey: {{ secret \"PROD_API_KEY\" }}"
`
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := engineInstance.RunPlaybook(ctx, []byte(playbookYAML))

	require.Error(t, err, "Playbook execution should fail")

	finalErrorString := err.Error()

	assert.NotContains(t, finalErrorString, secretValue, "Final error message should not contain the raw secret")
	assert.Contains(t, finalErrorString, "[REDACTED]", "Final error message should contain the redacted placeholder")
	assert.Contains(t, finalErrorString, "Authentication failed for apikey:", "The non-secret part of the error should be visible")
}