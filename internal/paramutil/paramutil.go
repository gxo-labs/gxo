package paramutil

import (
	"fmt"
	// Import the public GXO error types
	gxoerrors "github.com/gxo-labs/gxo/pkg/gxo/v1/errors"
	// Import reflect package, needed for Coalesce helper
	"reflect"
)

// GetRequiredString retrieves a required string parameter from the params map.
// It returns the string value and a nil error if the key exists and the value is a string.
// Otherwise, it returns an empty string and a ValidationError.
func GetRequiredString(params map[string]interface{}, key string) (string, error) {
	value, exists := params[key]
	if !exists {
		// Use the public ValidationError type
		return "", gxoerrors.NewValidationError(fmt.Sprintf("missing required parameter '%s'", key), nil)
	}

	strValue, ok := value.(string)
	if !ok {
		// Use the public ValidationError type
		return "", gxoerrors.NewValidationError(fmt.Sprintf("parameter '%s' must be a string, got %T", key, value), nil)
	}

	return strValue, nil
}

// GetOptionalString retrieves an optional string parameter from the params map.
// Returns the value and true if found and correct type, empty string and false if not found,
// or error if the key exists but has the wrong type.
func GetOptionalString(params map[string]interface{}, key string) (string, bool, error) {
	value, exists := params[key]
	if !exists {
		return "", false, nil
	}

	strValue, ok := value.(string)
	if !ok {
		// Use the public ValidationError type
		return "", false, gxoerrors.NewValidationError(fmt.Sprintf("parameter '%s' must be a string, got %T", key, value), nil)
	}

	return strValue, true, nil
}

// GetRequiredSlice retrieves a required slice parameter from the params map.
// It returns the []interface{} value and a nil error if the key exists and the value is a slice.
// Otherwise, it returns nil and a ValidationError.
func GetRequiredSlice(params map[string]interface{}, key string) ([]interface{}, error) {
	value, exists := params[key]
	if !exists {
		return nil, gxoerrors.NewValidationError(fmt.Sprintf("missing required parameter '%s'", key), nil)
	}

	// The YAML decoder unmarshals lists into []interface{}, so we check for that type.
	sliceValue, ok := value.([]interface{})
	if !ok {
		return nil, gxoerrors.NewValidationError(fmt.Sprintf("parameter '%s' must be a list/slice, got %T", key, value), nil)
	}

	return sliceValue, nil
}

// GetOptionalStringSlice retrieves an optional parameter that should be a slice of strings.
// Handles conversion from []interface{} if necessary.
// Returns the slice and true if found and correct type, nil and false if not found,
// or error if the key exists but has the wrong type or contents.
func GetOptionalStringSlice(params map[string]interface{}, key string) ([]string, bool, error) {
	value, exists := params[key]
	if !exists {
		return nil, false, nil
	}

	// Handle if the value is already a []string
	if stringSlice, isStringSlice := value.([]string); isStringSlice {
		return stringSlice, true, nil
	}

	// Handle if the value is []interface{} and attempt conversion
	sliceValue, ok := value.([]interface{})
	if !ok {
		// Use the public ValidationError type
		return nil, false, gxoerrors.NewValidationError(fmt.Sprintf("parameter '%s' must be a list/slice, got %T", key, value), nil)
	}

	// Convert elements from []interface{} to []string
	result := make([]string, 0, len(sliceValue))
	for i, item := range sliceValue {
		strItem, ok := item.(string)
		if !ok {
			// Use the public ValidationError type
			return nil, false, gxoerrors.NewValidationError(fmt.Sprintf("parameter '%s' must be a list/slice of strings, found non-string element at index %d (%T)", key, i, item), nil)
		}
		result = append(result, strItem)
	}

	return result, true, nil
}

// GetOptionalMap retrieves an optional parameter that should be a map[string]interface{}.
// Handles conversion from map[interface{}]interface{} if necessary (common from YAML).
// Returns the map and true if found and correct type, nil and false if not found,
// or error if the key exists but has the wrong type or non-string keys.
func GetOptionalMap(params map[string]interface{}, key string) (map[string]interface{}, bool, error) {
	value, exists := params[key]
	if !exists {
		return nil, false, nil
	}

	// Handle if the value is already map[string]interface{}
	mapValue, ok := value.(map[string]interface{})
	if ok {
		return mapValue, true, nil
	}

	// Handle if the value is map[interface{}]interface{} and attempt conversion
	if genericMap, isGenericMap := value.(map[interface{}]interface{}); isGenericMap {
		convertedMap := make(map[string]interface{}, len(genericMap))
		for k, v := range genericMap {
			strKey, ok := k.(string)
			if !ok {
				// Use the public ValidationError type
				return nil, false, gxoerrors.NewValidationError(fmt.Sprintf("parameter '%s' must be a map with string keys, found key of type %T", key, k), nil)
			}
			convertedMap[strKey] = v
		}
		return convertedMap, true, nil
	}

	// Use the public ValidationError type for other incorrect types
	return nil, false, gxoerrors.NewValidationError(fmt.Sprintf("parameter '%s' must be a map, got %T", key, value), nil)
}

