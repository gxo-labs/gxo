# GXO – The Declarative Automation Kernel

*A single‑binary runtime that compiles declarative YAML into supervised services, event‑driven workflows, and streaming data‑pipelines—all secured, observable, and composable through a principled, OSI‑style module stack.*

---

## Table of Contents

1.  [The Problem: The Glue Code Liability](#1-the-problem-the-glue-code-liability)
2.  [The GXO Vision: An Automation Kernel](#2-the-gxo-vision-an-automation-kernel)
3.  [Inside the Automation Kernel](#3-inside-the-automation-kernel)
4.  [The GXO Automation Model (GXO-AM)](#4-the-gxo-automation-model-gxo-am)
5.  [Core Capabilities: Composable Patterns](#5-core-capabilities-composable-patterns)
6.  [End‑to‑End Example: A Declarative Service](#6-end-to-end-example-a-declarative-service)
7.  [Introduction to GXO](#7-introduction-to-gxo)
8.  [Project Status & Roadmap](#8-project-status--roadmap)
9.  [License](#10-license)

---

## 1. The Problem: The Glue Code Liability

The modern automation landscape is a powerful but fragmented collection of specialized tools. The interstitial space between them—IaC, config management, CI/CD, SOAR—is now the primary source of operational fragility. This gap is filled with procedural, untestable, and unobservable **"glue code,"** creating a significant liability in any production stack.

This fragmentation leads to critical architectural gaps:
*   **The State Gap:** State from one system (e.g., Terraform outputs) is not natively available to another (e.g., a monitoring tool), requiring fragile, out-of-band data passing.
*   **The Logic Gap:** A complete operational process has its logic scattered across HCL, YAML, and Bash, with no single, auditable source of truth.
*   **The Paradigm Gap:** Teams are forced to choose between declarative end-state tools and imperative workflow engines, with no clean way to combine the two paradigms.

**GXO is designed to eliminate these gaps** by providing a unified runtime, not another niche tool.

## 2. The GXO Vision: An Automation Kernel

GXO, Go Execution and Orchestration, is architected as an **Automation Kernel**. Like an OS kernel, it provides a minimal, performant set of core services upon which all automation is built. It schedules and manages the execution of **`Workloads`**.

A `Workload` is the atomic, schedulable unit of automation in GXO. It is a declarative definition that fuses two key concepts:
*   A **`Process`**: The logic, or *what to do*. This is composed of a specific `module` and its `params`.
*   A **`Lifecycle`**: The execution policy, or *how and when to run it*. This can be `run_once` (for tasks), `supervise` (for services), `event_driven`, or `scheduled`.

This powerful abstraction allows GXO to manage ephemeral tasks, supervised services, and event-driven workflows within a single, coherent model, all defined in simple YAML.

```yaml
# A GXO playbook is a collection of Workloads.
workloads:
  - name: my_api_server
    # The Lifecycle defines the execution policy. The Kernel will keep this running.
    lifecycle:
      policy: supervise
      restart: on_failure
    # The Process defines the logic to execute.
    process:
      module: my_api_module
      params:
        port: 8080
```

## 3. Inside the Automation Kernel

The GXO Kernel is a single, statically-linked binary that provides a complete, self-contained execution environment. Its responsibilities are formally defined and directly parallel those of a traditional OS kernel.

*   **Workload Management & Scheduling:** A high-concurrency scheduler dispatches `Workloads` based on their `Lifecycle`. It can execute complex DAGs for tasks, supervise services with configurable restart policies, and trigger workflows from events or cron schedules.
*   **Secure State Management:** The Kernel manages a central, concurrency-safe state store. By default, `Workloads` receive a performant, cycle-safe **deep copy** of any state they read, guaranteeing immutability and preventing data corruption.
*   **Workspace & Artifact Management:** For each workflow instance, the Kernel creates and securely cleans up an ephemeral, isolated filesystem `Workspace`, providing a consistent execution environment for all `Workloads` in the DAG.
*   **Native Module System:** Automation logic is executed via trusted, in-process `Modules`, which are analogous to OS device drivers. This native execution model is a key differentiator from systems that delegate tasks to external workers.
*   **Native Inter-Workload Communication:** `Workloads` can be connected via high-throughput, back-pressured data streams (like Unix pipes) or communicate asynchronously via the state store, eliminating the need for external message queues.
*   **Security & Resource Protection:** The Kernel can apply OS-level sandboxing to `Workloads` using `seccomp`, Linux namespaces, and `cgroups`, providing strong isolation without requiring containerization.

## 4. The GXO Automation Model (GXO-AM)

The GXO module system is a layered architecture conceptually parallel to the **OSI model**. Each layer provides a discrete service, building upon the primitives of the layer below. This principled stack offers high-level convenience without sacrificing low-level control, allowing an operator to work at the highest layer that solves the problem efficiently.

| GXO Layer | Analogy | Purpose | Example Modules |
| :--- | :--- | :--- | :--- |
| **6 – Integration**| Ecosystem | Opinionated wrappers for external tools | `artifact:*`, `terraform:run` |
| **5 – Application** | Application | High‑level service clients | `http:request`, `database:query` |
| **4 – Data Plane**| Presentation (ETL) | Parsing, transformation, aggregation | `data:parse`, `data:map`, `data:join` |
| **3 – Protocol** | Transport/Session | Structured protocols (HTTP, SSH) | `http:listen`, `ssh:connect` |
| **2 – Connection**| Physical/Link | Raw TCP/UDP sockets and listeners | `connection:listen`, `connection:open` |
| **1 – System** | Bare Metal / Syscalls | Processes, filesystem, and kernel control | `exec`, `filesystem:*`, `control:*` |
| **0 – Kernel** | Hardware/OS | Scheduler · DAG · State · Streams | *(built‑in)* |

## 5. Core Capabilities: Composable Patterns

GXO's power is emergent. Complex platforms are not *features* of GXO; they are *patterns* composed from its primitives.

*   **CI/CD Pipelines:** Composed as a DAG of `run_once` `Workloads`. The Kernel's `Workspace` provides a shared filesystem, while `control:barrier` and `artifact:*` modules provide staging and artifact management.
*   **Idempotent Configuration Management:** Composed using `supervise` `Workloads` and idempotent L1/L5 modules (`filesystem:manage`, `database:query`) to enforce a desired state.
*   **SOAR & Event-Driven Automation:** Composed using an `event_driven` `Workload` (triggered by an `http:listen` or message queue module) that pipes data through the `Data Plane` for enrichment and filtering before taking action.
*   **Human-in-the-Loop Workflows:** Composed using the `control:wait_for_signal` module, which pauses a workflow and emits a resume token. The Kernel's `Resume Context` allows data from an external approval to be injected back into the paused instance's state.
*   **Stateful Stream Processing (ETL):** Composed as a DAG of `run_once` `Workloads` connected by GXO's native streaming data plane, using stateful modules like `data:aggregate` and `data:join`.

## 6. End‑to‑End Example: A Declarative Service

This example showcases GXO's unique power by composing low-level primitives into a complete, stateful TCP service—declaratively. This playbook turns GXO into a simple Key-Value server that listens on port 6380 and responds to `GET`, `SET`, and `PING` commands.

```yaml
# A GXO Playbook defining a complete, stateful Key-Value TCP Server.

# The 'vars' block initializes our key-value store in the Kernel's state.
vars:
  kv_store:
    initial_key: "hello world"
    another_key: "gxo is powerful"

workloads:
  # WORKLOAD 1: The supervised TCP listener (GXO-AM Layer 2).
  # Its only job is to run forever, accept connections, and produce a
  # stream of connection handle events.
  - name: kv_listener_service
    lifecycle:
      policy: supervise
      restart: on_failure
    process:
      module: connection:listen
      params:
        network: tcp
        address: ":6380"

  # WORKLOAD 2: The protocol handler pipeline.
  # Its 'event_driven' lifecycle subscribes it to the listener's event stream.
  # The Kernel spawns an ephemeral instance of this pipeline for each connection.
  - name: connection_handler_workflow
    lifecycle:
      policy: event_driven
      source: kv_listener_service
    process:
      # This sub-DAG defines the entire request/response logic.
      module: stream:pipeline
      params:
        steps:
          # Step 1: Read the client's command from the socket (Layer 2).
          - id: read_command
            uses: connection:read
            with: { connection_id: "{{ .connection_id }}", read_until: "\n" }

          # Step 2: Parse the command with a template (Layer 4).
          # This transforms the raw data string into a structured command object.
          - id: parse_command
            uses: data:map
            needs: [read_command]
            with:
              template: |
                {{- $parts := .data | trim | split " " -}}
                {{- $cmd := $parts | first | upper -}}
                {{- $output := dict "command" $cmd "key" ($parts | slice 1 | first) "value" ($parts | slice 2 | join " ") -}}
                {{- $output | toJson -}}

          # Step 3: Use conditional workloads to handle different commands.
          - id: handle_get
            uses: control:identity # A simple module to structure the response.
            when: '{{ .command == "GET" }}'
            with:
              response: 'OK {{ .kv_store | get .key | coalesce "not_found" }}'

          - id: handle_set # A full implementation would use a module to write back to state.
            uses: control:identity
            when: '{{ .command == "SET" }}'
            with: { response: "OK" }

          - id: handle_ping
            uses: control:identity
            when: '{{ .command == "PING" }}'
            with: { response: "PONG" }

          # Step 4: Formulate the final response, handling the default error case.
          # This step introspects the DAG to see which handler ran.
          - id: formulate_response
            uses: data:map
            needs: [handle_get, handle_set, handle_ping]
            with:
              template: |
                {{- if .handle_get -}} {{ .handle_get.response }}
                {{- else if .handle_set -}} {{ .handle_set.response }}
                {{- else if .handle_ping -}} {{ .handle_ping.response }}
                {{- else -}} ERR unknown_command
                {{- end -}}

          # Step 5: Write the response and close the connection (Layer 2).
          - id: write_response
            uses: connection:write
            needs: [formulate_response]
            with: { connection_id: "{{ .connection_id }}", data: "{{ . }}\\n" }

          - id: close_connection
            uses: connection:close
            needs: [write_response]
            with: { connection_id: "{{ .connection_id }}" }
```

This single file defines a complete, supervised, stateful, and observable microservice. It demonstrates how GXO's layered primitives can be composed to solve complex problems that would normally require a significant amount of imperative code and external tooling for process supervision and state management.

## 7. Introduction to GXO

GXO is not a simple script runner, but a comprehensive automation platform built on a formal architectural model. To fully understand its capabilities, we recommend exploring the documentation in the following order:

*   **[Why GXO?](docs/Why%20GXO.md)**: A high-level overview of GXO's philosophy and a comparison with other automation tools. **Start here to understand GXO's unique value proposition.**
*   **[Automation Kernel Definition](docs/Automation%20Kernel%20Definition.md)**: The formal, first-principles definition of what an Automation Kernel is and the six responsibilities it must fulfill. This is the "constitution" of the GXO project.
*   **[Flow Control in GXO](docs/Flow%20Control.md)**: A detailed guide to GXO's execution model, explaining how concepts like `run_once`, `supervise`, `when`, and `loop` create powerful and declarative control flows.
*   **[GXO Architecture Design](docs/GXO%20Architecture%20Design.md)**: The master technical blueprint detailing the GXO Kernel's design, core abstractions, and execution flow.
*   **[The GXO Standard Library (GXO-SL)](docs/Standard%20Library.md)**: A complete reference for all built-in modules, organized by the GXO Automation Model (GXO-AM).
*   **[Automation Pattern Taxonomy](docs/Automation%20Pattern%20Taxonomy.md)**: A reference that maps common, real-world automation patterns (CI/CD, SOAR, Idempotent Reconciliation) to their canonical implementation using GXO's primitives.
*   **[Module Developer Guide](docs/Module%20Developer%20Guide.md)**: The canonical guide for building your own custom GXO modules.
*   **[Security Architecture](docs/Security%20Architecture.md)**: A detailed look at GXO's "Defense in Depth" security model, threat model, and control implementations.

## 8. Project Status & Roadmap

GXO is currently in active development to realize the full architectural vision. The project does not currently have a stable release.

The development plan is organized into phased milestones, starting with the foundational refactor to the `Workload` model, followed by hardening the core, implementing the `gxo daemon` with a secure control plane, and then progressively building out the GXO Standard Library. See the [ROADMAP.md](ROADMAP.md) for the detailed engineering plan.

## 9. License

This project is licensed under the Apache License, Version 2.0. See the `LICENSE` file for the full text.
