# GXO Master Architecture & Design Specification

**Document ID:** GXO-ARCH-MASTER
**Version:** 1.0
**Status:** Canonical Technical Blueprint

## Abstract

The contemporary software automation landscape is characterized by a powerful but fragmented ecosystem of specialized tools. This paper introduces GXO (Go Execution and Orchestration), a novel system designed as a **Declarative Automation Kernel**. We posit that by unifying process supervision, event-driven orchestration, and streaming data flow into a single, high-performance, lightweight runtime, GXO creates a new class of automation platform. The kernel's central abstraction is the `Workload`, a declarative unit of automation defined by its `Process` logic and a specific `Lifecycle` policy. This document provides an exhaustive, unambiguous definition of the architecture, design, implementation details, algorithms, data structures, and security model for the GXO system. It serves as the master technical blueprint.

---

## Part I: The GXO Philosophy & Core Abstractions

At its core, GXO is an opinionated implementation of an **Automation Kernel**. This is a direct and intentional analogy to a modern operating system kernel. The GXO Kernel does not perform high-level application logic itself; instead, it provides a minimal, privileged, and highly performant set of core services upon which all other functionality is built. This design philosophy mandates a clean separation between the Kernel's responsibilities and the responsibilities of the `Modules` it executes.

### 1.1. The Kernel's Responsibilities
*   **Scheduling:** Managing the execution of `Workloads` based on their `Lifecycle` policy.
*   **State Management:** Providing a secure, concurrency-safe, and persistent store for runtime data.
*   **Dependency Management:** Constructing and executing a unified Directed Acyclic Graph (DAG) that comprehends both `Workload` and data-stream dependencies.
*   **Isolation & Security:** Providing the primitives (`Workspace`, security contexts) to isolate `Workloads` from each other and the host system.
*   **Inter-Process Communication (IPC):** Managing the native, backpressured data streams that serve as the primary IPC mechanism between `Workloads`.

### 1.2. The Core Abstractions
GXO's declarative language is built on a small, powerful set of orthogonal concepts. Understanding these is key to understanding the entire system.

1.  **The `Workload`:** This is the top-level, declarative unit in a GXO playbook. It is the only object a user needs to define. A playbook is a list of `Workloads`. It is the complete, schedulable entity that the GXO Kernel manages. A `Workload` is the fusion of *what* to do and *how/when* to do it.

2.  **The `Process`:** This is the reusable, inert definition of *automation logic*. It defines *what* a `Workload` does. It is composed of a `module` (e.g., `exec`) and its `params`. By separating the `Process` from its execution policy, the same logic can be used for a one-off task, a long-running service, or an event handler.

3.  **The `Lifecycle`:** This is the *execution policy*. It defines *how and when* the Kernel runs a `Process`. It is the second, mandatory part of a `Workload` definition. The standard lifecycles are:
    *   `run_once`: Execute the `Process` once and terminate. This is the fundamental building block for CI/CD steps and ad-hoc automation tasks. The current `v0.1.2a` engine is a pure `run_once` executor.
    *   `supervise`: Keep the `Process` running indefinitely as a service. The Kernel will monitor the `Process` and restart it based on a configured policy (e.g., `on_failure`, `always`), applying exponential back-off to prevent crash loops.
    *   `event_driven`: Instantiate and execute the `Process`'s DAG each time an event is received from a specified `source` `Workload` (e.g., an inbound connection from a `connection:listen` `Workload`).
    *   `scheduled`: Instantiate and execute the `Process`'s DAG on a recurring schedule, defined by a `cron` expression.

---

## Part II: Kernel-Level Primitives

These are the fundamental, non-module capabilities provided by the `gxo` runtime itself. They are implemented by the Kernel to provide a secure and consistent environment for all `Workloads`.

### 2.1. The `Workspace` Primitive

