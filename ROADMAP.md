# **GXO Project Roadmap**

**Document ID:** GXO-ROADMAP-V1
**Status:** Approved Strategic Plan

## **1. Introduction**

This document outlines the high-level, strategic development plan for the GXO Automation Kernel. Its purpose is to provide a clear, phased approach to achieving a production-ready, feature-complete GXO v1.0 release. The roadmap is sequenced to ensure architectural integrity, quality, and developer experience are established as a solid foundation before the full suite of modules is implemented.

The scope of this roadmap is the single-node **GXO Automation Kernel**. Advanced, multi-node capabilities, such as the conceptually planned "GXO Fabric," are considered future work that will be explored only after the successful completion of this v1.0 plan.

## **Phase 1: Foundational Refactor - Aligning with the `Workload` Model**

**Rationale:** The current codebase (`v0.1.2a`) is a proven `run_once` task executor. However, the master architecture is built on the more powerful and expressive `Workload`, `Process`, and `Lifecycle` abstractions. This phase is the most critical as it pays down all architectural debt and aligns the entire codebase with the project's core philosophy before any new features are built upon it. It is the act of rebuilding the foundation to be stronger and more versatile.

*   **Milestone 1.1: Unified Abstraction Refactor**
    *   **Objective:** Replace the legacy `Task` concept with the formal `Workload`, `Process`, and `Lifecycle` structs throughout the entire codebase, including the configuration, engine, and API layers.
    *   **Outcome:** The engine's internal logic will natively understand and operate on the master architectural concepts, preparing it for future lifecycle policies like `supervise` and `event_driven`.

*   **Milestone 1.2: Seamless Migration Path**
    *   **Objective:** Implement tooling to ensure a smooth transition for users of the legacy `Task`-based playbook format.
    *   **Outcome:** A new `gxo migrate` CLI command will allow users to automatically convert their old playbooks to the new `Workload` format. The `gxo run` command will gain a temporary, in-memory "shim" that can execute old playbooks while printing a clear deprecation warning.

## **Phase 2: Hardening the Core - Comprehensive Test Suite**

**Rationale:** Before expanding the platform's capabilities with new modules, we must guarantee that the newly refactored Kernel is correct, stable, and resilient. This phase establishes a high quality bar for all future development and provides a safety net against regressions.

*   **Milestone 2.1: Full Unit & Integration Test Coverage**
    *   **Objective:** Achieve high test coverage (>90%) for all core Kernel packages (`internal/engine`, `internal/config`, `internal/state`, etc.).
    *   **Outcome:** A suite of tests that validate the DAG builder, the scheduler's concurrency model (including `-race` validation), the policy framework, and the stream synchronization logic, ensuring the Kernel behaves as specified under all conditions.

*   **Milestone 2.2: Fuzz Testing for Security & Robustness**
    *   **Objective:** Implement fuzz tests for critical input-handling components.
    *   **Outcome:** Fuzz tests for the YAML parser and parameter templating engine will proactively discover edge cases and potential security vulnerabilities (e.g., panics, hangs) that are difficult to find with conventional unit tests.

## **Phase 3: Developer Experience - The Playbook Mocking Framework**

**Rationale:** To drive adoption and enable the creation of complex, reliable automation, users must have the confidence to test their playbooks without affecting live systems. This phase focuses on building a first-class, GXO-native testing and validation experience.

*   **Milestone 3.1: The `gxo test` Command**
    *   **Objective:** Introduce a new top-level CLI command for running test-specific playbooks.
    *   **Outcome:** A `gxo test` command that discovers and executes playbooks matching a specific pattern (e.g., `*.test.gxo.yaml`). It will provide structured test output (PASS/FAIL) and aggregate results, integrating smoothly into CI/CD pipelines.

*   **Milestone 3.2: The `test:*` Module Suite**
    *   **Objective:** Develop a dedicated suite of modules designed for use within test playbooks.
    *   **Outcome:**
        *   **`test:mock_http_server`:** A module that can stand up a temporary, in-memory HTTP server for a test's duration. It can be configured directly in the playbook YAML to respond to specific paths and methods with predefined data, allowing users to test `http:request` workloads without network access.
        *   **`test:assert`:** A module for making assertions about the state of a playbook run. It can compare registered variables against expected values, check task statuses, and fail the test run if an assertion is not met.

## **Phase 4: The Critical Path - REST API & ETL Enablement**

**Rationale:** This phase focuses on implementing the minimum viable set of modules required to deliver on GXO's core promise: bridging the gap between systems via API calls and processing the resulting data. This unlocks the most common and powerful use cases for "glue code" replacement and data integration.

*   **Milestone 4.1: Foundational System Primitives (Layer 1)**
    *   **Objective:** Implement the core modules for interacting with the local system and controlling workflow logic.
    *   **Modules:** `exec`, `filesystem:*` suite, `control:*` suite (`assert`, `identity`, `barrier`).

*   **Milestone 4.2: REST API Client (Layer 5)**
    *   **Objective:** Implement the universal HTTP client. This is the single most important module for external integration.
    *   **Module:** `http:request` (with full support for methods, headers, bodies, and authentication helpers).

*   **Milestone 4.3: Core Data Plane & Module Alignment (Layer 4)**
    *   **Objective:** Implement the essential ETL modules and align existing module names with the canonical GXO-SL specification.
    *   **Modules:** `data:parse` (with `json` and `text_lines` support), `data:map`, `data:filter`.
    *   **Action: Module Renaming**
        *   Rename `generate:from_list` module to `data:generate_from_list`.
        *   Rename `stream:join` module to `data:join`.
        *   Update all internal references, tests, and documentation to reflect these canonical names.

## **Phase 5: Expanding the Core Standard Library**

**Rationale:** With the critical path for `run_once` workflows delivered, this phase focuses on expanding GXO's capabilities to cover a wider spectrum of automation tasks by implementing the next set of modules from the GXO-SL. The development is prioritized by layer, building upon already-completed primitives.

*   **Milestone 5.1: The Network Stack (Layers 2 & 3)**
    *   **Objective:** Enable low-level network and protocol automation.
    *   **Modules:** `connection:*` suite, `http:listen`, `http:respond`, `ssh:connect`, `ssh:command`, `ssh:script`.

*   **Milestone 5.2: Advanced Data Plane & Application Modules (Layers 4 & 5)**
    *   **Objective:** Enhance ETL capabilities and add clients for common services.
    *   **Modules:** `data:join`, `data:aggregate`, `database:query`.

