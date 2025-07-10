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

# **GXO Master Engineering Plan: Phase 1**

**Document ID:** GXO-ENG-PLAN-P1
**Version:** 1.0
**Date:** 2025-07-08
**Status:** Approved for Execution

## **Phase 1: Foundational Refactor - Aligning with the `Workload` Model**

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

*   **New File: `internal/config/policy.go`**
    *   **Action:** Create this new file to centralize all policy-related definitions, improving separation of concerns.
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
            
            // ... other policy-specific fields will be added in later phases.
        }
        ```

*   **`internal/config/config.go`**
    *   **Action:** Perform a comprehensive refactor of the core data structures. The `Task` struct will be removed and replaced by `Workload` and `Process`. The top-level `Playbook` will be updated to use `workloads` instead of `tasks`.
    *   **Implementation Detail (Before):**
        ```go
        // Playbook represents the top-level structure...
        type Playbook struct {
            Tasks []Task `yaml:"tasks"`
            // ... other fields
        }

        // Task represents a single unit of work...
        type Task struct {
            Name string `yaml:"name,omitempty"`
            Type string `yaml:"type"`
            // ... other fields
        }
        ```
    *   **Implementation Detail (After):**
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
            When          string                 `yaml:"when,omitempty"`       // Behavior for 'run_once'
            Loop          interface{}            `yaml:"loop,omitempty"`       // Behavior for 'run_once'
            LoopControl   *LoopControlConfig     `yaml:"loop_control,omitempty"` // Behavior for 'run_once'
            Retry         *RetryConfig           `yaml:"retry,omitempty"`      // Behavior for 'run_once'
            Timeout       string                 `yaml:"timeout,omitempty"`    // Behavior for 'run_once'
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
              "properties": {
                "module": { "type": "string", "minLength": 1 },
                "params": { "type": "object", "additionalProperties": true }
              },
              "required": ["module"]
            }
            ```
        5.  Add a new `LifecyclePolicy` definition in `definitions`:
            ```json
            "LifecyclePolicy": {
              "type": "object",
              "properties": {
                "policy": {
                  "description": "The execution strategy for this workload.",
                  "type": "string",
                  "enum": ["run_once", "supervise", "event_driven", "scheduled"]
                },
                "restart_policy": { "type": "string", "enum": ["always", "on_failure", "never"] },
                "cron": { "type": "string" },
                "source": { "type": "string" }
              },
              "required": ["policy"]
            }
            ```

*   **`internal/config/validation.go`**
    *   **Action:** Rewrite the `ValidatePlaybookStructure` function to operate on the new `Workload` structs and add new validation rules specific to lifecycles.
    *   **Detailed Changes:**
        1.  The main loop must change from `for i := range p.Tasks` to `for i := range p.Workloads`.
        2.  All log messages and error text must be updated from "task" to "workload" (e.g., `fmt.Sprintf("workload %d ('%s')", i, workload.Name)`).
        3.  All references to `task.Type` must be changed to `workload.Process.Module`.
        4.  **Add new validation logic:**
            ```go
            // In ValidatePlaybookStructure's loop over workloads
            workload := &p.Workloads[i]
            
            // Check for required nested objects
            if workload.Lifecycle.Policy == "" {
                errs = append(errs, gxoerrors.NewValidationError(fmt.Sprintf("%s: lifecycle.policy is a required field", workloadDisplayName), nil))
            }
            if workload.Process.Module == "" {
                 errs = append(errs, gxoerrors.NewValidationError(fmt.Sprintf("%s: process.module is a required field", workloadDisplayName), nil))
            }

            // Initially, the `gxo run` command only supports the 'run_once' lifecycle.
            // This validation prevents users from trying to run other lifecycles
            // before the `gxo daemon` is implemented, avoiding confusion.
            if workload.Lifecycle.Policy != "run_once" {
                // This check prepares for the future and provides a clear error message.
                errs = append(errs, gxoerrors.NewValidationError(fmt.Sprintf("%s: lifecycle policy '%s' is not supported by 'gxo run'. Use 'gxo daemon' for this lifecycle.", workloadDisplayName, workload.Lifecycle.Policy), nil))
            }
            ```

---

### **Milestone 1.3: Implement Playbook Migration Shim and Tool**

**Objective:** Provide a seamless, robust, and user-friendly upgrade path for all existing `v0.1.2a` playbooks to the new `v1.0.0` `Workload`-based format.

**Rationale:** A hard break with the past is user-hostile. Existing playbooks are valuable assets. Suddenly failing all of them on upgrade would severely damage user trust. The **in-memory shim** provides immediate operational continuity by allowing `gxo run` to function with old playbooks (with a clear deprecation warning). The **`gxo migrate` command** provides the permanent, user-initiated solution. This dual approach respects existing processes while guiding users toward the new, superior format.

**Impacted Files & Detailed Changes:**

