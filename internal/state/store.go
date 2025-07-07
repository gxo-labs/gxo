package state

import (
	"errors"
)

// ErrKeyNotFound indicates that a requested key does not exist in the state store.
// Used by StateStore implementations when a key cannot be found for read or delete operations.
var ErrKeyNotFound = errors.New("key not found in state store")

// StateReader defines the read-only interface for accessing the central state.
// This interface is typically provided to components like plugins that only need
// to read state information (variables, registered results, task statuses).
// Implementations must be thread-safe.
//
// IMPORTANT (Immutability): When retrieving complex types like maps or slices,
// the caller receives a reference to the underlying data in many implementations
// (e.g., MemoryStateStore). The caller MUST treat this data as immutable
// to prevent race conditions and unexpected side effects.
type StateReader interface {
	// Get retrieves the value associated with the given key.
	// It returns the value (interface{}) and true if the key exists,
	// otherwise it returns nil and false.
	Get(key string) (interface{}, bool)

	// GetAll returns a representation of the entire current state map.
	// Depending on the implementation, this might be a copy or a reference.
	// Callers MUST treat the returned map and any nested complex types as immutable.
	// Be mindful of potential performance implications for large state sizes.
	GetAll() map[string]interface{}
}

// StateStore defines the full interface for the underlying storage mechanism
// used by the state manager, including read and write operations.
// Implementations are expected to be accessed via a mechanism that ensures thread-safety
// (e.g., the engine's internal locking or a concurrent-safe implementation).
type StateStore interface {
	// StateReader embeds the read-only methods (Get, GetAll).
	StateReader

	// Set stores the value associated with the given key, potentially overwriting
	// an existing value. Returns an error if the storage operation fails.
	Set(key string, value interface{}) error

	// Delete removes the key and its associated value from the store.
	// It returns ErrKeyNotFound if the key does not exist. Otherwise, returns nil
	// on success or another error if the underlying storage operation fails.
	Delete(key string) error

	// Load overwrites the entire current state with the provided data map.
	// This is typically used for initializing state at the beginning of an execution.
	// Returns an error if the loading operation fails.
	Load(data map[string]interface{}) error

	// Close releases any resources held by the state store implementation, such as
	// database connections or file handles. For in-memory stores, this might be a no-op.
	Close() error
}