*   **Milestone 5.3: The Integration Layer (Layer 6)**
    *   **Objective:** Provide opinionated, high-level wrappers for key ecosystem tools.
    *   **Modules:** `artifact:*` suite (including `object_storage:*` dependencies), `terraform:run`.

## **Phase 6: Production Hardening & Service Enablement**

**Rationale:** This phase implements the `gxo daemon`, transforming GXO from an ephemeral task runner into a true, long-running Automation Kernel. It focuses on the non-negotiable features required for production deployments: a persistent state store, a secure control plane, and the ability to manage supervised and event-driven workloads.

*   **Milestone 6.1: The `gxo daemon` and Lifecycle Supervisor**
    *   **Objective:** Implement the core `gxo daemon` process and the `supervise` and `event_driven` lifecycle reconcilers.
    *   **Key Features:** Implement the `gxo daemon` command, a gRPC API server for control, lifecycle reconcilers for `supervise` and `event_driven` workloads (including restart-backoff logic), and the `gxo ctl` command-line tool.

*   **Milestone 6.2: Control Plane Security (mTLS & RBAC)**
    *   **Objective:** Secure the `gxo daemon`'s gRPC control plane according to the security architecture.
    *   **Key Features:** Implement mandatory mTLS on the gRPC server, with support for `pki` and `allowlist` trust models. Implement a gRPC interceptor that performs Role-Based Access Control based on the client certificate's Subject Common Name.

*   **Milestone 6.3: Persistent & Encrypted State Store**
    *   **Objective:** Replace the in-memory state store with a persistent, production-grade alternative.
    *   **Key Features:** Create a `state.Store` implementation using BoltDB. Implement AEAD (AES-GCM) encryption for the state file, with the key provided via a secure mechanism. Provide offline tooling for key rotation.

*   **Milestone 6.4: Human-in-the-Loop (`Resume Context`)**
    *   **Objective:** Implement the `Resume Context` primitive to enable human-in-the-loop workflows.
    *   **Key Features:** Implement the `control:wait_for_signal` module. The daemon will manage unique, single-use tokens, persist the state of paused workflows, and expose a `Resume` gRPC endpoint. Implement the `gxo ctl resume` command to submit tokens and data payloads.

## **Phase 7: Advanced Workload & Supply Chain Security**

**Rationale:** With the daemon and its control plane secured, this phase focuses on hardening the execution environment of the workloads themselves and securing the supply chain of the modules they use.

*   **Milestone 7.1: Workload Sandboxing (`security_context`)**
    *   **Objective:** Implement OS-level sandboxing for workloads as defined in the `security_context` configuration block.
    *   **Key Features:** Update the configuration parser to accept the `security_context`. The `WorkloadRunner` will be enhanced to programmatically create and enter specified Linux namespaces (`mount`, `pid`), apply cgroup resource limits, and apply a restrictive `seccomp` profile before module execution.

*   **Milestone 7.2: Module Signing & Verification**
    *   **Objective:** Implement supply chain security by verifying the cryptographic signatures of modules before execution.
    *   **Key Features:** Create tooling to sign module binaries (e.g., via `cosign`). The `gxo daemon` will be configured with trusted public keys and a `fail-closed` policy. The engine will verify module signatures before execution, rejecting any that are invalid or untrusted.

## **Phase 8: Completing the GXO Standard Library**

**Rationale:** With a secure, production-ready kernel, this final phase focuses on implementing the full suite of modules defined in the GXO-SL, unlocking the platform's complete range of capabilities for cloud, container, and cryptographic automation.

*   **Milestone 8.1: Advanced Protocol & Crypto Modules**
    *   **Objective:** Implement modules for DNS, advanced SSH, and core cryptographic functions.
    *   **Modules:** `dns:query`, `ssh:upload`, `ssh:download`, `crypto:generate_key`, `crypto:encrypt`, `crypto:decrypt`.

*   **Milestone 8.2: Container & Vault Integration**
    *   **Objective:** Provide first-class integration with Docker/containers and HashiCorp Vault.
    *   **Modules:** `docker:run`, `docker:build`, `docker:push`, `vault:read`, `vault:write`.

*   **Milestone 8.3: High-Level Cloud Service Wrappers**
    *   **Objective:** Implement opinionated, high-level wrappers for common cloud operations.
    *   **Modules:** `aws:s3_sync`, `aws:ec2_instance`, `gcp:gcs_sync`, `gcp:gce_instance`, `azure:blob_sync`, `azure:vm_instance`, and other secrets manager integrations (`aws_secretsmanager:read`, etc.).

---

# **GXO Master Engineering Plan**

## **Phase 1: Foundational Refactor - Aligning with the `Workload` Model**

**Document ID:** GXO-ENG-PLAN-P1
**Version:** 1.0
**Date:** 2025-07-08
**Status:** Approved for Execution

### **Objective**

This foundational phase refactors the entire codebase to align with the master architecture's core abstractions: the `Workload`, `Process`, and `Lifecycle`. The current `Task` struct is a specific implementation of a `run_once` workload. This refactor makes the engine's core logic lifecycle-aware, a non-negotiable prerequisite for implementing the `gxo daemon` and fulfilling the project's vision.

### **Rationale**

The existing codebase is a stable and proven `run_once` task executor. However, to evolve into a true Automation Kernel capable of managing long-running services and event-driven workflows, the core data models must be elevated. A piecemeal approach of adding new features on top of the old `Task` model would lead to technical debt, inconsistent APIs, and a confusing user experience.

This phase is executed first to pay down all architectural debt upfront. By establishing the `Workload` as the single, unified primitive, we ensure that all future development—from the `gxo daemon` to new modules—is built upon a consistent, robust, and philosophically sound foundation. This is a "measure twice, cut once" approach that prioritizes long-term architectural integrity over short-term feature velocity.

---

### **Milestone 1.1: Redefine Core Configuration Model**

**Objective:** Introduce the `Workload`, `Process`, and `Lifecycle` structs into the configuration model, replacing the legacy `Task` concept as the primary declarative unit.

**Rationale:** This is the most critical architectural change. It makes the system's intent explicit by separating *what* a workload does (its `Process`) from *how* it is managed by the kernel (its `Lifecycle`). This establishes the conceptual foundation for all future development, including process supervision and event-driven execution.

**Impacted Files & Detailed Changes:**

