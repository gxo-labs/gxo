package config

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	gxoerrors "github.com/gxo-labs/gxo/pkg/gxo/v1/errors"
	"github.com/gxo-labs/gxo/internal/template"
)

// Pre-compiled regex for validating identifiers used in 'register', 'loop_var', etc.
var identifierRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// Pre-compiled regex for validating task names. Allows for more readable names than standard identifiers.
var taskNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// ValidatePlaybookStructure performs a comprehensive logical validation of the parsed Playbook struct.
// It checks for cross-field consistency, valid references, and other rules that cannot be
// fully expressed in JSON Schema alone. It returns a slice of all validation errors found.
func ValidatePlaybookStructure(p *Playbook) []error {
	var errs []error

	if len(p.Tasks) == 0 {
		errs = append(errs, gxoerrors.NewValidationError("playbook must contain at least one task in 'tasks' list", nil))
	}

	// Validate global policies at the playbook level.
	if p.ChannelPolicy != nil {
		if p.ChannelPolicy.BufferSize != nil && *p.ChannelPolicy.BufferSize < 0 {
			errs = append(errs, gxoerrors.NewValidationError("global channel_policy buffer_size cannot be negative", nil))
		}
	}
	if p.StatePolicy != nil {
		if p.StatePolicy.AccessMode != "" && p.StatePolicy.AccessMode != StateAccessDeepCopy && p.StatePolicy.AccessMode != StateAccessUnsafeDirectReference {
			errs = append(errs, gxoerrors.NewValidationError(fmt.Sprintf("global state_policy has invalid access_mode: '%s'", p.StatePolicy.AccessMode), nil))
		}
	}

	taskNames := make(map[string]bool)
	registeredVars := make(map[string]string)
	requiredTaskNames := make(map[string]struct{})
	// Create a dummy renderer to access the variable extraction logic.
	// We pass nil for dependencies because they are not needed for parsing variable names.
	dummyRenderer := template.NewGoRenderer(nil, nil, nil)

	for i := range p.Tasks {
		task := &p.Tasks[i] // Use a pointer to the task in the slice
		taskIdx := i
		taskDisplayName := fmt.Sprintf("task %d", taskIdx)
		if task.Name != "" {
			taskDisplayName = fmt.Sprintf("task %d ('%s')", taskIdx, task.Name)
		}

		if task.Name != "" {
			if !taskNameRegex.MatchString(task.Name) {
				errs = append(errs, gxoerrors.NewValidationError(fmt.Sprintf("%s: name contains invalid characters (allowed: alphanumeric, underscore, hyphen)", taskDisplayName), nil))
			}
			if _, exists := taskNames[task.Name]; exists {
				errs = append(errs, gxoerrors.NewValidationError(fmt.Sprintf("%s: duplicate task name found", taskDisplayName), nil))
			}
			taskNames[task.Name] = true
		}

		if task.Type == "" {
			errs = append(errs, gxoerrors.NewValidationError(fmt.Sprintf("%s: 'type' is required", taskDisplayName), nil))
		}

		// Validate task-specific state policy.
		if task.StatePolicy != nil {
			if task.StatePolicy.AccessMode != "" && task.StatePolicy.AccessMode != StateAccessDeepCopy && task.StatePolicy.AccessMode != StateAccessUnsafeDirectReference {
				errs = append(errs, gxoerrors.NewValidationError(fmt.Sprintf("%s: state_policy has invalid access_mode: '%s'", taskDisplayName, task.StatePolicy.AccessMode), nil))
			}
		}

		if task.Register != "" {
			if !identifierRegex.MatchString(task.Register) {
				errs = append(errs, gxoerrors.NewValidationError(fmt.Sprintf("%s: 'register' key '%s' is not a valid identifier", taskDisplayName, task.Register), nil))
			}
			if regTaskName, exists := registeredVars[task.Register]; exists {
				errs = append(errs, gxoerrors.NewValidationError(fmt.Sprintf("%s: 'register' key '%s' is already used by task '%s'", taskDisplayName, task.Register, regTaskName), nil))
			} else {
				registeredVars[task.Register] = task.Name
			}
			if task.Name == "" {
				errs = append(errs, gxoerrors.NewValidationError(fmt.Sprintf("%s: 'name' is required when 'register' is used", taskDisplayName), nil))
			}
		}

		for _, streamInputTarget := range task.StreamInputs {
			if !taskNameRegex.MatchString(streamInputTarget) {
				errs = append(errs, gxoerrors.NewValidationError(fmt.Sprintf("%s: 'stream_inputs' target '%s' contains invalid characters", taskDisplayName, streamInputTarget), nil))
			}
			if task.Name != "" && streamInputTarget == task.Name {
				errs = append(errs, gxoerrors.NewValidationError(fmt.Sprintf("%s: 'stream_inputs' cannot target itself", taskDisplayName), nil))
			}
			requiredTaskNames[streamInputTarget] = struct{}{}
		}

		// Validate loop_control configuration.
		if task.LoopControl != nil {
			if task.LoopControl.LoopVar != "" && !identifierRegex.MatchString(task.LoopControl.LoopVar) {
				errs = append(errs, gxoerrors.NewValidationError(fmt.Sprintf("%s: 'loop_control.loop_var' ('%s') is not a valid identifier", taskDisplayName, task.LoopControl.LoopVar), nil))
			}
			if task.LoopControl.Parallel < 0 {
				errs = append(errs, gxoerrors.NewValidationError(fmt.Sprintf("%s: 'loop_control.parallel' cannot be negative", taskDisplayName), nil))
			}
		}

		// Validate retry configuration.
		if task.Retry != nil {
			if task.Retry.Attempts < 1 {
				errs = append(errs, gxoerrors.NewValidationError(fmt.Sprintf("%s: 'retry.attempts' must be at least 1", taskDisplayName), nil))
			}
			var baseDelay time.Duration
			var delayErr error
			if task.Retry.Delay != "" {
				baseDelay, delayErr = time.ParseDuration(task.Retry.Delay)
				if delayErr != nil {
					errs = append(errs, gxoerrors.NewValidationError(fmt.Sprintf("%s: invalid format for 'retry.delay': %v", taskDisplayName, delayErr), nil))
				} else if baseDelay < 0 {
					errs = append(errs, gxoerrors.NewValidationError(fmt.Sprintf("%s: 'retry.delay' cannot be negative", taskDisplayName), nil))
				}
			}
			if task.Retry.MaxDelay != "" {
				maxDelay, maxDelayErr := time.ParseDuration(task.Retry.MaxDelay)
				if maxDelayErr != nil {
					errs = append(errs, gxoerrors.NewValidationError(fmt.Sprintf("%s: invalid format for 'retry.max_delay': %v", taskDisplayName, maxDelayErr), nil))
				} else if maxDelay > 0 && delayErr == nil && maxDelay < baseDelay {
					errs = append(errs, gxoerrors.NewValidationError(fmt.Sprintf("%s: 'retry.max_delay' (%v) cannot be less than 'retry.delay' (%v)", taskDisplayName, maxDelay, baseDelay), nil))
				}
			}
		}

		if task.Timeout != "" {
			if _, timeoutErr := time.ParseDuration(task.Timeout); timeoutErr != nil {
				errs = append(errs, gxoerrors.NewValidationError(fmt.Sprintf("%s: invalid format for 'timeout': %v", taskDisplayName, timeoutErr), nil))
			}
		}

		// Scan all templated fields for variable references to build dependencies.
		templatesToScan := collectTemplatesToScan(task)
		for _, tmplStr := range templatesToScan {
			vars, extractErr := dummyRenderer.ExtractVariables(tmplStr)
			if extractErr != nil {
				errs = append(errs, gxoerrors.NewValidationError(fmt.Sprintf("%s: error parsing template [%s]: %v", taskDisplayName, tmplStr, extractErr), extractErr))
				continue
			}
			for _, fullVarPath := range vars {
				if strings.HasPrefix(fullVarPath, template.GxoStateKeyPrefix+".tasks.") && strings.HasSuffix(fullVarPath, ".status") {
					parts := strings.Split(fullVarPath, ".")
					if len(parts) == 4 {
						referencedTaskName := parts[2]
						requiredTaskNames[referencedTaskName] = struct{}{}
						if task.Name != "" && referencedTaskName == task.Name {
							errs = append(errs, gxoerrors.NewValidationError(fmt.Sprintf("%s: task cannot depend on its own status via template ('%s')", taskDisplayName, fullVarPath), nil))
						}
					}
				} else if regTaskName, isRegistered := registeredVars[fullVarPath]; isRegistered {
					if task.Name != "" && regTaskName == task.Name {
						errs = append(errs, gxoerrors.NewValidationError(fmt.Sprintf("%s: task cannot depend on its own registered variable via template ('%s')", taskDisplayName, fullVarPath), nil))
					}
				}
			}
		}
	}

	// Final check: ensure all referenced tasks actually exist.
	for reqName := range requiredTaskNames {
		if _, exists := taskNames[reqName]; !exists {
			errs = append(errs, gxoerrors.NewValidationError(fmt.Sprintf("playbook validation failed: task name '%s' is referenced by another task but is not defined", reqName), nil))
		}
	}

	return errs
}

// collectTemplatesToScan gathers all string fields from a task that may contain templates.
func collectTemplatesToScan(task *Task) []string {
	templates := []string{}
	if task.When != "" {
		templates = append(templates, task.When)
	}
	if loopStr, ok := task.Loop.(string); ok && loopStr != "" {
		templates = append(templates, loopStr)
	}
	for _, paramValue := range task.Params {
		if strValue, ok := paramValue.(string); ok {
			if strings.Contains(strValue, "{{") && strings.Contains(strValue, "}}") {
				templates = append(templates, strValue)
			}
		}
	}
	return templates
}