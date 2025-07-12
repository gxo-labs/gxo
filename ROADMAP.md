# **GXO Project Roadmap**

**Document ID:** GXO-ROADMAP-V2
**Status:** Approved Strategic Plan

## **1. Introduction**

This document outlines the high-level, strategic development plan for the GXO Automation Kernel. Its purpose is to provide a clear, phased approach to achieving a production-ready, feature-complete GXO v1.0 release. The roadmap is sequenced to ensure architectural integrity, quality, and security are established as a solid foundation before the full suite of modules is implemented.

The scope of this roadmap is the single-node **GXO Automation Kernel**. Advanced, multi-node capabilities, such as the conceptually planned "GXO Fabric," are considered future work that will be explored only after the successful completion of this v1.0 plan.

## **Phase 1: Foundational Refactor - Aligning with the `Workload` Model**

**Rationale:** The current codebase (`v0.1.2a`) is a proven `run_once` task executor. However, the master architecture is built on the more powerful and expressive `Workload`, `Process`, and `Lifecycle` abstractions. This phase is the most critical as it pays down all architectural debt and aligns the entire codebase with the project's core philosophy. It is the non-negotiable prerequisite for implementing the `gxo daemon` and fulfilling the project's vision.

*   **Milestone 1.1: Unified Abstraction Refactor**
    *   **Objective:** Replace the legacy `Task` concept with the formal `Workload`, `Process`, and `Lifecycle` structs throughout the entire codebase, including the configuration, engine, and API layers.
    *   **Outcome:** The engine's internal logic will natively understand and operate on the master architectural concepts, preparing it for future lifecycle policies like `supervise` and `event_driven`.

## **Phase 2: Hardening the Core - Comprehensive Test Suite**

**Rationale:** Before building the long-running `gxo daemon`, we must guarantee that the newly refactored Kernel is correct, stable, and resilient. A crash in an ephemeral `gxo run` is an inconvenience; a crash in the daemon is an outage. This phase establishes a high quality bar for all future development and provides a safety net against regressions.

*   **Milestone 2.1: Full Unit & Integration Test Coverage**
    *   **Objective:** Achieve high test coverage (>90%) for all core Kernel packages (`internal/engine`, `internal/config`, `internal/state`, etc.).
    *   **Outcome:** A suite of tests that validate the DAG builder, the scheduler's concurrency model (including `-race` validation), the policy framework, and the stream synchronization logic, ensuring the Kernel behaves as specified under all conditions.

*   **Milestone 2.2: Fuzz Testing for Security & Robustness**
    *   **Objective:** Implement fuzz tests for critical input-handling components.
    *   **Outcome:** Fuzz tests for the YAML parser and parameter templating engine will proactively discover edge cases and potential security vulnerabilities (e.g., panics, hangs) that are difficult to find with conventional unit tests.

## **Phase 3: Service Enablement & Foundational Security**

**Rationale:** This phase implements the `gxo daemon`, transforming GXO from an ephemeral task runner into a true, long-running Automation Kernel. It focuses on the non-negotiable features required for production deployments: a persistent state store and a secure control plane. Security is built-in from the start, not added on later.

*   **Milestone 3.1: Persistent & Encrypted State Store**
    *   **Objective:** Replace the volatile in-memory state store with a persistent, production-grade alternative.
    *   **Key Features:** Create a `state.Store` implementation using a file-based, transactional embedded database like **BoltDB**. The store will be responsible for persisting active `Workload` configurations and their states. Implement **AEAD (AES-GCM) encryption** for the state file at rest, with the encryption key provided to the daemon via a secure mechanism (e.g., environment variable).

*   **Milestone 3.2: The `gxo daemon` and `supervise` Lifecycle Reconciler**
    *   **Objective:** Implement the core `gxo daemon` process and the first advanced lifecycle, `supervise`.
    *   **Key Features:** Implement the `gxo daemon` command, which starts a gRPC API server for control. Create the `supervise` lifecycle reconciler, including robust **restart-with-exponential-backoff** logic to prevent crash loops.

*   **Milestone 3.3: Control Plane Security (mTLS with Simple Setup)**
    *   **Objective:** Secure the `gxo daemon`'s gRPC control plane with a practical, developer-friendly approach.
    *   **Key Features:** Implement **mandatory mTLS** on the gRPC server. To simplify setup for dev/test environments, the daemon will support an **auto-generation mode** (`gxo daemon --generate-certs`) to create a self-signed CA and server/client certificates. For production, it will support the `pki` model using pre-existing CA-signed certificates. This ensures all communication is encrypted from day one, while avoiding initial setup pain.

*   **Milestone 3.4: The `gxo ctl` Command and Basic RBAC**
    *   **Objective:** Provide the client-side tooling to interact with the secure daemon.
    *   **Key Features:** Implement the `gxo ctl apply` and `gxo ctl remove` commands, which connect to the daemon over mTLS. Implement a basic gRPC interceptor that performs initial Role-Based Access Control (RBAC) by matching the **Subject Common Name (CN)** from the client certificate against an allowlist in the daemon's configuration.

## **Phase 4: Core Module Implementation (Layers 1-4)**

**Rationale:** With the daemon framework established, this phase focuses on implementing the foundational layers of the GXO Standard Library. These low-level modules are the prerequisites for nearly all advanced automation patterns and must exist before higher-level modules that depend on them can be built. This phase respects the GXO-AM's layered dependency model.

*   **Milestone 4.1: Foundational System Primitives (Layer 1)**
    *   **Objective:** Implement the core modules for interacting with the local system and controlling workflow logic.
    *   **Modules:** `exec`, `filesystem:*` suite, `control:*` suite (`assert`, `identity`, `barrier`).

*   **Milestone 4.2: The Network Stack (Layers 2 & 3)**
    *   **Objective:** Enable low-level network and protocol automation, which are the prerequisites for event-driven workflows and the `http:request` module.
    *   **Modules:** `connection:*` suite (`open`, `listen`, `read`, `write`, `close`), `http:listen`, `http:respond`.

*   **Milestone 4.3: Core Data Plane & Module Alignment (Layer 4)**
    *   **Objective:** Implement the essential ETL modules needed to process data from files and API responses, and align existing module names with the canonical GXO-SL specification.
    *   **Modules:** `data:parse` (with `json` and `text_lines` support), `data:map`, `data:filter`.
    *   **Action: Module Renaming**
        *   Rename `generate:from_list` module to `data:generate_from_list`.
        *   Rename `stream:join` module to `data:join`.
        *   Update all internal references, tests, and documentation to reflect these canonical names.

## **Phase 5: Expanding the Standard Library (Layers 5 & 6)**

**Rationale:** With the foundational module layers in place, this phase builds upon them to deliver high-value application and integration modules. This unlocks the most common and powerful use cases for "glue code" replacement and integration with the wider DevOps ecosystem.

*   **Milestone 5.1: REST API Client (Layer 5)**
    *   **Objective:** Implement the universal HTTP client. This is the single most important module for external integration.
    *   **Module:** `http:request` (with full support for methods, headers, bodies, and authentication helpers).

*   **Milestone 5.2: Advanced Data Plane & Application Modules (Layers 4 & 5)**
    *   **Objective:** Enhance ETL capabilities and add clients for common services.
    *   **Modules:** `data:aggregate`, `database:query`.
    *   **Note:** `data:join` was renamed in Phase 4.3; this milestone may involve enhancing its capabilities.

*   **Milestone 5.3: The Integration Layer (Layer 6)**
    *   **Objective:** Provide opinionated, high-level wrappers for key ecosystem tools.
    *   **Modules:** `artifact:*` suite (including `object_storage:*` dependencies), `terraform:run`, `ssh:connect`, `ssh:command`, `ssh:script`.

## **Phase 6: Advanced Workflows & Developer Experience**

**Rationale:** The platform is now highly functional with a rich module library. This phase focuses on delivering advanced, high-level workflow capabilities and the tooling required for users to reliably test their own complex playbooks.

*   **Milestone 6.1: The `event_driven` & `scheduled` Lifecycles**
    *   **Objective:** Implement the remaining advanced lifecycle reconcilers in the daemon.
    *   **Key Features:** Implement the `event_driven` reconciler, which subscribes to `source` workloads. Implement the `scheduled` reconciler for `cron`-based execution.

*   **Milestone 6.2: Human-in-the-Loop (`Resume Context`)**
    *   **Objective:** Implement the `Resume Context` primitive to enable interactive, approval-based workflows.
    *   **Key Features:** Implement the `control:wait_for_signal` module. The daemon will manage unique tokens, persist the state of paused workflows, and the `gxo ctl resume` command will inject data to resume execution.

*   **Milestone 6.3: The Playbook Mocking Framework**
    *   **Objective:** Introduce a first-class, GXO-native testing and validation experience.
    *   **Key Features:**
        *   A `gxo test` command that discovers and executes playbooks matching `*.test.gxo.yaml`, providing structured PASS/FAIL output.
        *   A `test:mock_http_server` module to stand up a temporary HTTP server for testing `http:request` workloads without network access.
        *   The `test:assert` module is now the canonical assertion tool, replacing `control:assert` for testing purposes.

## **Phase 7: Production Hardening & Advanced Security**

**Rationale:** With a feature-complete and testable platform, this final phase implements advanced security controls focused on hardening the workload execution environment and securing the module supply chain, preparing GXO for high-security production deployments.

*   **Milestone 7.1: Workload Sandboxing (`security_context`)**
    *   **Objective:** Implement OS-level sandboxing for workloads as defined in the `security_context` configuration block.
    *   **Key Features:** The `WorkloadRunner` will be enhanced to programmatically create and enter specified Linux namespaces (`mount`, `pid`), apply cgroup resource limits, and apply a restrictive `seccomp` profile before module execution.

*   **Mil.estone 7.2: Module Signing & Verification**
    *   **Objective:** Implement supply chain security by verifying the cryptographic signatures of modules before execution.
    *   **Key Features:** Create tooling to sign module binaries (e.g., via `cosign`). The `gxo daemon` will be configured with trusted public keys and a `fail-closed` policy. The engine will verify module signatures before execution, rejecting any that are invalid or untrusted.

---

# **GXO Master Engineering Plan: Phase 1**

**Document ID:** GXO-ENG-PLAN-P1
**Version:** 3.0
**Date:** 2025-07-12
**Status:** Approved for Execution

## **Phase 1: Foundational Refactor - Aligning with the `Workload` Model**

### **Objective**

This foundational phase refactors the entire codebase to align with the master architecture's core abstractions: the `Workload`, `Process`, and `Lifecycle`. The current `Task` struct is a specific implementation of a `run_once` workload. This refactor makes the engine's core logic lifecycle-aware, a non-negotiable prerequisite for implementing the `gxo daemon` and fulfilling the project's vision.

### **Rationale**

The existing codebase is a stable and proven `run_once` task executor. However, to evolve into a true Automation Kernel capable of managing long-running services and event-driven workflows, the core data models must be elevated. A piecemeal approach of adding new features on top of the old `Task` model would lead to technical debt, inconsistent APIs, and a confusing user experience.

This phase is executed first to pay down all architectural debt upfront. By establishing the `Workload` as the single, unified primitive, we ensure that all future development—from the `gxo daemon` to new modules—is built upon a consistent, robust, and philosophically sound foundation. This is a "measure twice, cut once" approach that prioritizes long-term architectural integrity over short-term feature velocity.

---

### **Milestone 1.1: Unified Abstraction Refactor**

**Objective:** Replace the legacy `Task` concept with the formal `Workload`, `Process`, and `Lifecycle` structs throughout the entire codebase, including the configuration, engine, and API layers.

**Rationale:** This single, sweeping refactor is the most critical step in the project. It aligns the code with the architecture, ensuring all components operate on the same core concepts. It encompasses changes to configuration models, validation, and all internal engine logic. A clean break from the old model is performed to avoid the complexity and overhead of maintaining a backward-compatibility shim for a pre-release product.

