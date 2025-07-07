# GXO – The Declarative Automation Kernel

*A single‑binary runtime that compiles declarative YAML into supervised services, event‑driven workflows, and streaming data‑pipelines—all secured, observable, and composable through a principled, OSI‑style module stack.*

---

## Table of Contents

1. [The Problem: The Glue Code Liability](#the-problem-the-glue-code-liability)
2. [The GXO Vision: An Automation Kernel](#the-gxo-vision-an-automation-kernel)
3. [Inside the Automation Kernel](#inside-the-automation-kernel)
4. [The GXO Automation Model (GXO-AM)](#the-gxo-automation-model-gxo-am)
5. [Core Capabilities: Composable Patterns](#core-capabilities-composable-patterns)
6. [End‑to‑End Example: A Declarative Service](#end-to-end-example-a-declarative-service)
7. [Quick Start](#quick-start)
8. [Documentation](#documentation)
9. [Project Status & Roadmap](#project-status--roadmap)
10. [Contributing](#contributing)
11. [License](#license)

---

## The Problem: The Glue Code Liability

The modern automation landscape is a powerful but fragmented collection of specialized tools. The interstitial space between them—IaC, config management, CI/CD, SOAR—is now the primary source of operational fragility. This gap is filled with procedural, untestable, and unobservable **"glue code,"** creating a significant liability in any production stack.

This fragmentation leads to critical architectural gaps:
*   **The State Gap:** State from one system (e.g., Terraform outputs) is not natively available to another (e.g., a monitoring tool), requiring fragile, out-of-band data passing.
*   **The Logic Gap:** A complete operational process has its logic scattered across HCL, YAML, and Bash, with no single, auditable source of truth.
*   **The Paradigm Gap:** Teams are forced to choose between declarative end-state tools and imperative workflow engines, with no clean way to combine the two paradigms.

**GXO is designed to eliminate these gaps** by providing a unified runtime, not another niche tool.

## The GXO Vision: An Automation Kernel

GXO is architected as an **Automation Kernel**. Like an OS kernel, it provides a minimal, performant set of core services upon which all automation is built. It schedules and manages the execution of **`Workloads`**.

A `Workload` is the atomic, schedulable unit of automation in GXO. It is a declarative definition that fuses two key concepts:
*   A **`Process`**: The logic, or *what to do*. This is composed of a specific `module` and its `params`.
*   A **`Lifecycle`**: The execution policy, or *how and when to run it*. This can be `run_once` (for tasks), `supervise` (for services), `event_driven`, or `scheduled`.

This powerful abstraction allows GXO to manage ephemeral tasks, supervised services, and event-driven workflows within a single, coherent model, all defined in simple YAML.

```yaml
# A GXO playbook is a collection of Workloads.
workloads:
  - name: my_api_server
    lifecycle:
      policy: supervise
      restart: on_failure
    process:
      module: my_api_module
      params:
        port: 8080
```

## Inside the Automation Kernel

The GXO Kernel is a single, statically-linked binary that provides a complete, self-contained execution environment. Its responsibilities are focused and clear:

*   **Scheduler:** A high-concurrency, lock-free scheduler dispatches `Workloads` to a worker pool. It supervises services with configurable restart policies, including exponential back-off.
*   **Unified DAG:** The engine builds a single Directed Acyclic Graph (DAG) that understands both state dependencies (e.g., "Workload B needs the output of Workload A") and native data-stream dependencies. This allows tasks and data pipelines to be seamlessly interwoven.
*   **Streaming Data Plane:** High-throughput, memory-efficient data flow is a first-class citizen. `Workloads` can be connected via buffered channels with intrinsic backpressure, allowing GXO to function as a powerful ETL engine without an external message bus.
*   **Secure State Store:** The Kernel manages a central state store. By default, `Workloads` receive a performant, cycle-safe **deep copy** of any state they read, guaranteeing immutability and preventing data corruption. An explicit policy (`unsafe_direct_reference`) is available for trusted, performance-critical workloads.
*   **Workspace Management:** For each pipeline instance, the Kernel creates and securely cleans up an ephemeral, isolated filesystem `Workspace`, providing a consistent execution environment for all `Workloads` in the DAG.
*   **Security Context:** The Kernel can apply OS-level sandboxing to `Workloads` using `seccomp`, Linux namespaces, and cgroups, providing strong isolation without requiring containerization.

## The GXO Automation Model (GXO-AM)

The GXO module system is a layered architecture conceptually parallel to the **OSI model**. Each layer provides a discrete service, building upon the primitives of the layer below. This principled stack offers high-level convenience without sacrificing low-level control, allowing an operator to work at the highest layer that solves the problem efficiently.

| GXO Layer | Analogy | Purpose | Example Modules |
| :--- | :--- | :--- | :--- |
| **0 – Kernel** | Hardware/OS | Scheduler · DAG · State · Streams | *(built‑in)* |
| **1 – System** | Bare Metal / Syscalls | Processes, filesystem, and kernel control | `exec`, `filesystem:*`, `control:*` |
| **2 – Connection**| Physical/Link | Raw TCP/UDP sockets and listeners | `connection:open`, `connection:listen` |
| **3 – Protocol** | Transport/Session | Structured protocols (HTTP, SSH) | `http:listen`, `ssh:connect` |
| **4 – Data Plane**| Presentation (ETL) | Parsing, transformation, aggregation | `data:parse`, `data:map`, `data:join` |
| **5 – Application** | Application | High‑level service clients | `http:request`, `database:query` |
| **6 – Integration**| Ecosystem | Opinionated wrappers for external tools | `artifact:*`, `terraform:run` |

## Core Capabilities: Composable Patterns

GXO's power is emergent. Complex platforms are not *features* of GXO; they are *patterns* composed from its primitives.

*   **CI/CD Pipelines:** Composed as a DAG of `run_once` `Workloads`. The Kernel's `Workspace` provides a shared filesystem, while `control:barrier` and `artifact:*` modules provide staging and artifact management.
*   **SOAR & Event-Driven Automation:** Composed using `event_driven` `Workloads` (triggered by `http:listen` or message queue modules) that pipe data through the `Data Plane` for enrichment and filtering before taking action.
*   **Human-in-the-Loop Workflows:** Composed using the `control:wait_for_signal` module, which pauses a workflow and emits a resume token. The Kernel's `Resume Context` allows data from an external approval to be injected back into the paused instance's state.
*   **Configuration Management:** Composed using `supervise` `Workloads` and idempotent L1/L5 modules (`filesystem:manage`, `database:query`) to enforce a desired state.

## End‑to‑End Example: A Declarative Service

A 40-line `Workload` that turns GXO into an **SSH guard**: a supervised service that listens for TCP connections, counts attempts per IP, and posts Slack alerts when thresholds are hit.

```yaml
workloads:
  # This workload is a long-running service that listens for TCP connections.
  # Its lifecycle is 'supervise', meaning the kernel will keep it running.
  - name: ssh_guardian_listener
    lifecycle:
      policy: supervise
      restart: always
    process:
      module: connection:listen
      params:
        network: tcp
        address: ":2222"

  # This workload is triggered by events from the listener. For each connection,
  # it runs a full, stateful data pipeline.
  - name: ssh_event_processor
    lifecycle:
      policy: event_driven
      source: ssh_guardian_listener # Consumes events from the listener.
    process:
      # The stream:pipeline module encapsulates a sub-DAG for data processing.
      module: stream:pipeline
      params:
        steps:
          # Step 1: Map the raw connection event to a structured record.
          - id: ssh_event
            uses: data:map # Data Plane (L4) module
            with:
              template: |
                { "remote_ip": "{{ .remote_addr }}", "timestamp": "{{ now }}" }

          # Step 2: Aggregate connection counts in a 5-minute tumbling window.
          # The engine's state store maintains the window's state.
          - id: aggregate
            uses: data:aggregate # Data Plane (L4) module
            needs: [ssh_event]
            with:
              group_by_fields: ["remote_ip"]
              aggregate_fields:
                - { name: attempts, op: count }
              window: "5m"

          # Step 3: Filter for aggregates that meet the alert threshold.
          - id: alert_filter
            uses: data:filter # Data Plane (L4) module
            needs: [aggregate]
            with:
              condition: "{{ .attempts }} >= 10"

          # Step 4: Post an alert to Slack using a high-level Application module.
          - id: slack_alert
            uses: http:request # Application Layer (L5) module
            needs: [alert_filter]
            with:
              method: POST
              url: "${SLACK_WEBHOOK}" # Secrets are resolved from the environment
              body: |
                { "text": "ALERT: SSH guard observed {{ .attempts }} attempts from {{ .remote_ip }} in 5 minutes." }
```

This single file defines a complete, supervised, stateful, and observable microservice. Stop the daemon and restart it, and the 5-minute aggregation window resumes from its persistent checkpoint.

## Documentation

To fully understand GXO's capabilities, explore the detailed documentation:

*   **[Why GXO?](docs/Why%20GXO.md)**: A high-level overview of GXO's philosophy and a comparison with other automation tools. Start here to understand GXO's unique value proposition.
*   **[Architecture Deep Dive](docs/Architecture.md)**: The master technical blueprint detailing the GXO Kernel's design, core abstractions, and execution flow.
*   **[The GXO Standard Library (GXO-SL)](docs/Standard%20Library.md)**: A complete reference for all built-in modules, organized by the GXO Automation Model (GXO-AM).
*   **[Module Developer Guide](docs/Module%20Developer%20Guide.md)**: The canonical guide for building your own custom GXO modules.
*   **[Security Architecture](docs/Security%20Architecture.md)**: A detailed look at GXO's "Defense in Depth" security model, threat model, and control implementations.
*   **[Advanced Example: A Custom TCP Server](docs/Example%20Custom%20Protocol.md)**: A practical example of how to build a stateful network service declaratively using GXO's low-level primitives.
*   **[Future Vision: GXO Fabric](docs/GXO%20Fabric.md)**: A forward-looking design proposal for federating multiple GXO kernels into a fault-tolerant, distributed mesh.

## Project Status & Roadmap

GXO is currently in very early active development, executing a phased plan to realize the full architectural vision. GXO does not currently have a stable release. See the [ROADMAP.md](ROADMAP.md) for the detailed engineering plan.

## License

Apache License, Version 2.0. See `LICENSE` for the full text.