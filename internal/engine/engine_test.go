package engine_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/gxo-labs/gxo/internal/engine"
	"github.com/gxo-labs/gxo/internal/events"
	"github.com/gxo-labs/gxo/internal/logger"
	intSecrets "github.com/gxo-labs/gxo/internal/secrets"
	"github.com/gxo-labs/gxo/internal/state"
	intTracing "github.com/gxo-labs/gxo/internal/tracing"

	gxo "github.com/gxo-labs/gxo/pkg/gxo/v1"
	gxov1state "github.com/gxo-labs/gxo/pkg/gxo/v1/state"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testTimeout = 7 * time.Second

func setupTestEngine(t *testing.T, mockRegistry *InMemoryRegistry) (*engine.Engine, gxov1state.Store) {
	t.Helper()
	log := logger.NewLogger("debug", "text", os.Stderr)
	stateStore := state.NewMemoryStateStore()
	secretsProvider := intSecrets.NewEnvProvider()
	eventBus := events.NewNoOpEventBus()
	workerPoolSize := 2

	noOpTracerProvider, err := intTracing.NewNoOpProvider()
	require.NoError(t, err, "Failed to create NoOp TracerProvider for test")

	opts := []gxo.EngineOption{
		gxo.WithStateStore(stateStore),
		gxo.WithSecretsProvider(secretsProvider),
		gxo.WithEventBus(eventBus),
		gxo.WithPluginRegistry(mockRegistry),
		gxo.WithWorkerPoolSize(workerPoolSize),
		gxo.WithTracerProvider(noOpTracerProvider),
	}

	engineInstance, err := engine.NewEngine(log, opts...)
	require.NoError(t, err)
	require.NotNil(t, engineInstance)
	return engineInstance, stateStore
}

func TestEngine_RunPlaybook_SingleSuccessfulTask(t *testing.T) {
	reg := NewInMemoryRegistry()
	err := RegisterTestMockModule(reg)
	require.NoError(t, err)
	engineInstance, stateStore := setupTestEngine(t, reg)

	playbookYAML := `
schemaVersion: "v1.0.0"
name: single_success_test
tasks:
  - name: task_a
    type: mock
    params:
      p1: "hello"
    register: task_a_output
`
	playbookBytes := []byte(playbookYAML)

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	report, execErr := engineInstance.RunPlaybook(ctx, playbookBytes)

	assert.NoError(t, execErr, "Playbook should complete without error")
	require.NotNil(t, report)
	assert.Equal(t, "Completed", report.OverallStatus)
	assert.Equal(t, 1, report.TotalTasks)
	assert.Equal(t, 1, report.CompletedTasks)

	registeredVal, found := stateStore.Get("task_a_output")
	assert.True(t, found, "Expected registered variable 'task_a_output' to be found")

	expectedMap := map[string]interface{}{"p1": "hello"}
	assert.Equal(t, expectedMap, registeredVal)

	statusVal, found := stateStore.Get("_gxo.tasks.task_a.status")
	assert.True(t, found, "Expected status for task 'task_a' to be found")
	assert.Equal(t, "Completed", statusVal)
}

func TestEngine_RunPlaybook_SequentialTasksWithStatePassing(t *testing.T) {
	reg := NewInMemoryRegistry()
	err := RegisterTestMockModule(reg)
	require.NoError(t, err)
	engineInstance, stateStore := setupTestEngine(t, reg)

	playbookYAML := `
schemaVersion: "v1.0.0"
name: sequence_state_test
vars:
  initial_var: "from_vars"
tasks:
  - name: task_producer
    type: mock
    params:
      input_val: "{{ .initial_var }}"
    register: produced_data

  - name: task_consumer
    type: mock
    params:
      consumed_val: "{{ .produced_data }}"
    register: consumer_result
`
	playbookBytes := []byte(playbookYAML)

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	report, execErr := engineInstance.RunPlaybook(ctx, playbookBytes)

	assert.NoError(t, execErr, "Playbook should complete without error")
	require.NotNil(t, report)
	assert.Equal(t, "Completed", report.OverallStatus)
	assert.Equal(t, 2, report.TotalTasks)
	assert.Equal(t, 2, report.CompletedTasks)

	statusProd, _ := stateStore.Get("_gxo.tasks.task_producer.status")
	statusCons, _ := stateStore.Get("_gxo.tasks.task_consumer.status")
	assert.Equal(t, "Completed", statusProd)
	assert.Equal(t, "Completed", statusCons)

	finalResult, found := stateStore.Get("consumer_result")
	require.True(t, found, "Expected 'consumer_result' to be registered")

	finalResultMap, ok := finalResult.(map[string]interface{})
	require.True(t, ok, "Final result should be a map")

	consumedVal, ok := finalResultMap["consumed_val"].(map[string]interface{})
	require.True(t, ok, "'consumed_val' should be a map")

	assert.Equal(t, "from_vars", consumedVal["input_val"])
}