---

#### **Part 1: Redefine Core Configuration Model**

**Objective:** Introduce the `Workload`, `Process`, and `Lifecycle` structs into the configuration model, replacing the legacy `Task` concept as the primary declarative unit.

**Rationale:** This is the most critical architectural change. It makes the system's intent explicit by separating *what* a workload does (its `Process`) from *how* it is managed by the kernel (its `Lifecycle`). This establishes the conceptual foundation for all future development, including process supervision and event-driven execution.

**Impacted Files & Detailed Changes:**

*   **New File: `internal/config/policy.go`**
    *   **Action:** Create this new file to centralize all policy-related definitions, improving separation of concerns.
    *   **Implementation Detail:**
        ```go
        package config

        // LifecyclePolicy defines how the GXO kernel manages a workload's execution.
        type LifecyclePolicy struct {
            // Policy is the mandatory execution strategy.
            // Valid values: "run_once", "supervise", "event_driven", "scheduled".
            Policy string `yaml:"policy"`

            // RestartPolicy defines the restart behavior for 'supervise' lifecycles.
            // Valid values: "always", "on_failure", "never". Optional.
            RestartPolicy string `yaml:"restart_policy,omitempty"`

            // Cron defines the schedule for 'scheduled' lifecycles using a standard
            // cron expression. Optional.
            Cron string `yaml:"cron,omitempty"`
            
            // Source defines the name of the workload that produces events for an
            // 'event_driven' lifecycle. Optional.
            Source string `yaml:"source,omitempty"`
        }
        ```

*   **`internal/config/config.go`**
    *   **Action:** Perform a comprehensive refactor of the core data structures. The `Task` struct will be removed and replaced by `Workload` and `Process`. The top-level `Playbook` will be updated to use `workloads` instead of `tasks`.
    *   **Implementation Detail (Before):**
        ```go
        // Playbook represents the top-level structure of a GXO playbook YAML file.
        type Playbook struct {
            Name          string                 `yaml:"name"`
            SchemaVersion string                 `yaml:"schemaVersion"`
            Vars          map[string]interface{} `yaml:"vars,omitempty"`
            Tasks         []Task                 `yaml:"tasks"`
            // ... other policy fields
        }

        // Task represents a single unit of work within a playbook.
        type Task struct {
            Name           string                 `yaml:"name,omitempty"`
            Type           string                 `yaml:"type"`
            Params         map[string]interface{} `yaml:"params,omitempty"`
            // ... other fields like Register, When, Loop, etc.
        }
        ```
    *   **Implementation Detail (After):**
        ```go
        // Process defines the logic of a workload: what it does.
        type Process struct {
            // Module is the name of the registered GXO module to execute. Formerly 'type'.
            Module string                 `yaml:"module"`
            // Params is a map of key-value pairs passed to the module.
            Params map[string]interface{} `yaml:"params,omitempty"`
        }

        // Workload is the atomic unit of automation in GXO. It fuses a Process with a Lifecycle.
        type Workload struct {
            // Name is the user-defined identifier for the workload.
            Name          string                 `yaml:"name"`
            // Lifecycle defines the execution policy for this workload.
            Lifecycle     LifecyclePolicy        `yaml:"lifecycle"`
            // Process defines the logic this workload will execute.
            Process       Process                `yaml:"process"`
            
            // All legacy task-level fields are preserved on the Workload struct.
            Register      string                 `yaml:"register,omitempty"`
            IgnoreErrors  bool                   `yaml:"ignore_errors,omitempty"`
            When          string                 `yaml:"when,omitempty"`       // Behavior for 'run_once'
            Loop          interface{}            `yaml:"loop,omitempty"`       // Behavior for 'run_once'
            LoopControl   *LoopControlConfig     `yaml:"loop_control,omitempty"` // Behavior for 'run_once'
            Retry         *RetryConfig           `yaml:"retry,omitempty"`      // Behavior for 'run_once'
            Timeout       string                 `yaml:"timeout,omitempty"`    // Behavior for 'run_once'
            StatePolicy   *StatePolicy           `yaml:"state_policy,omitempty"`
            // InternalID is a unique identifier assigned by the engine during loading.
            InternalID    string                 `yaml:"-"`
        }

        // Playbook represents the top-level structure of a GXO playbook YAML file.
        type Playbook struct {
            Name          string                 `yaml:"name"`
            SchemaVersion string                 `yaml:"schemaVersion"`
            Vars          map[string]interface{} `yaml:"vars,omitempty"`
            // The 'tasks' key is replaced with 'workloads'.
            Workloads     []Workload             `yaml:"workloads"`
            StatePolicy   *StatePolicy           `yaml:"state_policy,omitempty"`
            // ... other policy fields remain
        }
        ```

---

Of course. Here is the detailed engineering plan for Phase 2, which directly follows the approved roadmap and the completion of Phase 1. This plan focuses on creating a comprehensive and robust test suite for the newly refactored `Workload`-based engine.

---

# **GXO Master Engineering Plan: Phase 2**

**Document ID:** GXO-ENG-PLAN-P2
**Version:** 3.0
**Date:** 2025-07-12
**Status:** Approved for Execution

## **Phase 2: Hardening the Core - Comprehensive Test Suite**

### **Objective**

Before implementing the `gxo daemon` or other new features, establish a comprehensive, production-grade test suite for the newly refactored `v1.0.0` foundation. This phase creates all necessary `_test.go` files, ensuring correctness, concurrency safety, and performance of the existing codebase. It is a dedicated phase to pay down any "testing debt" and establish a high quality bar for all future development.

### **Rationale**

The Phase 1 refactor fundamentally changes the core data structures and logic of the GXO engine. The system as a whole needs a rigorous, end-to-end validation to confirm that all refactored parts integrate correctly. This phase ensures that the engine is not just functional, but also resilient against common issues like race conditions, deadlocks, and invalid user input. Building this test suite now provides a safety net that will catch regressions as we move into more complex feature development.

---

### **Milestone 2.1: Full Unit & Integration Test Coverage**

**Objective:** Achieve high test coverage (>90%) for all core Kernel packages by creating a comprehensive suite of unit and integration tests that validate every component's behavior after the Phase 1 refactor.

**Rationale:** This milestone forms the backbone of the project's quality assurance. Each component of the engine, from the CLI entry point to the deepest parts of the DAG builder and channel manager, must be validated in isolation and in concert with its dependencies. This ensures that the refactored engine is functionally correct and provides a stable base for future development.

**Impacted Files & Detailed Changes:**

*   **New File: `cmd/gxo/main_test.go`**
    *   **Action:** Create a new test file dedicated to testing the `main` package's handler functions, `runExecuteCommand` and `runValidateCommand`.
    *   **Implementation Detail:** These tests will involve mocking system-level functions to isolate the command logic. A common pattern is to replace `os.Exit` with a function that records the exit code in a variable. `stdout` and `stderr` can be captured by redirecting `os.Stdout` and `os.Stderr` to an in-memory buffer (`bytes.Buffer`) during the test.
    *   **Required Test Cases:**
        1.  **`TestRunExecute_Success`**: Provide a valid, simple `run_once` playbook. Verify that the final exit code is `0` and that key success messages are printed to the captured `stdout`.
        2.  **`TestRunExecute_ValidationFailure`**: Provide a playbook with a clear schema error (e.g., a required field is missing). Verify that the exit code is non-zero (e.g., `ExitUsageError` or `ExitFailure`) and that the captured `stderr` contains a clear "validation failed" error message.
        3.  **`TestRunExecute_RuntimeFailure`**: Provide a valid playbook where a workload is designed to fail (e.g., `exec` a non-existent command). Verify the exit code is `1` and `stderr` contains the failure details.
        4.  **`TestRunValidate_Success`**: Test the `gxo validate` command with a valid playbook. Verify exit code `0` and a "validation successful" message.
        5.  **`TestRunValidate_Failure`**: Test `gxo validate` with an invalid playbook. Verify exit code `1` and that `stderr` contains the validation error details.