A `Workspace` is an ephemeral, isolated, temporary directory on the host filesystem that the GXO Kernel creates and manages for a specific DAG instance (e.g., a single CI/CD pipeline run or a single `event_driven` workflow invocation).

*   **Purpose:** The `Workspace` provides a clean, private, and temporary filesystem for `Workloads` to perform their actions, such as checking out source code, creating build artifacts, or writing temporary files. This is the primary mechanism for stateful interaction with the filesystem within a single, coherent automation run.

*   **Lifecycle and Scope:** A unique `Workspace` is created by the Kernel at the very beginning of a DAG instance's execution. Every `Workload` within that specific DAG instance automatically executes with this directory as its current working directory. The absolute path to the `Workspace` is made available to all `Workloads` in the DAG via the built-in, read-only state variable `{{ ._gxo.workspace.path }}`. Upon the terminal completion (success or failure) of the *entire* DAG, the Kernel **MUST** guarantee the complete and recursive deletion of the `Workspace` directory. This is a critical cleanup operation handled by a `defer` block in the highest-level orchestrator to ensure it always runs.

*   **Security & Threat Mitigation:** The `Workspace` is a powerful primitive and a potential attack surface. Its implementation is governed by the following security requirements:
    *   **Threat: Path Traversal.** An attacker could craft a `Workload` with a parameter like `path: ../../etc/passwd` to attempt to access or modify files outside the intended directory.
        *   **Mitigation:** The GXO Kernel **MUST** treat the `Workspace` as a chroot jail. Before any module that interacts with the filesystem is invoked, the Kernel **MUST** resolve all file path parameters. Any path that resolves to a location outside the `Workspace`'s absolute path **MUST** be rejected with a fatal security error, terminating the `Workload`. This check is performed by the `WorkloadRunner` before invoking the module.
    *   **Threat: Data Leakage / Tainting.** A failed or malicious `Pipeline` could leave sensitive data (e.g., private keys, tokens) in its `Workspace`. If directory names were predictable, a subsequent, separate `Pipeline` could potentially access this stale data.
        *   **Mitigation:** The `Workspace` directory name **MUST** be cryptographically random and unpredictable. The Kernel will create it at a path like `/var/lib/gxo/workspaces/<UUID>`. The directory **MUST** be created with `0700` permissions, owned exclusively by the `gxo` user, preventing any other user on the system from accessing its contents.
    *   **Threat: Resource Exhaustion (Disk).** A `Workload` could perform an action (e.g., `git clone` of a massive repository) that fills the disk, causing a host-level Denial of Service.
        *   **Mitigation:** While GXO cannot enforce disk quotas directly, it is designed to operate correctly within them. The GXO operational guide and documentation **MUST** strongly recommend that the parent `workspaces` directory (`/var/lib/gxo/workspaces`) be located on a dedicated filesystem partition. This allows system administrators to apply standard OS-level user or group quotas (e.g., via `xfs_quota` or `ext4` project quotas) to the `gxo` user, providing a hard, OS-enforced limit on total disk consumption.

### 2.2. The `Resume Context` Primitive

The `Resume Context` is the Kernel mechanism that enables paused, human-in-the-loop `Workflows`. It is a feature of the `gxo daemon`'s control plane and state management system.

*   **Purpose:** To allow external systems or human operators to inject data into a paused workflow instance and signal it to continue execution.

*   **Components and Function:**
    1.  A `Workload` running the `control:wait_for_signal` module pauses its execution. Its `summary` contains a unique, single-use `token`.
    2.  This `token` is sent to an external system or operator (e.g., via a Slack message).
    3.  The operator makes a decision and uses the `gxo ctl` to resume the workflow, providing the `token` and a JSON `payload` containing the decision data:
        `gxo ctl resume --token <token_from_step_1> --payload '{"approved": true, "reason": "LGTM"}'`
    4.  The `gxo daemon` receives this gRPC request. It uses the `token` to look up the exact paused workflow instance in its persistent state store.
    5.  It then atomically **merges the `payload` into that specific instance's state store** under the reserved key `_gxo.resume_payload`.
    6.  Finally, it signals the paused `Workload` to resume execution. The next `Workload` in the DAG can now access the injected data (e.g., `{{ ._gxo.resume_payload.approved }}`) and proceed conditionally.

