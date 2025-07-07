package state

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/gxo-labs/gxo/internal/util"
)

// This file provides benchmarks to validate the performance characteristics of
// different state access strategies, as outlined in the "F1 Spec" of the
// GXO design documentation.

// benchmarkResult is a package-level variable to store the result of benchmark
// operations. This prevents the compiler from optimizing away the function call
// being benchmarked, ensuring accurate measurements.
var benchmarkResult interface{}

// createNestedMap creates a sample nested data structure for benchmarking deep copy performance.
func createNestedMap(depth, width int) map[string]interface{} {
	if depth <= 0 {
		return map[string]interface{}{"leaf_key": "leaf_value"}
	}
	m := make(map[string]interface{}, width)
	for i := 0; i < width; i++ {
		key := fmt.Sprintf("key_d%d_w%d", depth, i)
		m[key] = createNestedMap(depth-1, width)
	}
	return m
}

// largeNestedMap is a pre-generated, complex map used as the consistent input
// for all benchmark tests to ensure comparable results.
var largeNestedMap = createNestedMap(4, 10)

// BenchmarkGet_UnsafeDirectReference measures the performance of a direct map lookup
// with no copying. This represents the theoretical performance ceiling and is used
// as the baseline for "unsafe" access.
func BenchmarkGet_UnsafeDirectReference(b *testing.B) {
	store := NewMemoryStateStore()
	store.Set("test_key", largeNestedMap)
	// Access the internal map directly to simulate a true unsafe read.
	internalData := store.data

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Assign to the package-level var inside the loop to ensure usage.
		benchmarkResult = internalData["test_key"]
	}
}

// BenchmarkGet_HybridFastPath measures the performance of the production `DeepCopy`
// algorithm, which is now the default behavior of the store's Get method. This
// benchmark validates the performance of GXO's secure-by-default state access.
func BenchmarkGet_HybridFastPath(b *testing.B) {
	store := NewMemoryStateStore()
	store.Set("test_key", largeNestedMap)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// The store.Get method now implicitly calls util.DeepCopy.
		benchmarkResult, _ = store.Get("test_key")
	}
}

// BenchmarkGet_ReflectionOnly_WithCycleCheck measures the performance of a naive,
// pure reflection-based deep copy algorithm. This serves as a comparison to
// demonstrate the significant performance gains of our hybrid fast-path approach.
// It includes cycle detection to ensure a fair "apples-to-apples" comparison of
// safe copying algorithms.
func BenchmarkGet_ReflectionOnly_WithCycleCheck(b *testing.B) {
	store := NewMemoryStateStore()
	store.Set("test_key", largeNestedMap)
	// Get the raw value to pass to our pure-reflection test function.
	val := store.data["test_key"]

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Directly call the pure reflection-based copy for comparison.
		benchmarkResult = deepCopyReflection_withCycleCheck(val)
	}
}

// deepCopyReflection_withCycleCheck is a standalone, pure reflection-based deep copy
// that includes cycle detection. It is used *only for benchmarking* to provide a fair
// comparison against the main `util.DeepCopy` hybrid algorithm.
func deepCopyReflection_withCycleCheck(src interface{}) interface{} {
	if src == nil {
		return nil
	}
	// Use the same context type as the production algorithm for cycle detection.
	ctx := make(util.CycleDetectionContext)
	return copyRecursiveReflection(reflect.ValueOf(src), ctx)
}

// copyRecursiveReflection is a local, reflection-only copy implementation for benchmarking.
// It deliberately lacks the fast-path `switch` to accurately measure reflection overhead.
func copyRecursiveReflection(original reflect.Value, ctx util.CycleDetectionContext) interface{} {
	if !original.IsValid() {
		return nil
	}

	// Handle pointer-like types for cycle detection.
	switch original.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice:
		if original.IsNil() {
			return nil
		}
		addr := original.Pointer()
		if cpy, exists := ctx[addr]; exists {
			return cpy
		}
	}

	cpy := reflect.New(original.Type()).Elem()

	// Register the new copy in the context before recursing to handle cycles.
	switch original.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice:
		ctx[original.Pointer()] = cpy.Interface()
	}

	switch original.Kind() {
	case reflect.Ptr:
		// Note: The logic inside the cases now uses the recursive helper `copyRecursiveReflection`.
		newPtr := reflect.New(original.Type().Elem())
		// Recursively copy the value the pointer points to.
		copiedElem := copyRecursiveReflection(original.Elem(), ctx)
		if copiedElem != nil {
			newPtr.Elem().Set(reflect.ValueOf(copiedElem))
		}
		return newPtr.Interface()

	case reflect.Interface:
		if original.IsNil() {
			return nil
		}
		// Recursively copy the concrete value held by the interface.
		return copyRecursiveReflection(original.Elem(), ctx)

	case reflect.Slice:
		cpy.Set(reflect.MakeSlice(original.Type(), original.Len(), original.Cap()))
		for i := 0; i < original.Len(); i++ {
			cpy.Index(i).Set(reflect.ValueOf(copyRecursiveReflection(original.Index(i), ctx)))
		}

	case reflect.Map:
		cpy.Set(reflect.MakeMap(original.Type()))
		for _, key := range original.MapKeys() {
			copiedKey := copyRecursiveReflection(key, ctx)
			copiedValue := copyRecursiveReflection(original.MapIndex(key), ctx)
			cpy.SetMapIndex(reflect.ValueOf(copiedKey), reflect.ValueOf(copiedValue))
		}

	default:
		// For primitive types, direct assignment is a safe copy.
		cpy.Set(original)
	}

	return cpy.Interface()
}