*   **`internal/config/policy.go`**
    *   **Action:** Centralize all policy-related definitions, improving separation of concerns.
    *   **Implementation Detail:**
        ```go
        package config

        // LifecyclePolicy defines how the GXO kernel manages a workload's execution.
        type LifecyclePolicy struct {
            Policy       string `yaml:"policy"` // "run_once", "supervise", "event_driven", "scheduled"
            
            // Fields for 'supervise' lifecycle
            RestartPolicy string `yaml:"restart_policy,omitempty"` // "always", "on_failure", "never"
            
            // Fields for 'scheduled' lifecycle
            Cron string `yaml:"cron,omitempty"`
            
            // Fields for 'event_driven' lifecycle
            Source string `yaml:"source,omitempty"`
        }
        ```

*   **`internal/config/config.go`**
    *   **Action:** Perform a comprehensive refactor of the core data structures. The `Task` struct will be removed and replaced by `Workload` and `Process`. The top-level `Playbook` will be updated to use `workloads` instead of `tasks`.
    *   **Implementation Detail:**
        ```go
        // Process defines the logic of a workload: what it does.
        type Process struct {
            Module string                 `yaml:"module"` // Formerly 'type'
            Params map[string]interface{} `yaml:"params,omitempty"`
        }

        // Workload is the atomic unit of automation in GXO.
        type Workload struct {
            Name          string                 `yaml:"name"`
            Lifecycle     LifecyclePolicy        `yaml:"lifecycle"`
            Process       Process                `yaml:"process"`
            
            // Legacy task-level fields, now part of the Workload
            Register      string                 `yaml:"register,omitempty"`
            IgnoreErrors  bool                   `yaml:"ignore_errors,omitempty"`
            When          string                 `yaml:"when,omitempty"`
            Loop          interface{}            `yaml:"loop,omitempty"`
            LoopControl   *LoopControlConfig     `yaml:"loop_control,omitempty"`
            Retry         *RetryConfig           `yaml:"retry,omitempty"`
            Timeout       string                 `yaml:"timeout,omitempty"`
            StatePolicy   *StatePolicy           `yaml:"state_policy,omitempty"`
            InternalID    string                 `yaml:"-"`
        }

        // Playbook represents the top-level structure of a GXO playbook YAML file.
        type Playbook struct {
            Name          string                 `yaml:"name"`
            SchemaVersion string                 `yaml:"schemaVersion"`
            Vars          map[string]interface{} `yaml:"vars,omitempty"`
            Workloads     []Workload             `yaml:"workloads"` // Replaces 'tasks'
            StatePolicy   *StatePolicy           `yaml:"state_policy,omitempty"`
        }
        ```

---

### **Milestone 1.2: Update Schema and Validation Logic**

**Objective:** Align the static validation layer (JSON Schema and Go-based logical validation) with the new `Workload` configuration model to provide immediate and accurate user feedback.

**Rationale:** Static validation is the first line of defense against user misconfiguration. Keeping it synchronized with the core model provides immediate, clear feedback and prevents invalid configurations from ever reaching the engine. This is critical for developer experience and system stability.

**Impacted Files & Detailed Changes:**

*   **`internal/config/gxo_schema_v1.0.0.json`**
    *   **Action:** This file must be comprehensively updated to define the `v1.0.0` schema based on `Workloads`.
    *   **Detailed Changes:**
        1.  Under `properties`, change the `tasks` property to `workloads`. Update the `required` array to include `workloads`.
        2.  In `definitions`, rename the `Task` definition to `Workload`.
        3.  Update the new `Workload` definition:
            *   Remove the `type` and `params` properties.
            *   Add a required `process` property that references a new `Process` definition.
            *   Add a required `lifecycle` property that references a new `LifecyclePolicy` definition.
            *   Update the `required` array for a `Workload` to be `["name", "lifecycle", "process"]`.
        4.  Add a new `Process` definition in `definitions`:
            ```json
            "Process": {
              "type": "object",
              "properties": { "module": { "type": "string" }, "params": { "type": "object" } },
              "required": ["module"]
            }
            ```
        5.  Add a new `LifecyclePolicy` definition in `definitions`:
            ```json
            "LifecyclePolicy": {
              "type": "object",
              "properties": {
                "policy": { "type": "string", "enum": ["run_once", "supervise", "event_driven", "scheduled"] },
                "restart_policy": { "type": "string", "enum": ["always", "on_failure", "never"] }
              },
              "required": ["policy"]
            }
            ```

*   **`internal/config/validation.go`**
    *   **Action:** Rewrite the `ValidatePlaybookStructure` function to operate on the new `Workload` structs and add new validation rules specific to lifecycles.
    *   **Detailed Changes:**
        1.  The main loop must change from `for i := range p.Tasks` to `for i := range p.Workloads`.
        2.  All log messages and error text must be updated from "task" to "workload".
        3.  All references to `task.Type` must be changed to `workload.Process.Module`.
        4.  Add new validation logic for required nested objects (`lifecycle`, `process`) and to ensure that `gxo run` only accepts `run_once` lifecycles, providing a clear error message for other policies.

---

### **Milestone 1.3: Implement Playbook Migration Shim and Tool**

**Objective:** Provide a seamless, robust, and user-friendly upgrade path for all existing `v0.1.2a` playbooks to the new `v1.0.0` `Workload`-based format.

**Rationale:** A hard break with the past is user-hostile. The in-memory shim provides immediate operational continuity by allowing `gxo run` to function with old playbooks (with a clear deprecation warning). The `gxo migrate` command provides the permanent, user-initiated solution.

**Impacted Files & Detailed Changes:**

