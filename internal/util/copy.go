package util

import "reflect"

// CycleDetectionContext holds state for a single DeepCopy operation to handle cycles.
// It maps the memory address of an original pointer-like object (map, slice, ptr)
// to its new copy. The key is the address obtained via reflect.ValueOf(src).Pointer().
// This type is exported so it can be used in benchmark tests in other packages.
type CycleDetectionContext map[uintptr]interface{}

// DeepCopy creates a deep copy of a given value. It is safe for cyclic data structures.
// It uses a fast path for common types and falls back to a safe but slower
// reflection path for complex or unknown types.
func DeepCopy(src interface{}) interface{} {
	if src == nil {
		return nil
	}
	// The context is created once per top-level DeepCopy call.
	ctx := make(CycleDetectionContext)
	return deepCopyRecursive(src, ctx)
}

// deepCopyRecursive is the internal worker for the deep copy operation.
func deepCopyRecursive(src interface{}, ctx CycleDetectionContext) interface{} {
	if src == nil {
		return nil
	}

	// Use reflection to get the value and handle potential cycles for pointer-like types.
	original := reflect.ValueOf(src)
	kind := original.Kind()

	// Only maps, slices, and pointers can be part of a cycle.
	if kind == reflect.Map || kind == reflect.Slice || kind == reflect.Ptr {
		// Using .Pointer() is a safe and idiomatic way to get a unique identifier
		// for a specific map, slice, or pointer instance for cycle detection.
		addr := original.Pointer()
		if cpy, exists := ctx[addr]; exists {
			// CYCLE DETECTED: We have already started copying this object.
			// Return the (partially complete) copy to break the infinite loop.
			return cpy
		}
	}

	// Fast Path: Use a type switch for the most common data types.
	switch v := src.(type) {
	case map[string]interface{}:
		addr := reflect.ValueOf(v).Pointer()
		// Create a new map and register it in the context *before* recursing.
		cpy := make(map[string]interface{}, len(v))
		ctx[addr] = cpy
		for key, value := range v {
			cpy[key] = deepCopyRecursive(value, ctx)
		}
		return cpy

	case []interface{}:
		addr := reflect.ValueOf(v).Pointer()
		// Create a new slice and register it *before* recursing.
		cpy := make([]interface{}, len(v), cap(v))
		ctx[addr] = cpy
		for i, value := range v {
			cpy[i] = deepCopyRecursive(value, ctx)
		}
		return cpy

	// Primitive types are immutable or copied by value, so we can return them directly.
	case string, int, int64, int32, int16, int8, uint, uint64, uint32, uint16, uint8, float64, float32, bool, complex64, complex128:
		return v

	default:
		// Fallback Path: Use full reflection for any other type. This ensures
		// correctness for complex structs, arrays, or custom types.
		return deepCopyReflection(original, ctx)
	}
}

// deepCopyReflection is the fallback implementation using Go's `reflect` package.
// It is initiated with the context to maintain cycle detection across copy modes.
func deepCopyReflection(original reflect.Value, ctx CycleDetectionContext) interface{} {
	if !original.IsValid() {
		return nil
	}

	// We already checked for cycles on map/slice/ptr in the main function,
	// but we must still register the copy before recursing.
	cpy := reflect.New(original.Type()).Elem()

	switch original.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Struct, reflect.Array:
		if original.CanAddr() {
			// For addressable values, use their address as the key.
			// Note: We are registering the address of the *copy's* value, which is correct
			// for the recursive call's context.
			ctx[original.Addr().Pointer()] = cpy.Addr().Interface()
		}
	}

	// Recursively copy based on the kind of the original value.
	switch original.Kind() {
	case reflect.Ptr:
		if original.IsNil() {
			return nil
		}
		addr := original.Pointer()
		if existingCopy, exists := ctx[addr]; exists {
			return existingCopy
		}
		newPtr := reflect.New(original.Type().Elem())
		ctx[addr] = newPtr.Interface()
		copiedElem := deepCopyRecursive(original.Elem().Interface(), ctx)
		if copiedElem != nil {
			newPtr.Elem().Set(reflect.ValueOf(copiedElem))
		}
		return newPtr.Interface()

	case reflect.Interface:
		if original.IsNil() {
			return nil
		}
		return deepCopyRecursive(original.Elem().Interface(), ctx)

	case reflect.Slice:
		if original.IsNil() {
			return nil
		}
		cpy.Set(reflect.MakeSlice(original.Type(), original.Len(), original.Cap()))
		// Register the new slice copy before iterating to handle nested cycles.
		ctx[original.Pointer()] = cpy.Interface()
		for i := 0; i < original.Len(); i++ {
			cpy.Index(i).Set(reflect.ValueOf(deepCopyRecursive(original.Index(i).Interface(), ctx)))
		}

	case reflect.Map:
		if original.IsNil() {
			return nil
		}
		cpy.Set(reflect.MakeMap(original.Type()))
		// Register the new map copy before iterating.
		ctx[original.Pointer()] = cpy.Interface()
		for _, key := range original.MapKeys() {
			originalValue := original.MapIndex(key)
			copiedValue := deepCopyRecursive(originalValue.Interface(), ctx)
			copiedKey := deepCopyRecursive(key.Interface(), ctx)
			cpy.SetMapIndex(reflect.ValueOf(copiedKey), reflect.ValueOf(copiedValue))
		}

	case reflect.Struct:
		for i := 0; i < original.NumField(); i++ {
			if cpy.Field(i).CanSet() {
				fieldCopy := deepCopyRecursive(original.Field(i).Interface(), ctx)
				if fieldCopy != nil {
					cpy.Field(i).Set(reflect.ValueOf(fieldCopy))
				}
			}
		}

	case reflect.Array:
		for i := 0; i < original.Len(); i++ {
			elemCopy := deepCopyRecursive(original.Index(i).Interface(), ctx)
			if elemCopy != nil {
				cpy.Index(i).Set(reflect.ValueOf(elemCopy))
			}
		}

	default:
		cpy.Set(original)
	}

	return cpy.Interface()
}