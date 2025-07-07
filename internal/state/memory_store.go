// internal/state/memory_store.go
package state

import (
	"maps"
	"strings"
	"sync"

	"github.com/gxo-labs/gxo/internal/util"
	gxo "github.com/gxo-labs/gxo/pkg/gxo/v1/state"
)

// MemoryStateStore implements the StateStore interface using a standard Go map
// protected by a sync.RWMutex for thread-safety. It provides a basic, volatile
// state storage mechanism suitable for single-process execution or testing.
// A key feature is that all read operations return a deep copy of the data,
// guaranteeing immutability from the caller's perspective.
type MemoryStateStore struct {
	data map[string]interface{}
	mu   sync.RWMutex
}

// NewMemoryStateStore creates and initializes a new, empty MemoryStateStore.
func NewMemoryStateStore() *MemoryStateStore {
	return &MemoryStateStore{
		data: make(map[string]interface{}),
	}
}

// Get retrieves a deep copy of the value associated with the given key.
// It is thread-safe due to the read lock.
// This enforces immutability for the caller, preventing accidental modification
// of the shared state through references to maps or slices.
func (s *MemoryStateStore) Get(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, exists := s.data[key]
	if !exists {
		return nil, false
	}
	// Return a deep copy to enforce immutability.
	return util.DeepCopy(val), true
}

// GetAll returns a deep, nested copy of the entire internal state map.
// It unnflattens keys with dots (e.g., "a.b.c") into a nested map structure
// (e.g., map[a:map[b:map[c:...]]]) suitable for direct use by the template engine.
func (s *MemoryStateStore) GetAll() map[string]interface{} {
	s.mu.RLock()
	// Create a shallow copy of the internal map to work with, minimizing lock time.
	flatData := maps.Clone(s.data)
	s.mu.RUnlock()

	// Perform the unflattening and deep copying outside the lock.
	nestedData := unflattenAndDeepCopy(flatData)
	return nestedData
}

// unflattenAndDeepCopy converts a flat map with dot-notation keys into a nested map.
func unflattenAndDeepCopy(flatData map[string]interface{}) map[string]interface{} {
	nestedMap := make(map[string]interface{})
	if flatData == nil {
		return nestedMap
	}

	for key, value := range flatData {
		parts := strings.Split(key, ".")
		currentMap := nestedMap

		// Traverse or create the nested structure.
		for _, part := range parts[:len(parts)-1] {
			if _, ok := currentMap[part]; !ok {
				// If the path doesn't exist, create a new map.
				currentMap[part] = make(map[string]interface{})
			}
			// Type assert to traverse deeper. If the assertion fails, it indicates
			// a key collision (e.g., trying to create "a.b" when "a" is already a string).
			// We will overwrite in this case, as it's a playbook design issue.
			if nextMap, ok := currentMap[part].(map[string]interface{}); ok {
				currentMap = nextMap
			} else {
				// Overwrite the non-map value with a new map.
				newMap := make(map[string]interface{})
				currentMap[part] = newMap
				currentMap = newMap
			}
		}

		// Set the final value, ensuring it's a deep copy.
		currentMap[parts[len(parts)-1]] = util.DeepCopy(value)
	}
	return nestedMap
}

// Set stores the value associated with the given key, potentially overwriting.
// It is thread-safe due to the write lock.
// If 'value' is a complex type (map/slice), its reference is stored. It will be
// deep-copied upon read via Get or GetAll.
func (s *MemoryStateStore) Set(key string, value interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
	return nil
}

// Delete removes the key and its associated value from the store.
// It is thread-safe due to the write lock.
// Returns ErrKeyNotFound if the key does not exist.
func (s *MemoryStateStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data[key]; !exists {
		return ErrKeyNotFound
	}
	delete(s.data, key)
	return nil
}

// Load replaces the entire internal state map with a shallow copy of the provided data.
// It is thread-safe due to the write lock.
func (s *MemoryStateStore) Load(data map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Use maps.Clone for a clean, efficient shallow copy.
	s.data = maps.Clone(data)
	if s.data == nil {
		s.data = make(map[string]interface{})
	}
	return nil
}

// Close is a no-op for the MemoryStateStore as there are no external resources.
func (s *MemoryStateStore) Close() error {
	return nil
}

// Compile-time checks to ensure MemoryStateStore implements both the internal
// and public state store interfaces.
var _ StateStore = (*MemoryStateStore)(nil)
var _ gxo.Store = (*MemoryStateStore)(nil)