*   **`internal/config/load.go`**
    *   **Action:** Refactor the `LoadPlaybook` function to implement the in-memory migration shim.
    *   **Detailed Logic:**
        1.  **Light Unmarshal:** In `LoadPlaybook`, before strict unmarshaling, define a local `migrationHelper` struct and unmarshal the YAML into it.
            ```go
            type migrationHelper struct {
                // Legacy key
                Tasks     []Task `yaml:"tasks"`
                // New key
                Workloads []Workload `yaml:"workloads"`
                // Pass-through other playbook fields
                Name          string                 `yaml:"name"`
                SchemaVersion string                 `yaml:"schemaVersion"`
                Vars          map[string]interface{} `yaml:"vars,omitempty"`
                StatePolicy   *StatePolicy           `yaml:"state_policy,omitempty"`
            }
            // yaml.Unmarshal(playbookYAML, &helper)
            ```
        2.  **Detect & Migrate:** Check if `len(helper.Tasks) > 0 && len(helper.Workloads) == 0`.
        3.  **Log Deprecation:** If migrating, log a `WARN` message: `"Playbook uses the deprecated 'tasks' key. It is being migrated in-memory. To upgrade the file permanently, run 'gxo migrate -f <file>'."`
        4.  **Perform Conversion:** Iterate over `helper.Tasks` and transform into `helper.Workloads`.
            ```go
            // Inside the migration logic loop
            newWorkloads := make([]Workload, len(helper.Tasks))
            for i, task := range helper.Tasks {
                newWorkloads[i] = Workload{
                    Name: task.Name,
                    Lifecycle: LifecyclePolicy{
                        Policy: "run_once", // Default lifecycle for all legacy tasks
                    },
                    Process: Process{
                        Module: task.Type,
                        Params: task.Params,
                    },
                    // Copy all other fields from task to workload
                    Register: task.Register,
                    IgnoreErrors: task.IgnoreErrors,
                    When: task.When,
                    Loop: task.Loop,
                    LoopControl: task.LoopControl,
                    Retry: task.Retry,
                    Timeout: task.Timeout,
                    StatePolicy: task.StatePolicy,
                }
            }
            helper.Workloads = newWorkloads
            helper.Tasks = nil // Important: nil out the old slice
            ```
        5.  **Re-Marshal & Strict Unmarshal:** Convert the modified `helper` struct back into YAML bytes, then use these bytes for the final, strict unmarshal into the official `config.Playbook` struct.

*   **New File: `cmd/gxo/migrate.go`**
    *   **Action:** Create this file to define the new `gxo migrate` command using the Cobra library.
    *   **Detailed Logic (`RunE` function):**
        1.  Define a required `-f, --filename` flag.
        2.  Read the legacy playbook file.
        3.  Call `config.LoadPlaybook` (which now contains the migration shim) to get a modern, in-memory `Playbook` object.
        4.  Marshal the returned `Playbook` object back to YAML using an encoder that does not emit zero-value fields (`omitempty`).
        5.  Print the resulting YAML to standard output.

*   **New File: `internal/config/load_test.go`**
    *   **Action:** Create a dedicated test file to validate the migration shim's behavior.
    *   **Required Test Cases:** `TestLoadPlaybook_WithLegacyTasksKey`, `TestLoadPlaybook_WithModernWorkloadsKey`, `TestLoadPlaybook_WithAmbiguousKeys`.

---

### **Milestone 1.4: Execute Cross-Cutting Refactor**

**Objective:** Systematically propagate the `Task` -> `Workload` rename and the `Type` -> `Process.Module` change through every layer of the application.

**Rationale:** A partial rename is a source of confusion and a breeding ground for bugs. This must be a single, sweeping operation to ensure the entire system speaks the new architectural language.

**Impacted Files & Detailed Changes (Checklist):**

1.  **Engine Core (`internal/engine/`):**
    *   Rename `task_runner.go` to `workload_runner.go`.
    *   `workload_runner.go`: Rename `TaskRunner` to `WorkloadRunner`, `ExecuteTask` to `ExecuteWorkload`.
    *   `dag.go`: Rename `Node.Task` to `Node.Workload`.
    *   `engine.go`: Rename variables and update log messages.
    *   Update all `engine_*_test.go` files to use `workloads:`.

2.  **Public API & Events (`pkg/gxo/v1/` and `internal/events/`):**
    *   `pkg/gxo/v1/api.go`: Rename `TaskResult` to `WorkloadResult`, `ExecutionReport.TaskResults` to `WorkloadResults` (including JSON tags).
    *   `pkg/gxo/v1/events/bus.go`: Rename constants (`TaskStart` -> `WorkloadStart`) and `Event` struct fields (`TaskName` -> `WorkloadName`).
    *   `internal/engine/engine.go`: Update event emission logic.
    *   `internal/events/metrics_listener.go`: Update event handling logic.

3.  **Module & Template Interface (`internal/module/` and `internal/template/`):**
    *   `internal/module/module.go`: Rename `ExecutionContext.Task()` to `ExecutionContext.Workload()`.
    *   `internal/template/template.go`: Update `ExtractVariables` to recognize `_gxo.workloads.<name>.status`, with backward compatibility.

---

## **Phase 2: Hardening the Core - Comprehensive Test Suite**

**Document ID:** GXO-ENG-PLAN-P2
**Version:** 1.0
**Date:** 2025-07-09
**Status:** Approved for Execution

### **Objective**

Before implementing the `gxo daemon` or other new features, establish a comprehensive, production-grade test suite for the newly refactored `v1.0.0` foundation. This phase creates all necessary `_test.go` and `_bench_test.go` files, ensuring correctness, concurrency safety, and performance of the existing codebase. It is a dedicated phase to pay down any "testing debt" and establish a high quality bar for all future development.

### **Rationale**

The Phase 1 refactor fundamentally changes the core data structures and logic of the GXO engine. While individual components may have tests, the system as a whole needs a rigorous, end-to-end validation to confirm that all refactored parts integrate correctly. This phase ensures that the engine is not just functional, but also resilient against common issues like race conditions, deadlocks, and invalid user input. Building this test suite now provides a safety net that will catch regressions as we move into more complex feature development in subsequent phases.

---

### **Milestone 2.1: CLI and System Abstraction Test Suites**

**Objective:** Ensure the command-line interface and underlying OS command execution abstractions are fully tested and behave as expected under various conditions.

**Rationale:** The CLI is the primary user entry point for `gxo run`. Its behavior, including flag parsing, error reporting, and exit codes, must be predictable and correct. The command execution abstraction is a critical component for the `exec` module and must be proven to be robust and secure.

**Impacted Files & Detailed Changes:**

*   **New File: `cmd/gxo/main_test.go`**
    *   **Action:** Create a new test file dedicated to testing the `main` package's handler functions.
    *   **Implementation Detail:**
        ```go
        // Example test case for successful execution
        func TestRunExecute_Success(t *testing.T) {
            // Setup: Capture stdout/stderr, mock os.Exit
            oldStdout := os.Stdout
            r, w, _ := os.Pipe()
            os.Stdout = w
            // ...
            
            // Action: Call main.runExecuteCommand(...) with a valid playbook
            
            // Assert: Check exit code, check captured stdout for success messages
        }
        ```
    *   **Required Test Cases:** `TestRunExecute_Success`, `TestRunExecute_ValidationFailure`, `TestRunExecute_RuntimeFailure`, `TestRunValidate_Success`, `TestRunValidate_Failure`.