func TestEngine_RunPlaybook_TaskFailureHaltsExecution(t *testing.T) {
	reg := NewInMemoryRegistry()
	err := RegisterTestMockModule(reg)
	require.NoError(t, err)
	engineInstance, stateStore := setupTestEngine(t, reg)

	playbookYAML := `
schemaVersion: "v1.0.0"
name: failure_test
tasks:
  - name: task_a
    type: mock
    params:
      p1: "value"
    register: task_a_res

  - name: task_fail
    type: mock
    params:
      fail_message: "Something went wrong"

  - name: task_c
    type: mock
    params:
      dep_check: "{{ ._gxo.tasks.task_fail.status }}"
`
	playbookBytes := []byte(playbookYAML)

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	report, execErr := engineInstance.RunPlaybook(ctx, playbookBytes)

	require.Error(t, execErr, "Expected playbook execution to return an error")
	assert.Contains(t, execErr.Error(), "playbook finished due to fatal error: Something went wrong", "Expected specific fatal error message")
	require.NotNil(t, report)
	assert.Equal(t, "Failed", report.OverallStatus)

	assert.Equal(t, 3, report.TotalTasks)
	assert.Equal(t, 1, report.CompletedTasks)
	assert.Equal(t, 2, report.FailedTasks)
	assert.Equal(t, 0, report.SkippedTasks)

	statusA, foundA := stateStore.Get("_gxo.tasks.task_a.status")
	statusFail, foundFail := stateStore.Get("_gxo.tasks.task_fail.status")
	statusC, foundC := stateStore.Get("_gxo.tasks.task_c.status")

	assert.True(t, foundA)
	assert.Equal(t, "Completed", statusA)
	assert.True(t, foundFail)
	assert.Equal(t, "Failed", statusFail)
	assert.True(t, foundC)
	assert.Equal(t, "Pending", statusC, "Task C should remain Pending as its dependency failed")
}

func TestEngine_RunPlaybook_IgnoredFailureAllowsContinuation(t *testing.T) {
	reg := NewInMemoryRegistry()
	err := RegisterTestMockModule(reg)
	require.NoError(t, err)
	engineInstance, stateStore := setupTestEngine(t, reg)

	playbookYAML := `
schemaVersion: "v1.0.0"
name: ignore_failure_test
tasks:
  - name: task_a
    type: mock

  - name: task_fail_ignored
    type: mock
    params:
      fail_message: "Fail but ignore"
    ignore_errors: true

  - name: task_c
    type: mock
    params:
       dep_check: "{{ ._gxo.tasks.task_fail_ignored.status }}"
  - name: task_d
    type: mock
`
	playbookBytes := []byte(playbookYAML)

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	report, execErr := engineInstance.RunPlaybook(ctx, playbookBytes)

	require.Error(t, execErr, "Playbook should finish with an error indicating failed tasks, even if ignored")
	assert.Contains(t, execErr.Error(), "playbook finished with one or more failed tasks", "Expected generic failure message")
	require.NotNil(t, report)
	assert.Equal(t, "Failed", report.OverallStatus)

	assert.Equal(t, 4, report.TotalTasks)
	assert.Equal(t, 2, report.CompletedTasks)
	assert.Equal(t, 2, report.FailedTasks)
	assert.Equal(t, 0, report.SkippedTasks)

	statusA, _ := stateStore.Get("_gxo.tasks.task_a.status")
	statusFail, _ := stateStore.Get("_gxo.tasks.task_fail_ignored.status")
	statusC, _ := stateStore.Get("_gxo.tasks.task_c.status")
	statusD, _ := stateStore.Get("_gxo.tasks.task_d.status")

	assert.Equal(t, "Completed", statusA)
	assert.Equal(t, "Failed", statusFail, "Ignored task should still have Failed status")
	assert.Equal(t, "Pending", statusC, "Task C should remain Pending")
	assert.Equal(t, "Completed", statusD)
}