// GetOptionalInt retrieves an optional integer parameter, attempting coercion from compatible types.
// Returns the int value and true if found and coercible, 0 and false if not found,
// or error if the key exists but value type is incompatible or conversion overflows.
func GetOptionalInt(params map[string]interface{}, key string) (int, bool, error) {
	value, exists := params[key]
	if !exists {
		return 0, false, nil
	}

	switch v := value.(type) {
	case int:
		return v, true, nil
	case int8:
		return int(v), true, nil
	case int16:
		return int(v), true, nil
	case int32:
		return int(v), true, nil
	case int64:
		intValue := int(v)
		// Check for overflow on 32-bit systems where int might be smaller than int64.
		if int64(intValue) != v {
			// Use the public ValidationError type
			return 0, false, gxoerrors.NewValidationError(fmt.Sprintf("parameter '%s' value %v overflows standard int type", key, v), nil)
		}
		return intValue, true, nil
	case float32:
		// Allow conversion only if it represents a whole number.
		if v == float32(int(v)) {
			return int(v), true, nil
		}
		// Use the public ValidationError type
		return 0, false, gxoerrors.NewValidationError(fmt.Sprintf("parameter '%s' is a non-integer float (%v), cannot convert to int", key, v), nil)
	case float64:
		// Allow conversion only if it represents a whole number.
		if v == float64(int(v)) {
			return int(v), true, nil
		}
		// Use the public ValidationError type
		return 0, false, gxoerrors.NewValidationError(fmt.Sprintf("parameter '%s' is a non-integer float (%v), cannot convert to int", key, v), nil)
	default:
		// Use the public ValidationError type for incompatible types
		return 0, false, gxoerrors.NewValidationError(fmt.Sprintf("parameter '%s' must be an integer or whole number, got %T", key, value), nil)
	}
}

// GetOptionalBool retrieves an optional boolean parameter.
// Returns the bool value and true if found and correct type, false and false if not found,
// or error if the key exists but value type is not boolean.
func GetOptionalBool(params map[string]interface{}, key string) (bool, bool, error) {
	value, exists := params[key]
	if !exists {
		return false, false, nil
	}

	boolValue, ok := value.(bool)
	if !ok {
		// Use the public ValidationError type
		return false, false, gxoerrors.NewValidationError(fmt.Sprintf("parameter '%s' must be a boolean, got %T", key, value), nil)
	}

	return boolValue, true, nil
}

// CheckRequired validates that all keys in the 'required' list exist in the params map.
// Returns a ValidationError if any required key is missing.
func CheckRequired(params map[string]interface{}, required []string) error {
	for _, key := range required {
		if _, exists := params[key]; !exists {
			// Use the public ValidationError type
			return gxoerrors.NewValidationError(fmt.Sprintf("missing required parameter '%s'", key), nil)
		}
	}
	return nil
}

// CheckAllowed validates that only keys from the 'allowed' list exist in the params map.
// Returns a ValidationError if any unexpected key is found. Skips check if 'allowed' is empty.
func CheckAllowed(params map[string]interface{}, allowed []string) error {
	if len(allowed) == 0 {
		return nil // No restriction if allowed list is empty.
	}

	// Use a map for efficient lookup of allowed keys.
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		allowedSet[key] = struct{}{}
	}

	// Check each key present in the input params.
	for key := range params {
		if _, isAllowed := allowedSet[key]; !isAllowed {
			// Use the public ValidationError type
			return gxoerrors.NewValidationError(fmt.Sprintf("unknown parameter '%s' provided", key), nil)
		}
	}
	return nil
}

// CheckExclusive ensures that at most one key from the 'exclusiveKeys' list exists in the params map.
// Returns a ValidationError if two or more exclusive keys are present simultaneously.
func CheckExclusive(params map[string]interface{}, exclusiveKeys []string) error {
	foundCount := 0
	var firstFoundKey string
	for _, key := range exclusiveKeys {
		if _, exists := params[key]; exists {
			// If we already found one exclusive key, finding another is an error.
			if foundCount > 0 {
				// Use the public ValidationError type
				return gxoerrors.NewValidationError(fmt.Sprintf("parameters '%s' and '%s' are mutually exclusive", firstFoundKey, key), nil)
			}
			foundCount++
			firstFoundKey = key // Remember the first one found for the error message.
		}
	}
	return nil // No conflict found.
}

// Coalesce returns the first non-nil value among the provided arguments.
// It handles typed nils correctly (e.g., nil pointers, interfaces, slices, maps).
// Useful for setting defaults where map lookups or function calls might yield nil.
func Coalesce(values ...interface{}) interface{} {
	for _, v := range values {
		// Check if the value is valid and not a typed nil.
		if v != nil {
			rv := reflect.ValueOf(v)
			// Check if the kind is one that can be nil AND if it is actually nil.
			switch rv.Kind() {
			case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func:
				if rv.IsNil() {
					continue // Skip typed nils.
				}
			}
			// If it's not nil and not a typed nil, return it.
			return v
		}
	}
	// Return nil if all provided values were nil or typed nils.
	return nil
}