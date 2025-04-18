# GXO: Go Execution and Orchestration

**GXO is a modern, high-performance automation engine designed for orchestrating complex workflows involving diverse systems and data streams. It is built in Go around a core philosophy: the Automation Kernel.**

---

## Table of Contents

1.  [Overview](#overview)
2.  [The Automation Kernel Philosophy](#the-automation-kernel-philosophy)
3.  [Core Principles](#core-principles)
4.  [Key Features (V0.1 Alpha Engine)](#key-features-v01-alpha-engine)
5.  [Why GXO?](#why-gxo)
6.  [Modularity & The Tiered Hierarchy](#modularity--the-tiered-hierarchy)
7.  [Getting Started](#getting-started)
8.  [Contributing](#contributing)
9.  [License](#license)
10. [Vision](#vision)

---

## Overview

GXO (Go Execution and Orchestration) provides a robust platform for automating sequences of tasks that interact with APIs, transform data, manage infrastructure, run commands, and more. Many modern workflows require gluing together disparate tools, often leading to performance bottlenecks, complex state management, and brittle integrations, especially when dealing with streaming data or complex dependencies.

GXO tackles these challenges head-on. At the center of the GXO Platform is the **`gxo` binary — the Automation Kernel**, responsible for playbook execution, orchestration, and modular task routing. This command-line executable parses declarative YAML **Playbooks** and executes the defined **Tasks**. The broader GXO platform vision includes components like a persistent daemon (`gxod`) and control utility (`gxoctl`), but the foundation is the `gxo` engine itself.

GXO draws inspiration from tools like Ansible, Kubernetes, and other declarative systems. Like Ansible, it uses YAML-based playbooks to describe workflows. Like Kubernetes, it’s built around a DAG of desired state, executed concurrently for efficiency. But unlike both, GXO is a modular automation kernel — optimized for data streaming, concurrency, and plug-and-play extensibility, without requiring a control plane or runtime dependencies.

Written entirely in Go, `gxo` leverages Go's concurrency primitives and performance characteristics to deliver efficient and reliable workflow execution. GXO is designed to run anywhere: as a CLI, in containers, embedded in pipelines, or as a long-lived daemon via the future `gxod`.

## The Automation Kernel Philosophy

GXO is architected around the concept of an **Automation Kernel**.

> **Definition**: An Automation Kernel is a lightweight, extensible core runtime designed to execute orchestrated tasks, services, and data pipelines across diverse environments. It provides foundational automation services—task execution, dependency management (DAG), data transport (streaming), templating, state management—as composable primitives, exposing a modular API for building higher-level capabilities.

Instead of being a monolithic platform attempting to do everything, the `gxo` engine focuses on being the **best possible execution core**. It provides the essential, performant services needed to run complex automation reliably:

*   Parsing and understanding declarative workflows.
*   Analyzing task dependencies (both state-based and data-stream-based).
*   Scheduling and executing tasks concurrently based on a Directed Acyclic Graph (DAG).
*   Managing workflow state securely and efficiently.
*   Providing mechanisms for efficient, low-latency data flow between tasks (streaming).
*   Offering a clear, robust interface for extending functionality via **Modules**.

This kernel approach promotes flexibility, performance, and maintainability. The core engine remains focused and optimized, while specific functionalities are encapsulated in discrete modules.

## Core Principles

*   **Streaming Native:** Designed from the ground up for efficient, low-latency data flow between tasks using Go channels, ideal for data-intensive pipelines.
*   **Modularity & Composability:** Functionality is delivered via self-contained **Modules**. Complex workflows are built by composing these modules declaratively in Playbooks.
*   **Performance:** Leverages Go's concurrency model (goroutines, channels) and efficiency for fast execution, especially for I/O-bound tasks and data streaming.
*   **Declarative Execution:** Define *what* needs to be done in YAML Playbooks; `gxo` figures out *how* to execute it based on dependencies and directives.
*   **Extensibility:** Clear interfaces (Modules, future Hooks/Events) allow `gxo` to be easily extended and integrated into larger systems.

## Key Features (V0.1 Alpha Engine)

The initial V0.1 Alpha release focuses on delivering the **complete core engine logic**:

*   **Declarative YAML Playbooks:** Define workflows with tasks, variables, and control directives.
*   **DAG-Based Execution:** Automatically determines task execution order based on:
    *   **Streaming Dependencies:** Explicit `stream_input` links (pointing to a single producer task in V0.1) for channel-based data flow.
    *   **State Dependencies:** Implicit dependencies derived from tasks consuming results (`register`) or status (`_gxo.tasks...`) of others via templates.
*   **Concurrent Task Execution:** Runs independent tasks in parallel for faster completion.
*   **Streaming Data Flow:** Uses buffered Go channels (`chan map[string]interface{}`) for efficient record passing between tasks.
*   **Native State Management:** In-memory, thread-safe storage for playbook variables, registered task results (using native Go types), and task status tracking.
*   **Go `text/template` Templating:** Render parameters, conditional (`when`), and looping (`loop`) directives using Go's built-in templating engine with access to state.
*   **Rich Task Directives:** Control execution flow with `when`, `loop` (with parallelism control), `retry` (with configurable delay/attempts), `register`, and `ignore_errors`.
*   **Robust Error Handling:** Distinguishes between fatal task errors and non-fatal record-processing errors (via `errChan`), supporting `ignore_errors` logic.
*   **Modularity:** Compile-time module registration system with a clearly defined `module.Module` interface.
*   **`exec` Primitive Module:** The first module, allowing execution of local shell commands, fully implementing the module interface.
*   **Dry Run Mode:** Simulate playbook execution without performing side effects via the `-dry-run` flag.

## Why GXO?

Existing orchestration platforms are often:

*   **Script-heavy and Imperative:** Focusing on *how* tasks run step-by-step, leading to complex and brittle glue code.
*   **Lacking True Modularity:** Offering monolithic plugins or integrations that are hard to compose or extend cleanly.
*   **Weak at Concurrent Data Handling:** Struggling with efficient processing of large or continuous data streams between tasks.
*   **Difficult to Extend Cleanly:** Requiring deep integration or complex SDKs to add custom functionality.

GXO solves these problems by being a lightweight, streaming-native kernel that treats orchestration like **infrastructure**, not like glue code. It provides:

*   **Declarative Power:** Focus on *what* needs to happen; the kernel optimizes the *how*.
*   **Composable Modules:** Build complex workflows by connecting robust, reusable components.
*   **First-Class Streaming:** Efficiently handle data pipelines alongside traditional task orchestration.
*   **Performance by Design:** Leverage Go's concurrency and efficiency for demanding workloads.
*   **Clean Extensibility:** A clear module interface makes adding new capabilities straightforward.

## Modularity & The Tiered Hierarchy

Modularity is central to GXO's design. Functionality is provided by **Modules**, which are self-contained Go packages implementing the `module.Module` interface.

We envision modules organized in a **Tiered Hierarchy**:

1.  **Tier 1: Primitives:** These modules provide fundamental, low-level building blocks that interact directly with the operating system or core network protocols. They aim for minimal abstraction and maximal control.
    *   **Example (`exec` - V0.1 Alpha):** Executes local commands.
    *   **Example (Future `tcp`):** Provides raw TCP socket communication capabilities (connect, listen, send, receive).
    *   **Example (Future `filesystem`):** Provides basic file operations (read, write, list, stat).

2.  **Tier 2+: Services:** These modules build upon Primitives (or other Services) to offer higher-level, more abstract capabilities, often interacting with specific applications, protocols, or APIs. They encapsulate common patterns and reduce boilerplate.
    *   **Example (Future `ssh`):** Would *use* the `tcp` primitive module's capability to establish the underlying connection, then layer the SSH protocol logic on top to provide secure command execution or file transfer. It wouldn't need to reimplement TCP handling.
    *   **Example (Future `http_request`):** Would likely use a `tcp` primitive (or a standard Go `net/http` library wrapper) to make HTTP calls.
    *   **Example (Future `rest_api`):** Could build upon `http_request` to simplify interactions with specific RESTful APIs (handling authentication, common headers, JSON parsing).
    *   **Example (Future `file_transfer`):** Could use `filesystem` and potentially `ssh` or other protocol modules.

This tiered approach promotes:

*   **Reusability:** Low-level logic (like handling TCP connections) is implemented once in a primitive module and reused by many higher-level service modules.
*   **Separation of Concerns:** Modules focus on specific tasks, making them easier to develop, test, and maintain.
*   **Faster Development:** Complex capabilities can be built more quickly by composing existing primitives and services.

## Getting Started

*(Note: GXO is currently in early development. This section describes the intended usage once stable builds are available.)*

1.  **Installation:** (Instructions TBD - likely download binary or use `go install`)
2.  **Create a Playbook:** Define your workflow in a YAML file (e.g., `my_playbook.yml`).
    ```yaml
    name: simple_command_example
    tasks:
      - name: say_hello
        type: exec # Use the built-in 'exec' module
        params:
          command: echo
          args:
            - "Hello from GXO!"
        register: hello_output # Store the result in state

      - name: show_output
        type: exec
        params:
          # Use the registered result from the previous task
          command: echo
          args:
            - "Task 1 said:"
            - "{{ .hello_output.stdout }}" # Access state via Go templating
    ```
3.  **Run the Playbook:**
    ```bash
    gxo -playbook my_playbook.yml
    ```
4.  **Expected Output (Example):**
    ```log
    INFO[0000] Starting playbook execution: simple_command_example
    INFO[0000] Building execution DAG...
    INFO[0000] DAG built successfully. Found 2 tasks.
    INFO[0000] Running task: say_hello
    INFO[0000] module=exec task=say_hello Executing command: /bin/echo [Hello from GXO!]
    INFO[0000] Task 'say_hello' completed successfully. Registering result to 'hello_output'.
    INFO[0000] Running task: show_output
    INFO[0000] module=exec task=show_output Executing command: /bin/echo [Task 1 said: Hello from GXO!
    ]
    INFO[0000] Task 'show_output' completed successfully.
    INFO[0000] Playbook execution finished successfully: simple_command_example
    ```
5.  **Explore:** Check the `examples/` directory (coming soon) for more complex use cases demonstrating features like streaming, looping, conditionals, and retries. Dive into the code, starting with `cmd/gxo/main.go` and the `internal/engine/` package to understand the core logic.

## Contributing

We welcome contributions! If you're interested in fixing bugs, adding features, creating new modules, or improving documentation, please check out our (upcoming) Contribution Guidelines.

## License

GXO is licensed under the Apache License, Version 2.0. See the [LICENSE](LICENSE) file for details.

## Vision

The V0.1 Alpha release establishes the complete core `gxo` engine. Future development aims to:

*   Add more **Primitive** and **Service Modules** (filesystem, networking, APIs, cloud services, databases, etc.).
*   Implement the **`gxod` daemon** for API-driven execution, scheduling, and persistent state.
*   Develop the **`gxoctl` control utility** for interacting with `gxod`.
*   Introduce advanced features like dynamic module loading, richer templating options (e.g., Jinja2), enhanced secrets management, and comprehensive observability integrations (hooks, events).
*   Refine policies for fine-grained control over execution behavior (e.g., channel backpressure, task error handling).

Our goal is to build a powerful, flexible, and performant **GXO Platform** for modern automation challenges, all built upon the solid foundation of the `gxo` Automation Kernel.
