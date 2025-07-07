package secrets

import (
	"context"
	"os"

	gxo "github.com/gxo-labs/gxo/pkg/gxo/v1/secrets" // Use pkg interface
)

// EnvProvider implements the secrets Provider interface, retrieving secrets
// from environment variables.
type EnvProvider struct{}

// NewEnvProvider creates a new environment variable secrets provider.
func NewEnvProvider() *EnvProvider {
	return &EnvProvider{}
}

// GetSecret retrieves the value of an environment variable.
// It returns the value and true if the variable is set, otherwise empty string and false.
// Errors are generally not expected unless there's an underlying OS issue (rare).
func (p *EnvProvider) GetSecret(_ context.Context, key string) (string, bool, error) {
	value, found := os.LookupEnv(key)
	return value, found, nil
}

// Ensure EnvProvider implements the interface
var _ gxo.Provider = (*EnvProvider)(nil)