*   **New File: `internal/command/command_test.go`**
    *   **Action:** Create a new test file to exhaustively test the `defaultRunner.Run` method.
    *   **Implementation Detail:**
        ```go
        // Define a mock command runner for tests
        type mockCmdRunner struct {
            // fields to control mock behavior (e.g., exit code, stdout)
        }
        func (m *mockCmdRunner) Run(...) (*command.CommandResult, error) {
            // return canned results
        }
        ```
    *   **Required Test Cases:** `TestRun_Success`, `TestRun_NonZeroExit`, `TestRun_ContextCancellation`, `TestRun_CommandNotFound`.

---

### **Milestone 2.2: Configuration and Engine Core Test Suites**

**Objective:** Test the core logic of configuration loading, DAG building, and workload execution orchestration to ensure the refactored engine is functionally correct.

**Rationale:** These components form the brain of GXO. Any errors in DAG construction, dependency resolution, or status management can lead to incorrect execution order, deadlocks, or silent failures. These tests validate the fundamental correctness of the orchestration logic.

**Impacted Files & Detailed Changes:**

*   **`internal/config/load_test.go`:** Enhance file to test `LoadPlaybook` with the new `Workload` syntax. Test cases: `TestLoadPlaybook_ValidWorkload`, `TestLoadPlaybook_MissingRequiredFields`.
*   **New File: `internal/config/validation_test.go`:** Create a dedicated test suite for `ValidatePlaybookStructure`. Test cases: `TestValidate_DependencyCycle`, `TestValidate_SelfReference`, `TestValidate_InvalidIdentifier`, `TestValidate_UndefinedReference`.
*   **New File: `internal/engine/channel_manager_test.go`:** Test the `ChannelManager`'s logic for creating and managing streaming channels. Test cases: `TestCreateChannels_FanInFanOut`, `TestManagedChannel_OverflowBlock`.
*   **New File: `internal/engine/dag_test.go`:** Isolate and test the `BuildDAG` function. Test cases: `TestBuildDAG_StateAndStreamDependencies`, `TestBuildDAG_PolicyResolution`.
*   **`engine_test.go`, `engine_policy_test.go`, `engine_security_test.go`:** A full review and update of all existing engine-level integration tests to use the new `workloads:` syntax.

---

### **Milestone 2.3: Advanced Concurrency and Fuzz Testing**

**Objective:** Go beyond standard unit tests to find more subtle bugs in complex, concurrent, or security-sensitive areas of the codebase.

**Rationale:** Concurrency bugs and parsing vulnerabilities are notoriously difficult to find with simple unit tests. Fuzzing and targeted race condition tests are necessary to build confidence in the system's robustness.

**Impacted Files & Detailed Changes:**

*   **New File: `internal/config/fuzz_test.go`**
    *   **Action:** Create a fuzz test for `config.LoadPlaybook` using Go's `testing.F` framework.
    *   **Implementation Detail:**
        ```go
        func FuzzLoadPlaybook(f *testing.F) {
            // Seed with valid examples
            f.Add([]byte(`name: "valid_fuzz" ...`))
            
            f.Fuzz(func(t *testing.T, data []byte) {
                // The only goal is to not panic
                _, _ = config.LoadPlaybook(data, "fuzz_input")
            })
        }
        ```

*   **New File: `internal/events/backpressure_test.go`**
    *   **Action:** Create a targeted integration test for the `ChannelEventBus` backpressure mechanism.
    *   **Implementation Detail:** Create a bus with a small buffer, fill it, and assert that a subsequent `Emit` call drops the event and logs a warning.

*   **`engine_security_test.go`:** Add a new test case, `TestSecretRedaction_RaceCondition`, to target the thread-safety of the secret redaction mechanism using multiple goroutines and `go test -race`.

---

## **Phase 3: Developer Experience - The Playbook Mocking Framework**

**Document ID:** GXO-ENG-PLAN-P3
**Version:** 1.0
**Date:** 2025-07-10
**Status:** Approved for Execution

### **Objective**

To accelerate adoption and enable the creation of complex, reliable automation, users must have the confidence to test their playbooks without affecting live systems. This phase focuses on building a first-class, GXO-native testing and validation experience by introducing a dedicated test runner and a suite of mocking modules.

### **Rationale**

A robust testing framework is a feature, not an afterthought. Providing developers with the tools to write unit and integration tests for their own playbooks significantly increases the quality and reliability of the automation they build. It allows them to verify complex logic, test error handling paths, and validate data transformations in a fast, isolated, and repeatable manner. By building this framework directly into GXO, we treat "playbook testing" as a core competency of the platform, enabling a Test-Driven Development (TDD) approach for automation engineers.

---

### **Milestone 3.1: The `gxo test` Command**

**Objective:** Introduce a new top-level CLI command for discovering and running test-specific playbooks, providing structured output suitable for both human and CI/CD consumption.

**Rationale:** A dedicated test runner provides a clear separation between production execution (`gxo run`) and testing. It allows for test-specific configurations, output formats, and behaviors, creating a user experience that is familiar to software developers and easy to integrate into automated pipelines.

**Impacted Files & Detailed Changes:**

*   **`cmd/gxo/main.go`:** Add a new `test` command to the root Cobra command.
*   **New File: `cmd/gxo/test.go`:** Define the `gxo test` command, its flags (`-v`, `--run`), and its execution logic.
    *   **Detailed Logic (`runTestCommand` function):**
        1.  **Discovery:** Use `filepath.Walk` to discover files matching `*.test.gxo.yaml`.
        2.  **Execution Loop:** Iterate through each discovered test file. Inside the loop:
            ```go
            // Simplified logic
            fmt.Printf("=== RUN   %s\n", testFile.Path)
            engine := createNewTestEngine() // Helper to get a fresh engine
            report, err := engine.RunPlaybook(ctx, testFile.Content)
            if err != nil || report.OverallStatus != "Completed" {
                fmt.Printf("--- FAIL: %s (%v)\n", testFile.Name, report.Duration)
                // Print detailed error from report
            } else {
                fmt.Printf("--- PASS: %s (%v)\n", testFile.Name, report.Duration)
            }
            ```
        3.  **Exit Code:** Maintain a boolean flag. If any test fails, set it to true. Exit `1` if the flag is true, else `0`.

---

### **Milestone 3.2: The `test:*` Module Suite**

**Objective:** Develop a dedicated suite of modules designed for use within test playbooks to enable mocking of external systems and making assertions about playbook state.

**Rationale:** To write effective unit tests for playbooks, developers need to control the test environment completely. This requires the ability to mock external dependencies like HTTP APIs and to make concrete assertions about the results of the playbook run. These modules provide those fundamental testing primitives.

**Impacted Files & Detailed Changes:**

