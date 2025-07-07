package config

import (
	_ "embed" // Required for //go:embed directive
	"fmt"
	"sync"

	// Import public error types used
	gxoerrors "github.com/gxo-labs/gxo/pkg/gxo/v1/errors"
	// Import schema validation library
	"github.com/xeipuuv/gojsonschema"
	// Import YAML parsing library needed for conversion
	"gopkg.in/yaml.v3"
)

// Embed the schema file content directly into the compiled binary.
// The path is relative to the location of this Go source file.
// Ensure gxo_schema_v1.0.0.json exists in internal/config/
//
//go:embed gxo_schema_v1.0.0.json
var schemaV1Bytes []byte

// Global variables for schema loading and caching.
var (
	// schemaV1Loader holds the schema content loaded from the embedded bytes.
	schemaV1Loader gojsonschema.JSONLoader
	// schemaV1 holds the compiled schema object for efficient validation.
	schemaV1 *gojsonschema.Schema
	// schemaOnce ensures the schema is loaded and compiled only once.
	schemaOnce sync.Once
	// schemaErr stores any error encountered during the one-time schema load.
	schemaErr error
)

// loadSchema ensures the embedded schema is loaded and compiled thread-safely, only once.
// It returns the compiled schema or an error if loading/compiling failed.
func loadSchema() (*gojsonschema.Schema, error) {
	// Execute the loading logic only once, even if called concurrently.
	schemaOnce.Do(func() {
		// Check if the embedding process failed to load the file content.
		if len(schemaV1Bytes) == 0 {
			schemaErr = gxoerrors.NewConfigError("embedded schema 'gxo_schema_v1.0.0.json' is empty or not found (ensure file exists in internal/config/)", nil)
			return
		}
		// Create a loader for the schema validator from the embedded bytes.
		schemaV1Loader = gojsonschema.NewBytesLoader(schemaV1Bytes)
		// Compile the schema using the loader.
		schemaV1, schemaErr = gojsonschema.NewSchema(schemaV1Loader)
		if schemaErr != nil {
			// Wrap compilation errors for better context.
			schemaErr = gxoerrors.NewConfigError("failed to compile embedded schema 'gxo_schema_v1.0.0.json'", schemaErr)
		}
	})
	// Return the cached compiled schema and any error encountered during loading.
	return schemaV1, schemaErr
}

// ValidateWithSchema validates the given YAML document bytes against the embedded GXO v1.0.0 schema.
// It handles YAML-to-JSON conversion required by the validator.
func ValidateWithSchema(documentYAML []byte) error {
	// Load (or retrieve cached) compiled schema.
	schema, err := loadSchema()
	if err != nil {
		// Return error if schema loading itself failed.
		return err
	}

	// The gojsonschema library works with JSON-like data structures (map[string]interface{}, []interface{}, etc.).
	// We need to unmarshal the input YAML into a generic interface{} first.
	var jsonData interface{}
	// Use the standard YAML library to parse the YAML bytes into Go data types.
	// Note: We don't use strict unmarshalling here as we only need the structure
	// for validation, not necessarily conforming to the specific Playbook struct fields yet.
	if err := yaml.Unmarshal(documentYAML, &jsonData); err != nil {
		return gxoerrors.NewConfigError("failed to parse playbook YAML for schema validation", err)
	}

	// Create a document loader for the validator from the parsed Go data structure.
	docLoader := gojsonschema.NewGoLoader(jsonData)

	// Perform the validation against the compiled schema.
	result, err := schema.Validate(docLoader)
	if err != nil {
		// Handle errors during the validation process itself (unlikely).
		return gxoerrors.NewConfigError("schema validation process failed", err)
	}

	// Check if the document is valid according to the schema.
	if !result.Valid() {
		// If invalid, construct a detailed error message listing validation failures.
		errMsg := "Playbook failed JSON schema validation:"
		// Iterate through the validation errors reported by the library.
		for _, desc := range result.Errors() {
			// Try to get a meaningful field path from the error context.
			field := desc.Field()
			if field == "(root)" || field == "" {
				// Use context path for root-level errors or when field is empty.
				field = desc.Context().String()
			}
			// Append the specific validation failure message.
			errMsg += fmt.Sprintf("\n  - Field '%s': %s", field, desc.Description())
		}
		// Return a structured ValidationError.
		return gxoerrors.NewValidationError(errMsg, nil)
	}

	// Document is valid according to the schema.
	return nil
}