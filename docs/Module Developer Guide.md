# **GXO Module Developer Guide**

**Document ID:** GXO-DEVGUIDE-MODULES
**Version:** 1.0
**Status:** Canonical Reference

Welcome to the GXO Module Developer Guide! This document is the single source of truth for creating high-quality, performant, and secure modules for the GXO Automation Kernel.

GXO's power is realized through its module ecosystem. By following this guide, you will learn how to build modules that seamlessly integrate with the Kernel's core services—scheduling, state management, and the native streaming data plane—to solve real-world automation challenges.

## Table of Contents

1.  [**Module Philosophy: The GXO Automation Model (GXO-AM)](#1-module-philosophy-the-gxo-automation-model-gxo-am)
2.  [**Anatomy of a GXO Module**](#2-anatomy-of-a-gxo-module)
    *   2.1. The Go Package
    *   2.2. The `Module` Struct
    *   2.3. The `Perform` Method: The Heart of the Module
    *   2.4. The `init()` Registration Function
3.  [**The `Perform` Method: A Deep Dive**](#3-the-perform-method-a-deep-dive)
    *   3.1. `context.Context`: The Lifeline
    *   3.2. `params`: Validating User Input
    *   3.3. `state.StateReader`: Accessing Playbook State
    *   3.4. `inputs`: Consuming Streaming Data
    *   3.5. `outputChans`: Producing Streaming Data
    *   3.6. `errChan`: Reporting Non-Fatal Errors
    *   3.7. Return Values (`summary`, `error`): Results and Fatal Errors
4.  [**Step-by-Step Tutorial: Building a Basic Module (`filesystem:read`)**](#4-step-by-step-tutorial-building-a-basic-module-filesystemread)
    *   Step 1: File Structure
    *   Step 2: The Struct and Factory
    *   Step 3: Implementing `Perform`
    *   Step 4: Registration
    *   Step 5: The Complete Module
5.  [**Advanced Tutorial: Building a Streaming Module (`data:filter`)**](#5-advanced-tutorial-building-a-streaming-module-datafilter)
    *   Step 1: The `Perform` Logic
    *   Step 2: The Fan-In/Fan-Out Pattern
    *   Step 3: Using the `Renderer` for Per-Record Evaluation
    *   Step 4: The Complete Streaming Module
6.  [**Module Development Best Practices (The GXO Way)**](#6-module-development-best-practices-the-gxo-way)
    *   The Golden Rule: A Module Does One Thing Well
    *   Parameter Validation is Non-Negotiable
    *   Always Respect the Context
    *   Handle Errors Gracefully
    *   Honor the State Immutability Contract
    *   Security is Your Responsibility
    *   Write Clear and Structured Logs
7.  [**Testing Your Module**](#7-testing-your-module)
    *   7.1. Unit Testing
    *   7.2. Integration Testing
8.  [**Appendix A: The Module Execution Context**](#appendix-a-the-module-execution-context)
9.  [**Appendix B: `paramutil` Quick Reference**](#appendix-b-paramutil-quick-reference)

---

## **1. Module Philosophy: The GXO Automation Model (GXO-AM)**

Before writing code, it's essential to understand the GXO philosophy. GXO modules are designed to fit into the **GXO Automation Model (GXO-AM)**, a layered architecture inspired by the OSI model.

| GXO Layer | Analogy | Purpose | Your Module's Focus |
| :--- | :--- | :--- | :--- |
| **1 – System** | Bare Metal / Syscalls | Processes, filesystem, and kernel control | Direct interaction with the host OS. |
| **2 – Connection**| Physical/Link | Raw TCP/UDP sockets and listeners | Managing raw network connections. |
| **3 – Protocol** | Transport/Session | Structured protocols (HTTP, SSH) | Implementing protocol-specific logic. |
| **4 – Data Plane**| Presentation (ETL) | Parsing, transformation, aggregation | Manipulating streams of structured data. |
| **5 – Application** | Application | High‑level service clients | Providing a convenient API for a specific service. |
| **6 – Integration**| Ecosystem | Opinionated wrappers for external tools | Creating a seamless "better together" experience. |

When you build a module, think about which layer it belongs to. A module should do one thing well and rely on lower-level modules (or the Kernel itself) for other functions. For example, an L5 `http:request` module is built on the L2 `connection:*` primitives, which are managed by the Kernel.

## **2. Anatomy of a GXO Module**

A GXO module is a self-contained Go package that implements a specific piece of automation logic. It consists of four key parts.

### **2.1. The Go Package**
A module lives in its own package, typically under the `modules/` directory following the GXO-AM structure. For example, the `exec` module is at `modules/exec/exec.go`.

### **2.2. The `Module` Struct**
This is a simple struct that will represent your module. It can hold unexported dependencies if needed, but for most simple modules, it will be empty.

```go
// in modules/my_module/my_module.go
package my_module

// MyModule implements the plugin.Module interface.
type MyModule struct{}
```

### **2.3. The `Perform` Method: The Heart of the Module**
This is where all the logic happens. The GXO Kernel's `WorkloadRunner` will call this method, providing it with everything it needs to execute.

```go
import (
    "context"
    "github.com/gxo-labs/gxo/pkg/gxo/v1/plugin"
    "github.com/gxo-labs/gxo/pkg/gxo/v1/state"
)

func (m *MyModule) Perform(
    ctx context.Context,
    params map[string]interface{},
    execCtx plugin.ExecutionContext,
    inputs map[string]<-chan map[string]interface{},
    outputChans []chan<- map[string]interface{},
    errChan chan<- error,
) (summary interface{}, err error) {
    // Your module logic goes here...
    return "success", nil
}
```
We will explore each of these parameters in detail in the next section.

### **2.4. The `init()` Registration Function**
To make your module available to the GXO Kernel, you must register it in a Go `init()` function within your module's package. This ensures that simply importing your package is enough to make the module available to the engine.

```go
import "github.com/gxo-labs/gxo/internal/module"

func init() {
    // The first argument is the name used in the playbook YAML.
    // The second is the factory function that creates a new instance.
    module.Register("my_module:action", func() plugin.Module { return &MyModule{} })
}
```

## **3. The `Perform` Method: A Deep Dive**

Understanding the `Perform` method's signature is the key to writing effective modules.

`Perform(ctx, params, execCtx, inputs, outputChans, errChan) (summary, error)`

### **3.1. `context.Context`**
The `ctx` is the module's lifeline to the GXO Kernel.

*   **Purpose:** It carries request-scoped data, deadlines, and cancellation signals.
*   **Implementation Guidance:**
    *   **MUST** respect cancellation. Any long-running or blocking operation (network call, long loop, channel read) must be wrapped in a `select` statement that also checks `ctx.Done()`.
    *   **MUST** check for Dry Run mode. Before performing any side effect (writing a file, making an API call), check if you're in Dry Run mode.

```go
// Check for cancellation
select {
case <-time.After(5 * time.Second):
    // continue...
case <-ctx.Done():
    return nil, ctx.Err() // Return the context error immediately
}

// Check for Dry Run mode
if isDryRun := ctx.Value(plugin.DryRunKey{}) == true; isDryRun {
    execCtx.Logger().Infof("[Dry Run] Would have performed action X.")
    return map[string]interface{}{"dry_run": true}, nil
}
```

### **3.2. `params map[string]interface{}`**
This map contains the parameters provided by the user in the `workload.process.params` block of their playbook.

*   **Purpose:** To configure your module's behavior for a specific workload.
*   **Implementation Guidance:**
    *   **MUST** validate all parameters. Never assume a parameter exists or has the correct type.
    *   **MUST** use the `internal/paramutil` package for all validation. This ensures consistent error messages and behavior across the entire GXO ecosystem.
    *   The GXO Kernel renders all string parameters before passing them to `Perform`. Your module receives the final, resolved values.

```go
import "github.com/gxo-labs/gxo/internal/paramutil"

// Get a required string parameter
targetURL, err := paramutil.GetRequiredString(params, "url")
if err != nil {
    return nil, err // Return validation errors directly
}

// Get an optional integer parameter with a default
retries, _, _ := paramutil.GetOptionalInt(params, "retries")
if retries == 0 {
    retries = 3 // Apply default
}
```

### **3.3. `plugin.ExecutionContext`**
The `execCtx` provides access to Kernel-level services during execution.

*   **Purpose:** To give your module safe, controlled access to the logger, state store, and renderer.
*   **Implementation Guidance:**
    *   **`execCtx.Logger()`**: Use this for all logging. It's a structured logger that is pre-configured with workload context.
    *   **`execCtx.State()`**: A **read-only** interface to the state store. Use it to retrieve playbook variables or the results of other workloads. **You MUST treat any map or slice returned from the state as immutable.**
    *   **`execCtx.Renderer()`**: An instance of the template renderer. Use this if your module needs to evaluate template strings *per-record* in a streaming context (see the `data:filter` example).

```go
// Logging
execCtx.Logger().Debugf("Starting operation...")

// Reading from state
previousResult, found := execCtx.State().Get("some_other_workload_result")
if !found {
    // handle missing state...
}
```

### **3.4. `inputs map[string]<-chan map[string]interface{}`**
This map provides access to all incoming data streams.

*   **Purpose:** To consume records from one or more upstream producer workloads.
*   **Implementation Guidance:**
    *   The `map` key is the **internal ID** of the producer workload, allowing you to distinguish between different input streams.
    *   **You MUST read from every channel in the `inputs` map until it is closed.** Failing to do so will cause the producer workload to block forever, deadlocking the playbook. This is the most common mistake in streaming module development.
    *   Use a `sync.WaitGroup` and a separate goroutine for each input channel to handle fan-in concurrently and safely.

```go
var wg sync.WaitGroup
wg.Add(len(inputs))

for producerID, inputChan := range inputs {
    go func(id string, ch <-chan map[string]interface{}) {
        defer wg.Done()
        for record := range ch {
            // Process the record...
            execCtx.Logger().Debugf("Received record from producer '%s'", id)
        }
    }(producerID, inputChan)
}

wg.Wait() // Block until all input streams are fully drained
```

### **3.5. `outputChans []chan<- map[string]interface{}`**
This slice contains all the outgoing data streams.

*   **Purpose:** To produce records for downstream consumer workloads.
*   **Implementation Guidance:**
    *   If your module is a producer, you **MUST** write each record you generate to **every channel** in the `outputChans` slice. This is how GXO implements fan-out.
    *   Writes to the output channel must respect context cancellation.

```go
for _, record := range myGeneratedRecords {
    for _, out := range outputChans {
        select {
        case out <- record: // Write to the channel
        case <-ctx.Done(): // Abort if cancelled
            return nil, ctx.Err()
        }
    }
}
```
The Kernel will automatically `close()` all output channels after your `Perform` method returns successfully. You do not need to close them yourself.

### **3.6. `errChan chan<- error`**
This is a "side channel" for reporting non-fatal errors.

*   **Purpose:** To report errors that occur while processing a single record in a stream, without failing the entire workload.
*   **Implementation Guidance:**
    *   If you encounter a recoverable error (e.g., a single malformed record), wrap it in `gxoerrors.NewRecordProcessingError` and send it on the `errChan`.
    *   The Kernel will log this error but allow the workload to continue processing other records.
    *   Do not close this channel.

```go
import "github.com/gxo-labs/gxo/pkg/gxo/v1/errors"

for record := range ch {
    if err := processRecord(record); err != nil {
        // Report a non-fatal error
        processingErr := gxoerrors.NewRecordProcessingError("my_module:action", record["id"], err)
        select {
        case errChan <- processingErr:
        case <-ctx.Done():
            return
        }
        continue // Move to the next record
    }
}
```

### **3.7. Return Values (`summary`, `error`)**
These are the final results of your module's execution.

*   **`summary interface{}`:** The value to be stored in the state store if the user specifies the `register` directive. This can be any Go type that can be handled by the state store (structs, maps, slices, primitives). Return `nil` if there is no meaningful summary.
*   **`error`:** A **fatal error**. Returning a non-nil error will cause the workload to enter a `Failed` state and will halt the entire playbook (unless `ignore_errors: true` is set). Return `nil` for successful completion.

## **4. Step-by-Step Tutorial: Building a Basic Module (`filesystem:read`)**

Let's build the `filesystem:read` module. It's a simple, non-streaming module that reads a file's content.

### **Step 1: File Structure**
Create the file: `modules/filesystem/read/read.go`

### **Step 2: The Struct and Factory**

```go
// in modules/filesystem/read/read.go
package read

import (
    "context"
    "os"
    "path/filepath"

    "github.com/gxo-labs/gxo/internal/paramutil"
    "github.com/gxo-labs/gxo/pkg/gxo/v1/plugin"
)

// ReadModule implements the logic for reading a file.
type ReadModule struct{}

// NewReadModule is the factory function for creating new instances.
func NewReadModule() plugin.Module {
    return &ReadModule{}
}
```

### **Step 3: Implementing `Perform`**

```go
func (m *ReadModule) Perform(
    ctx context.Context,
    params map[string]interface{},
    execCtx plugin.ExecutionContext,
    inputs map[string]<-chan map[string]interface{},
    outputChans []chan<- map[string]interface{},
    errChan chan<- error,
) (interface{}, error) {
    // 1. Validate Parameters
    path, err := paramutil.GetRequiredString(params, "path")
    if err != nil {
        return nil, err
    }

    // 2. Handle Dry Run
    if isDryRun := ctx.Value(plugin.DryRunKey{}) == true; isDryRun {
        execCtx.Logger().Infof("[Dry Run] Would read file at path: %s", path)
        // Return a mock summary for dry run
        return map[string]interface{}{"content": "", "dry_run": true}, nil
    }

    // 3. Core Logic
    workspacePath, _ := execCtx.State().Get("_gxo.workspace.path")
    fullPath := filepath.Join(workspacePath.(string), path)

    // Check for path traversal attacks (though workspace logic should prevent this).
    // This is an example of defense-in-depth.
    if !filepath.IsLocal(fullPath) {
         return nil, fmt.Errorf("security violation: path traversal detected in '%s'", path)
    }

    content, err := os.ReadFile(fullPath)
    if err != nil {
        // Return a fatal error if the file can't be read.
        return nil, fmt.Errorf("failed to read file '%s': %w", path, err)
    }

    // 4. Return Summary
    summary := map[string]interface{}{
        "content": string(content),
    }

    return summary, nil
}
```

### **Step 4: Registration**

```go
// in modules/filesystem/read/read.go (continued)
import "github.com/gxo-labs/gxo/internal/module"

func init() {
    module.Register("filesystem:read", NewReadModule)
}
```
*Note: We also need to add a blank import `_ "github.com/gxo-labs/gxo/modules/filesystem/read"` to `cmd/gxo/main.go` to ensure the `init` function runs.*

### **Step 5: The Complete Module**
Putting it all together results in a complete, robust, and secure module file.

## **5. Advanced Tutorial: Building a Streaming Module (`data:filter`)**

Let's build `data:filter`. It consumes a stream, evaluates a condition for each record, and produces a stream of records that match.

### **Step 1: The `Perform` Logic**
The core logic involves iterating over the input, rendering the `condition` template for each record, and forwarding the record if the condition is met.

### **Step 2: The Fan-In/Fan-Out Pattern**
Since `filter` can only logically have one input stream, we will simplify and assume a single input for this example. The fan-out logic, however, is crucial.

### **Step 3: Using the `Renderer` for Per-Record Evaluation**
The `condition` parameter is a template string. It must be rendered against each incoming record. We get the renderer from the `ExecutionContext`.

### **Step 4: The Complete Streaming Module**

```go
// in modules/data/filter/filter.go
package filter

import (
    "context"
    "fmt"
    "strconv"
    "strings"
    "sync"

    "github.com/gxo-labs/gxo/internal/module"
    "github.com/gxo-labs/gxo/internal/paramutil"
    "github.com/gxo-labs/gxo/pkg/gxo/v1/plugin"
)

type FilterModule struct{}

func NewFilterModule() plugin.Module { return &FilterModule{} }

func init() {
    module.Register("data:filter", NewFilterModule)
}

func (m *FilterModule) Perform(
    ctx context.Context,
    params map[string]interface{},
    execCtx plugin.ExecutionContext,
    inputs map[string]<-chan map[string]interface{},
    outputChans []chan<- map[string]interface{},
    errChan chan<- error,
) (interface{}, error) {
    condition, err := paramutil.GetRequiredString(params, "condition")
    if err != nil {
        return nil, err
    }

    if len(inputs) != 1 {
        return nil, fmt.Errorf("data:filter expects exactly one stream_input, but got %d", len(inputs))
    }

    var inputChan <-chan map[string]interface{}
    for _, ch := range inputs {
        inputChan = ch // Get the single input channel
    }

    renderer := execCtx.Renderer()

    var recordsIn, recordsOut int64
    var wg sync.WaitGroup
    wg.Add(1)

    go func() {
        defer wg.Done()
        for record := range inputChan {
            recordsIn++
            
            // Render the condition for this specific record.
            resultStr, err := renderer.Render(condition, record)
            if err != nil {
                // Report a non-fatal error for this record.
                errChan <- fmt.Errorf("failed to render condition for record: %w", err)
                continue
            }

            // If condition is "truthy", forward the record.
            if evaluateCondition(resultStr) {
                recordsOut++
                for _, out := range outputChans {
                    select {
                    case out <- record:
                    case <-ctx.Done():
                        return
                    }
                }
            }
        }
    }()

    wg.Wait() // Wait for the processing goroutine to finish.

    summary := map[string]interface{}{
        "records_in":  recordsIn,
        "records_out": recordsOut,
    }
    return summary, nil
}

// evaluateCondition checks if a string is "truthy".
func evaluateCondition(s string) bool {
    sLower := strings.ToLower(strings.TrimSpace(s))
    switch sLower {
    case "false", "no", "off", "0", "":
        return false
    default:
        // Attempt to parse as a float to handle numeric "0.0"
        if num, err := strconv.ParseFloat(s, 64); err == nil {
            return num != 0
        }
        return true
    }
}
```

## **6. Module Development Best Practices (The GXO Way)**

Adhere to these principles to create high-quality, maintainable, and secure GXO modules.

*   **The Golden Rule: A Module Does One Thing Well**
    Your module should have a single, clear purpose that aligns with one of the GXO-AM layers. Avoid creating monolithic modules that try to do too much.

*   **Parameter Validation is Non-Negotiable**
    *   **MUST** use `internal/paramutil` for all parameter access.
    *   **MUST** check for required parameters and validate the types of optional ones.
    *   **MUST** return clear `ValidationError`s so users can easily fix their playbooks.

*   **Always Respect the Context**
    *   **MUST** check for context cancellation (`ctx.Done()`) in any long-running loop or before/during any blocking I/O operation.
    *   **MUST** honor the `DryRunKey` and avoid all side effects when it is present.

*   **Handle Errors Gracefully**
    *   Distinguish between fatal and non-fatal errors.
    *   Return a non-nil `error` only for unrecoverable, workload-level failures.
    *   Use the `errChan` to report recoverable, record-level errors in streaming modules.

*   **Honor the State Immutability Contract**
    *   **MUST NOT** modify any map or slice retrieved from the `state.StateReader`. Treat all state as read-only. The Kernel's default `deep_copy` policy enforces this, but your module should be written to respect this contract even in `unsafe_direct_reference` mode.

*   **Security is Your Responsibility**
    *   Sanitize all inputs, especially those used to construct shell commands or file paths.
    *   Never log raw secret values. The Kernel's "Taint and Redact" system helps, but your module should not be a source of leaks.
    *   Be aware of potential path traversal vulnerabilities and use `filepath.IsLocal` or similar checks when dealing with user-provided paths.

*   **Write Clear and Structured Logs**
    *   Use the logger from the `ExecutionContext`.
    *   Use `Debugf` for detailed internal logic, `Infof` for significant state changes, and `Warnf`/`Errorf` for problems.

## **7. Testing Your Module**

Thorough testing is a requirement for GXO modules. Your tests should cover both the module's isolated logic (unit tests) and its behavior within the GXO engine (integration tests).

### **7.1. Unit Testing**
Unit tests should focus on the `Perform` method in isolation.

*   **Dependencies:** Mock all dependencies, including `state.StateReader`, `plugin.ExecutionContext`, and the `Renderer`.
*   **Coverage:** Test all parameter validation paths, the core logic, error conditions, and dry run behavior.
*   **Example (`filesystem:read`):**
    ```go
    // in modules/filesystem/read/read_test.go
    func TestReadPerform(t *testing.T) {
        // ... test setup ...

        // Mock StateReader to provide a workspace path
        mockState := new(MockStateReader)
        mockState.On("Get", "_gxo.workspace.path").Return("/tmp/gxo_ws_123", true)

        // Mock ExecutionContext
        mockExecCtx := new(MockExecutionContext)
        mockExecCtx.On("Logger").Return(testLogger)
        mockExecCtx.On("State").Return(mockState)

        // Test case: successful read
        params := map[string]interface{}{"path": "test.txt"}
        os.WriteFile("/tmp/gxo_ws_123/test.txt", []byte("hello"), 0644)

        mod := NewReadModule()
        summary, err := mod.Perform(context.Background(), params, mockExecCtx, nil, nil, nil)

        assert.NoError(t, err)
        assert.Equal(t, map[string]interface{}{"content": "hello"}, summary)

        // ... more test cases for file not found, dry run, etc. ...
    }
    ```

### **7.2. Integration Testing**
Integration tests execute your module within a real GXO engine instance to verify its interaction with Kernel services.

*   **Playbooks:** Write small, focused test playbooks in the `*.test.gxo.yaml` format.
*   **Assertions:** Use the `test:assert` module to make assertions about the state after your module runs. Use the `test:mock_*` modules to mock external dependencies.
*   **Example (`filesystem:read`):**
    ```yaml
    # in modules/filesystem/read/read.test.gxo.yaml
    workloads:
      # Setup: Create a file to read
      - name: setup_file
        process:
          module: filesystem:write
          params:
            path: "test_to_read.txt"
            content: "hello from integration test"

      # Action: Run the module under test
      - name: run_read_module
        needs: [setup_file]
        process:
          module: filesystem:read
          params:
            path: "test_to_read.txt"
        register: read_result

      # Verification: Assert the result is correct
      - name: verify_result
        needs: [run_read_module]
        process:
          module: test:assert
          params:
            assertions:
              - actual: '{{ .read_result.content }}'
                equal_to: "hello from integration test"
    ```

---

## **Appendix A: The Module Execution Context**
The `plugin.ExecutionContext` is your module's primary interface to the Kernel's services.

| Method | Return Type | Description |
| :--- | :--- | :--- |
| `Logger()` | `log.Logger` | Returns a pre-configured, structured logger with workload context. |
| `State()` | `state.StateReader` | Provides safe, read-only access to the playbook state. |
| `Renderer()`| `template.Renderer` | Provides access to the template engine for runtime rendering. |

## **Appendix B: `paramutil` Quick Reference**

Always use these helpers in your `Perform` method for parameter validation.

*   `paramutil.GetRequiredString(params, "key")`
*   `paramutil.GetOptionalString(params, "key")`
*   `paramutil.GetRequiredSlice(params, "key")`
*   `paramutil.GetOptionalStringSlice(params, "key")`
*   `paramutil.GetOptionalMap(params, "key")`
*   `paramutil.GetOptionalInt(params, "key")`
*   `paramutil.GetOptionalBool(params, "key")`
*   `paramutil.CheckRequired(params, []string{"key1", "key2"})`
*   `paramutil.CheckExclusive(params, []string{"keyA", "keyB"})`