*   **`internal/config/load.go`**
    *   **Action:** Refactor the `LoadPlaybook` function to implement the in-memory migration shim. This logic must execute *before* the strict unmarshaling into the final `config.Playbook` struct.
    *   **Detailed Logic:**
        1.  **Light Unmarshal:** Define a local, temporary `migrationHelper` struct that contains *both* a `Tasks []config.Task` field (with the old `tasks` YAML tag) and the new `Workloads []config.Workload` field. Unmarshal the raw playbook YAML into this helper struct *without* strict mode.
        2.  **Detect & Migrate:** Check if `len(helper.Tasks) > 0` and `len(helper.Workloads) == 0`. If true, it's a legacy playbook.
        3.  **Log Deprecation:** If migration is triggered, use the logger to emit a clear `WARN` level message: `"Playbook uses the deprecated 'tasks' key. It is being migrated in-memory. To upgrade the file permanently, run 'gxo migrate -f <file>'. The 'tasks' key will be removed in a future version."`
        4.  **Perform Conversion:** Iterate over `helper.Tasks` and create a new `[]Workload`. For each old `Task`, create a new `Workload` and map the fields:
            *   `Name` -> `Name`
            *   `Type` -> `Process.Module`
            *   `Params` -> `Process.Params`
            *   Set `Lifecycle.Policy` to the default `"run_once"`.
            *   Copy all other fields (`Register`, `When`, `Loop`, etc.) to the new `Workload` struct.
        5.  **Re-Marshal:** Convert the modified `helper` struct (which now has a populated `Workloads` field and an empty `Tasks` field) back into YAML bytes.
        6.  **Strict Unmarshal:** Use these newly generated YAML bytes for the final, strict unmarshal into the official `config.Playbook` struct. The rest of the engine will be completely unaware that a migration occurred.

*   **New File: `cmd/gxo/migrate.go`**
    *   **Action:** Create this file to define the new `gxo migrate` command using the Cobra library.
    *   **Detailed Logic (`RunE` function):**
        1.  Define and require a `-f, --filename` flag.
        2.  Read the legacy playbook file specified by the flag.
        3.  Call the (now-refactored) `config.LoadPlaybook` function. This will automatically run the in-memory migration shim and return a fully compliant, in-memory `v1.0.0` `Playbook` object.
        4.  Marshal the returned `Playbook` object back to YAML.
        5.  Print the resulting YAML to standard output.

*   **New File: `internal/config/load_test.go`**
    *   **Action:** Create a dedicated test file to validate the migration shim's behavior.
    *   **Required Test Cases:**
        1.  `TestLoadPlaybook_WithLegacyTasksKey`: Provide a valid legacy playbook. Assert that it loads without error, the returned `Playbook` object has the correct number of `Workloads`, and each migrated workload has the `"run_once"` lifecycle.
        2.  `TestLoadPlaybook_WithModernWorkloadsKey`: Provide a modern playbook. Assert that it loads without error and that no deprecation warning is logged.
        3.  `TestLoadPlaybook_WithAmbiguousKeys`: Provide a playbook that defines *both* `tasks:` and `workloads:`. Assert that `LoadPlaybook` returns a `ValidationError` stating that the keys are mutually exclusive.

---

### **Milestone 1.4: Execute Cross-Cutting Refactor**

**Objective:** Systematically propagate the `Task` -> `Workload` rename and the `Type` -> `Process.Module` change through every layer of the application to ensure conceptual consistency and prevent runtime bugs.

**Rationale:** A partial rename is a source of confusion for developers and a breeding ground for subtle bugs. This must be a single, sweeping, and complete operation to ensure the entire system speaks the new architectural language.

**Impacted Files & Detailed Changes (Checklist):**

1.  **Engine Core (`internal/engine/`):**
    *   `task_runner.go` -> **Rename file to `workload_runner.go`**.
    *   `workload_runner.go`:
        *   Rename `TaskRunner` struct to `WorkloadRunner`.
        *   Rename `ExecuteTask` method to `ExecuteWorkload`, changing its signature to accept `*config.Workload`.
    *   `dag.go`:
        *   Rename `Node.Task` field to `Node.Workload` (of type `*config.Workload`). Update all internal logic.
    *   `engine.go`:
        *   Rename field `taskRunner` to `workloadRunner`.
        *   Rename all local variables: `taskID` -> `workloadID`, `taskStatuses` -> `workloadStatuses`.
        *   Update all log messages to use the term "workload".
    *   `engine_test.go`, `engine_policy_test.go`, `engine_security_test.go`:
        *   Update all test playbooks in these files to use the new `workloads:` syntax.

2.  **Public API & Events (`pkg/gxo/v1/` and `internal/events/`):**
    *   `pkg/gxo/v1/api.go`:
        *   Rename `TaskResult` struct to `WorkloadResult`.
        *   Rename `ExecutionReport.TaskResults` field to `WorkloadResults`.
        *   **CRITICAL:** Update the JSON struct tags to match the new field names (e.g., `json:"workload_results"`) to avoid breaking external JSON consumers.
    *   `pkg/gxo/v1/events/bus.go`:
        *   Rename event constants: `TaskStart` -> `WorkloadStart`, `TaskEnd` -> `WorkloadEnd`, `TaskStatusChanged` -> `WorkloadStatusChanged`.
        *   Update the `Event` struct fields: `TaskName` -> `WorkloadName`, `TaskID` -> `WorkloadID`.
    *   `internal/engine/engine.go` (Event Emission):
        *   Update the `handleWorkloadCompletion` function to emit the new `Workload...` event types.
        *   **Deprecation Strategy:** For one minor version, emit *both* the old `Task...` and new `Workload...` events to provide a backward-compatibility window for any external event consumers.
    *   `internal/events/metrics_listener.go`:
        *   Update the `handleEvent` switch statement to listen for the *new* `Workload...` event types.