*   **New File: `modules/test/mock_http_server/mock_http_server.go`**
    *   **Action:** Create the `test:mock_http_server` module.
    *   **Synopsis:** Stands up a temporary, in-memory HTTP server for the duration of a test.
    *   **Implementation (`Perform` method):**
        1.  Parse the `handlers` parameter.
        2.  Create a `http.ServeMux`.
        3.  For each handler, register a function on the mux that checks `http.Request.Method` and `http.Request.URL.Path`.
        4.  If a request matches, write the configured `status_code`, `headers`, and `body` to the `http.ResponseWriter`.
        5.  Start `httptest.NewServer` with the configured mux.
        6.  The `Perform` method must block until its context is cancelled. A `select { case <-ctx.Done(): }` will achieve this. This ensures the server runs for the whole playbook.
        7.  Return `{ "server_url": server.URL }` as the summary.

*   **New File: `modules/test/assert/assert.go`**
    *   **Action:** Create the `test:assert` module.
    *   **Synopsis:** Makes assertions about the state of a playbook run.
    *   **Implementation (`Perform` method):**
        1.  Parse the `assertions` list parameter.
        2.  Loop through each assertion map.
        3.  Use a `switch` statement on the keys of the assertion map (`equal_to`, `contains`, `is_true`, etc.).
        4.  Inside each case, perform the corresponding check (e.g., `reflect.DeepEqual` for `equal_to`).
        5.  If an assertion fails, return an immediate fatal error: `return nil, gxoerrors.NewValidationError(...)`.
        6.  If all assertions pass, return `{ "assertions_passed": count }`.

---

## **Phase 4: The Critical Path - REST API & ETL Enablement**

**Document ID:** GXO-ENG-PLAN-P4
**Version:** 1.0
**Date:** 2025-07-10
**Status:** Approved for Execution

### **Objective**

Implement the minimum viable set of modules required to deliver on GXO's core promise: bridging the gap between systems via API calls and processing the resulting data. This phase unlocks the most common and powerful use cases for "glue code" replacement and data integration, providing immediate, high-value capabilities to users.

### **Rationale**

While the full GXO Standard Library is extensive, a small subset of modules enables a vast majority of common automation workflows. Prioritizing this "critical path" allows the project to deliver a highly useful product faster. The ability to call a REST API, parse its JSON response, and act on that data is the quintessential "glue code" task that GXO is designed to solve elegantly. This phase delivers that core experience.

---

### **Milestone 4.1: Foundational System Primitives (Layer 1)**

**Objective:** Implement the core modules for interacting with the local system and controlling workflow logic.
**Impacted Files:**
*   `modules/exec/exec.go`
*   New Directory: `modules/filesystem/`
*   New Directory: `modules/control/`

*   **Module API: `exec`**
    *   **Synopsis:** Executes local system commands.
    *   **Parameters:** `command` (string, required), `args` (list[string]), `environment` (list[string]).
    *   **Summary:** `{ "stdout": string, "stderr": string, "exit_code": int }`.

*   **Module API: `filesystem:manage`**
    *   **Synopsis:** Idempotently manages the state of files and directories.
    *   **Parameters:** `path` (string, required), `state` (string, required, choices: `present`, `absent`, `directory`), `mode` (string), `owner` (string), `group` (string), `recursive` (bool).
    *   **Summary:** `{ "path": string, "state": string, "changed": bool }`.

---

### **Milestone 4.2: REST API Client (Layer 5)**

**Objective:** Implement the universal HTTP client.
**Impacted Files:** `modules/http/request.go`

*   **Module API: `http:request`**
    *   **Synopsis:** The universal, all-in-one client for any HTTP-based API.
    *   **Parameters:**
        | Name | Type | Required? | Description |
        |---|---|---|---|
        | `url` | string | Yes | The URL of the endpoint to request. |
        | `method` | string | No | HTTP method (GET, POST, etc.). Defaults to `GET`. |
        | `headers` | map | No | A map of request headers. |
        | `body` | string | No | The request body. |
        | `timeout` | string | No | Request timeout (e.g., "10s"). |
        | `skip_tls_verify` | bool | No | If true, skips TLS certificate verification. |
        | `auth` | map | No | A map specifying authentication, e.g., `{ "basic": { "user": "u", "pass": "p" } }`. |
    *   **Summary:** `{ "status_code": int, "headers": map, "body": string, "json_body": any, "latency_ms": int }`.

---

### **Milestone 4.3: Core Data Plane & Module Alignment (Layer 4)**

**Objective:** Implement essential ETL modules and align existing module names with the canonical GXO-SL specification.
**Impacted Files:** `modules/data/` directory.

*   **Module API: `data:parse`**
    *   **Synopsis:** Converts raw data into a stream of structured records.
    *   **Parameters:** `content` (string, required), `format` (string, required, choices: `json`, `text_lines`).
    *   **Output Stream:** A stream of `map[string]interface{}` records.

*   **Module API: `data:map`**
    *   **Synopsis:** Transforms each record in a stream using a template.
    *   **Parameters:** `template` (string, required).
    *   **Stream Behavior:** Consumes one stream, produces one stream.

*   **Action: Module Renaming**
    *   Execute `git mv internal/modules/generate/from_list internal/modules/data/generate_from_list`. Update registration name to `data:generate_from_list`.
    *   Execute `git mv internal/modules/stream/join internal/modules/data/join`. Update registration name to `data:join`.
    *   Update all internal references, tests, and documentation.

---

## **Phase 5: Expanding the Core Standard Library**

**Document ID:** GXO-ENG-PLAN-P5
**Version:** 1.0
**Date:** 2025-07-11
**Status:** Approved for Execution

### **Objective**

With the critical path for `run_once` workflows delivered, this phase focuses on expanding GXO's capabilities to cover a wider spectrum of automation tasks by implementing the next set of modules from the GXO-SL. The development is prioritized by layer, building upon already-completed primitives.

### **Rationale**

A rich "batteries-included" standard library is what transforms a powerful engine into a productive and versatile platform. Implementing this set of modules demonstrates the robustness of the underlying GXO Automation Model and provides users with a comprehensive, first-party toolkit for a wide array of automation challenges.

---

### **Milestone 5.1: The Network Stack (Layers 2 & 3)**

*   **Objective:** Enable low-level network automation and allow users to build custom GXO-native network services.
*   **New Directory: `internal/connections`**
    *   **`manager.go`:** Create a new `ConnectionManager` service within the GXO Kernel. It will use a `sync.Map` to hold open `net.Conn` objects, keyed by a UUID `connection_id`.