---

## Part III: System Architecture & Execution Flow

### 3.1. High-Level Component Diagram
(The Mermaid diagram from the previous response is retained here, as it accurately reflects the target architecture).

### 3.2. Detailed Execution Lifecycle
1.  **Instantiation:** A `Workload` execution is initiated by `gxo run` or the `gxo daemon`'s `Supervisor`.
2.  **Workspace Creation:** The Kernel creates a secure, temporary `Workspace`.
3.  **DAG Construction:** The `DAGBuilder` analyzes the playbook, building a unified dependency graph based on `stream_inputs`, state variables, and status checks. It resolves all policies and detects cycles.
4.  **Channel & Stream Topology Creation:** The `ChannelManager` inspects the DAG and creates all necessary buffered Go channels and `sync.WaitGroup`s for stream synchronization. The `WaitGroup`-based mechanism is critical for preventing race conditions, ensuring a producer `Workload` does not terminate until all its consumers have finished reading its stream.
5.  **Scheduling & Execution:** The main scheduling loop identifies ready `Workloads` and dispatches them to a pool of worker goroutines.
6.  **Workload Execution (`WorkloadRunner`):** For each `Workload` instance:
    *   A task-local `SecretTracker` and `Renderer` are created.
    *   The `when` condition is evaluated.
    *   Loop items are resolved and iterated over.
    *   `params` are rendered. Any `secret` function calls "taint" the `SecretTracker`.
    *   The `module` is invoked. It executes within the context of the `Workspace`.
    *   After `Perform` returns, its `summary` is sanitized by the "Taint and Redact" system before being written to state.
7.  **State Transition & Completion:** Upon a `Workload`'s completion, its status is updated, signaling downstream dependents.
8.  **Teardown:** Once the DAG is complete, the Kernel guarantees the secure deletion of the `Workspace`.

---

## Part IV: Security Architecture

GXO's security model is foundational, based on Defense in Depth and Zero Trust principles.

### 4.1. Control Plane Security
*   **Mandatory Mutual TLS (mTLS):** All `gxo ctl` to `gxo daemon` communication **MUST** use mTLS.
*   **Pluggable Trust Models:** Supports `pki` (CA-based) and `allowlist` (certificate fingerprint) modes.
*   **Certificate-based RBAC:** Authorizes actions based on the client certificate's identity.

### 4.2. Workload Isolation & Sandboxing
*   **Purpose:** To contain the blast radius of a potentially compromised `Workload`. GXO provides a declarative `security_context` to orchestrate OS-level primitives.
*   **Mechanisms:**
    *   **Filesystem Isolation:** `mount` namespaces and `pivot_root`.
    *   **Resource Limiting:** `cgroups` for CPU and memory.
    *   **System Call Filtering:** `seccomp-bpf` with safe default profiles.
    *   **Process Isolation:** `PID` and `IPC` namespaces.

### 4.3. Data Security & Integrity
*   **"Taint and Redact" System:** This is a critical GXO feature. Secrets resolved via the `secret` template function are "tainted" in a per-workload-instance `SecretTracker`. Any attempt to write a tainted value to the state store via `register` or log it in an error message will result in the value being replaced with `[REDACTED_SECRET]`. This prevents accidental credential leakage.
*   **State Encryption at Rest:** The daemon's BoltDB state file will support optional AEAD encryption.
*   **Module Signature Verification:** The daemon will support a `fail-closed` policy to only execute `Workloads` that use modules with a valid cryptographic signature (e.g., via Cosign), preventing the execution of unauthorized or tampered code.