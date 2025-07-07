// modules/stream/join/join_test.go
package join_test

import (
	"context"
	"os"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/gxo-labs/gxo/internal/engine"
	"github.com/gxo-labs/gxo/internal/events"
	"github.com/gxo-labs/gxo/internal/logger"
	"github.com/gxo-labs/gxo/internal/module"
	"github.com/gxo-labs/gxo/internal/secrets"
	"github.com/gxo-labs/gxo/internal/state"
	intTracing "github.com/gxo-labs/gxo/internal/tracing"
	"github.com/gxo-labs/gxo/modules/generate/from_list"
	"github.com/gxo-labs/gxo/modules/stream/join"
	gxo "github.com/gxo-labs/gxo/pkg/gxo/v1"
	"github.com/gxo-labs/gxo/pkg/gxo/v1/plugin"
	pkgstate "github.com/gxo-labs/gxo/pkg/gxo/v1/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCollectorModule is a test utility module that collects all records
// from its input streams and signals completion via a channel.
type mockCollectorModule struct {
	Records  []map[string]interface{}
	mu       sync.Mutex
	DoneChan chan struct{} // Channel to signal completion.
}

// NewMockCollectorModule creates an initialized collector module.
func NewMockCollectorModule() *mockCollectorModule {
	return &mockCollectorModule{
		Records:  make([]map[string]interface{}, 0),
		DoneChan: make(chan struct{}),
	}
}

func (m *mockCollectorModule) Perform(ctx context.Context, params map[string]interface{}, sr pkgstate.StateReader, inputs map[string]<-chan map[string]interface{}, outputs []chan<- map[string]interface{}, errChan chan<- error) (interface{}, error) {
	log := logger.NewDefaultLogger("debug").With("module", "collector")
	defer close(m.DoneChan) // Signal completion when Perform returns.

	log.Debugf("Collector Perform started.")
	var wg sync.WaitGroup
	wg.Add(len(inputs))
	for producerID, input := range inputs {
		log.Debugf("Collector starting goroutine for input from producer: %s", producerID)
		go func(ch <-chan map[string]interface{}) {
			defer wg.Done()
			for record := range ch {
				log.Debugf("Collector received record: %#v", record)
				m.mu.Lock()
				m.Records = append(m.Records, record)
				m.mu.Unlock()
			}
			log.Debugf("Collector finished consuming a channel.")
		}(input)
	}
	log.Debugf("Collector waiting for all input channels to close...")
	wg.Wait()
	log.Debugf("Collector finished. Total records collected: %d", len(m.Records))
	return map[string]interface{}{"records_collected": len(m.Records)}, nil
}

// setupTestEngineWithJoin creates a new engine instance with all necessary
// modules for the join tests registered. It returns the engine and the
// collector instance for result verification.
func setupTestEngineWithJoin(t *testing.T) (*engine.Engine, *mockCollectorModule) {
	t.Helper()
	log := logger.NewLogger("debug", "text", os.Stderr)
	stateStore := state.NewMemoryStateStore()
	secretsProvider := secrets.NewEnvProvider()
	eventBus := events.NewNoOpEventBus()
	reg := module.NewStaticRegistry()

	require.NoError(t, reg.Register("stream:join", join.NewJoinModule))
	require.NoError(t, reg.Register("generate:from_list", fromlist.NewFromListModule))

	collector := NewMockCollectorModule()
	require.NoError(t, reg.Register("collector", func() plugin.Module { return collector }))

	noOpTracerProvider, err := intTracing.NewNoOpProvider()
	require.NoError(t, err)

	opts := []gxo.EngineOption{
		gxo.WithStateStore(stateStore),
		gxo.WithSecretsProvider(secretsProvider),
		gxo.WithEventBus(eventBus),
		gxo.WithPluginRegistry(reg),
		gxo.WithWorkerPoolSize(4),
		gxo.WithTracerProvider(noOpTracerProvider),
		gxo.WithDefaultTimeout(15 * time.Second),
	}

	engineInstance, err := engine.NewEngine(log, opts...)
	require.NoError(t, err)
	return engineInstance, collector
}

// runTestAndAssert runs a playbook and waits for the collector OR the playbook to finish.
func runTestAndAssert(t *testing.T, playbookYAML string, assertionFunc func(*testing.T, *gxo.ExecutionReport, error, *mockCollectorModule)) {
	t.Helper()
	engine, collector := setupTestEngineWithJoin(t)

	var report *gxo.ExecutionReport
	var playbookErr error
	playbookDone := make(chan struct{})

	go func() {
		defer close(playbookDone)
		report, playbookErr = engine.RunPlaybook(context.Background(), []byte(playbookYAML))
	}()

	select {
	case <-collector.DoneChan:
		t.Log("Collector signaled completion.")
		<-playbookDone
	case <-playbookDone:
		t.Log("Playbook finished before collector (expected in failure cases).")
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out waiting for playbook or collector to finish.")
	}

	assertionFunc(t, report, playbookErr, collector)
}

// getFloat is a helper to safely convert an interface{} to float64 for comparisons.
func getFloat(t *testing.T, val interface{}) float64 {
	t.Helper()
	switch v := val.(type) {
	case int:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case float32:
		return float64(v)
	case float64:
		return v
	default:
		t.Fatalf("unexpected numeric type: %T", val)
		return 0
	}
}

func TestInnerJoin(t *testing.T) {
	playbookYAML := `
schemaVersion: v1.0.0
name: inner_join_test
vars:
  users:
    - { user_id: 1, name: "Alice" }
    - { user_id: 2, name: "Bob" }
  orders:
    - { order_id: 101, user_id: 1, item: "Laptop" }
    - { order_id: 102, user_id: 3, item: "Mouse" }
    - { order_id: 103, user_id: 1, item: "Keyboard" }
tasks:
  - name: get_users
    type: generate:from_list
    params: { items: "{{ .users }}" }
  - name: get_orders
    type: generate:from_list
    params: { items: "{{ .orders }}" }
  - name: join_streams
    type: stream:join
    stream_inputs: [get_users, get_orders]
    params:
      join_type: "inner"
      on:
        - { stream: "get_users", role: "build", field: "user_id" }
        - { stream: "get_orders", role: "probe", field: "user_id" }
  - name: collect_results
    type: collector
    stream_inputs: [join_streams]
`
	runTestAndAssert(t, playbookYAML, func(t *testing.T, report *gxo.ExecutionReport, err error, collector *mockCollectorModule) {
		require.NoError(t, err)
		require.NotNil(t, report)
		require.Equal(t, "Completed", report.OverallStatus)
		require.Len(t, collector.Records, 2, "Inner join should produce 2 records")

		sort.Slice(collector.Records, func(i, j int) bool {
			orderI := getFloat(t, collector.Records[i]["order_id"])
			orderJ := getFloat(t, collector.Records[j]["order_id"])
			return orderI < orderJ
		})

		assert.Equal(t, 101.0, getFloat(t, collector.Records[0]["order_id"]))
		assert.Equal(t, "Alice", collector.Records[0]["name"])
		assert.Equal(t, 103.0, getFloat(t, collector.Records[1]["order_id"]))
		assert.Equal(t, "Alice", collector.Records[1]["name"])
	})
}

func TestLeftJoin(t *testing.T) {
	playbookYAML := `
schemaVersion: v1.0.0
name: left_join_test
vars:
  users:
    - { user_id: 1, name: "Alice" }
  orders:
    - { order_id: 101, user_id: 1, item: "Laptop" }
    - { order_id: 102, user_id: 3, item: "Mouse" }
tasks:
  - name: get_users
    type: generate:from_list
    params: { items: "{{ .users }}" }
  - name: get_orders
    type: generate:from_list
    params: { items: "{{ .orders }}" }
  - name: join_streams
    type: stream:join
    stream_inputs: [get_users, get_orders]
    params:
      join_type: "left"
      on:
        - { stream: "get_users", role: "build", field: "user_id" }
        - { stream: "get_orders", role: "probe", field: "user_id" }
      output:
        merge_strategy: "nested"
        nesting_keys: { get_users: "user", get_orders: "order" }
  - name: collect_results
    type: collector
    stream_inputs: [join_streams]
`
	runTestAndAssert(t, playbookYAML, func(t *testing.T, report *gxo.ExecutionReport, err error, collector *mockCollectorModule) {
		require.NoError(t, err)
		require.NotNil(t, report)
		require.Equal(t, "Completed", report.OverallStatus)
		require.Len(t, collector.Records, 2, "Left join should produce 2 records")

		sort.Slice(collector.Records, func(i, j int) bool {
			orderI := collector.Records[i]["order"].(map[string]interface{})
			orderJ := collector.Records[j]["order"].(map[string]interface{})
			return getFloat(t, orderI["order_id"]) < getFloat(t, orderJ["order_id"])
		})

		// Matched record from probe stream
		assert.NotNil(t, collector.Records[0]["user"])
		assert.Equal(t, "Alice", collector.Records[0]["user"].(map[string]interface{})["name"])
		assert.NotNil(t, collector.Records[0]["order"])
		assert.Equal(t, 101.0, getFloat(t, collector.Records[0]["order"].(map[string]interface{})["order_id"]))

		// Unmatched record from probe stream
		assert.Nil(t, collector.Records[1]["user"], "Unmatched probe record should have nil for build side")
		assert.NotNil(t, collector.Records[1]["order"])
		assert.Equal(t, 102.0, getFloat(t, collector.Records[1]["order"].(map[string]interface{})["order_id"]))
	})
}

func TestRightJoin(t *testing.T) {
	playbookYAML := `
schemaVersion: v1.0.0
name: right_join_test
vars:
  users:
    - { user_id: 1, name: "Alice" }
    - { user_id: 2, name: "Bob" }
  orders:
    - { order_id: 101, user_id: 1, item: "Laptop" }
tasks:
  - name: get_users
    type: generate:from_list
    params: { items: "{{ .users }}" }
  - name: get_orders
    type: generate:from_list
    params: { items: "{{ .orders }}" }
  - name: join_streams
    type: stream:join
    stream_inputs: [get_users, get_orders]
    params:
      join_type: "right"
      on:
        - { stream: "get_users", role: "build", field: "user_id" }
        - { stream: "get_orders", role: "probe", field: "user_id" }
      output:
        merge_strategy: "nested"
        nesting_keys: { get_users: "user", get_orders: "order" }
  - name: collect_results
    type: collector
    stream_inputs: [join_streams]
`
	runTestAndAssert(t, playbookYAML, func(t *testing.T, report *gxo.ExecutionReport, err error, collector *mockCollectorModule) {
		require.NoError(t, err)
		require.NotNil(t, report)
		require.Equal(t, "Completed", report.OverallStatus)
		require.Len(t, collector.Records, 2, "Right join should produce 2 records")

		var unmatched, matched map[string]interface{}
		for _, r := range collector.Records {
			if r["order"] == nil {
				unmatched = r
			} else {
				matched = r
			}
		}

		require.NotNil(t, unmatched, "Should have found one unmatched build record (Bob)")
		require.NotNil(t, matched, "Should have found one matched record (Alice)")

		assert.Equal(t, "Bob", unmatched["user"].(map[string]interface{})["name"])
		assert.Equal(t, "Alice", matched["user"].(map[string]interface{})["name"])
		assert.Equal(t, 101.0, getFloat(t, matched["order"].(map[string]interface{})["order_id"]))
	})
}

func TestOuterJoin(t *testing.T) {
	playbookYAML := `
schemaVersion: v1.0.0
name: outer_join_test
vars:
  users:
    - { user_id: 1, name: "Alice" }
    - { user_id: 2, name: "Bob" }
  orders:
    - { order_id: 101, user_id: 1, item: "Laptop" }
    - { order_id: 102, user_id: 3, item: "Mouse" }
tasks:
  - name: get_users
    type: generate:from_list
    params: { items: "{{ .users }}" }
  - name: get_orders
    type: generate:from_list
    params: { items: "{{ .orders }}" }
  - name: join_streams
    type: stream:join
    stream_inputs: [get_users, get_orders]
    params:
      join_type: "outer"
      on:
        - { stream: "get_users", role: "build", field: "user_id" }
        - { stream: "get_orders", role: "probe", field: "user_id" }
      output:
        merge_strategy: "nested"
        nesting_keys: { get_users: "user", get_orders: "order" }
  - name: collect_results
    type: collector
    stream_inputs: [join_streams]
`
	runTestAndAssert(t, playbookYAML, func(t *testing.T, report *gxo.ExecutionReport, err error, collector *mockCollectorModule) {
		require.NoError(t, err)
		require.NotNil(t, report)
		require.Equal(t, "Completed", report.OverallStatus)
		require.Len(t, collector.Records, 3, "Outer join should produce 3 records")

		var matched, unmatchedUser, unmatchedOrder int
		for _, r := range collector.Records {
			user, uOK := r["user"]
			order, oOK := r["order"]

			if uOK && user != nil && oOK && order != nil {
				matched++
			} else if uOK && user != nil {
				unmatchedUser++
			} else if oOK && order != nil {
				unmatchedOrder++
			}
		}

		assert.Equal(t, 1, matched, "Expected 1 fully matched record")
		assert.Equal(t, 1, unmatchedUser, "Expected 1 unmatched user record")
		assert.Equal(t, 1, unmatchedOrder, "Expected 1 unmatched order record")
	})
}

func TestError_MaxBuildRecordsExceeded(t *testing.T) {
	playbookYAML := `
schemaVersion: v1.0.0
name: limit_test
vars:
  users:
    - { user_id: 1 }
    - { user_id: 2 }
    - { user_id: 3 }
  orders:
    - { order_id: 999, user_id: 999 }
tasks:
  - name: get_users
    type: generate:from_list
    params: { items: "{{ .users }}" }
  - name: get_orders
    type: generate:from_list
    params: { items: "{{ .orders }}" }
  - name: join_streams
    type: stream:join
    stream_inputs: [get_users, get_orders]
    params:
      join_type: "inner"
      on:
        - { stream: "get_users", role: "build", field: "user_id" }
        - { stream: "get_orders", role: "probe", field: "user_id" }
      limits:
        max_build_records: 2
  - name: collect_results
    type: collector
    stream_inputs: [join_streams]
`
	runTestAndAssert(t, playbookYAML, func(t *testing.T, report *gxo.ExecutionReport, err error, collector *mockCollectorModule) {
		require.Error(t, err, "Playbook should fail when max_build_records is exceeded")
		assert.Contains(t, err.Error(), "max_build_records of 2 reached")
		require.NotNil(t, report)
		assert.Equal(t, "Failed", report.OverallStatus)
		require.NotNil(t, report.TaskResults["join_streams"])
		assert.Equal(t, "Failed", report.TaskResults["join_streams"].Status)
		assert.Len(t, collector.Records, 0, "Collector should receive no records when join task fails")
	})
}