*   **New Directory: `modules/connection/`**
    *   **`listen.go`:** `connection:listen` module. Its `Perform` method will start a `net.Listener` in a goroutine. On `Accept()`, it will store the `net.Conn` in the `ConnectionManager` and emit a record with the `{ "connection_id": "..." }` on its output stream.
    *   Other modules (`open`, `read`, `write`, `close`) will take a `connection_id` parameter, look up the `net.Conn` in the `ConnectionManager`, and perform the corresponding I/O operation.
*   **`modules/http/listen.go` & `respond.go`:** The `http:listen` module will consume the stream from `connection:listen`, parse HTTP requests, and produce a stream of `{ "request_id": "..." }` handles. `http:respond` will use this handle to reply.
*   **New Directory: `modules/ssh/`:** Implement `ssh:connect`, `ssh:command`, `ssh:script` using the standard `crypto/ssh` library.

### **Milestone 5.2: Advanced Data Plane & Application Modules (Layers 4 & 5)**

*   **Objective:** Enhance ETL capabilities and add clients for common services.
*   **`modules/data/aggregate.go`:** Implement `data:aggregate`. This will be a stateful streaming module. Its `Perform` method will maintain an internal map to store aggregate state (e.g., counts, sums). It will use `time.Ticker` for windowed aggregation or a simple counter for count-based aggregation to flush results.
*   **`modules/database/query.go`:** Implement `database:query`. It will use Go's standard `database/sql` package. For `SELECT` queries, its `Perform` method will use a `for rows.Next()` loop to iterate over the result set and send each row as a record on its output stream.

### **Milestone 5.3: The Integration Layer (Layer 6)**

*   **Objective:** Provide opinionated, high-level wrappers for key ecosystem tools.
*   **New Directory: `modules/object_storage/`:** First, implement a generic Layer 5 `object_storage` suite (`get_object`, `put_object`) for S3-compatible APIs.
*   **`modules/artifact/upload.go`:** The `artifact:upload` module will use `object_storage:put_object`. It will compute a file checksum, upload the file, and return a structured **Artifact Handle** (a map with name, version, checksum, remote location) in its `summary`.
*   **`modules/terraform/run.go`:** The `terraform:run` module will be a wrapper around the `exec` module. After a successful `terraform apply`, its `Perform` method will execute a second command, `terraform output -json`, parse the output, and return the structured data in its `summary`.

---

## **Phase 6: Production Hardening & Service Enablement**

**Document ID:** GXO-ENG-PLAN-P6
**Version:** 1.0
**Date:** 2025-07-12
**Status:** Approved for Execution

### **Objective**

Implement the `gxo daemon`, transforming GXO from an ephemeral task runner into a true, long-running Automation Kernel. This phase focuses on the non-negotiable features required for production deployments: a persistent state store, a secure control plane, and the ability to manage supervised and event-driven workloads.

### **Rationale**

The architectural vision of GXO as a unified runtime for services, events, and tasks can only be realized through a persistent, long-running daemon process. This phase builds that daemon, its secure control plane, and the core lifecycle reconcilers, unlocking the platform's most powerful capabilities and preparing it for production use.

---

### **Milestone 6.1: The `gxo daemon` and Lifecycle Supervisor**

*   **Objective:** Implement the core `gxo daemon` process and the `supervise` and `event_driven` lifecycle reconcilers.
*   **New Files & Detailed Changes:**
    *   **New File: `api/v1/daemon.proto`:** Define the gRPC service.
        ```protobuf
        syntax = "proto3";
        package gxo.daemon.v1;
        
        service GxoDaemon {
          rpc ApplyPlaybook(ApplyRequest) returns (ApplyResponse);
          rpc GetWorkloadStatus(StatusRequest) returns (StatusResponse);
          rpc ResumeWorkflow(ResumeRequest) returns (ResumeResponse);
        }
        // ... define message types ...
        ```
    *   **New File: `cmd/gxo/daemon.go`:** Create the `gxo daemon` Cobra command. It initializes the engine, starts the gRPC server, and enters the main reconciliation loop.
    *   **New File: `internal/daemon/reconciler.go`:** Implement the `supervise` reconciler.
        ```go
        // Simplified logic for supervise reconciler loop
        func (r *Reconciler) runSuperviseLoop(ctx context.Context, workload *config.Workload) {
            for {
                select { case <-ctx.Done(): return }
                
                _, err := r.engine.RunWorkload(ctx, workload) // New engine method
                if err == nil {
                    // Handle case where supervised process exits cleanly
                }
                
                // Implement exponential backoff for restarts
                delay := calculateBackoff(...) 
                time.Sleep(delay)
            }
        }
        ```
    *   **New Directory: `cmd/gxo-ctl/`:** Create the `gxo-ctl` CLI binary.

### **Milestone 6.2: Control Plane Security (mTLS & RBAC)**

*   **Objective:** Secure the `gxo daemon`'s gRPC control plane according to the security architecture.
*   **Impacted Files & Detailed Changes:**
    *   **`internal/daemon/server.go`:**
        *   **mTLS Integration:** Modify gRPC server initialization.
            ```go
            // Load server cert and key, and client CA
            tlsConfig := &tls.Config{
                Certificates: []tls.Certificate{serverCert},
                ClientCAs:    clientCAs,
                ClientAuth:   tls.RequireAndVerifyClientCert,
            }
            serverOptions := []grpc.ServerOption{grpc.Creds(credentials.NewTLS(tlsConfig))}
            grpcServer := grpc.NewServer(serverOptions...)
            ```
    *   **New File: `internal/daemon/interceptor/auth.go`:**
        *   **Action:** Create a unary gRPC interceptor for authorization.
        *   **RBAC Logic:**
            ```go
            func (i *AuthInterceptor) Authorize(ctx context.Context, ...) (interface{}, error) {
                p, ok := peer.FromContext(ctx)
                if !ok { return nil, status.Error(codes.Unauthenticated, "no peer found") }
                
                tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
                // ... extract client cert subject CN ...
                
                role := i.rbacPolicy.GetRole(clientCN)
                if !role.CanAccess(method) {
                    return nil, status.Error(codes.PermissionDenied, "access denied")
                }
                return handler(ctx, req)
            }
            ```

### **Milestone 6.3: Persistent & Encrypted State Store**

