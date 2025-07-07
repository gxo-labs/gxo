package state

import (
	"errors"
)

// ErrKeyNotFound indicates that a requested key does not exist in the state store.
var ErrKeyNotFound = errors.New("key not found in state store")

// StateReader defines the read-only interface for accessing the central state.
// Modules receive an implementation of this interface. Implementations must be
// thread-safe.
//
// IMPORTANT: When retrieving complex types like maps or slices, the caller
// receives a reference to the underlying data in many implementations (like MemoryStore).
// The caller MUST treat this data as immutable to prevent race conditions and
// unexpected side effects.
type StateReader interface {
	// Get retrieves the value associated with the given key.
	// It returns the value (interface{}) and true if the key exists,
	// otherwise it returns nil and false.
	Get(key string) (interface{}, bool)

	// GetAll returns a representation of the entire state map.
	// Callers should be mindful of the potential size of the state.
	// Implementations should clarify whether this is a copy or a reference,
	// but callers MUST treat the result as read-only, especially nested complex types.
	GetAll() map[string]interface{}
}

// Store defines the interface for the underlying storage mechanism
// used by the state manager. Implementations must be thread-safe.
type Store interface {
	StateReader // Embed the read-only interface

	// Set stores the value associated with the given key, potentially overwriting
	// an existing value. Returns an error if the operation fails.
	Set(key string, value interface{}) error

	// Delete removes the key and its associated value from the store.
	// It returns ErrKeyNotFound if the key does not exist. Otherwise, returns nil
	// on success or another error if the operation fails.
	Delete(key string) error

	// Load overwrites the current state with the provided map.
	// This is typically used for initializing state (e.g., loading initial vars).
	// Returns an error if the operation fails.
	Load(data map[string]interface{}) error

	// Close releases any resources held by the store (e.g., database connections).
	Close() error
}
