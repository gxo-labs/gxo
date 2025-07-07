package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gxoerrors "github.com/gxo-labs/gxo/pkg/gxo/v1/errors"
	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v3"
)

// SupportedSchemaVersionConstraint defines the SemVer constraint that loaded playbooks
// must satisfy. For a v1 engine, we only accept v1 playbooks.
const SupportedSchemaVersionConstraint = "v1"

// LoadPlaybook reads the specified YAML file bytes, unmarshals into a Playbook struct,
// validates against the embedded JSON schema, checks schema version compatibility,
// performs logical validation, and assigns internal IDs.
func LoadPlaybook(playbookYAML []byte, filePathHint string) (*Playbook, error) {
	if len(playbookYAML) == 0 {
		return nil, gxoerrors.NewConfigError("playbook content cannot be empty", nil)
	}

	// Step 1: Validate against the JSON Schema for basic structure and types.
	if err := ValidateWithSchema(playbookYAML); err != nil {
		return nil, gxoerrors.NewConfigError(fmt.Sprintf("playbook '%s' failed schema validation", filePathHint), err)
	}

	// Step 2: Unmarshal into Go struct using strict decoding to catch unknown fields.
	var playbook Playbook
	if err := yamlUnmarshalStrict(playbookYAML, &playbook); err != nil {
		return nil, gxoerrors.NewConfigError(fmt.Sprintf("failed to parse playbook YAML '%s'", filePathHint), err)
	}
	playbook.FilePath = filePathHint

	// Step 3: Check Schema Version Compatibility.
	if playbook.SchemaVersion == "" {
		return nil, gxoerrors.NewValidationError(fmt.Sprintf("playbook '%s' is missing required 'schemaVersion' field", filePathHint), nil)
	}
	playbookSemVer := playbook.SchemaVersion
	if !strings.HasPrefix(playbookSemVer, "v") {
		playbookSemVer = "v" + playbookSemVer
	}
	if !semver.IsValid(playbookSemVer) {
		return nil, gxoerrors.NewValidationError(fmt.Sprintf("playbook '%s' has invalid 'schemaVersion' format: '%s'", filePathHint, playbook.SchemaVersion), nil)
	}

	// Check if the major version of the playbook schema matches the engine's supported major version.
	if semver.Major(playbookSemVer) != SupportedSchemaVersionConstraint {
		return nil, gxoerrors.NewValidationError(
			fmt.Sprintf("playbook '%s' schemaVersion '%s' is not compatible with engine requirement '%s'",
				filePathHint, playbook.SchemaVersion, SupportedSchemaVersionConstraint),
			nil,
		)
	}

	// Step 4: Perform detailed logical validation on the Go struct.
	validationErrs := ValidatePlaybookStructure(&playbook)
	if len(validationErrs) > 0 {
		// Combine multiple validation errors into a single, clear message.
		var errorMessages []string
		for _, vErr := range validationErrs {
			errorMessages = append(errorMessages, vErr.Error())
		}
		combinedMessage := fmt.Sprintf("playbook '%s' has %d validation error(s):\n- %s",
			filePathHint, len(errorMessages), strings.Join(errorMessages, "\n- "))
		return nil, gxoerrors.NewValidationError(combinedMessage, validationErrs[0])
	}

	// Step 5: Assign internal IDs to tasks after all validation has passed.
	assignInternalTaskIDs(&playbook)

	return &playbook, nil
}

// LoadPlaybookFromFile is a convenience function to read a playbook from disk.
func LoadPlaybookFromFile(filePath string) (*Playbook, error) {
	if filePath == "" {
		return nil, gxoerrors.NewConfigError("playbook file path cannot be empty", nil)
	}
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, gxoerrors.NewConfigError(fmt.Sprintf("failed to get absolute path for '%s'", filePath), err)
	}
	yamlFile, err := os.ReadFile(absPath)
	if err != nil {
		return nil, gxoerrors.NewConfigError(fmt.Sprintf("failed to read playbook file '%s'", absPath), err)
	}
	return LoadPlaybook(yamlFile, absPath)
}

// assignInternalTaskIDs assigns a unique InternalID to each task. This ID is used
// for all internal engine operations, such as DAG construction and state tracking.
// It prefers the user-defined `name` but generates a stable, index-based ID if `name` is absent.
func assignInternalTaskIDs(playbook *Playbook) {
	for i := range playbook.Tasks {
		task := &playbook.Tasks[i]
		if task.Name != "" {
			task.InternalID = task.Name
		} else {
			// Use a prefix that is guaranteed not to clash with user-defined names.
			task.InternalID = fmt.Sprintf("__task_idx_%d", i)
		}
	}
}

// yamlUnmarshalStrict provides stricter YAML unmarshalling by disallowing unknown fields.
// This helps users catch typos or unsupported configuration options in their playbooks early.
func yamlUnmarshalStrict(in []byte, out interface{}) error {
	decoder := yaml.NewDecoder(strings.NewReader(string(in)))
	// This crucial setting makes the parser return an error if the YAML
	// contains fields that are not defined in the target Go struct.
	decoder.KnownFields(true)
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("YAML parsing error: %w", err)
	}
	return nil
}