*   **New File: `internal/command/command_test.go`**
    *   **Action:** Create a new test file to exhaustively test the `defaultRunner.Run` method.
    *   **Implementation Detail:** The current `command.go` uses a `Runner` interface. The tests will not use the `defaultRunner` directly. Instead, they will create a mock `Runner` that allows for precise control over the `exec.Cmd` behavior without actually running system commands. This is crucial for fast, reliable, and platform-independent tests.
    *   **Required Test Cases:**
        1.  **`TestRun_Success`**: Mock a command like `echo "hello"`. Configure the mock to produce specific `stdout`, `stderr`, and an exit code of `0`. Verify that the returned `CommandResult` struct contains exactly this data.
        2.  **`TestRun_NonZeroExit`**: Mock a command that exits with code `12`. Verify that the returned `result.ExitCode` is `12` and that the `Run` function itself returns a `nil` error (as the command execution was successful, even if the command's internal logic failed).
        3.  **`TestRun_ContextCancellation`**: Start a mock command that simulates a long-running process (e.g., `sleep 5`). Create a `context.WithCancel` and cancel it shortly after starting the command. Verify that the `Run` function returns `context.Canceled` as its error.
        4.  **`TestRun_CommandNotFound`**: Mock the behavior of `exec.LookPath` failing. Verify that the `Run` function returns an appropriate `exec.ErrNotFound` error.

*   **New File: `internal/config/validation_test.go`**
    *   **Action:** Create a dedicated test suite for `ValidatePlaybookStructure`.
    *   **Implementation Detail:** Write separate test functions for *each* specific validation rule, providing minimal playbook snippets that trigger the error. This makes tests easy to debug and maintain.
    *   **Required Test Cases:**
        1.  **`TestValidate_DependencyCycle`**: Test with a playbook that has a clear A -> B -> A dependency (e.g., `workload_A` depends on `workload_B` via a `when` clause, and `workload_B` depends on `workload_A`). Assert a cycle error is returned.
        2.  **`TestValidate_SelfReference`**: Test a workload that depends on itself (e.g., `when: "{{ ._gxo.workloads.my_workload.status == 'Completed' }}"`). Assert an error is returned.
        3.  **`TestValidate_InvalidIdentifier`**: Test with invalid names for `register` or `loop_var` that do not match the identifier regex. Assert an error.
        4.  **`TestValidate_BadDurationString`**: Test with an invalid format in a `retry.delay` or `timeout` field (e.g., `"5xyz"`). Assert an error.
        5.  **`TestValidate_UndefinedReference`**: Test a workload that depends on a workload name that does not exist. Assert an error.
        6.  **`TestValidate_InvalidLifecycleCombinations`**: Test that a `restart_policy` is rejected if the `lifecycle.policy` is not `supervise`.

*   **New File: `internal/engine/channel_manager_test.go`**
    *   **Action:** Test the `ChannelManager`'s logic for creating and managing streaming channels and their overflow policies.
    *   **Implementation Detail:** Use hand-crafted `config.Playbook` and `engine.DAG` structs to precisely control the test scenarios without needing to parse full YAML playbooks.
    *   **Required Test Cases:**
        1.  **`TestCreateChannels_FanInFanOut`**: Build a mock DAG with one producer (`P`) and two consumers (`C1`, `C2`) that both list `P` in their `stream_inputs`. Call `CreateChannels` and assert that the internal maps of the `ChannelManager` are wired correctly (e.g., `producerChannels` for `P` has 2 channels, and the `consumerProducerToChannel` maps for `C1` and `C2` each point to their respective channel).
        2.  **`TestManagedChannel_OverflowBlock`**: Create a `managedChannel` with a buffer size of 1 and a "block" policy. Fill the buffer, then start a new goroutine to write to it again. Assert that the goroutine blocks. Then, read from the channel and assert the goroutine unblocks.
        3.  **`TestManagedChannel_OverflowDrop`**: Test the "drop_new" policy. Fill the buffer, then try to write again. Assert that the write call returns a `PolicyViolationError` immediately and does not block.

*   **New File: `internal/engine/dag_test.go`**
    *   **Action:** Isolate and test the `BuildDAG` function.
    *   **Implementation Detail:** Use simple, hand-crafted `config.Playbook` structs as input instead of parsing YAML. This allows for precise testing of the DAG logic itself.
    *   **Required Test Cases:**
        1.  **`TestBuildDAG_StateAndStreamDependencies`**: Create a playbook where Workload A produces a stream for B, and Workload C depends on the registered result of B (`when: '{{ .result_b.status == "Completed" }}'`). Assert that the resulting DAG has the correct A -> B -> C dependency chain.
        2.  **`TestBuildDAG_PolicyResolution`**: Create a playbook with a global `state_policy` and a workload with an overriding `state_policy`. Assert that the final `Node` in the DAG has the correctly merged, workload-specific policy.

*   **Update `engine_test.go`, `engine_policy_test.go`, `engine_security_test.go`**
    *   **Action:** Perform a full review and update of all existing engine-level integration tests.
    *   **Detailed Changes:**
        1.  Change all playbook YAML in these tests to use the new `workloads:`, `process:`, and `lifecycle:` syntax.
        2.  Update assertions that check registered state to look for `_gxo.workloads...` instead of `_gxo.tasks...`.
        3.  Ensure that tests for `when`, `loop`, and `retry` continue to pass with the new `Workload` struct.

---

# **GXO Master Engineering Plan: Phase 2**

**Document ID:** GXO-ENG-PLAN-P2
**Version:** 3.0
**Date:** 2025-07-12
**Status:** Approved for Execution

## **Phase 2: Hardening the Core - Comprehensive Test Suite**

### **Objective**

Before implementing the `gxo daemon` or other new features, establish a comprehensive, production-grade test suite for the newly refactored `v1.0.0` foundation. This phase creates all necessary `_test.go` and `_bench_test.go` files, ensuring correctness, concurrency safety, and performance of the existing codebase. It is a dedicated phase to pay down any "testing debt" and establish a high quality bar for all future development.

### **Rationale**

The Phase 1 refactor fundamentally changes the core data structures and logic of the GXO engine. While individual components may have tests, the system as a whole needs a rigorous, end-to-end validation to confirm that all refactored parts integrate correctly. This phase ensures that the engine is not just functional, but also resilient against common issues like race conditions, deadlocks, and invalid user input. Building this test suite now provides a safety net that will catch regressions as we move into more complex feature development in subsequent phases.

---

### **Milestone 2.1: Full Unit & Integration Test Coverage**

**Objective:** Achieve high test coverage (>90%) for all core Kernel packages, validating the behavior of the refactored engine from the CLI down to the core logic.

**Rationale:** This milestone ensures that every critical component of the `gxo run` command's execution path is validated. By testing the CLI, system abstractions, configuration loading, and engine orchestration, we build a solid, verifiable baseline before adding new features.

**Impacted Files & Detailed Changes:**

*   **New File: `cmd/gxo/main_test.go`**
    *   **Action:** Create a test file for the `main` package's handlers. This requires mocking system-level functions like `os.Exit` and capturing `stdout`/`stderr` to verify CLI behavior without terminating the test process.
    *   **Implementation Detail:** A test helper function will be created to orchestrate this.
        ```go
        // testutil/cli_test_harness.go
        package testutil

        func ExecuteCommand(args []string) (stdout, stderr string, exitCode int) {
            // ... capture stdout/stderr to bytes.Buffer ...
            // ... temporarily replace os.Exit with a function that records the code ...
            
            // This is a simplified view; the actual harness will be more robust.
            main.runExecuteCommand(args) // Assuming `runExecuteCommand` is exported for testing
            
            // ... restore os.Exit, read buffers ...
            return stdoutStr, stderrStr, capturedExitCode
        }
        ```
    *   **Required Test Cases:**
        *   `TestRunExecute_Success`: Provide a valid, simple `run_once` playbook. Verify the exit code is `0` and key success messages are printed to `stdout`.
        *   `TestRunExecute_ValidationFailure`: Provide a playbook with a schema error. Verify the exit code is non-zero and `stderr` contains a clear "validation failed" error.
        *   `TestRunExecute_RuntimeFailure`: Provide a valid playbook where a workload is designed to fail. Verify the exit code is non-zero and `stderr` contains the failure details.
        *   `TestRunValidate_Success`: Test `gxo validate` with a valid playbook. Verify exit code `0`.
        *   `TestRunValidate_Failure`: Test `gxo validate` with an invalid playbook. Verify a non-zero exit code.

*   **New File: `internal/command/command_test.go`**
    *   **Action:** Exhaustively test the `defaultRunner.Run` method.
    *   **Rationale:** The `exec` module depends entirely on this component. Its correctness, especially regarding context cancellation and error handling, is critical.
    *   **Required Test Cases:**
        *   `TestRun_Success`: Mock a command like `echo "hello"`. Verify the returned `CommandResult` struct contains the correct `stdout`, `stderr`, and an exit code of `0`.
        *   `TestRun_NonZeroExit`: Mock a command that exits with code `12`. Verify `result.ExitCode` is `12` and the function itself returns a `nil` error (as the command *ran* successfully).
        *   `TestRun_ContextCancellation`: Start a mock command that simulates a long-running process (`sleep 5`). Create a `context.WithCancel` and cancel it shortly after starting. Verify the `Run` function returns `context.Canceled`.
        *   `TestRun_CommandNotFound`: Mock the behavior of `exec.LookPath` failing. Verify `Run` returns an `exec.ErrNotFound` error.

*   **New File: `internal/config/validation_test.go`**
    *   **Action:** Create a dedicated test suite for the refactored `ValidatePlaybookStructure`.
    *   **Rationale:** Validating the validator is essential. These tests ensure our static checks are catching common playbook errors correctly.
    *   **Required Test Cases (one function per rule):**
        *   `TestValidate_DependencyCycle`: Test a playbook with an A -> B -> A dependency. Assert a cycle error is returned.
        *   `TestValidate_SelfReference`: Test a workload depending on its own status. Assert an error.
        *   `TestValidate_InvalidIdentifier`: Test with invalid names for `register` or `loop_var` (e.g., "invalid-name"). Assert an error.
        *   `TestValidate_UndefinedReference`: Test a workload that depends on a non-existent workload name. Assert an error.
        *   `TestValidate_InvalidLifecycleForRun`: Test a workload with `lifecycle: { policy: supervise }`. Assert `ValidatePlaybookStructure` returns an error stating this is not supported by `gxo run`.

*   **New File: `internal/engine/channel_manager_test.go`**
    *   **Action:** Test the `ChannelManager`'s logic for creating and managing streaming channels and their synchronization `WaitGroup`s.
    *   **Rationale:** This component is the heart of GXO's streaming data plane. Incorrect `WaitGroup` handling would lead to deadlocks or race conditions.
    *   **Required Test Cases:**
        *   `TestCreateChannels_FanInFanOut`: Build a mock DAG with a fan-out producer (one output) and a fan-in consumer (multiple inputs). Call `CreateChannels` and assert the internal maps of the `ChannelManager` are wired correctly (correct number of channels, correct producer/consumer relationships, correct `WaitGroup` counts).
        *   `TestManagedChannel_OverflowBlock`: Create a `managedChannel` with a buffer size of 1 and a "block" policy. Fill the buffer, then start a new goroutine to write to it again. Assert the goroutine blocks. Then, read from the channel and assert the goroutine unblocks.
        *   `TestManagedChannel_OverflowDrop`: Test the "drop_new" policy. Fill the buffer, then try to write again. Assert the write call returns a `PolicyViolationError` immediately and does not block.

*   **New File: `internal/engine/dag_test.go`**
    *   **Action:** Isolate and test the `BuildDAG` function using hand-crafted `config.Playbook` structs as input instead of parsing YAML.
    *   **Rationale:** This allows for precise testing of the DAG logic itself, separate from configuration loading.
    *   **Required Test Cases:**
        *   `TestBuildDAG_StateAndStreamDependencies`: Create a playbook where Workload A produces a stream for B, and Workload C depends on the registered result of B. Assert the resulting DAG has the correct A -> B and B -> C dependency edges.
        *   `TestBuildDAG_PolicyResolution`: Create a playbook with a global `StatePolicy` and a workload with an overriding policy. Assert the final `Node` in the DAG has the correctly merged, workload-specific policy.
        *   `TestBuildDAG_WithLoopVariable`: Create a playbook where a workload's `when` condition references `{{ .item }}`. Ensure this does not create a dependency on itself.

---

### **Milestone 2.2: Fuzz Testing for Security & Robustness**

**Objective:** Go beyond standard unit tests to find more subtle bugs in complex, concurrent, or security-sensitive areas of the codebase.

**Rationale:** Concurrency bugs (races, deadlocks) and parsing vulnerabilities are notoriously difficult to find with simple, predictable unit tests. Fuzzing and targeted race condition tests are necessary to build confidence in the system's robustness under unpredictable or high-stress conditions.

**Impacted Files & Detailed Changes:**

*   **New File: `internal/config/fuzz_test.go`**
    *   **Action:** Create a fuzz test for `config.LoadPlaybook` using Go's built-in `testing.F` framework.
    *   **Rationale:** The YAML parser and validation logic are primary attack surfaces. A malformed playbook should never cause the GXO engine to panic.
    *   **Implementation Detail:**
        ```go
        // internal/config/fuzz_test.go
        package config_test

        import (
            "testing"
            "github.com/gxo-labs/gxo/internal/config"
        )

        func FuzzLoadPlaybook(f *testing.F) {
            // Seed the fuzzer with valid YAML snippets to guide it.
            f.Add([]byte(`name: "valid_fuzz_playbook"\nschemaVersion: "v1.0.0"\nworkloads:\n- name: w1\n  lifecycle: { policy: run_once }\n  process: { module: exec }`))
            
            // The Fuzz function will be called with mutated versions of the seed data.
            f.Fuzz(func(t *testing.T, data []byte) {
                // The only goal is to ensure LoadPlaybook does not panic on any input.
                // We don't care about the error return value here.
                _ = config.LoadPlaybook(data, "fuzz_input.yaml")
            })
        }
        ```

*   **`engine_security_test.go`**
    *   **Action:** Add a new test case, `TestSecretRedaction_RaceCondition`, to specifically target the thread-safety of the secret redaction mechanism.
    *   **Rationale:** Secret tracking involves a per-workload map that is accessed during parameter rendering. If a workload uses parallel loops, this map could be accessed concurrently. This test verifies that the `SecretTracker` is safe for concurrent use.
    *   **Implementation Detail:**
        ```go
        // in engine_security_test.go
        func TestSecretRedaction_RaceCondition(t *testing.T) {
            t.Parallel() // Allow this test to run in parallel with others.
            
            // Setup engine with a mock secrets provider...
            engine, _, mockSecrets := setupSecurityTestEngine(t)
            mockSecrets.AddSecret("MY_SECRET", "supersecretvalue")
            
            // A playbook with a parallel loop that uses a secret.
            playbookYAML := `
            name: race_test
            schemaVersion: "v1.0.0"
            workloads:
              - name: parallel_secret_users
                lifecycle: { policy: run_once }
                loop: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
                loop_control:
                  parallel: 4
                process:
                  module: mock
                  params:
                    # Each parallel iteration will access the 'secret' function.
                    info: "User {{ .item }} uses key: {{ secret 'MY_SECRET' }}"
                register: all_results
            `
            
            // The test passes if it completes without the Go race detector (`go test -race`)
            // reporting any data races. No explicit assertions are needed.
            _, err := engine.RunPlaybook(context.Background(), []byte(playbookYAML))
            require.NoError(t, err)
        }
        ```

Of course. With the core engine refactored and hardened, we can now proceed to build the daemon itself.

This document represents the detailed engineering plan for **Phase 3** of the GXO Project Roadmap (ID: GXO-ROADMAP-V2). It is designed to be an exhaustive guide for implementing the `gxo daemon` and its foundational security and state management capabilities.

---

# **GXO Master Engineering Plan: Phase 3**

**Document ID:** GXO-ENG-PLAN-P3
**Version:** 3.0
**Date:** 2025-07-12
**Status:** Approved for Execution

## **Phase 3: Service Enablement & Foundational Security**

### **Objective**

Implement the `gxo daemon`, transforming GXO from an ephemeral task runner into a true, long-running Automation Kernel. This phase focuses on the non-negotiable features required for production deployments: a persistent state store, a secure control plane, and the ability to manage supervised workloads. Security is built-in from the start, not added on later.

### **Rationale**

The architectural vision of GXO as a unified runtime for services, events, and tasks can only be realized through a persistent, long-running daemon process. This phase builds that daemon, its secure control plane, and the core lifecycle reconcilers, unlocking the platform's most powerful capabilities and preparing it for production use. A secure-by-default posture is established early to ensure all subsequent features are built upon a hardened foundation.

---

### **Milestone 3.1: Persistent & Encrypted State Store**

**Objective:** Replace the volatile in-memory state store with a persistent, production-grade alternative using BoltDB.

**Rationale:** A daemon must survive restarts and maintain its state. The `MemoryStateStore` is insufficient for this purpose. BoltDB is chosen for its simplicity, transactional guarantees, and lack of external dependencies, making it a perfect fit for a self-contained GXO daemon. Encryption at rest is a foundational security requirement.

**Impacted Files & Detailed Changes:**

*   **New Directory: `internal/state/boltdb/`**
*   **New File: `internal/state/boltdb/store.go`**
    *   **Action:** Implement the `state.Store` interface using BoltDB.
    *   **Implementation Detail:**
        ```go
        package boltdb

        import (
            "github.com/boltdb/bolt"
            // ... other imports: crypto/aes, crypto/cipher, encoding/gob, etc.
        )
        
        // BoltStore implements the state.Store interface.
        type BoltStore struct {
            db         *bolt.DB
            bucketName []byte
            aead       cipher.AEAD // AEAD cipher for encryption/decryption
        }

        // NewBoltStore creates a new store, opening/creating the DB file.
        func NewBoltStore(path string, bucketName string, encryptionKey []byte) (*BoltStore, error) {
            db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
            // ... error handling ...

            // Initialize the AEAD cipher (e.g., AES-GCM) with the encryptionKey
            // ... cipher initialization ...

            // Ensure the bucket exists
            err = db.Update(func(tx *bolt.Tx) error {
                _, txErr := tx.CreateBucketIfNotExists([]byte(bucketName))
                return txErr
            })
            // ... error handling ...

            return &BoltStore{db: db, bucketName: []byte(bucketName), aead: aead}, nil
        }

        // Set serializes, encrypts, and writes a value to the DB.
        func (s *BoltStore) Set(key string, value interface{}) error {
            return s.db.Update(func(tx *bolt.Tx) error {
                b := tx.Bucket(s.bucketName)
                
                // 1. Serialize value to bytes using gob
                var buf bytes.Buffer
                if err := gob.NewEncoder(&buf).Encode(&value); err != nil {
                    return fmt.Errorf("failed to gob-encode value for key '%s': %w", key, err)
                }
                serializedBytes := buf.Bytes()
                
                // 2. Encrypt serialized bytes
                nonce := make([]byte, s.aead.NonceSize())
                if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
                    return fmt.Errorf("failed to generate nonce: %w", err)
                }
                encryptedBytes := s.aead.Seal(nonce, nonce, serializedBytes, nil)
                
                // 3. Put encrypted bytes into the bucket
                return b.Put([]byte(key), encryptedBytes)
            })
        }

        // Get decrypts and deserializes a value from the DB.
        func (s *BoltStore) Get(key string) (interface{}, bool) {
            var value interface{}
            err := s.db.View(func(tx *bolt.Tx) error {
                b := tx.Bucket(s.bucketName)
                encryptedBytes := b.Get([]byte(key))
                if encryptedBytes == nil {
                    return bolt.ErrKeyNotFound // Use a specific error to signal not found
                }
                
                // 1. Decrypt bytes
                nonceSize := s.aead.NonceSize()
                if len(encryptedBytes) < nonceSize {
                    return fmt.Errorf("invalid encrypted data: too short")
                }
                nonce, ciphertext := encryptedBytes[:nonceSize], encryptedBytes[nonceSize:]
                serializedBytes, err := s.aead.Open(nil, nonce, ciphertext, nil)
                if err != nil {
                    return fmt.Errorf("failed to decrypt value for key '%s': %w", key, err)
                }

                // 2. Deserialize bytes using gob
                buf := bytes.NewBuffer(serializedBytes)
                return gob.NewDecoder(buf).Decode(&value)
            })
            
            if err != nil {
                // Log the error for debugging, but treat it as not found for the caller
                // log.Errorf("Failed to get key '%s': %v", key, err)
                return nil, false
            }
            return value, true
        }
        
        // ... Implement other state.Store methods (Delete, Load, GetAll, Close) ...
        ```
*   **New Directory: `cmd/gxo-admin/`**
*   **New File: `cmd/gxo-admin/rekey.go`**
    *   **Action:** Create an `gxo-admin state rekey` command for offline state re-encryption.
    *   **Implementation Detail:** The command will take flags for the state file path, old key, and new key. It will open the BoltDB, iterate over every key-value pair, decrypt with the old key, re-encrypt with the new key, and write the new value back in a single transaction.

---

### **Milestone 3.2: The `gxo daemon` and `supervise` Lifecycle Reconciler**

**Objective:** Implement the core `gxo daemon` process and the first advanced lifecycle, `supervise`.

**Rationale:** This milestone brings the Automation Kernel to life as a long-running process. The `supervise` lifecycle is implemented first as it's the most common use case for a daemon and provides a clear pattern for managing persistent workloads.

**Impacted Files & Detailed Changes:**

*   **New File: `api/v1/daemon.proto`**
    *   **Action:** Define the initial gRPC service for daemon control using Protocol Buffers.
    *   **Implementation Detail:**
        ```protobuf
        syntax = "proto3";
        package gxo.daemon.v1;
        option go_package = "github.com/gxo-labs/gxo/pkg/gxo/v1/api";

        service GxoDaemon {
          // ApplyPlaybook applies a playbook, causing the daemon to add/update/remove workloads.
          // It's an idempotent operation based on the playbook's name.
          rpc ApplyPlaybook(ApplyRequest) returns (ApplyResponse);
          // RemovePlaybook removes all workloads associated with a previously applied playbook.
          rpc RemovePlaybook(RemoveRequest) returns (RemoveResponse);
        }
        
        message ApplyRequest {
          bytes playbook_yaml = 1;
        }

        message ApplyResponse {
          string playbook_name = 1;
          string status = 2; // e.g., "Applied", "Failed"
          string message = 3;
        }

        message RemoveRequest {
          string playbook_name = 1;
        }

        message RemoveResponse {
          string status = 1;
        }
        ```
*   **New File: `cmd/gxo/daemon.go`**
    *   **Action:** Create the `gxo daemon` Cobra command.
    *   **Implementation Detail:** This command will initialize the engine (with the new BoltDB state store), start the gRPC server in a goroutine, and then start the main daemon controller, blocking until the process is terminated.
*   **New Directory: `internal/daemon/`**
*   **New File: `internal/daemon/controller.go`**
    *   **Action:** Implement the main daemon controller.
    *   **Implementation Detail:** The controller will hold an in-memory map of `map[string]context.CancelFunc` to manage the lifecycle of active workload reconcilers. When a playbook is applied, it will diff the desired workloads against the active ones. It will start new reconcilers for new workloads, stop them for removed workloads, and signal updates for changed ones.
*   **New File: `internal/daemon/reconciler_supervise.go`**
    *   **Action:** Implement the `supervise` lifecycle reconciler.
    *   **Implementation Detail:** This will be a struct with a `Run` method that takes a `context.Context` and a `*config.Workload`. The `Run` method contains the core reconciliation loop for a single supervised workload.
        ```go
        // Simplified logic for the supervise reconciler loop
        func (r *SuperviseReconciler) Run(ctx context.Context, workload *config.Workload) {
            const baseDelay = 1 * time.Second
            const maxDelay = 1 * time.Minute
            var currentDelay time.Duration

            for {
                select {
                case <-ctx.Done(): return // Stop if workload is removed by controller
                default:
                }
                
                if currentDelay > 0 {
                    time.Sleep(currentDelay) // Apply backoff delay before restarting
                }

                // A new engine method is needed to run a single workload instance
                // It respects the workload's timeout.
                _, err := r.engine.RunWorkload(ctx, workload)
                
                if ctx.Err() != nil { return } // Context was cancelled, exit loop cleanly

                if err == nil && workload.Lifecycle.RestartPolicy != "always" {
                    log.Infof("Supervised workload '%s' completed successfully, stopping as per policy.", workload.Name)
                    return // Exits if policy is on_failure or never
                }
                
                // On failure or if 'always' is set, calculate next backoff delay
                if currentDelay == 0 {
                    currentDelay = baseDelay
                } else {
                    currentDelay = time.Duration(float64(currentDelay) * 2.0)
                }
                if currentDelay > maxDelay { currentDelay = maxDelay }
            }
        }
        ```

---

### **Milestone 3.3: Control Plane Security (mTLS with Simple Setup)**

**Objective:** Secure the `gxo daemon`'s gRPC control plane with mandatory mTLS, providing a developer-friendly way to generate self-signed certificates for testing.

**Rationale:** A control plane that accepts unauthenticated commands is an unacceptable security risk. mTLS is implemented from the very first version of the daemon to enforce strong, cryptographic identity for all clients. The auto-generation feature removes the high barrier to entry that PKI management can present for local development and testing.

**Impacted Files & Detailed Changes:**

*   **New Directory: `internal/pki/`**
*   **New File: `internal/pki/generate.go`**
    *   **Action:** Add functions to programmatically generate a self-signed CA, and server/client certificates signed by that CA, using Go's `crypto/x509` and `crypto/tls` packages. This avoids shelling out to `openssl`.
*   **`cmd/gxo/daemon.go`**
    *   **Action:** Add a `--generate-certs` flag. When used, it calls the `pki.generate` helpers to create `ca.pem`, `server.pem`, `server.key`, `client.pem`, `client.key` in the GXO config directory, prints instructions, and then exits.
*   **New File: `internal/daemon/server.go`**
    *   **Action:** Implement the gRPC server, loading TLS credentials at startup and requiring client certificate verification.
    *   **Implementation Detail:**
        ```go
        // In the daemon startup logic
        serverCert, err := tls.LoadX509KeyPair("server.crt", "server.key")
        // ... handle error ...

        caCert, err := os.ReadFile("ca.pem")
        // ... handle error ...
        caPool := x509.NewCertPool()
        caPool.AppendCertsFromPEM(caCert)

        creds := credentials.NewTLS(&tls.Config{
            Certificates: []tls.Certificate{serverCert},
            ClientCAs:    caPool,
            ClientAuth:   tls.RequireAndVerifyClientCert, // Enforce mTLS
        })

        serverOptions := []grpc.ServerOption{grpc.Creds(creds)}
        grpcServer := grpc.NewServer(serverOptions...)
        // ... register gRPC services ...
        ```

---

### **Milestone 3.4: The `gxo ctl` Command and Basic RBAC**

**Objective:** Provide the client-side tooling to interact with the secure daemon and implement an initial, simple RBAC mechanism.

**Rationale:** The `gxo ctl` tool is the user's primary interface to the daemon. It must be able to handle mTLS authentication seamlessly. A basic RBAC system based on certificate identity is implemented to enforce the principle of least privilege from the start.

**Impacted Files & Detailed Changes:**

*   **New Directory: `cmd/gxo-ctl/`**
*   **`main.go`, `apply.go`, `remove.go`:** Create the `gxo-ctl` binary with its initial subcommands. The root command will have persistent flags for `--server`, `--ca-cert`, `--client-cert`, and `--client-key`.
*   **New File: `cmd/gxo-ctl/client.go`**
    *   **Action:** Create a helper function to build the gRPC client connection with the required mTLS credentials.
*   **New File: `internal/daemon/interceptor/auth.go`**
    *   **Action:** Create a unary gRPC interceptor for authorization.
    *   **Implementation Detail:**
        ```go
        // Simplified RBAC logic in the interceptor
        func (i *AuthInterceptor) Authorize(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
            p, ok := peer.FromContext(ctx)
            // ... error handling for peer ...
            
            tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
            // ... error handling for certs ...
            
            // Extract the Common Name from the verified client certificate
            clientCN := tlsInfo.State.VerifiedChains[0][0].Subject.CommonName
            
            // The policy is a simple map[string][]string loaded from config,
            // mapping a CN to a list of allowed RPC methods.
            allowedMethods, userFound := i.rbacPolicy[clientCN]
            if !userFound {
                return nil, status.Error(codes.PermissionDenied, "client CN not in allowlist")
            }
            
            methodIsAllowed := false
            for _, allowedMethod := range allowedMethods {
                if allowedMethod == info.FullMethod {
                    methodIsAllowed = true
                    break
                }
            }

            if !methodIsAllowed {
                return nil, status.Error(codes.PermissionDenied, "access to method denied")
            }

            return handler(ctx, req)
        }
        ```
*   **`internal/daemon/controller.go`:** The daemon controller will be responsible for loading the RBAC policy map from the main GXO configuration file at startup and passing it to the interceptor.

---

# **GXO Master Engineering Plan: Phase 4**

**Document ID:** GXO-ENG-PLAN-P4
**Version:** 3.0
**Date:** 2025-07-12
**Status:** Approved for Execution

## **Phase 4: Event-Driven Automation & Human-in-the-Loop**

### **Objective**

With the daemon framework in place, this phase expands its capabilities to handle reactive and interactive workflows, which are core to GXO's vision of replacing complex glue code in areas like security orchestration and multi-stage deployments. This requires building out the lower layers of the GXO-AM and the corresponding lifecycle reconcilers.

### **Rationale**

Modern automation is increasingly reactive. Systems must respond to external events—a security alert, a git push, an incoming API call—not just run on a schedule. This phase delivers the foundational networking modules (`connection:*`, `http:*`) and the `event_driven` lifecycle, allowing GXO to act as a native network server. It also implements the `control:wait_for_signal` module, a powerful primitive for building workflows that require human approval, a common and difficult pattern to implement with traditional tools.

---

### **Milestone 4.1: The Network Stack (Layers 2 & 3)**

**Objective:** Enable low-level network and protocol automation, which are the prerequisites for the `event_driven` lifecycle and higher-level modules like `http:request`.

**Rationale:** The GXO-AM mandates that high-level modules are built upon low-level primitives. Before we can have an `event_driven` workload triggered by an HTTP request, the kernel must first understand how to listen for raw TCP connections (Layer 2) and how to parse the HTTP protocol (Layer 3). This milestone builds that foundation.

**Impacted Files & Detailed Changes:**

*   **New Directory: `internal/connections/`**
*   **New File: `internal/connections/manager.go`**
    *   **Action:** Create a new `ConnectionManager` service within the GXO Kernel. This service is essential for managing the state of long-lived network connections across different workloads.
    *   **Implementation Detail:**
        ```go
        package connections

        import (
            "net"
            "sync"
            "github.com/google/uuid"
        )

        // Manager holds open network connections, keyed by a unique ID.
        type Manager struct {
            mu          sync.RWMutex
            connections map[string]net.Conn
        }
        
        // Add stores a new connection and returns its unique ID.
        func (m *Manager) Add(conn net.Conn) string {
            m.mu.Lock()
            defer m.mu.Unlock()
            id := uuid.NewString()
            m.connections[id] = conn
            return id
        }
        
        // Get retrieves a connection by its ID.
        func (m *Manager) Get(id string) (net.Conn, bool) {
            m.mu.RLock()
            defer m.mu.RUnlock()
            conn, found := m.connections[id]
            return conn, found
        }
        
        // Remove closes the connection and removes it from the manager.
        func (m *Manager) Remove(id string) error {
            m.mu.Lock()
            defer m.mu.Unlock()
            conn, found := m.connections[id]
            if !found {
                return ErrConnectionNotFound
            }
            delete(m.connections, id)
            return conn.Close()
        }
        ```
    *   **`internal/engine/engine.go`:** The `ConnectionManager` will be instantiated within `NewEngine` and passed to the `WorkloadRunner`.

*   **New Directory: `modules/connection/`**
    *   **Action:** Implement the full suite of Layer 2 connection modules. These modules interact directly with the `ConnectionManager`.
    *   **`listen.go`:** Implement `connection:listen`. Its `Perform` method will start a `net.Listener` in a dedicated goroutine. On `listener.Accept()`, it will add the new `net.Conn` to the `ConnectionManager` and then write a record `{ "connection_id": "...", "remote_addr": "..." }` to its output channel(s). This module is designed to run in a `supervise` lifecycle.
    *   **`open.go`:** Implement `connection:open`. Its `Perform` calls `net.Dial`, adds the connection to the `ConnectionManager`, and returns the `{ "connection_id": "..." }` in its summary.
    *   **`read.go`, `write.go`, `close.go`:** These modules will take a `connection_id` as a required parameter. They will look up the `net.Conn` in the `ConnectionManager` and perform the corresponding I/O operation (`conn.Read`, `conn.Write`, `manager.Remove`).

*   **New Directory: `modules/http/`**
    *   **`listen.go` & `respond.go`:** Implement the Layer 3 `http:listen` and `http:respond` modules.
    *   **`http:listen` Implementation:**
        1.  This is a streaming module that consumes records from a `connection:listen` workload.
        2.  Its `Perform` method will loop over its input channel, receiving `connection_id`s.
        3.  For each `connection_id`, it gets the `net.Conn` from the `ConnectionManager`.
        4.  It then calls `http.ReadRequest` to parse a full HTTP request from the connection's byte stream.
        5.  A new `request_id` is generated and associated with the `*http.Request` and the original `connection_id` in a new, internal `RequestManager` (similar to the `ConnectionManager`).
        6.  It emits a structured record onto its output stream: `{ "request_id": "...", "method": "GET", "path": "/foo", "headers": {...} }`.
    *   **`http:respond` Implementation:** Takes a `request_id`, looks up the original connection, and writes a well-formed HTTP response using `*http.Response.Write`.

---

### **Milestone 4.2: The `event_driven` Lifecycle Reconciler**

**Objective:** Implement the `event_driven` lifecycle reconciler within the daemon, enabling reactive workflows.

**Rationale:** This is the second major lifecycle that the daemon must support. It's the core mechanism that allows GXO to act as a SOAR platform, webhook handler, or custom server. Its implementation depends directly on the streaming capabilities delivered in Milestone 4.1.

**Impacted Files & Detailed Changes:**

*   **New File: `internal/daemon/reconciler_event.go`**
    *   **Action:** Implement the `event_driven` lifecycle reconciler.
    *   **Implementation Detail:**
        ```go
        package daemon

        // EventReconciler manages an event_driven workload.
        type EventReconciler struct {
            engine *engine.Engine
            // ... other dependencies
        }

        func (r *EventReconciler) Run(ctx context.Context, workload *config.Workload) {
            // 1. Find the source workload from the daemon's active workload list.
            sourceWorkload, found := r.controller.GetWorkload(workload.Lifecycle.Source)
            if !found {
                // Log fatal error for this workload; it cannot run.
                return
            }

            // 2. Get the output channel for the source workload.
            // This requires a new mechanism in the engine/channel_manager to get
            // a handle to a producer's output stream dynamically.
            eventChan, err := r.engine.GetStreamFor(sourceWorkload)
            if err != nil {
                // Log fatal error
                return
            }
            
            // 3. Enter the event processing loop.
            for {
                select {
                case <-ctx.Done(): // Stop if the event_driven workload is removed.
                    return
                case eventRecord, ok := <-eventChan:
                    if !ok { // Source stream closed.
                        log.Infof("Source stream for '%s' closed. Event reconciler shutting down.", workload.Name)
                        return
                    }

                    // 4. On each event, spawn a goroutine to run an ephemeral instance
                    // of this workload's DAG.
                    go func(record map[string]interface{}) {
                        // Create a new, isolated context for this single execution.
                        instanceCtx, cancel := context.WithCancel(context.Background())
                        defer cancel()
                        
                        // The engine needs a new method to run a workload DAG that
                        // is seeded with initial state from the event record.
                        log.Infof("Event received. Triggering execution of workload '%s'", workload.Name)
                        _, runErr := r.engine.RunEphemeralDAG(instanceCtx, workload, record)
                        if runErr != nil {
                            log.Errorf("Ephemeral execution of '%s' failed: %v", workload.Name, runErr)
                        }
                    }(eventRecord)
                }
            }
        }
        ```
*   **`internal/daemon/controller.go`:** The main controller will now, upon applying a playbook, identify `event_driven` workloads and launch an `EventReconciler` goroutine for each one, passing it the necessary context.

---

### **Milestone 4.3: Human-in-the-Loop (`Resume Context`)**

**Objective:** Implement the `Resume Context` primitive to enable interactive, approval-based workflows that can pause and wait for external input.

**Rationale:** This feature provides a robust solution for a notoriously difficult automation problem: staging deployments with manual approval gates. Implementing it now leverages the daemon's persistent state store and gRPC control plane, showcasing the power of GXO's integrated architecture.

**Impacted Files & Detailed Changes:**

*   **New Directory: `modules/control/`**
*   **New File: `modules/control/wait_for_signal.go`**
    *   **Action:** Implement the `control:wait_for_signal` module.
    *   **Implementation Detail:** This module's `Perform` method will be very simple. It will return a special, sentinel error that the engine is designed to recognize.
        ```go
        package wait_for_signal

        import "errors"
        
        var ErrPauseWorkflow = errors.New("gxo: signal to pause workflow")

        func (m *WaitForSignalModule) Perform(...) (interface{}, error) {
            // The module itself does nothing but signal the engine.
            return nil, ErrPauseWorkflow
        }
        ```
*   **`internal/engine/workload_runner.go`**
    *   **Action:** Modify the `WorkloadRunner` to check for the `ErrPauseWorkflow` sentinel error.
    *   **Implementation Detail:** If `module.Perform` returns this specific error, the `WorkloadRunner` will not treat it as a failure. Instead, it will propagate this sentinel error up to the daemon's reconciler.
*   **`internal/daemon/reconciler_runonce.go`** (A new reconciler for `run_once` workloads managed by the daemon may be needed, or this logic goes in the controller).
    *   **Action:** Implement the pause logic.
    *   **Implementation Detail:**
        1.  When a workload execution returns `ErrPauseWorkflow`, the reconciler catches it.
        2.  It generates a unique, cryptographically secure token (e.g., UUID).
        3.  It takes a snapshot of the *entire current state* of that specific workflow instance.
        4.  It stores the token, the workflow state snapshot, and the paused workload's ID in a new dedicated bucket in BoltDB: `paused_workflows`.
        5.  It sets the workload's status to `Paused` in the main state store.
*   **`internal/daemon/server.go`**
    *   **Action:** Add a `ResumeWorkflow` RPC endpoint to the gRPC server.
    *   **Implementation Detail:**
        1.  The `ResumeWorkflow` method takes a `token` and a JSON `payload`.
        2.  It looks up the token in the `paused_workflows` BoltDB bucket. If not found, it returns `NotFound`.
        3.  It retrieves the paused workflow's state snapshot and the workload ID.
        4.  It **merges the provided JSON `payload`** into the state snapshot under the reserved key `_gxo.resume_payload`.
        5.  It signals the main daemon controller to "resume" the paused workload, providing its ID and the newly hydrated state.
        6.  The controller then finds the paused workload and re-schedules its remaining downstream dependencies to run with the updated state.
*   **`cmd/gxo-ctl/resume.go`**
    *   **Action:** Add the `gxo-ctl resume --token <token> --payload '{"approved": true}'` command to call the new gRPC endpoint.

---

# **GXO Master Engineering Plan: Phase 5**

**Document ID:** GXO-ENG-PLAN-P5
**Version:** 4.0
**Date:** 2025-07-12
**Status:** Approved for Execution

## **Phase 5: Expanding the Standard Library (Layers 5 & 6)**

### **Objective**

With the foundational module layers (1-4) in place, this phase builds upon them to deliver high-value application and integration modules. This unlocks the most common and powerful use cases for "glue code" replacement and integration with the wider DevOps ecosystem.

### **Rationale**

The lower-level modules provide the *capability* to interact with any system, but the higher-level modules provide the *convenience* that drives adoption. Implementing the Layer 5 `http:request` and Layer 6 `terraform:run` modules provides immediate, high-impact solutions to common engineering problems and clearly demonstrates the power of the GXO Automation Model, where complex integrations are built by composing simpler, layered primitives. This phase is critical for showcasing GXO's value proposition as a practical, day-to-day automation tool.

---

### **Milestone 5.1: REST API Client (Layer 5)**

**Objective:** Implement the universal `http:request` module. This is the single most important module for external system integration.

**Rationale:** The vast majority of modern automation involves interacting with REST APIs. A powerful, convenient, and robust `http:request` module is the gateway to integrating GXO with virtually any other platform or service. This module is built upon the Layer 2/3 primitives conceptually but is implemented using Go's mature `net/http` library for performance and feature completeness, abstracting away the raw socket handling for the user.

**Impacted Files & Detailed Changes:**

*   **New File: `modules/http/request.go`**
    *   **Action:** Create the `http:request` module.
    *   **Implementation Detail (`HttpRequestModule` struct and `Perform` method):**
        ```go
        package http

        import (
            "bytes"
            "context"
            "crypto/tls"
            "encoding/json"
            "fmt"
            "io"
            "net/http"
            "time"
            
            "github.com/gxo-labs/gxo/internal/module"
            "github.com/gxo-labs/gxo/internal/paramutil"
            "github.com/gxo-labs/gxo/pkg/gxo/v1/plugin"
            "github.com/gxo-labs/gxo/pkg/gxo/v1/state"
        )
        
        // HttpRequestModule implements the http:request GXO module.
        type HttpRequestModule struct {
            // httpClient is reused across Perform calls for connection pooling (keep-alives).
            httpClient *http.Client
        }

        func init() {
            module.Register("http:request", NewHttpRequestModule)
        }

        // NewHttpRequestModule is the factory for creating the module instance.
        func NewHttpRequestModule() plugin.Module {
            return &HttpRequestModule{
                httpClient: &http.Client{},
            }
        }

        // Perform executes the HTTP request.
        func (m *HttpRequestModule) Perform(
            ctx context.Context,
            params map[string]interface{},
            stateReader state.StateReader,
            // ... other standard Perform args
        ) (interface{}, error) {
            
            // 1. Parameter Validation
            url, err := paramutil.GetRequiredString(params, "url")
            if err != nil { return nil, err }

            method, _, _ := paramutil.GetOptionalString(params, "method")
            if method == "" { method = "GET" }

            body, _, _ := paramutil.GetOptionalString(params, "body")
            
            headers, _, err := paramutil.GetOptionalMap(params, "headers")
            if err != nil { return nil, err }

            timeoutStr, _, _ := paramutil.GetOptionalString(params, "timeout")
            
            auth, _, err := paramutil.GetOptionalMap(params, "auth")
            if err != nil { return nil, err }
            
            skipVerify, _, _ := paramutil.GetOptionalBool(params, "skip_tls_verify")

            // 2. Configure HTTP Client with Timeout and TLS settings
            // A new transport and client are configured for each call to respect per-call settings.
            transport := http.DefaultTransport.(*http.Transport).Clone()
            transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: skipVerify}
            client := &http.Client{Transport: transport}
            if timeoutStr != "" {
                timeout, err := time.ParseDuration(timeoutStr)
                if err != nil { return nil, fmt.Errorf("invalid timeout format: %w", err) }
                client.Timeout = timeout
            }
            
            // 3. Request Creation with Context
            req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBufferString(body))
            if err != nil {
                return nil, fmt.Errorf("failed to create http request: %w", err)
            }
            
            // 4. Configure Headers & Auth
            for k, v := range headers {
                req.Header.Set(k, fmt.Sprintf("%v", v))
            }
            if err := configureAuth(req, auth); err != nil {
                return nil, err
            }

            // 5. Execute Request
            startTime := time.Now()
            resp, err := client.Do(req)
            latency := time.Since(startTime)
            if err != nil {
                return nil, fmt.Errorf("http request failed: %w", err)
            }
            defer resp.Body.Close()

            // 6. Response Handling
            respBodyBytes, err := io.ReadAll(resp.Body)
            if err != nil {
                return nil, fmt.Errorf("failed to read response body: %w", err)
            }
            
            // 7. Construct Summary
            summary := map[string]interface{}{
                "status_code": resp.StatusCode,
                "headers":     resp.Header,
                "body":        string(respBodyBytes),
                "latency_ms":  latency.Milliseconds(),
            }

            // 8. Attempt to parse JSON body
            contentType := resp.Header.Get("Content-Type")
            if strings.Contains(contentType, "application/json") {
                var jsonBody interface{}
                if err := json.Unmarshal(respBodyBytes, &jsonBody); err == nil {
                    summary["json_body"] = jsonBody
                }
            }

            return summary, nil
        }
        
        // configureAuth is a helper to handle the 'auth' block.
        func configureAuth(req *http.Request, auth map[string]interface{}) error {
            if auth == nil { return nil }
            if basicAuth, ok := auth["basic"].(map[string]interface{}); ok {
                user, uOK := basicAuth["user"].(string)
                pass, pOK := basicAuth["pass"].(string)
                if !uOK || !pOK {
                    return fmt.Errorf("'auth.basic' requires 'user' and 'pass' string fields")
                }
                req.SetBasicAuth(user, pass)
            }
            // ... Add other auth types like 'bearer' here in the future ...
            return nil
        }
        ```
    *   **Module API:**
        | Name | Type | Required? | Description |
        |---|---|---|---|
        | `url` | string | Yes | The URL of the endpoint to request. |
        | `method` | string | No | HTTP method (GET, POST, etc.). Defaults to `GET`. |
        | `headers` | map | No | A map of request headers. |
        | `body` | string | No | The request body. |
        | `timeout` | string | No | Request timeout (e.g., "10s"). |
        | `auth` | map | No | A map specifying authentication, e.g., `{ "basic": { "user": "u", "pass": "{{ secret 'p' }}" } }`. |
    *   **Summary Structure:** `{ "status_code": int, "headers": map, "body": string, "json_body": any, "latency_ms": int }`.

---

### **Milestone 5.2: Advanced Data Plane & Application Modules (Layers 4 & 5)**

**Objective:** Enhance ETL capabilities and add clients for common data services, building on the core primitives.

**Rationale:** While the critical path covers basic data processing, advanced use cases require more powerful tools like stateful aggregations over time and direct database interaction. This milestone delivers those capabilities, making GXO a viable platform for more complex data integration tasks.

**Impacted Files & Detailed Changes:**

*   **New File: `modules/data/aggregate.go`**
    *   **Action:** Implement the stateful streaming module `data:aggregate`.
    *   **Implementation Detail (`Perform` method):**
        1.  **State Structure:** Define an internal `aggregationGroup` struct to hold state for each group key.
            ```go
            type aggregationGroup struct {
                count int64
                sum   float64
                // ... other aggregate states
            }
            ```
        2.  **Main Loop:** The `Perform` method will start a goroutine to read from the single input channel.
        3.  **Grouping:** For each incoming record, it will construct a composite key (string) from the values of the `group_by_fields`.
        4.  **State Update:** It will look up the `aggregationGroup` for that key in an internal `map[string]*aggregationGroup` and update its values (increment count, add to sum, etc.).
        5.  **Windowing Logic:**
            *   **For `window`:** A `time.Ticker` will be used. A separate goroutine will `select` on the ticker. On each tick, it will lock the state map, iterate through all groups, emit the completed aggregate records, and then reset/delete the groups.
            *   **For `count`:** After updating a group's state, the code will check if `group.count >= configured_count`. If so, it will emit the record and reset that group's state immediately.
        6.  **Termination Handling:** A `sync.WaitGroup` must be used to ensure the `Perform` method only returns after the input channel is fully drained and the ticker goroutine (if any) has been gracefully stopped. Any final, non-empty aggregate groups must be flushed before returning.
*   **New File: `modules/database/query.go`**
    *   **Action:** Implement the `database:query` module.
    *   **Implementation Detail (`Perform` method):**
        1.  **Driver Imports:** The Go file must have blank imports for the required database drivers to ensure they are compiled in.
            ```go
            import (
                _ "github.com/lib/pq" // PostgreSQL driver
                _ "github.com/go-sql-driver/mysql" // MySQL driver
            )
            ```
        2.  **Parameter Parsing:** Get `driver` name, `dsn` (Data Source Name) string, `query` string, and optional `params` list.
        3.  **Connection:** Use `sql.Open(driver, dsn)` to get a `*sql.DB` connection pool object. Immediately call `defer db.Close()`.
        4.  **Execution:** Use a helper function `isSelectQuery(query)` to determine the query type.
            *   **For `SELECT` queries (Streaming Producer):**
                ```go
                rows, err := db.QueryContext(ctx, query, queryParams...)
                if err != nil { return nil, err }
                defer rows.Close()

                columns, err := rows.Columns()
                if err != nil { return nil, err }
                
                for rows.Next() {
                    values := make([]interface{}, len(columns))
                    scanArgs := make([]interface{}, len(columns))
                    for i := range values { scanArgs[i] = &values[i] }

                    if err := rows.Scan(scanArgs...); err != nil { /* handle error */ }
                    
                    record := make(map[string]interface{})
                    for i, colName := range columns {
                        // Handle potential []byte from DB -> string
                        if b, ok := values[i].([]byte); ok {
                           record[colName] = string(b)
                        } else {
                           record[colName] = values[i]
                        }
                    }
                    // Fan-out record to all output channels
                    for _, out := range outputChans { out <- record }
                }
                return summary, nil
                ```
            *   **For non-SELECT queries:**
                ```go
                result, err := db.ExecContext(ctx, query, queryParams...)
                if err != nil { return nil, err }
                rowsAffected, err := result.RowsAffected()
                // ... handle error ...
                return map[string]interface{}{"rows_affected": rowsAffected}, nil
                ```

---

### **Milestone 5.3: The Integration Layer (Layer 6)**

**Objective:** Provide opinionated, high-level wrappers for key ecosystem tools to create a seamless "better together" experience.

**Rationale:** While users *could* interact with tools like Terraform or SSH using the generic `exec` module, providing dedicated, intelligent wrappers greatly improves the user experience. It reduces boilerplate, enforces best practices, and allows GXO to handle complex state-passing automatically, directly solving the "State Gap" problem.

**Impacted Files & Detailed Changes:**

*   **New Directory: `modules/object_storage/`**
    *   **Action:** First, implement a generic Layer 5 `object_storage` suite (`get_object`, `put_object`) for S3-compatible APIs using the AWS SDK for Go v2. This is a prerequisite for the `artifact` module.
*   **New Directory: `modules/artifact/`**
    *   **Action:** Create the `artifact:upload` and `artifact:download` modules.
    *   **`upload.go` Implementation Detail (`Perform` method):**
        1.  The `artifact:upload` module will be a high-level wrapper that internally orchestrates logic.
        2.  It takes `path` (local file in workspace) and `name` (logical artifact name) as parameters. It can also take an optional `version`.
        3.  **Step 1:** Open the file at `path` and create a `sha256.Hasher`. Use `io.Copy` to both calculate the file's checksum and prepare it for upload.
        4.  **Step 2:** Construct the remote object key, e.g., `<base_prefix>/<name>/<version>/<filename>`.
        5.  **Step 3:** Use the `object_storage:put_object` module's logic (or its underlying SDK calls) to upload the file stream.
        6.  **Step 4:** Return a structured **Artifact Handle** map in its `summary`: `{ "name": "my-app", "version": "v1.2.3", "checksum_sha256": "...", "remote_path": "s3://..." }`.
*   **New Directory: `modules/terraform/`**
    *   **Action:** Create the `terraform:run` module.
    *   **Implementation Detail (`Perform` method):**
        1.  This module will internally use the `internal/command` runner, just like the `exec` module.
        2.  It takes parameters: `path` (to Terraform files), `action` (`apply`, `plan`, `destroy`), and `vars` (a map).
        3.  **Step 1:** Execute `terraform init -input=false -no-color` in the `path` directory. Check the exit code and fail if non-zero.
        4.  **Step 2:** Construct the arguments for the main `terraform` command. For `apply`, this would be `["apply", "-auto-approve", "-no-color"]`. Iterate through the `vars` map and append `-var="key=value"` for each entry.
        5.  **Step 3:** Execute the main command. Capture `stdout` and `stderr` and stream them to the GXO logger in real-time for user feedback. Fail if the exit code is non-zero.
        6.  **Step 4 (for `apply` only):** If the `apply` succeeds, immediately execute `terraform output -json -no-color`.
        7.  **Step 5:** Capture the stdout of the `output` command, which is a JSON string. Unmarshal this JSON into a `map[string]interface{}`. This map contains the structured Terraform outputs.
        8.  **Step 6:** Return this map of outputs as the module's `summary`. This directly solves the state-passing problem.
*   **New Directory: `modules/ssh/`**
    *   **Action:** Implement `ssh:connect`, `ssh:command`, `ssh:script` using `golang.org/x/crypto/ssh`.
    *   **Implementation Detail:**
        *   **`ssh:connect`**: Its `Perform` method will parse `host`, `user`, `password`/`private_key` params. It will create an `ssh.ClientConfig` and call `ssh.Dial`. The resulting `*ssh.Client` will be stored in the `internal/connections` manager (keyed by a new `connection_id`) and the ID returned in the summary.
        *   **`ssh:command`**: Takes a `connection_id`, looks up the `*ssh.Client`, creates a new session with `client.NewSession()`, runs the command with `session.CombinedOutput()`, and returns the output and exit code.
        *   **`ssh:script`**: Will use SFTP (by opening a new SFTP client from the `*ssh.Client`) to upload the local script to a temporary location on the remote host (`/tmp/...`), then use `ssh:command` logic to execute `chmod +x` and run the script, and finally use SFTP again to remove the script in a `defer` block.

---

# **GXO Master Engineering Plan: Phase 6**

**Document ID:** GXO-ENG-PLAN-P6
**Version:** 4.0
**Date:** 2025-07-12
**Status:** Approved for Execution

## **Phase 6: Advanced Workflows & Developer Experience**

### **Objective**

The platform is now highly functional with a rich module library. This phase focuses on delivering the remaining advanced, high-level workflow capabilities (`event_driven`, `scheduled`, human-in-the-loop) and the crucial tooling required for users to reliably test their own complex playbooks.

### **Rationale**

This phase completes the vision of GXO as a truly unified automation kernel. The `event_driven` and `scheduled` lifecycles unlock entire categories of automation (reactive servers, cron job replacement) that are difficult or impossible with traditional task runners. The `Resume Context` for human-in-the-loop workflows solves a critical enterprise use case. Finally, the native testing framework (`gxo test`) elevates GXO from a powerful tool to a mature, professional platform by enabling a Test-Driven Development (TDD) lifecycle for automation engineers.

---

### **Milestone 6.1: The `event_driven` & `scheduled` Lifecycles**

**Objective:** Implement the remaining advanced lifecycle reconcilers in the daemon.

**Rationale:** This milestone delivers the final core execution paradigms. The `event_driven` reconciler enables GXO to act as a reactive server or message queue consumer, while the `scheduled` reconciler provides a robust, integrated replacement for system `cron`.

**Impacted Files & Detailed Changes:**

*   **New File: `internal/daemon/reconciler_event.go`**
    *   **Action:** Implement the `event_driven` lifecycle reconciler.
    *   **Implementation Detail (`EventReconciler.Run` method):**
        1.  **Source Lookup:** The `Run` method receives the `event_driven` workload. It first queries the `daemon.Controller` to get a handle to the `source` workload specified in `workload.Lifecycle.Source`. If the source workload does not exist or is not a streaming producer, the reconciler logs a fatal error and exits.
        2.  **Stream Subscription:** A new method, `engine.SubscribeToStream(workloadID string) (<-chan map[string]interface{}, error)`, must be added. This method will allow a component to get a new, unique fan-out channel for a given producer workload's output stream. The `EventReconciler` will call this to get its `eventChan`.
        3.  **Event Loop:** The reconciler enters its main `for` loop, selecting on its context (for shutdown) and the `eventChan`.
            ```go
            func (r *EventReconciler) Run(ctx context.Context, workload *config.Workload) {
                // ... setup and source lookup ...
                
                eventChan, err := r.engine.SubscribeToStream(sourceWorkload.InternalID)
                // ... handle error ...

                for {
                    select {
                    case <-ctx.Done():
                        log.Infof("Event reconciler for '%s' shutting down.", workload.Name)
                        return
                    case eventRecord, ok := <-eventChan:
                        if !ok {
                            log.Warnf("Source stream for '%s' closed. Event reconciler exiting.", workload.Name)
                            return
                        }
                        
                        // Spawn a new goroutine for each event to prevent blocking the event loop.
                        go r.executeEphemeralInstance(eventRecord, workload)
                    }
                }
            }
            ```
        4.  **Ephemeral Execution:** The `executeEphemeralInstance` helper function is responsible for running the workload's DAG for a single event.
            *   It creates a new background context for the instance.
            *   It creates a **new, temporary, in-memory state store** for the execution.
            *   It **seeds this state store** with the `eventRecord` data, making it available to the workload's templates.
            *   It calls a new engine method, `engine.RunSingleWorkloadDAG(ctx, workload, tempStateStore)`, which runs the logic for just that workload and its dependencies (if any are defined within a `stream:pipeline`).
            *   Any errors from the ephemeral run are logged, but do not crash the reconciler itself.

*   **New File: `internal/daemon/reconciler_scheduled.go`**
    *   **Action:** Implement the `scheduled` lifecycle reconciler.
    *   **Implementation Detail (`ScheduledReconciler.Run` method):**
        1.  **Cron Parsing:** The `Run` method will use a robust cron parsing library (e.g., `github.com/robfig/cron/v3`) to parse the `workload.Lifecycle.Cron` expression.
        2.  **Scheduling Loop:**
            ```go
            func (r *ScheduledReconciler) Run(ctx context.Context, workload *config.Workload) {
                schedule, err := cron.ParseStandard(workload.Lifecycle.Cron)
                // ... handle parse error ...

                for {
                    // Calculate the time of the next activation.
                    nextActivation := schedule.Next(time.Now())
                    timer := time.NewTimer(time.Until(nextActivation))

                    select {
                    case <-ctx.Done():
                        timer.Stop()
                        return
                    case <-timer.C:
                        // Time for an execution.
                        log.Infof("Cron schedule triggered for workload '%s'.", workload.Name)
                        go r.executeEphemeralInstance(workload) // Same ephemeral logic as event_driven
                    }
                }
            }
            ```
*   **`internal/daemon/controller.go`:** The main controller's reconciliation logic will be updated to launch `EventReconciler` or `ScheduledReconciler` goroutines for workloads with the corresponding lifecycle policies.

---

### **Milestone 6.2: Human-in-the-Loop (`Resume Context`)**

**Objective:** Implement the `Resume Context` primitive to enable interactive, approval-based workflows that can pause and wait for external input.

**Rationale:** This feature provides a robust solution for a notoriously difficult automation problem: staging deployments with manual approval gates. Implementing it now leverages the daemon's persistent state store and gRPC control plane, showcasing the power of GXO's integrated architecture.

**Impacted Files & Detailed Changes:**

*   **New File: `modules/control/wait_for_signal.go`**
    *   **Action:** Implement the `control:wait_for_signal` module.
    *   **Implementation Detail:** This module's `Perform` method is a simple but critical piece of the puzzle. It does nothing but signal the engine.
        ```go
        package wait_for_signal

        import "errors"
        
        // ErrPauseWorkflow is a sentinel error used to signal the engine to pause execution.
        // It is not a "real" error in the sense of a failure.
        var ErrPauseWorkflow = errors.New("gxo: signal to pause workflow")

        func (m *WaitForSignalModule) Perform(...) (interface{}, error) {
            // The module's only job is to return this specific error.
            // The engine/daemon is responsible for all pause logic.
            return nil, ErrPauseWorkflow
        }
        ```
*   **`internal/engine/workload_runner.go`**
    *   **Action:** Modify the `WorkloadRunner` to recognize and propagate the `ErrPauseWorkflow` sentinel error.
    *   **Implementation Detail:** Inside `executeSingleWorkloadInstance`, after `plugin.Perform` returns, add a check:
        ```go
        if errors.Is(performErr, wait_for_signal.ErrPauseWorkflow) {
            // Do not treat this as a failure. Propagate the sentinel error
            // so the daemon's reconciler can catch it.
            return summary, performErr
        }
        ```
*   **`internal/daemon/controller.go`**
    *   **Action:** Modify the daemon's `run_once` execution logic to handle the pause signal.
    *   **Implementation Detail:**
        1.  When a `run_once` workload returns `ErrPauseWorkflow`, the controller catches it.
        2.  It generates a unique, cryptographically secure token (e.g., UUID).
        3.  It serializes the **entire current state** of that specific workflow instance (using `state.GetAll()`).
        4.  It stores the token, the serialized workflow state, and the paused workload's ID in a new dedicated bucket in BoltDB: `paused_workflows`.
        5.  It returns a specific `summary` from the paused workload containing the token, so it can be registered and used (e.g., posted to Slack).
*   **`internal/daemon/server.go`**
    *   **Action:** Add a `ResumeWorkflow` RPC endpoint to the gRPC server.
    *   **Implementation Detail:**
        1.  The `ResumeWorkflow` method takes a `token` and a JSON `payload`.
        2.  It looks up the token in the `paused_workflows` BoltDB bucket. If not found or already used, it returns `NotFound`.
        3.  It retrieves the paused workflow's state snapshot and the paused workload's ID.
        4.  It deserializes the state, unmarshals the `payload`, and **merges the payload** into the state under the key `_gxo.resume_payload`.
        5.  It signals the main daemon controller to **resume** the playbook from the paused workload, providing its ID and the newly hydrated state. The controller then re-schedules the downstream dependencies to run with the updated state.
        6.  The token is deleted from BoltDB to ensure it is single-use.
*   **`cmd/gxo-ctl/resume.go`**
    *   **Action:** Add the `gxo-ctl resume --token <token> --payload '{"approved": true}'` command to call the new gRPC endpoint.

---

### **Milestone 6.3: The Playbook Mocking Framework**

**Objective:** Introduce a first-class, GXO-native testing and validation experience.

**Rationale:** To drive adoption and enable the creation of complex, reliable automation, users must have the confidence to test their playbooks without affecting live systems. This phase builds a dedicated testing framework directly into the GXO toolchain.

**Impacted Files & Detailed Changes:**

*   **New File: `cmd/gxo/test.go`**
    *   **Action:** Define the `gxo test` command, its flags (`-v`, `--run`), and its execution logic.
    *   **Implementation Detail:** The `runTestCommand` function will recursively discover files matching `*.test.gxo.yaml`, filter them based on the `--run` regex flag, and execute each one in an isolated engine instance, printing structured `=== RUN`, `--- PASS/FAIL`, and `FAIL` summary output similar to `go test`.
*   **New Directory: `modules/test/`**
*   **New File: `modules/test/mock_http_server.go`**
    *   **Action:** Create the `test:mock_http_server` module.
    *   **Implementation Detail (`Perform` method):**
        1.  Use Go's `net/http/httptest` to create an `httptest.NewServer`.
        2.  The server's handler will be a `http.HandlerFunc` that iterates through the configured `handlers` from the module's params, matching on request method and path.
        3.  The `Perform` method **must block** until its `context` is cancelled to keep the server alive for the entire playbook run. A `select { case <-ctx.Done(): }` achieves this.
        4.  It returns the server's URL in its summary: `{ "server_url": server.URL }`.
*   **New File: `modules/test/assert.go`**
    *   **Action:** Create the `test:assert` module. This module officially replaces `control:assert`, which should now be deprecated and removed. `test:assert` is for testing contexts, while workload validation should be done with `when` or other control flow.
    *   **Implementation Detail (`Perform` method):**
        1.  Parse the `assertions` list parameter.
        2.  Loop through each assertion and use a `switch` statement on the operator key (`equal_to`, `contains`, `is_true`, etc.).
        3.  If any assertion fails, return an immediate `gxoerrors.NewValidationError` with a descriptive message (e.g., `assertion failed: expected 'a' to be equal to 'b'`).
        4.  If all pass, return `{ "assertions_passed": count }`.

---

# **GXO Master Engineering Plan: Phase 7**

**Document ID:** GXO-ENG-PLAN-P7
**Version:** 4.0
**Date:** 2025-07-12
**Status:** Approved for Execution

## **Phase 7: Production Hardening & Advanced Security**

### **Objective**

With a feature-complete and testable platform, this final phase implements advanced security controls focused on hardening the workload execution environment and securing the module supply chain, preparing GXO for high-security production deployments.

### **Rationale**

A secure platform requires defense in depth. While Phase 3 secured the "front door" (the control plane), this phase builds the "internal walls" by isolating workloads from each other and the host system using OS-level sandboxing. It also secures the "supply chain" by ensuring that only trusted, verified modules can be executed, preventing the introduction of malicious code into the platform. These capabilities are essential for earning the trust of security teams and for operating GXO with a least-privilege security posture.

---

### **Milestone 7.1: Workload Sandboxing (`security_context`)**

**Objective:** Implement OS-level sandboxing for workloads as defined in the `security_context` configuration block.

**Rationale:** By default, workloads run with the same permissions as the `gxo daemon` process. This is a potential attack vector. A compromised workload could interfere with other workloads or the host system. The `security_context` provides a declarative way to apply strong, OS-native isolation mechanisms (namespaces, cgroups, seccomp) to dramatically reduce the blast radius of a compromised workload.

**Impacted Files & Detailed Changes:**

*   **`internal/config/policy.go`:**
    *   **Action:** Add the `SecurityContext` struct and its child structs to the policy definitions. This makes the security configuration part of the formal playbook structure.
    *   **Implementation Detail:**
        ```go
        // In internal/config/policy.go
        type SecurityContext struct {
            // SeccompProfilePath specifies the path to a JSON seccomp-bpf filter policy file.
            // If set to "default", a restrictive default profile is used.
            SeccompProfilePath string `yaml:"seccomp_profile_path,omitempty"`

            // ResourceLimits defines cgroup resource constraints for the workload.
            ResourceLimits *ResourceLimits `yaml:"resource_limits,omitempty"`

            // Namespaces defines which Linux namespaces to create for the workload.
            Namespaces *NamespaceOptions `yaml:"namespaces,omitempty"`
        }

        type ResourceLimits struct {
            MemoryBytes int64 `yaml:"memory_bytes,omitempty"`
            CPUQuotaUS  int64 `yaml:"cpu_quota_us,omitempty"`
            CPUPeriodUS int64 `yaml:"cpu_period_us,omitempty"`
        }

        type NamespaceOptions struct {
            PID    bool `yaml:"pid,omitempty"`    // Isolate process IDs
            Mount  bool `yaml:"mount,omitempty"`  // Isolate filesystem mount points
            IPC    bool `yaml:"ipc,omitempty"`    // Isolate inter-process communication
            UTS    bool `yaml:"uts,omitempty"`    // Isolate hostname
            User   bool `yaml:"user,omitempty"`   // Isolate user/group IDs
            Net    bool `yaml:"net,omitempty"`    // Isolate network stack
        }
        ```
*   **`internal/config/config.go`:**
    *   **Action:** Add the `SecurityContext` field to the `Workload` struct.
    *   **Implementation Detail:**
        ```go
        // In internal/config/config.go
        type Workload struct {
            // ... all other workload fields
            SecurityContext *config.SecurityContext `yaml:"security_context,omitempty"`
        }
        ```

*   **New Directory: `internal/sandbox/`**
    *   **Action:** Create a new package to encapsulate all low-level sandboxing logic.
    *   **New Files: `internal/sandbox/cgroups.go`, `seccomp.go`, `namespaces.go`**
        *   **`cgroups.go`:** Will contain functions to interact with the cgroup v2 unified hierarchy via the standard `/sys/fs/cgroup` filesystem. Functions will include `CreateSlice`, `SetMemoryMax`, `SetCPUWeight`, and `AddProcess`.
        *   **`seccomp.go`:** Will use a library like `seccomp-golang` to load a JSON policy file and apply the seccomp-bpf filter to the current process. It will include an embedded `default-minimal.json` profile.
        *   **`namespaces.go`:** Will contain helpers that return the correct flags (e.g., `syscall.CLONE_NEWPID`, `syscall.CLONE_NEWNS`) to be used in `exec.Cmd.SysProcAttr`.

*   **`internal/engine/workload_runner.go`:**
    *   **Action:** This is the core enforcement point. The `ExecuteWorkload` method must be re-architected to support forking a sandboxed child process. This cannot be done in the same process space and requires a re-entrant binary.
    *   **Implementation Strategy: Re-entrant Binary Fork/Exec Model**
        1.  **Introduce an internal command:** Add a new, hidden command to `cmd/gxo/main.go`, e.g., `gxo --internal-run-sandboxed-workload`. This command will be called by the daemon on itself. It will not be visible in the main help text.
        2.  **Modify `ExecuteWorkload`:**
            *   Check if `workload.SecurityContext` is defined.
            *   If **NO**, execute the module `in-process` as it does today.
            *   If **YES**:
                a.  Serialize the necessary context for the workload (its full `config.Workload` definition, any required initial state variables) into a temporary file.
                b.  Use `exec.Command(os.Args[0], "--internal-run-sandboxed-workload", "--context-file", tempFilePath)`.
                c.  Configure the `cmd.SysProcAttr` field with the required `Cloneflags` by calling helpers in `internal/sandbox/namespaces.go`.
                d.  Before starting the command, use the `internal/sandbox/cgroups` helpers to create the cgroup slice and write the resource limits. After the command starts, add its `cmd.Process.Pid` to that cgroup.
                e.  The parent daemon process will manage the child process, capture its `stdout`/`stderr` (which will contain the JSON result), and wait for it to exit.
    *   **Implementation of the internal command (`--internal-run-sandboxed-workload`):**
        1.  This command handler will *not* initialize a full engine.
        2.  It will parse the `--context-file` flag and deserialize the workload context.
        3.  It will call the `internal/sandbox/seccomp` helper to apply the seccomp-bpf filter. This **must** be done before any other significant action in the child process.
        4.  It will then instantiate and run the *single* workload's `Perform` method in-process.
        5.  Finally, it will serialize the `summary` and `error` result to `stdout` as a single JSON object for the parent daemon to read and process.

---

### **Milestone 7.2: Module Signing & Verification**

**Objective:** Implement supply chain security by verifying the cryptographic signatures of modules before execution.

**Rationale:** A sophisticated attacker might not attack the daemon directly but instead try to inject a malicious or backdoored module into the execution environment. This milestone prevents that vector by treating modules as software artifacts whose integrity and authenticity must be cryptographically verified, in line with modern supply chain security best practices (like SLSA). This is a critical feature for any organization where software supply chain integrity is a priority.

**Impacted Files & Detailed Changes:**

*   **New File: `cmd/gxo-admin/sign.go`**
    *   **Action:** Create a new `gxo-admin sign-module` command.
    *   **Rationale:** Provide a canonical, user-friendly way for module developers and platform administrators to sign their custom modules.
    *   **Implementation Detail:**
        1.  This tool will be a wrapper around the `sigs.k8s.io/release-utils/sign` and `github.com/sigstore/cosign` libraries.
        2.  It will take flags: `--module-path` (the path to the `.so` file), `--key` (path to the private key file), and `--output-signature` (path for the `.sig` file).
        3.  The command will:
            a.  Calculate the SHA256 digest of the module binary.
            b.  Use the `cosign` library to sign the digest with the provided private key (e.g., using `cosign.SignBlob`).
            c.  Write the resulting signature, encoded in base64, to the specified output file.

*   **`internal/daemon/controller.go`**
    *   **Action:** Modify the daemon's startup configuration to load a set of trusted public keys for module verification.
    *   **Implementation Detail:** The daemon's main configuration file will be updated to accept a new section:
        ```yaml
        module_verification:
          # A list of paths to PEM-encoded public keys.
          trusted_public_keys:
            - /etc/gxo/keys/prod_module_signer.pub
            - /etc/gxo/keys/dev_module_signer.pub
          # Policy can be "enforce" (fail-closed) or "log_only" (permissive).
          policy: "enforce"
        ```
        The daemon controller will load these keys into a `ModuleVerifier` object at startup.

*   **New File: `internal/engine/verifier.go`**
    *   **Action:** Create a `ModuleVerifier` component responsible for the verification logic.
*   **`internal/engine/workload_runner.go`**
    *   **Action:** This is the enforcement point. Before executing a workload, the runner must call the `ModuleVerifier`.
    *   **Implementation Detail:**
        1.  A new `ModuleVerifier` component is created during engine initialization and passed to the `WorkloadRunner`.
        2.  When `ExecuteWorkload` is called, it gets the `moduleName` from `workload.Process.Module`.
        3.  The `WorkloadRunner` calls `verifier.Verify(moduleName)`.
        4.  The `Verify` method's logic:
            a.  First, it checks against an embedded manifest of standard library modules and their digests. If the module is a built-in GXO module, it is considered trusted by default.
            b.  If it's a custom module (not in the manifest), the verifier will determine its path on the filesystem.
            c.  It will look for a corresponding signature file (e.g., `<module_path>.sig`). If not found and policy is `enforce`, it fails.
            d.  It calculates the SHA256 digest of the module binary on disk.
            e.  It uses the `cosign` library (`cosign.VerifyBlobSignature`) to verify the signature against the digest using the set of loaded trusted public keys.
            f.  If verification fails, `Verify` returns a fatal `ModuleSignatureError`.
        5.  If `verifier.Verify` returns an error, `ExecuteWorkload` immediately fails the workload without executing its `Perform` method.