func TestEngine_RunPlaybook_WhenConditionSkip(t *testing.T) {
	reg := NewInMemoryRegistry()
	err := RegisterTestMockModule(reg)
	require.NoError(t, err)
	engineInstance, stateStore := setupTestEngine(t, reg)

	playbookYAML := `
schemaVersion: "v1.0.0"
name: when_skip_test
vars:
  should_run: false
tasks:
  - name: task_a
    type: mock

  - name: task_b_skipped
    type: mock
    when: "{{ .should_run }}"

  - name: task_c
    type: mock
    params:
        dep_check: "{{ ._gxo.tasks.task_b_skipped.status }}"
`
	playbookBytes := []byte(playbookYAML)

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	report, execErr := engineInstance.RunPlaybook(ctx, playbookBytes)

	assert.NoError(t, execErr, "Playbook should complete successfully when tasks are skipped")
	require.NotNil(t, report)
	assert.Equal(t, "Completed", report.OverallStatus)

	assert.Equal(t, 3, report.TotalTasks)
	assert.Equal(t, 2, report.CompletedTasks)
	assert.Equal(t, 0, report.FailedTasks)
	assert.Equal(t, 1, report.SkippedTasks)

	statusA, _ := stateStore.Get("_gxo.tasks.task_a.status")
	statusB, _ := stateStore.Get("_gxo.tasks.task_b_skipped.status")
	statusC, _ := stateStore.Get("_gxo.tasks.task_c.status")

	assert.Equal(t, "Completed", statusA)
	assert.Equal(t, "Skipped", statusB, "Task B should have Skipped status")
	assert.Equal(t, "Completed", statusC, "Task C should complete as dependency was Skipped")
}

func TestEngine_RunPlaybook_ContextCancellation(t *testing.T) {
	reg := NewInMemoryRegistry()
	err := RegisterTestMockModule(reg)
	require.NoError(t, err)
	engineInstance, stateStore := setupTestEngine(t, reg)

	playbookYAML := `
schemaVersion: "v1.0.0"
name: cancel_test
tasks:
  - name: task_a_slow
    type: mock
    params:
      _mock_delay: "2s"
    register: task_a_res

  - name: task_b
    type: mock
    params:
      input: "{{ .task_a_res }}"
`
	playbookBytes := []byte(playbookYAML)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	report, execErr := engineInstance.RunPlaybook(ctx, playbookBytes)

	require.Error(t, execErr, "Expected playbook execution to return an error due to cancellation")
	assert.True(t, errors.Is(execErr, context.DeadlineExceeded) || errors.Is(execErr, context.Canceled),
		"Expected context deadline exceeded or canceled error, got: %v", execErr)
	require.NotNil(t, report)
	assert.Equal(t, "Failed", report.OverallStatus)

	statusA, foundA := stateStore.Get("_gxo.tasks.task_a_slow.status")
	statusB, foundB := stateStore.Get("_gxo.tasks.task_b.status")

	assert.True(t, foundA)
	assert.Equal(t, "Failed", statusA, "Task A should be Failed due to timeout/cancellation")
	assert.True(t, foundB)
	assert.Equal(t, "Pending", statusB, "Task B should remain Pending")
}

func TestEngine_RunPlaybook_StallDetection_IgnoreErrorsStableState(t *testing.T) {
	reg := NewInMemoryRegistry()
	err := RegisterTestMockModule(reg)
	require.NoError(t, err)
	engineInstance, stateStore := setupTestEngine(t, reg)

	playbookYAML := `
schemaVersion: "v1.0.0"
name: stall_ignore_error_stable
tasks:
  - name: task_fail_ignored
    type: mock
    params:
      fail_message: "Fail but ignore"
    ignore_errors: true

  - name: task_never_runs
    type: mock
    params:
       dep_check: "{{ ._gxo.tasks.task_fail_ignored.status }}"
`
	playbookBytes := []byte(playbookYAML)

	const testStallCheckInterval = 1 * time.Second
	const testStallCheckTolerance = 5
	ctx, cancel := context.WithTimeout(context.Background(), (testStallCheckTolerance+3)*testStallCheckInterval)
	defer cancel()
	report, execErr := engineInstance.RunPlaybook(ctx, playbookBytes)

	require.Error(t, execErr, "Playbook should finish with an error indicating failed tasks")
	assert.Contains(t, execErr.Error(), "playbook finished with one or more failed tasks")
	assert.NotContains(t, execErr.Error(), "stalled", "Should not report stall for stable state with ignored error")
	assert.False(t, errors.Is(execErr, context.DeadlineExceeded), "Should not time out")
	require.NotNil(t, report)
	assert.Equal(t, "Failed", report.OverallStatus)

	statusFail, _ := stateStore.Get("_gxo.tasks.task_fail_ignored.status")
	statusNever, _ := stateStore.Get("_gxo.tasks.task_never_runs.status")

	assert.Equal(t, "Failed", statusFail)
	assert.Equal(t, "Pending", statusNever, "Dependent task should remain Pending")
}