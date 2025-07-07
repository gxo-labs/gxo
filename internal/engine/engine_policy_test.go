// internal/engine/engine_policy_test.go
package engine_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gxo-labs/gxo/internal/config"
	gxoerrors "github.com/gxo-labs/gxo/pkg/gxo/v1/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEngine_DefaultStatePolicy verifies that a playbook with no state_policy
// defined runs successfully, implying the safe default ('deep_copy') is used.
func TestEngine_DefaultStatePolicy(t *testing.T) {
	reg := NewInMemoryRegistry()
	require.NoError(t, RegisterTestMockModule(reg))
	engineInstance, _ := setupTestEngine(t, reg)

	playbookYAML := `
schemaVersion: v1.0.0
name: default_state_policy_test
tasks:
  - name: task_a
    type: mock
    params:
      p1: "hello"
`
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	report, err := engineInstance.RunPlaybook(ctx, []byte(playbookYAML))

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, "Completed", report.OverallStatus)
}

// TestEngine_GlobalStatePolicy verifies that a playbook with a global state_policy
// is parsed and executed successfully.
func TestEngine_GlobalStatePolicy(t *testing.T) {
	reg := NewInMemoryRegistry()
	require.NoError(t, RegisterTestMockModule(reg))
	engineInstance, _ := setupTestEngine(t, reg)

	playbookYAML := `
schemaVersion: v1.0.0
name: global_state_policy_test
state_policy:
  access_mode: "unsafe_direct_reference"
tasks:
  - name: task_a
    type: mock
`
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	report, err := engineInstance.RunPlaybook(ctx, []byte(playbookYAML))

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, "Completed", report.OverallStatus)
}

// TestEngine_TaskLevelStatePolicyOverride verifies a task-level policy is applied.
func TestEngine_TaskLevelStatePolicyOverride(t *testing.T) {
	reg := NewInMemoryRegistry()
	require.NoError(t, RegisterTestMockModule(reg))
	engineInstance, _ := setupTestEngine(t, reg)

	playbookYAML := `
schemaVersion: v1.0.0
name: task_level_override_policy_test
state_policy:
  access_mode: "deep_copy"
tasks:
  - name: task_a
    type: mock
    state_policy:
      access_mode: "unsafe_direct_reference"
`
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	report, err := engineInstance.RunPlaybook(ctx, []byte(playbookYAML))

	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, "Completed", report.OverallStatus)
}

// TestEngine_InvalidStatePolicy ensures the validation catches invalid access modes.
func TestEngine_InvalidStatePolicy(t *testing.T) {
	playbookYAML := `
schemaVersion: v1.0.0
name: invalid_policy_test
state_policy:
  access_mode: "invalid_mode"
tasks:
  - name: task_a
    type: mock
`
	_, err := config.LoadPlaybook([]byte(playbookYAML), "invalid_policy_test.yml")
	require.Error(t, err, "Expected an error for invalid state policy")

	var configErr *gxoerrors.ConfigError
	ok := errors.As(err, &configErr)
	require.True(t, ok, "Expected a ConfigError wrapping the validation error")
	assert.Contains(t, configErr.Error(), "state_policy.access_mode must be one of the following")
}

// TestEngine_CycleDetection verifies that a direct dependency cycle is caught
// during the DAG building phase, preventing execution.
func TestEngine_CycleDetection(t *testing.T) {
	reg := NewInMemoryRegistry()
	require.NoError(t, RegisterTestMockModule(reg))
	engineInstance, _ := setupTestEngine(t, reg)

	// This playbook has a direct cycle: task_a depends on task_b's status,
	// and task_b depends on task_a's status. The 'when' clauses are now
	// syntactically correct, allowing the DAG builder to see the cycle.
	playbookYAML := `
schemaVersion: v1.0.0
name: cycle_test
tasks:
  - name: task_a
    type: mock
    when: "{{ eq ._gxo.tasks.task_b.status ` + "`" + `Completed` + "`" + ` }}"

  - name: task_b
    type: mock
    when: "{{ eq ._gxo.tasks.task_a.status ` + "`" + `Completed` + "`" + ` }}"
`
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	_, err := engineInstance.RunPlaybook(ctx, []byte(playbookYAML))

	require.Error(t, err, "Expected an error due to dependency cycle")
	assert.Contains(t, err.Error(), "cycle detected in task dependencies")
}

// TestEngine_FailsOnMissingTemplateVariable tests that a task fails if it
// depends on a variable from a skipped task. This replaces the flawed stall test.
func TestEngine_FailsOnMissingTemplateVariable(t *testing.T) {
	reg := NewInMemoryRegistry()
	require.NoError(t, RegisterTestMockModule(reg))
	engineInstance, _ := setupTestEngine(t, reg)

	// This playbook creates a situation where task_b depends on a variable
	// from task_a, which is skipped. With strict templating, this must fail.
	playbookYAML := `
schemaVersion: v1.0.0
name: missing_variable_failure_test
vars:
  run_task_a: false
tasks:
  - name: task_a_skipped
    type: mock
    when: "{{ eq .run_task_a true }}"
    register: i_will_never_be_set

  - name: task_b_fails
    type: mock
    params:
       some_param: "{{ .i_will_never_be_set }}"
`
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	report, err := engineInstance.RunPlaybook(ctx, []byte(playbookYAML))

	// ASSERT: The playbook must now fail with a fatal error.
	require.Error(t, err, "Expected an error due to failed template rendering")
	require.NotNil(t, report)
	assert.Equal(t, "Failed", report.OverallStatus)
	// ASSERT: The error message must indicate a template execution failure.
	assert.Contains(t, err.Error(), "parameter resolution failed", "Error should be about template resolution")
	assert.Contains(t, err.Error(), "map has no entry for key \"i_will_never_be_set\"", "Error should specify the missing key")

	// ASSERT: Check task statuses.
	require.NotNil(t, report.TaskResults["task_a_skipped"])
	assert.Equal(t, "Skipped", report.TaskResults["task_a_skipped"].Status)
	require.NotNil(t, report.TaskResults["task_b_fails"])
	assert.Equal(t, "Failed", report.TaskResults["task_b_fails"].Status)
}