3.  **Module & Template Interface (`internal/module/` and `internal/template/`):**
    *   `internal/module/module.go`:
        *   Rename the `ExecutionContext.Task()` method to `ExecutionContext.Workload() *config.Workload`.
    *   `internal/template/template.go`:
        *   Update `ExtractVariables` to recognize the new state path `_gxo.workloads.<name>.status`.
        *   **Backward Compatibility:** For a limited time, the logic should also recognize the old `_gxo.tasks.<name>.status` path and log a deprecation warning if it is used.

Of course. Here is the complete and exhaustive engineering plan for **Phase 2**, following the same detailed format.

---

# **GXO Master Engineering Plan: Phase 2**

**Document ID:** GXO-ENG-PLAN-P2
**Version:** 1.0
**Date:** 2025-07-09
**Status:** Approved for Execution

## **Phase 2: Hardening the Core - Comprehensive Test Suite**

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
    *   **Action:** Create a new test file dedicated to testing the `main` package's handler functions (`runExecuteCommand`, `runValidateCommand`).
    *   **Implementation Detail:** This will require mocking system-level functions. A common pattern is to replace `os.Exit` with a function that records the exit code in a variable. `stdout` and `stderr` can be captured by redirecting `os.Stdout` and `os.Stderr` to an in-memory buffer (`bytes.Buffer`) during the test.
    *   **Required Test Cases:**
        1.  `TestRunExecute_Success`: Provide a valid, simple `run_once` playbook. Verify that the final exit code is `0` and that key success messages are printed to the captured `stdout`.
        2.  `TestRunExecute_ValidationFailure`: Provide a playbook with a clear schema error (e.g., a required field is missing). Verify that the exit code is `2` (UsageError) or `1` (Failure) and that the captured `stderr` contains a clear "validation failed" error message.
        3.  `TestRunExecute_RuntimeFailure`: Provide a valid playbook where a workload is designed to fail (e.g., `exec` a non-existent command). Verify the exit code is `1` and `stderr` contains the failure details.
        4.  `TestRunValidate_Success`: Test the `gxo validate` command with a valid playbook. Verify exit code `0` and a "validation successful" message.
        5.  `TestRunValidate_Failure`: Test `gxo validate` with an invalid playbook. Verify exit code `1` and that `stderr` contains the validation error details.

