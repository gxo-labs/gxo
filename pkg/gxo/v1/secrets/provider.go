package secrets

import "context"

// Provider defines the interface for retrieving secrets.
// Implementations could fetch secrets from environment variables, files, Vault, etc.
type Provider interface {
	// GetSecret retrieves the value of a secret identified by the given key.
	// Returns the secret value (as a string for simplicity, though implementations
	// might handle structured secrets internally) and true if found, or an empty
	// string and false if not found. Returns an error if retrieval fails for
	// reasons other than not found (e.g., permissions, backend connection issues).
	// The context can be used for cancellation or passing request-scoped information.
	GetSecret(ctx context.Context, key string) (string, bool, error)
}