*   **Objective:** Replace the in-memory state store with a persistent, production-grade alternative.
*   **New Files & Detailed Changes:**
    *   **New Directory: `internal/state/boltdb/`**
    *   **`store.go`:** Implement the `state.Store` interface using BoltDB.
        ```go
        func (s *BoltStore) Set(key string, value interface{}) error {
            return s.db.Update(func(tx *bolt.Tx) error {
                b := tx.Bucket([]byte(s.bucketName))
                // Serialize value to bytes (e.g., JSON)
                // Encrypt bytes using AEAD cipher
                // return b.Put([]byte(key), encryptedBytes)
            })
        }
        ```
    *   **`encryption.go`:** Implement AEAD (AES-GCM) encryption/decryption helpers.
    *   **New Directory: `cmd/gxo-admin/`**
        *   **`rekey.go`:** Create the `gxo-admin rekey-state` command for offline state re-encryption.

### **Milestone 6.4: Human-in-the-Loop (`Resume Context`)**

*   **Objective:** Implement the `Resume Context` primitive to enable human-in-the-loop workflows.
*   **New Files & Detailed Changes:**
    *   **`modules/control/wait_for_signal.go`:** Implement `control:wait_for_signal`. Its `Perform` method will return a special, sentinel error.
    *   **`internal/daemon/reconciler.go`:** When the "pause" sentinel error is received, the reconciler will generate a unique token and store it and the workflow's state in BoltDB.
    *   **`internal/daemon/server.go`:** Add a `Resume(token, payload)` RPC. It will find the workflow, merge the `payload` into its state under `_gxo.resume_payload`, and signal the reconciler to continue.
    *   **`cmd/gxo-ctl/resume.go`:** Add the `gxo-ctl resume --token <token> --payload '{...}'` command.

---

## **Phase 7: Advanced Workload & Supply Chain Security**

**Document ID:** GXO-ENG-PLAN-P7
**Version:** 1.0
**Date:** 2025-07-13
**Status:** Approved for Execution

### **Objective**

With the daemon and its control plane secured, this phase focuses on hardening the execution environment of the workloads themselves and securing the supply chain of the modules they use.

### **Rationale**

A secure platform requires defense in depth. While Phase 6 secured the "front door," this phase builds the "internal walls" by isolating workloads from each other and the host system. It also secures the "supply chain" by ensuring that only trusted, verified modules can be executed, preventing the introduction of malicious code into the platform.

---

### **Milestone 7.1: Workload Sandboxing (`security_context`)**

*   **Objective:** Implement OS-level sandboxing for workloads as defined in the `security_context` configuration block.
*   **Impacted Files & Detailed Changes:**
    *   **`internal/config/config.go`:** Add a `SecurityContext` struct to the `Workload` definition.
    *   **`internal/engine/workload_runner.go`:**
        *   **Action:** Refactor `executeSingleWorkloadInstance`. Before invoking a module, check for a `SecurityContext`.
        *   **Implementation Detail:**
            ```go
            // In executeSingleWorkloadInstance, before calling module.Perform
            if workload.SecurityContext != nil {
                // This is a simplified representation. The actual implementation is complex.
                // It requires a re-entrant binary or a fork/exec model.
                cmd := exec.Command(os.Args[0], "internal-exec-sandboxed", workload.ID)
                cmd.SysProcAttr = &syscall.SysProcAttr{
                    Cloneflags: syscall.CLONE_NEWNS | syscall.CLONE_NEWPID | ...,
                }
                // ... setup cgroups, seccomp, pipes for stdin/stdout ...
                // The main `gxo` binary needs a new internal command `internal-exec-sandboxed`
                // that re-initializes just enough to run the single workload.
                err := cmd.Run()
                // ... handle results ...
                return summary, err
            }
            // ... existing module.Perform call ...
            ```

### **Milestone 7.2: Module Signing & Verification**

*   **Objective:** Implement supply chain security by verifying the cryptographic signatures of modules before execution.
*   **New Files & Detailed Changes:**
    *   **New File: `cmd/gxo-admin/sign.go`:** Create a `gxo-admin sign-module` command using `cosign` libraries to sign module digests.
    *   **`internal/module/registry.go`:**
        *   **Action:** Enhance the `Get` method of the daemon's `Registry`.
        *   **Implementation Detail:** `Get` will now locate the module's signature, verify it against trusted public keys configured in the daemon, and return a `ModuleSignatureError` if verification fails.

---

## **Phase 8: Complete the Full GXO Standard Library**

**Document ID:** GXO-ENG-PLAN-P8
**Version:** 1.0
**Date:** 2025-07-14
**Status:** Approved for Execution

### **Objective**

With a secure, production-ready kernel, this final phase focuses on implementing the full suite of modules defined in the GXO-SL, unlocking the platform's complete range of capabilities for cloud, container, and cryptographic automation.

### **Rationale**

The final set of standard library modules provides high-value, out-of-the-box integrations that solve common and complex automation problems. Completing the GXO-SL fulfills the promise of a "batteries-included" platform and provides the key building blocks for advanced use cases in modern cloud-native environments.

---

### **Milestone 8.1: Advanced Protocol & Crypto Modules**

*   **Objective:** Implement modules for DNS, advanced SSH, and core cryptographic functions.
*   **Modules & APIs:**
    *   **`dns:query`:** Params: `name` (string, req), `type` (string). Summary: `{ "answers": []string }`.
    *   **`ssh:upload`:** Params: `connection_id` (string, req), `source` (string, req), `destination` (string, req).
    *   **`crypto:generate_key`:** Params: `type` (string, req, choices: `rsa`, `ed25519`), `bits` (int). Summary: `{ "public_key": string, "private_key": string }`.

### **Milestone 8.2: Container & Vault Integration**

*   **Objective:** Provide first-class integration with Docker/containers and HashiCorp Vault.
*   **Modules & APIs:**
    *   **`docker:build`:** Params: `path` (string, req), `tag` (string, req), `build_args` (map). Summary: `{ "image_id": string, "tags": []string }`.
    *   **`vault:read`:** Params: `path` (string, req). Summary: `{ "data": map }`.

### **Milestone 8.3: High-Level Cloud Service Wrappers**

*   **Objective:** Implement opinionated, high-level wrappers for common cloud operations.
*   **Modules & APIs:**
    *   **`aws:s3_sync`:** Params: `source` (string, req), `bucket` (string, req), `prefix` (string), `delete` (bool).
    *   **`aws:ec2_instance`:** Params: `name` (string, req), `state` (string, req, choices: `present`, `absent`, `running`, `stopped`), `instance_type` (string), `ami` (string). Summary: `{ "instance_id": string, "public_ip": string, "private_ip": string, "state": string }`.