*   **New File: `internal/command/command_test.go`**
    *   **Action:** Create a new test file to exhaustively test the `defaultRunner.Run` method.
    *   **Implementation Detail:** The current `command.go` already uses a `Runner` interface, which is excellent. The tests will not use the `defaultRunner` directly. Instead, they will create a mock `Runner` that allows for precise control over the `exec.Cmd` behavior without actually running system commands. This is crucial for fast, reliable, and platform-independent tests.
        *   A mock command struct can be created to satisfy an interface that mimics `*exec.Cmd`. The test will then inject this mock.
    *   **Required Test Cases:**
        1.  `TestRun_Success`: Mock a command like `echo "hello"`. Configure the mock to produce specific `stdout`, `stderr`, and an exit code of `0`. Verify that the returned `CommandResult` struct contains exactly this data.
        2.  `TestRun_NonZeroExit`: Mock a command that exits with code `12`. Verify that the returned `result.ExitCode` is `12` and that the `Run` function itself returns a `nil` error (as the command execution was successful, even if the command's internal logic failed).
        3.  `TestRun_ContextCancellation`: Start a mock command that simulates a long-running process (e.g., `sleep 5`). Create a `context.WithCancel` and cancel it shortly after starting the command. Verify that the `Run` function returns `context.Canceled` as its error.
        4.  `TestRun_CommandNotFound`: Mock the behavior of `exec.LookPath` failing. Verify that the `Run` function returns an appropriate `exec.ErrNotFound` error.

---

### **Milestone 2.2: Configuration and Engine Core Test Suites**

**Objective:** Test the core logic of configuration loading, DAG building, and workload execution orchestration to ensure the refactored engine is functionally correct.

**Rationale:** These components form the brain of GXO. Any errors in DAG construction, dependency resolution, or status management can lead to incorrect execution order, deadlocks, or silent failures. These tests validate the fundamental correctness of the orchestration logic.

**Impacted Files & Detailed Changes:**

*   **`internal/config/load_test.go`**
    *   **Action:** Enhance this file (or create it if it doesn't exist) to specifically test `LoadPlaybook` with the new `Workload` syntax.
    *   **Required Test Cases:**
        1.  `TestLoadPlaybook_ValidWorkload`: Load a modern playbook using the `workloads:` key. Assert no error is returned and the `Playbook` struct is populated correctly.
        2.  `TestLoadPlaybook_MissingRequiredFields`: Test playbooks that are missing `workload.name`, `workload.lifecycle.policy`, or `workload.process.module`. Assert that a `ValidationError` is returned with a clear message for each case.
        3.  Re-run the tests from Milestone 1.3 (`TestLoadPlaybook_WithLegacyTasksKey`, `TestLoadPlaybook_WithAmbiguousKeys`) to ensure the migration shim continues to work as expected after the refactor.

*   **New File: `internal/config/validation_test.go`**
    *   **Action:** Create a dedicated test suite for `ValidatePlaybookStructure`.
    *   **Required Test Cases:** Create separate test functions for *each* specific validation rule:
        1.  `TestValidate_DependencyCycle`: Test with a playbook that has a clear A -> B -> A dependency. Assert a cycle error is returned.
        2.  `TestValidate_SelfReference`: Test a workload that depends on itself (e.g., `when: "{{ ._gxo.workloads.my_task.status == 'Completed' }}"`). Assert an error is returned.
        3.  `TestValidate_InvalidIdentifier`: Test with invalid names for `register` or `loop_var`. Assert an error.
        4.  `TestValidate_BadDurationString`: Test with an invalid format in a `retry.delay` or `timeout` field. Assert an error.
        5.  `TestValidate_UndefinedReference`: Test a workload that depends on a workload name that does not exist. Assert an error.

*   **New File: `internal/engine/channel_manager_test.go`**
    *   **Action:** Test the `ChannelManager`'s logic for creating and managing streaming channels and their overflow policies.
    *   **Required Test Cases:**
        1.  `TestCreateChannels_FanInFanOut`: Build a mock DAG with a fan-out producer and a fan-in consumer. Call `CreateChannels` and assert that the internal maps of the `ChannelManager` are wired correctly (correct number of channels, correct producer/consumer relationships).
        2.  `TestManagedChannel_OverflowBlock`: Create a `managedChannel` with a buffer size of 1 and a "block" policy. Fill the buffer, then start a new goroutine to write to it again. Assert that the goroutine blocks. Then, read from the channel and assert the goroutine unblocks.
        3.  `TestManagedChannel_OverflowDrop`: Test the "drop_new" policy. Fill the buffer, then try to write again. Assert that the write call returns a `PolicyViolationError` immediately and does not block.

*   **New File: `internal/engine/dag_test.go`**
    *   **Action:** Isolate and test the `BuildDAG` function.
    *   **Implementation Detail:** Use simple, hand-crafted `config.Playbook` structs as input instead of parsing YAML. This allows for precise testing of the DAG logic itself.
    *   **Required Test Cases:**
        1.  `TestBuildDAG_StateAndStreamDependencies`: Create a playbook where Task A produces a stream for B, and Task C depends on the registered result of B. Assert that the resulting DAG has the correct A -> B -> C dependency chain.
        2.  `TestBuildDAG_PolicyResolution`: Create a playbook with a global policy and a task with an overriding policy. Assert that the final `Node` in the DAG has the correctly merged, task-specific policy.

*   **`engine_test.go`, `engine_policy_test.go`, `engine_security_test.go`**
    *   **Action:** A full review and update of all existing engine-level integration tests.
    *   **Detailed Changes:**
        1.  Change all playbook YAML in these tests to use the new `workloads:`, `process:`, and `lifecycle:` syntax.
        2.  Update assertions that check registered state to look for `_gxo.workloads...` instead of `_gxo.tasks...`.
        3.  Ensure that tests for `when`, `loop`, and `retry` continue to pass with the new `Workload` struct.

---

### **Milestone 2.3: Advanced Concurrency and Fuzz Testing**

**Objective:** Go beyond standard unit tests to find more subtle bugs in complex, concurrent, or security-sensitive areas of the codebase.

**Rationale:** Concurrency bugs (races, deadlocks) and parsing vulnerabilities are notoriously difficult to find with simple, predictable unit tests. Fuzzing and targeted race condition tests are necessary to build confidence in the system's robustness under unpredictable or high-stress conditions.

**Impacted Files & Detailed Changes:**

*   **New File: `internal/config/fuzz_test.go`**
    *   **Action:** Create a fuzz test for `config.LoadPlaybook` using Go's built-in `testing.F` framework.
    *   **Implementation Detail:**
        1.  The fuzzer (`f.Fuzz(func(t *testing.T, playbookBytes []byte) { ... })`) will be the test's core.
        2.  Seed the fuzzer with valid YAML snippets using `f.Add(...)`. Include examples of all major features (loops, when, all policies, etc.).
        3.  Inside the fuzz function, call `config.LoadPlaybook`. The only assertion needed is that the function **does not panic**. The goal of this test is to find inputs that crash the YAML parser or the validation logic.

*   **New File: `internal/events/backpressure_test.go`**
    *   **Action:** Create a targeted integration test for the `ChannelEventBus` backpressure mechanism.
    *   **Implementation Detail:**
        1.  Create a `ChannelEventBus` with a small buffer (e.g., size 2).
        2.  Create a mock logger that captures log messages instead of printing them. Inject this into the bus.
        3.  Fill the event bus buffer completely by calling `Emit` twice.
        4.  Call `Emit` a third time.
        5.  Assert that the third call does not block (it should return immediately).
        6.  Assert that the mock logger captured a log message containing "dropping event" or "buffer full".

*   **`engine_security_test.go`**
    *   **Action:** Add a new test case, `TestSecretRedaction_RaceCondition`, to specifically target the thread-safety of the secret redaction mechanism.
    *   **Implementation Detail:**
        1.  The test function will use `t.Parallel()` to indicate it can run alongside other parallel tests.
        2.  It will use a `sync.WaitGroup` to launch multiple (e.g., 10) goroutines concurrently.
        3.  Each goroutine will execute a simple playbook that uses the `secret` template function and registers the result. This will cause concurrent access to the `SecretTracker` and redaction logic.
        4.  The test will pass if it completes without the Go race detector (`go test -race`) reporting any data races. This proves that the per-instance `SecretTracker` and the redaction logic are thread-safe.

---

# **GXO Master Engineering Plan: Phase 3**

**Document ID:** GXO-ENG-PLAN-P3
**Version:** 1.0
**Date:** 2025-07-10
**Status:** Approved for Execution

## **Phase 3: Developer Experience - The Playbook Mocking Framework**

### **Objective**

To accelerate adoption and enable the creation of complex, reliable automation, users must have the confidence to test their playbooks without affecting live systems. This phase focuses on building a first-class, GXO-native testing and validation experience by introducing a dedicated test runner and a suite of mocking modules.

### **Rationale**

A robust testing framework is a feature, not an afterthought. Providing developers with the tools to write unit and integration tests for their own playbooks significantly increases the quality and reliability of the automation they build. It allows them to verify complex logic, test error handling paths, and validate data transformations in a fast, isolated, and repeatable manner. By building this framework directly into GXO, we treat "playbook testing" as a core competency of the platform, enabling a Test-Driven Development (TDD) approach for automation engineers.

---

### **Milestone 3.1: The `gxo test` Command**

**Objective:** Introduce a new top-level CLI command for discovering and running test-specific playbooks, providing structured output suitable for both human and CI/CD consumption.

**Rationale:** A dedicated test runner provides a clear separation between production execution (`gxo run`) and testing. It allows for test-specific configurations, output formats, and behaviors, creating a user experience that is familiar to software developers and easy to integrate into automated pipelines.

**Impacted Files & Detailed Changes:**

*   **`cmd/gxo/main.go`**
    *   **Action:** Modify the main CLI router (assumed to be Cobra from the roadmap) to add a new top-level `test` command. The main function will delegate to a new handler function for this command.
    *   **Implementation Detail:**
        ```go
        // In the root command's init() function
        rootCmd.AddCommand(newTestCmd())
        ```

*   **New File: `cmd/gxo/test.go`**
    *   **Action:** Create this file to define the `gxo test` command, its flags, and its execution logic.
    *   **Implementation Detail (Cobra Command):**
        ```go
        // testCmd represents the test command
        var testCmd = &cobra.Command{
            Use:   "test [path...]",
            Short: "Executes GXO test playbooks",
            Long:  `Discovers and executes GXO test playbooks (files ending in *.test.gxo.yaml)
in the specified directories or files.`,
            RunE: runTestCommand,
        }

        func init() {
            testCmd.Flags().BoolP("verbose", "v", false, "Enable verbose test output")
            testCmd.Flags().String("run", "", "Run only tests matching the regular expression")
            // ... other standard test flags
        }
        ```
    *   **Detailed Logic (`runTestCommand` function):**
        1.  **Discovery:** The function will walk the filesystem paths provided as arguments (or the current directory if none are provided). It will discover all files matching the pattern `*.test.gxo.yaml`.
        2.  **Execution Loop:** It will iterate through each discovered test file.
        3.  **Test Execution:** For each file, it will create a new instance of the GXO engine configured specifically for testing (e.g., with a higher default log level if `-v` is passed). It will then call `engine.RunPlaybook`.
        4.  **Result Reporting:** It will inspect the returned `ExecutionReport` and error. A successful test run is one that returns no error. It will print structured output to `stdout` in a format similar to `go test`:
            ```
            === RUN   path/to/my_first.test.gxo.yaml
            --- PASS: my_first_test (3.45s)
            === RUN   path/to/another_test.test.gxo.yaml
            --- FAIL: another_test (1.23s)
                workload 'assert_api_response' failed: validation error: API status code was 500, expected 200
            FAIL
            ```
        5.  **Exit Code:** The command will exit with code `0` if all tests pass, and `1` if any test fails.

---

### **Milestone 3.2: The `test:*` Module Suite**

**Objective:** Develop a dedicated suite of modules designed for use within test playbooks to enable mocking of external systems and making assertions about playbook state.

**Rationale:** To write effective unit tests for playbooks, developers need to control the test environment completely. This requires the ability to mock external dependencies like HTTP APIs and to make concrete assertions about the results of the playbook run. These modules provide those fundamental testing primitives.

**Impacted Files & Detailed Changes:**

*   **New File: `modules/test/mock_http_server/mock_http_server.go`**
    *   **Action:** Create the `test:mock_http_server` module.
    *   **Synopsis:** Stands up a temporary, in-memory HTTP server for the duration of a test.
    *   **Description:** This module starts a real HTTP server on a random, available localhost port. It is configured declaratively with a list of expected requests and their corresponding responses. When the test playbook finishes, the GXO engine will terminate the module, and the server will be shut down automatically. This allows testing of `http:request` workloads without any network access.
    *   **Supported Lifecycles:** `run_once`
    *   **Parameters:**
        | Name | Type | Required? | Description |
        |---|---|---|---|
        | `handlers` | list[map] | Yes | A list of handler definitions. Each map defines an expectation. |
    *   **Handler Map Structure:**
        | Key | Type | Description |
        |---|---|---|
        | `request` | map | Defines the expected incoming request. |
        | `response` | map | Defines the response to send if the request matches. |
    *   **Request Map Structure:** `{ "method": "GET", "path": "/api/v1/users" }`
    *   **Response Map Structure:** `{ "status_code": 200, "body": "{\"id\": 1}", "headers": {"Content-Type": "application/json"} }`
    *   **Return Values / Summary:** `{ "server_url": string }` containing the base URL of the running mock server (e.g., `http://127.0.0.1:54321`). This can be used by subsequent `http:request` workloads.

*   **New File: `modules/test/assert/assert.go`**
    *   **Action:** Create the `test:assert` module.
    *   **Synopsis:** Makes assertions about the state of a playbook run.
    *   **Description:** This module is the core of playbook validation. It provides a rich set of assertion types to check values from the state store. If any assertion fails, the module returns a fatal error with a descriptive message, which causes the `gxo test` runner to mark the test as failed.
    *   **Supported Lifecycles:** `run_once`
    *   **Parameters:**
        | Name | Type | Required? | Description |
        |---|---|---|---|
        | `assertions`| list[map] | Yes | A list of assertion definitions to evaluate. |
    *   **Assertion Map Structure:** Each assertion is a map that must contain `actual` and one assertion operator key (e.g., `equal_to`, `contains`).
        | Key | Type | Description |
        |---|---|---|
        | `actual` | any | The value to test, typically from a template variable (e.g., `{{ .my_result }}`). |
        | `equal_to` | any | Asserts that `actual` is deeply equal to this value. |
        | `not_equal_to`| any | Asserts that `actual` is not equal to this value. |
        | `contains` | string | Asserts that `actual` (which must be a string, list, or map) contains this value. |
        | `is_true` | bool | Asserts that `actual` evaluates to `true`. |
        | `is_nil` | bool | Asserts that `actual` is `nil`. |
        | `matches_regex`| string | Asserts that `actual` (must be a string) matches the given regular expression. |
    *   **Return Values / Summary:** `{ "assertions_passed": int }` on success. On failure, returns a fatal error.
    *   **Example Test Playbook (`my_api.test.gxo.yaml`):**
        ```yaml
        workloads:
          # Setup: Stand up a mock server for our API
          - name: start_mock_api
            process:
              module: test:mock_http_server
              params:
                handlers:
                  - request: { method: "GET", path: "/api/users/1" }
                    response: { status_code: 200, body: '{"name": "Alice"}' }
            register: mock_server

          # Action: Run the workload that calls the API
          - name: get_user_data
            process:
              module: http:request
              params:
                url: "{{ .mock_server.server_url }}/api/users/1"
            register: api_response

          # Verification: Assert the results are correct
          - name: verify_response
            process:
              module: test:assert
              params:
                assertions:
                  - actual: "{{ .api_response.status_code }}"
                    equal_to: 200
                  - actual: "{{ .api_response.json_body.name }}"
                    equal_to: "Alice"
        ```

---

# **GXO Master Engineering Plan: Phase 4**

**Document ID:** GXO-ENG-PLAN-P4
**Version:** 1.0
**Date:** 2025-07-10
**Status:** Approved for Execution

## **Phase 4: The Critical Path - REST API & ETL Enablement**

### **Objective**

Implement the minimum viable set of modules required to deliver on GXO's core promise: bridging the gap between systems via API calls and processing the resulting data. This phase unlocks the most common and powerful use cases for "glue code" replacement and data integration, providing immediate, high-value capabilities to users.

### **Rationale**

While the full GXO Standard Library is extensive, a small subset of modules enables a vast majority of common automation workflows. Prioritizing this "critical path" allows the project to deliver a highly useful product faster. The ability to call a REST API, parse its JSON response, and act on that data is the quintessential "glue code" task that GXO is designed to solve elegantly. This phase delivers that core experience.

---

### **Milestone 4.1: Foundational System Primitives (Layer 1)**

**Objective:** Implement the core modules for interacting with the local system and controlling workflow logic. These are prerequisites for almost any real-world playbook.

**Rationale:** These modules provide the basic building blocks for file manipulation and logical control that are essential for setting up test conditions, managing temporary data, and creating dynamic, conditional workflows.

**Impacted Files & Detailed Changes:**

*   **New Directory: `modules/exec/`**
    *   **Action:** Create `modules/exec/exec.go`.
    *   **Module:** `exec`
    *   **Implementation Detail:** The `Perform` method must use the `internal/command.Runner` abstraction. It must check the context for the `DryRunKey`. The summary it returns must be a map: `{ "stdout": string, "stderr": string, "exit_code": int }`.

*   **New Directory: `modules/filesystem/`**
    *   **Action:** Create the files for the `filesystem` module suite: `read.go`, `write.go`, `stat.go`, `list.go`, `manage.go`. Each will contain a separate module struct.
    *   **Modules:** `filesystem:read`, `filesystem:write`, `filesystem:stat`, `filesystem:list`, `filesystem:manage`.
    *   **Implementation Detail:** All path-based operations **MUST** resolve paths relative to the `Workspace` to prevent path traversal. The `filesystem:list` module must be implemented as a streaming producer, emitting one record for each file/directory found. The `filesystem:manage` module must be idempotent.

*   **New Directory: `modules/control/`**
    *   **Action:** Create the files for the `control` module suite: `assert.go`, `identity.go`, `barrier.go`.
    *   **Modules:** `control:assert`, `control:identity`, `control:barrier`.
    *   **Implementation Detail:** The `control:barrier` is a streaming-only module. Its `Perform` method should use a `sync.WaitGroup` to wait for all channels in its `stream_inputs` to be closed. It does not need to read any records from the channels.

---

### **Milestone 4.2: REST API Client (Layer 5)**

**Objective:** Implement the universal HTTP client. This is the single most important module for external system integration.

**Rationale:** The vast majority of modern automation involves interacting with REST APIs. A powerful, convenient, and robust `http:request` module is the gateway to integrating GXO with virtually any other platform or service.

**Impacted Files & Detailed Changes:**

*   **New Directory: `modules/http/`**
    *   **Action:** Create `modules/http/request.go`.
    *   **Module:** `http:request`
    *   **Implementation Detail:**
        1.  This module will use Go's standard `net/http` client. It should manage a client instance that can be reused for performance (e.g., keep-alives).
        2.  It must handle all major HTTP methods (GET, POST, PUT, DELETE, PATCH, etc.).
        3.  It needs to support setting custom headers, request bodies (as a string), and URL query parameters.
        4.  It must include a `skip_tls_verify` parameter for test environments, but log a prominent security warning if it is used.
        5.  The `summary` it returns must be a rich map: `{ "status_code": int, "headers": map, "body": string, "json_body": any, "latency_ms": int }`.
        6.  The implementation should automatically attempt to parse the response body as JSON if the `Content-Type` header is `application/json`, populating the `json_body` field. This is a significant quality-of-life feature.

---

### **Milestone 4.3: Core Data Plane (Layer 4)**

**Objective:** Implement the essential ETL modules needed to process data from the `http:request` module's responses.

**Rationale:** Getting data from an API is only half the battle. The other half is parsing, transforming, and filtering that data to extract the specific information needed for subsequent steps. These modules provide that capability.

**Impacted Files & Detailed Changes:**

*   **New Directory: `modules/data/`**
    *   **Action:** Create the initial set of data plane modules: `parse.go`, `map.go`, `filter.go`.
    *   **Module: `data:parse`**
        *   **Implementation Detail:** The initial version needs to support `format: "json"` and `format: "text_lines"`. For JSON, it will use `json.Unmarshal` to parse the `content` into a Go `[]interface{}` or `map[string]interface{}` and then emit each element/value as a separate record on its output stream. For `text_lines`, it will split the `content` by newlines and emit each line as a record: `{ "line": "..." }`.
    *   **Module: `data:map`**
        *   **Implementation Detail:** This module's `Perform` method will iterate over its input stream. For each record, it will execute a Go template provided in its `template` parameter. The result of the template execution will be the new record emitted on its output stream.
    *   **Module: `data:filter`**
        *   **Implementation Detail:** This module will also iterate over its input stream. It will execute a Go template from its `condition` parameter for each record. If the template's output evaluates to "truthy," the original, unmodified record is passed through to the output stream. Otherwise, it is discarded.

---

# **GXO Master Engineering Plan: Phase 5**

**Document ID:** GXO-ENG-PLAN-P5
**Version:** 1.0
**Date:** 2025-07-10
**Status:** Approved for Execution

## **Phase 5: Completing the Vision - Full Standard Library**

### **Objective**

With the critical path for API and ETL workflows delivered, this phase focuses on expanding GXO's capabilities to cover the full spectrum of automation tasks by implementing the remainder of the GXO-SL. This will round out the platform, enabling low-level network automation, server-side implementations, advanced data processing, and seamless integration with key ecosystem tools like Terraform and artifact repositories.

### **Rationale**

A rich "batteries-included" standard library is what transforms a powerful engine into a productive and versatile platform. Implementing the full GXO-SL demonstrates the robustness of the underlying GXO-AM (Automation Model) and provides users with a comprehensive, first-party toolkit for nearly any automation challenge, reinforcing the value proposition of GXO as a unified runtime. The development is sequenced by layer, building upon already-completed primitives.

---

### **Milestone 5.1: The Network Stack (Layers 2 & 3)**

**Objective:** Enable low-level network automation and allow users to build custom GXO-native network services.

**Rationale:** Direct socket and protocol-level control is a key differentiator for GXO, allowing it to move beyond simple task execution and into the realm of network testing, security monitoring, and custom service implementation, as demonstrated by the declarative KV-server example.

**Impacted Files & Detailed Changes:**

*   **New Directory: `modules/connection/`**
    *   **Action:** Create the full suite of connection-level modules: `open.go`, `listen.go`, `read.go`, `write.go`, `close.go`.
    *   **Modules:** `connection:open`, `connection:listen`, `connection:read`, `connection:write`, `connection:close`.
    *   **Implementation Detail:** These modules will interact with a new `internal/connections` manager service within the GXO Kernel. This service will be responsible for holding open socket connections and mapping them to the opaque `connection_id` handles that are passed between workloads. The `connection:listen` module is a streaming producer that will emit connection handles. The other modules will take a `connection_id` as a parameter to operate on the correct socket.

*   **`modules/http/`**
    *   **Action:** Create `modules/http/listen.go` and `modules/http/respond.go`.
    *   **Modules:** `http:listen`, `http:respond`.
    *   **Implementation Detail:** The `http:listen` module will be a streaming consumer that takes its `stream_input` from a `connection:listen` workload. It will parse the raw byte stream from the connection according to the HTTP/1.1 spec and produce a stream of `request_id` handles. The `http:respond` module will take a `request_id` to send a response back on the correct connection.

*   **New Directory: `modules/ssh/`**
    *   **Action:** Create the full suite of SSH modules.
    *   **Modules:** `ssh:connect`, `ssh:command`, `ssh:script`.
    *   **Implementation Detail:** This suite will be built on top of Go's standard `crypto/ssh` library. `ssh:connect` will establish a persistent connection and return a handle, similar to the `connection` suite.

---

### **Milestone 5.2: Advanced Data Plane & Application Modules (Layers 4 & 5)**

**Objective:** Complete the Data Plane to enable advanced ETL and add clients for common data services.

**Rationale:** While the critical path covers basic data processing, advanced use cases require more powerful tools like joining disparate data sources and performing stateful aggregations. This milestone delivers those capabilities.

**Impacted Files & Detailed Changes:**

*   **`modules/data/`**
    *   **Action:** Create `modules/data/join.go` and `modules/data/aggregate.go`.
    *   **Module: `data:join`**
        *   **Implementation Detail:** This module will implement a two-phase in-memory hash join. It will read all records from its "build" side stream(s) into a hash map keyed by the join field. Then, it will read the "probe" side stream and look up matches in the hash map, emitting joined records. It must support `inner`, `left`, `right`, and `outer` join types.
    *   **Module: `data:aggregate`**
        *   **Implementation Detail:** This is a stateful streaming module. It will maintain internal state (e.g., counts, sums) for different groups of records. It will use timers (for `window` mode) or counters (for `count` mode) to know when to flush an aggregate group to its output stream and reset the state for that group.

*   **New Directory: `modules/database/`**
    *   **Action:** Create `modules/database/query.go`.
    *   **Module:** `database:query`.
    *   **Implementation Detail:** This module will use Go's standard `database/sql` package. It will take connection parameters (or a reference to a configured profile) and a SQL query. For `SELECT` statements, it must be a streaming producer, iterating over `sql.Rows` and emitting one GXO record per row. For other statements (`INSERT`, `UPDATE`), it should return a summary with `{ "rows_affected": int }`.

---

### **Milestone 5.3: The Integration Layer (Layer 6)**

**Objective:** Provide opinionated, high-level wrappers for key ecosystem tools to create a seamless, "better together" experience for common DevOps and GitOps workflows.

**Rationale:** While users *could* interact with tools like Terraform or Artifactory using the `exec` and `http:request` modules, providing dedicated, intelligent wrappers greatly improves the user experience, reduces boilerplate, and allows GXO to handle complex state-passing automatically.

**Impacted Files & Detailed Changes:**

*   **New Directory: `modules/artifact/`**
    *   **Action:** Create `modules/artifact/upload.go` and `modules/artifact/download.go`. This suite depends on an underlying `object_storage` module.
    *   **Modules:** `artifact:upload`, `artifact:download`.
    *   **Implementation Detail:** First, a generic `object_storage` suite (L5) must be built to interact with S3-compatible APIs. The `artifact:upload` module will then use this primitive. It will take a local path from the `Workspace` and a logical name (e.g., `my-app:v1.2.3`). It will compute a checksum, upload the file using `object_storage:put_object`, and return a structured **Artifact Handle** (a map containing the logical name, version, checksum, and remote location) as its `summary`. The `artifact:download` module will take this Handle as a parameter and use `object_storage:get_object` to retrieve the file into the current `Workspace`.

*   **New Directory: `modules/terraform/`**
    *   **Action:** Create `modules/terraform/run.go`.
    *   **Module:** `terraform:run`.
    *   **Implementation Detail:** This module is an intelligent wrapper around the `exec` module. It will execute `terraform apply`, `plan`, etc. Its key feature is that after a successful `apply`, it will automatically run `terraform output -json` in the same directory, parse the resulting JSON, and return it as a structured map in its `summary`. This directly solves the state-passing problem between Terraform and subsequent configuration steps.


