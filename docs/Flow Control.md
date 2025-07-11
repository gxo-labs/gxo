# **Flow Control in GXO: A Unified Model for Declarative and Imperative Automation**

## 1. Abstract

The history of automation is marked by a persistent conflict between two paradigms: declarative systems that define a desired end-state, and imperative systems that define an explicit sequence of steps. This division forces engineers to use separate, often poorly integrated, tools for infrastructure provisioning, configuration management, and workflow orchestration, leading to fragile "glue code," architectural seams, and operational complexity.

GXO is architected from first principles to resolve this conflict. It achieves this not by creating another scripting language embedded in YAML, but by providing a unified composition model for automation. Its core abstractions—the `Workload`, `Process`, and `Lifecycle`—allow users to declaratively compose simple sequential tasks, complex conditional and parallel workflows, long-running supervised services, and event-driven automation, all within a single, coherent framework.

This document provides an explanation of GXO's flow control mechanisms. It details how the engine's architecture unifies these disparate paradigms, offering the expressiveness of imperative control flow without sacrificing the readability, composability, and reliability of declarative automation. It is the canonical reference for understanding how GXO schedules, sequences, and manages the execution of automation logic.

## 2. The GXO "Object Model": Composition Over Scripting

To understand flow control in GXO, one must first abandon the mental model of a top-to-bottom script. GXO does not execute a playbook; it *reconciles* one. While YAML is a data-serialization format, GXO's design intentionally applies object-oriented *principles* to its configuration model. This "object model" is the foundation of its powerful flow control system.

### 2.1. Encapsulation: The `Workload`

The `Workload` is the fundamental, atomic "object" in GXO. It is a self-contained unit that encapsulates *what* to do (the `Process`) and *how/when* to do it (the `Lifecycle`). This clean separation of concerns, is the cornerstone of GXO's flexibility.

*   **The `Process`:** This block defines the *inert logic* of the workload. It specifies a `module` (the implementation) and its `params` (the data). A `Process` block on its own does nothing; it is a reusable blueprint for an action.
*   **The `Lifecycle`:** This block defines the *execution policy* that the GXO Kernel will apply to the `Process`. It answers questions like: Should this run once? Should it be kept running forever? Should it run on a schedule?

```yaml
# A Workload "object" cleanly encapsulates policy and logic.
workloads:
  - name: my_api_server
    # Lifecycle (Policy): The "how" and "when". This is the execution contract.
    lifecycle:
      policy: supervise
      restart: on_failure
    # Process (Data + Behavior): The "what". This is the automation logic.
    process:
      module: my_api_module
      params: { port: 8080 }
```

### 2.2. Composition: The Playbook

A GXO playbook is not a script. It is a declarative composition of `Workload` objects. The GXO Kernel acts as an intelligent runtime that parses this composition, analyzes the relationships between the `Workload` objects, and orchestrates their execution according to their declared dependencies and lifecycle policies. This composition is realized as a Directed Acyclic Graph (DAG) internally.

### 2.3. Polymorphism: The Power of the `Lifecycle`

GXO's most powerful architectural feature is **Lifecycle Polymorphism**. This is the principle that the same `Process` can exhibit entirely different runtime behaviors simply by being composed with a different `Lifecycle` policy. This allows a single piece of automation logic to be reused across radically different operational paradigms.

Consider this `Process` definition, which defines the logic for running a health check:
```yaml
process:
  module: exec
  params:
    command: "/usr/local/bin/my-health-check"
```
By composing this `Process` with different `Lifecycle` policies, we achieve completely different flow control outcomes:
1.  **As a one-off task in a CI pipeline:**
    `lifecycle: { policy: run_once }`
2.  **As a supervised, self-healing service monitor:**
    `lifecycle: { policy: supervise, restart: on_failure }`
3.  **As a scheduled, periodic job:**
    `lifecycle: { policy: scheduled, cron: "*/5 * * * *" }`
4.  **As a reactive handler triggered by an external event:**
    `lifecycle: { policy: event_driven, source: some_event_stream }`

This architectural choice—composing polymorphic `Workloads`—is the foundation upon which all GXO flow control is built. It eliminates the need for separate tools to handle tasks, services, and scheduled jobs.

## 3. Detailed Flow Control Mechanisms

Flow control in GXO emerges from the interaction of several architectural components. For platform developers, understanding how these mechanisms interoperate is key to leveraging the full power of the system.

### 3.1. Dependency-Based Flow: The Unified DAG

For `run_once` lifecycles—the foundation of imperative workflows like CI/CD pipelines or multi-step provisioning—the primary flow control mechanism is the **Directed Acyclic Graph (DAG)**. The GXO Kernel builds this graph at runtime by statically analyzing the playbook, automatically inferring dependencies without requiring explicit `depends_on` keys.

#### **Mechanism 1: Implicit State Dependencies (Sequential Flow)**
This is the primary mechanism for establishing a sequence and passing data between steps. A workload implicitly depends on another if its `params` or `when` clause contains a Go template that references the registered result of the other workload.

*   **Engine Behavior:** During the `BuildDAG` phase, the Kernel's template engine performs a pre-rendering pass on all template strings. It uses `ExtractVariables` to identify all referenced variables (e.g., `{{ .task_a_result.stdout }}`). If a variable corresponds to a `register` key of another workload, a **state dependency edge** is created in the DAG from the producer (`task_a`) to the consumer. A workload with unresolved state dependencies will not be scheduled.
*   **Architectural Role:** Enables a clean, data-driven approach to sequential execution. The flow of control follows the flow of data, a robust and easy-to-reason-about pattern.
*   **Example:** `task_b` will not be scheduled until `task_a` completes successfully and registers the `task_a_result` variable into the state store.
    ```yaml
    workloads:
      - name: task_a
        lifecycle: { policy: run_once }
        process: { module: exec, params: { command: "date" } }
        register: task_a_result

      - name: task_b
        lifecycle: { policy: run_once }
        process:
          module: exec
          params: { command: 'echo "Task A finished at: {{ .task_a_result.stdout }}"' }
    ```

#### **Mechanism 2: Explicit Stream Dependencies (Parallel Data Flow)**
For data-intensive pipelines, dependencies are declared explicitly via the `stream_inputs` key. This allows for high-throughput, parallel processing where consumers can process data as soon as producers generate it.

*   **Engine Behavior:** When the Kernel sees a `stream_inputs` key, it creates a **stream dependency edge** in the DAG. It then instantiates a buffered Go channel for each link. Unlike state dependencies, a stream consumer does not have to wait for the producer to *complete*. The consumer is scheduled to run concurrently and begins reading from the channel as soon as the producer starts writing to it. The Kernel's `ChannelManager` handles the complex synchronization (`sync.WaitGroup`) to ensure the producer workload does not terminate until all its consumers have fully drained its stream.
*   **Architectural Role:** This is the core of GXO's ETL and streaming data processing capabilities, enabling a powerful, back-pressured data plane without external message queues.
*   **Example:** `data_processor` and `data_producer` run in parallel.
    ```yaml
    workloads:
      - name: data_producer
        lifecycle: { policy: run_once }
        process: { module: data:generate_from_list, params: { items: [1, 2, 3] } }

      - name: data_processor
        lifecycle: { policy: run_once }
        stream_inputs: [data_producer]
        process: { module: passthrough }
    ```

#### **Mechanism 3: Synchronization Primitives (`control:barrier`)**
The `control:barrier` module is a specialized workload that acts as a powerful synchronization point, enabling the creation of explicit stages in a complex parallel workflow.

*   **Engine Behavior:** A barrier workload is a pure stream consumer. It takes one or more `stream_inputs`. The Kernel schedules it, but the `control:barrier` module's `Perform` method simply blocks until it has received an End-of-Stream signal on *all* of its input channels. Once all producers have completed, the barrier completes successfully, allowing workloads that depend on it to proceed.
*   **Architectural Role:** This is the canonical way to implement fan-in logic. It is essential for CI/CD pipelines where, for example, a "deploy" stage must only begin after multiple, parallel "build" and "test" stages have all succeeded.
*   **Example:** `deploy_stage` will only be considered for execution after `test_barrier` completes, which in turn only completes after both `build_backend` and `build_frontend` have finished.
    ```yaml
    workloads:
      - name: build_backend
        # ...
      - name: build_frontend
        # ...
      - name: test_barrier
        lifecycle: { policy: run_once }
        process: { module: control:barrier }
        stream_inputs: [build_backend, build_frontend]

      - name: deploy_stage
        lifecycle: { policy: run_once }
        when: '{{ eq ._gxo.workloads.test_barrier.status "Completed" }}'
        # ...
    ```

### 3.2. Intra-Workload Flow Control

While the DAG and Lifecycle policies govern the "macro" flow of execution between workloads, GXO also provides a rich set of directives to control the "micro" flow *within* a single `run_once` workload instance. These directives provide the fine-grained, imperative-style control necessary for building complex and resilient automation steps.

#### **Mechanism 1: Conditional Execution (`when`)**

The `when` directive is GXO's primary mechanism for conditional branching. It allows a workload to be executed or skipped based on the current state of the system, enabling dynamic and context-aware workflows.

*   **Engine Behavior:** Before a workload is dispatched to a worker for execution, the GXO Kernel evaluates the Go template string provided in the `when` key. This evaluation happens against the full, read-only state store, giving the condition access to playbook variables, secrets, and the registered results of any completed prerequisite workloads. The engine evaluates the rendered string for "truthiness":
    *   **Truthy Values:** `true`, any non-zero number, any non-empty string. If the result is truthy, the workload proceeds with its execution as normal.
    *   **Falsy Values:** `false`, `0`, `""` (empty string), `nil`. If the result is falsy, the Kernel bypasses execution entirely. It immediately transitions the workload's status to `Skipped`.
*   **Architectural Role:** The `Skipped` status is a terminal state, but it is not a failure. Any downstream workloads that depend on a skipped workload (either via state or stream dependencies) will have their dependency satisfied and will become eligible for execution. This allows for the creation of optional paths in a workflow. The `when` directive is the canonical way to implement feature flags, environment-specific logic (e.g., "run this only in production"), and validation checks (e.g., "run this only if the previous step's exit code was 0").

*   **Example 1: Environment-Specific Deployment**
    This workload will only run if the playbook's `env` variable is set to `"production"`.
    ```yaml
    vars:
      env: "staging"
    workloads:
      - name: deploy_to_production
        lifecycle: { policy: run_once }
        when: '{{ eq .env "production" }}' # This will evaluate to false.
        process:
          module: exec
          params: { command: "./scripts/deploy_prod.sh" }
    # Result: The 'deploy_to_production' workload will be Skipped.
    ```

*   **Example 2: Conditional on a Previous Step's Output**
    This workload checks the `stdout` of a previous command to decide whether to proceed.
    ```yaml
    workloads:
      - name: check_disk_space
        lifecycle: { policy: run_once }
        process:
          module: exec
          params: { command: "df -h / | tail -n 1 | awk '{print $5}' | tr -d '%'" }
        register: disk_usage

      - name: run_cleanup_script
        lifecycle: { policy: run_once }
        # The template converts the stdout string to an integer and checks if it's > 90.
        when: '{{ gt (int .disk_usage.stdout) 90 }}'
        process:
          module: exec
          params: { command: "./scripts/cleanup.sh" }
    ```

#### **Mechanism 2: Iterative Execution (`loop`)**

The `loop` directive is GXO's declarative replacement for imperative `for` loops. It enables a single workload definition to be executed multiple times over a collection of items, dramatically reducing boilerplate for repetitive tasks.

*   **Engine Behavior:** The Kernel first resolves the `loop` value. This value can be a literal list defined directly in the YAML or a Go template string that resolves to a list or map from the state store. The `TaskRunner` then iterates over this collection. For each item in the collection, it creates a separate execution instance of the workload's `Process`.
    *   **Loop Variable:** Within each iteration, the current item is injected into the template context. By default, it is available as the `item` variable, but this can be customized using `loop_control.loop_var`.
    *   **Parallel Execution:** The `loop_control.parallel` key instructs the `TaskRunner` to use a semaphore to run up to `N` iterations concurrently, providing a simple way to parallelize tasks. If not specified, loops run sequentially.
    *   **Completion and Results:** The workload as a whole is considered complete only after *all* loop iterations have finished. If `register` is used, the result will be a list containing the summary from each successful iteration. If any single iteration fails, the entire workload fails.

*   **Architectural Role:** This is the canonical pattern for performing idempotent actions across a set of similar resources. Common use cases include creating a list of users, installing a list of software packages, creating multiple cloud resources, or applying a configuration template to a list of target devices.

*   **Example 1: Creating Multiple User Directories**
    This workload iterates over a list of names and creates a home directory for each. The iterations run in parallel, up to a limit of 5 at a time.
    ```yaml
    workloads:
      - name: create_user_dirs
        lifecycle: { policy: run_once }
        loop: ["alice", "bob", "carol", "dave", "eve"]
        loop_control:
          loop_var: "username"
          parallel: 5
        process:
          module: filesystem:manage
          params:
            path: "/home/{{ .username }}"
            state: "directory"
            owner: "{{ .username }}"
            mode: "0700"
    ```

*   **Example 2: Querying a Database Based on Previous Results**
    This example shows how `loop` can iterate over the registered result of a previous workload.
    ```yaml
    workloads:
      - name: get_active_user_ids
        lifecycle: { policy: run_once }
        process: { module: database:query, params: { query: "SELECT id FROM users WHERE active=true" } }
        register: active_users

      - name: refresh_user_cache
        lifecycle: { policy: run_once }
        # Loop over the list of row objects from the database query.
        loop: "{{ .active_users.rows }}"
        loop_control:
          loop_var: "user_row"
        process:
          module: http:request
          params:
            method: "POST"
            url: "https://cache-service.internal/refresh"
            body: '{ "user_id": {{ .user_row.id }} }'
    ```

#### **Mechanism 3: Resilient Execution (`retry` and `ignore_errors`)**

GXO provides declarative mechanisms for building resilient workflows that can gracefully handle transient failures.

*   **`retry` Directive**
    *   **Engine Behavior:** When a workload is configured with a `retry` block, the `TaskRunner` wraps the call to its `Perform` method in a retry loop. If `Perform` returns an error, the `TaskRunner` does not immediately fail the workload. Instead, it consults the retry policy. It will wait for the specified `delay`, potentially increasing that delay based on the `backoff_factor` and randomizing it with `jitter`, and then re-invoke `Perform`. This cycle repeats up to the configured number of `attempts`. The workload only enters a `Failed` state if all attempts are exhausted.
    *   **Architectural Role:** This provides robust, declarative handling for transient errors, such as temporary network outages, API rate limiting, or race conditions in distributed systems, without requiring custom scripting logic.

    *   **Example:** An HTTP request that retries up to 3 times on failure, with an exponential backoff.
        ```yaml
        workloads:
          - name: call_flaky_api
            lifecycle: { policy: run_once }
            process:
              module: http:request
              params: { url: "https://api.example.com/data" }
            retry:
              attempts: 3
              delay: "2s"         # Start with a 2-second delay
              backoff_factor: 2.0   # Double the delay on each failure (2s, 4s)
              jitter: 0.1         # Add +/- 10% randomization to the delay
        ```

*   **`ignore_errors: true` Directive**
    *   **Engine Behavior:** If a workload ultimately fails (after exhausting all retries), the `ignore_errors: true` directive changes the behavior of the main engine scheduler. The workload's status is still set to `Failed`, and its direct dependents will not run. However, the Kernel will not halt the entire playbook execution. It will continue to schedule other independent branches of the DAG that do not depend on the failed workload.
    *   **Architectural Role:** This is useful for non-critical clean-up tasks, "best-effort" notifications, or optional steps in a workflow whose failure should not prevent the primary goal from being achieved.

    *   **Example:** A CI pipeline where a failure to post a Slack notification does not fail the entire build.
        ```yaml
        workloads:
          - name: run_tests
            # ...
          - name: post_slack_update
            when: '{{ eq ._gxo.workloads.run_tests.status "Completed" }}'
            lifecycle: { policy: run_once }
            ignore_errors: true # If the Slack API is down, don't fail the build.
            process:
              module: slack:post_message
              params: { message: "Tests passed!" }
        ```

### 3.4. Resilient and Advanced Flow Control

Beyond establishing the primary execution graph, GXO provides a suite of mechanisms for creating resilient, real-world automation that can handle failure, repetition, and complex runtime paradigms. These features are not afterthoughts; they are deeply integrated into the Kernel's execution model.

#### **Mechanism 1: Resilient Execution (`retry` and `ignore_errors`)**

Real-world automation must contend with transient failures. GXO provides declarative directives to build this resilience directly into the playbook, eliminating the need for fragile wrapper scripts with retry loops.

*   **The `retry` Directive**
    *   **Engine Behavior:** When a workload is configured with a `retry` block, the `TaskRunner` wraps the call to its `Perform` method in a sophisticated retry loop. If `Perform` returns an error, the `TaskRunner` intercepts it and, instead of immediately failing the workload, consults the retry policy. It will wait for the specified `delay`, potentially increasing that delay based on the `backoff_factor` and randomizing it with `jitter` to avoid thundering herd problems. It then re-invokes the module's `Perform` method with a fresh context. This entire cycle of failure, delay, and re-execution is managed within the execution of a single workload instance and is repeated up to the configured number of `attempts`. The workload only transitions to a `Failed` state if all attempts are exhausted.
    *   **Architectural Role:** This provides robust, declarative handling for transient errors, such as temporary network outages, API rate limiting, or race conditions in distributed systems. It makes workflows inherently more reliable without complicating the playbook with external logic.
    *   **Example: Resiliently Calling a Flaky API**
        This workload attempts to fetch data from an API that may occasionally fail. It will retry up to 3 times, waiting 2 seconds after the first failure, 4 seconds after the second, plus or minus 10% jitter.
        ```yaml
        workloads:
          - name: call_flaky_api
            lifecycle: { policy: run_once }
            process:
              module: http:request
              params: { url: "https://api.example.com/data" }
            retry:
              attempts: 3
              delay: "2s"         # Start with a 2-second delay
              backoff_factor: 2.0   # Double the delay on each failure (2s, 4s)
              jitter: 0.1         # Add +/- 10% randomization to the delay
        ```

*   **The `ignore_errors: true` Directive**
    *   **Engine Behavior:** If a workload ultimately fails (after exhausting all retries), the `ignore_errors: true` directive changes the behavior of the main engine scheduler. The workload's status is still authoritatively set to `Failed`, and any direct dependents (both state and stream) will *not* be scheduled to run. However, this failure is treated as non-fatal *to the playbook as a whole*. The Kernel will not halt the entire execution graph and will continue to schedule other independent branches of the DAG that do not depend on the failed workload.
    *   **Architectural Role:** This is essential for non-critical "best-effort" tasks. Common use cases include sending optional notifications (e.g., a Slack message whose failure shouldn't fail a build), running non-essential cleanup tasks, or collecting optional telemetry where a failure is acceptable.
    *   **Example: Non-Critical Notification in a CI Pipeline**
        In this pipeline, a failure to post a success message to Slack will be logged, but it will not cause the entire CI run to be marked as failed.
        ```yaml
        workloads:
          - name: run_critical_tests
            # ...
          - name: post_success_notification
            # This depends on the tests completing successfully.
            when: '{{ eq ._gxo.workloads.run_critical_tests.status "Completed" }}'
            lifecycle: { policy: run_once }
            ignore_errors: true # If the Slack API is down, don't fail the build.
            process:
              module: slack:post_message
              params: { message: "Build {{ ._gxo.playbook.id }} passed tests!" }
        ```

#### **Mechanism 2: Lifecycle-Driven Flow**
This is where GXO's architecture moves beyond the capabilities of a simple workflow engine and becomes a true automation platform. The flow of control is no longer limited to a one-shot DAG but can become a continuous or reactive process. This is managed by the long-running **`gxo daemon`**, which contains internal *reconciler loops* that continuously monitor and manage workloads based on their declared lifecycle policy.

*   **The `supervise` Lifecycle**
    *   **Engine Behavior:** When the daemon loads a workload with a `supervise` lifecycle, its "supervisor reconciler" takes ownership. The reconciler's control loop is simple but powerful: `IS_PROCESS_RUNNING? -> (if no) -> START_PROCESS`. It starts the workload's `Process` and then monitors the process handle. If the process exits, the reconciler consults the `restart` policy (`on_failure`, `always`, `never`). If a restart is required, it applies an exponential back-off delay to prevent rapid crash-loops before re-launching the process. The "flow" is a continuous reconciliation toward the declared state: "this process should be running."
    *   **Architectural Role:** This directly replaces external process managers like `systemd`, `supervisord`, or `pm2`. It allows the definition of a long-running service to live in the same declarative playbook as the automation that deploys or configures it, unifying the entire application lifecycle.
    *   **Example: Running a Web Server as a Supervised Service**
        ```yaml
        workloads:
          - name: my_production_web_server
            lifecycle:
              policy: supervise
              restart: on_failure # Only restart if it exits with a non-zero code.
            process:
              module: exec
              params:
                command: "/opt/my-app/bin/server"
                args: ["--config", "/etc/my-app/config.json"]
        ```

*   **The `event_driven` Lifecycle**
    *   **Engine Behavior:** The daemon's "event-loop reconciler" subscribes to the event stream produced by the specified `source` workload (e.g., a `connection:listen` workload). The flow is reactive: `WAIT_FOR_EVENT -> (on event) -> INSTANTIATE_AND_RUN_DAG`. For every single event received from the source, the Kernel instantiates a new, ephemeral, and completely isolated execution of the event-driven workload's process. This new instance gets its own DAG, its own temporary `Workspace`, and its own state scope (though it can read global state).
    *   **Architectural Role:** This is the core of GXO's ability to act as a networking and event-processing platform. It is the canonical pattern for building custom network servers (as seen in the `Example Custom Protocol.md`), message queue consumers, and webhook handlers, all declaratively.
    *   **Example: A Declarative TCP Server**
        Here, `connection_handler` runs a new pipeline for every connection accepted by `tcp_listener`.
        ```yaml
        workloads:
          - name: tcp_listener
            lifecycle: { policy: supervise, restart: always }
            process:
              module: connection:listen
              params: { network: tcp, address: ":8080" }

          - name: connection_handler
            lifecycle:
              policy: event_driven
              source: tcp_listener # Subscribes to the listener's event stream.
            process:
              module: stream:pipeline
              params:
                steps:
                  # This sub-DAG runs for every single inbound connection.
                  - { id: read, uses: connection:read, with: { ... } }
                  - { id: process, uses: data:map, with: { ... } }
                  - { id: respond, uses: connection:write, with: { ... } }
        ```

*   **The `scheduled` Lifecycle**
    *   **Engine Behavior:** The daemon's "cron reconciler" parses the `cron` expression and schedules future executions. At each specified time, it triggers a new, ephemeral execution of the workload's DAG, identical to how an `event_driven` workload is triggered, but based on time instead of an external event.
    *   **Architectural Role:** This replaces system `cron` or other external schedulers for running periodic automation like daily reports, nightly backups, or hourly health checks, bringing scheduling into the same unified GXO model.
    *   **Example: Nightly Database Backup**
        ```yaml
        workloads:
          - name: nightly_db_backup
            lifecycle:
              policy: scheduled
              cron: "0 2 * * *" # Run every day at 2:00 AM.
            process:
              module: exec
              params: { command: "/usr/local/bin/backup_database.sh" }
        ```

#### **Mechanism 3: Human-in-the-Loop (`control:wait_for_signal`)**
This advanced mechanism allows a GXO workflow to explicitly pause its execution and cede control to an external entity, enabling interactive or approval-based flows that are impossible in purely linear systems.

*   **Engine Behavior:**
    1.  A workload running the `control:wait_for_signal` module executes within the `gxo daemon`.
    2.  The module's `Perform` method makes a request to the Kernel's state manager, signaling its intent to pause.
    3.  The Kernel generates a unique, cryptographically secure, single-use **resume token**.
    4.  It then takes a complete, consistent snapshot of that *specific workflow instance's state* and persists it to a durable store (e.g., BoltDB).
    5.  The workload's status is set to `Paused`, and the resume token is returned as its `summary`, which can then be sent to an external system (e.g., posted to a Slack channel). The workflow's execution context remains in memory but is dormant.
    6.  An operator or external system issues a `gxo ctl resume --token <token> --payload '{"approved": true, "approver": "ops-lead"}'` command to the daemon's gRPC API.
    7.  The daemon validates the token, finds the persisted state snapshot, and **atomically merges the provided JSON payload** into the state.
    8.  Finally, it signals the paused workload's reconciler to "wake up" and continue execution from the exact point it left off, now with the new approval data available in its state.
*   **Architectural Role:** This is a powerful and robust primitive for building complex automation that requires human judgment. It is the canonical solution for multi-stage deployment pipelines with manual approval gates, interactive configuration wizards where a user must provide input mid-flow, and complex incident response runbooks that pause to wait for an engineer's diagnosis before taking automated remediation steps. It completely eliminates the need for brittle, external polling loops or complex state machines.

*   **Example: A Deployment Pipeline with a Manual Approval Gate**
    ```yaml
    workloads:
      - name: build_and_test
        # ...
      - name: wait_for_prod_approval
        when: '{{ eq ._gxo.workloads.build_and_test.status "Completed" }}'
        lifecycle: { policy: run_once }
        process:
          module: control:wait_for_signal
          params:
            context: # This data is stored with the paused state for context.
              artifact_version: "{{ .build_and_test.summary.version }}"
              deploy_target: "production-us-east-1"
        register: approval_request

      - name: deploy_to_prod
        # This workload depends on the approval data injected by the 'resume' command.
        when: '{{ and (eq ._gxo.workloads.wait_for_prod_approval.status "Completed") (.approval_request.resume_payload.approved) }}'
        process:
          # ...
    ```

## 5. Conclusion: A Unified and Composable System

GXO's approach to flow control is fundamentally different from other tools. It does not provide a new scripting language embedded in YAML. Instead, it provides a powerful, declarative **composition engine** that unifies previously separate automation paradigms.

*   **Imperative Workflows are Composed:** A CI/CD pipeline is not a script; it is a **composition** of `run_once` workloads connected by an intelligently derived DAG. Conditionals (`when`) and loops (`loop`) are declarative properties of these workload objects, not free-floating control statements.
*   **Declarative End-States are Composed:** A supervised service is not a special entity; it is a **composition** of a `Process` and a `supervise`d `Lifecycle`.

This unified architecture allows a single `gxo.yaml` file to define both the imperative workflow that builds and tests an application and the declarative end-state that runs that application in production. By unifying these paradigms under a single, coherent model, GXO eliminates the architectural gaps and brittle glue code that plague modern automation platforms, providing a truly next-generation foundation for systems engineering.

---

## 6. Appendix: Learning from History—How GXO Avoids Prior Art Pitfalls

The history of automation tools is littered with attempts to bridge the declarative-imperative divide. These attempts have revealed a consistent set of pitfalls. GXO's architecture was purpose-built around these lessons.

| Pitfall | Historical Example | GXO's Architectural Mitigation |
| :--- | :--- | :--- |
| **"YAML-Turing" Complexity** | Ansible/Jinja or Jenkinsfiles becoming unmaintainable scripts-in-YAML. | **The Module Escalation Rule.** Heavy logic is encapsulated in versioned, testable Go modules. The YAML *orchestrates*; the module *implements*. This maintains a clean separation of concerns. |
| **Expressiveness vs. Simplicity Tension** | Rigid declarative tools (early Tekton, Jenkins Declarative) forcing users to "escape hatch" to external scripts, breaking the model. | **Lifecycle Polymorphism.** GXO's model is inherently more expressive, covering tasks, services, and event-driven patterns without new syntax. The `connection:*` primitives (Layer 2) provide a powerful, low-level way to build any protocol *within* the GXO model. |
| **Maintainability & Testing Gaps** | Inability to unit-test complex Ansible playbooks or CI/CD YAML configurations easily. | **Native Testing Framework.** The planned `gxo test` command and `test:*` module suite are a first-class feature, enabling hermetic testing of playbooks with mocked dependencies and assertions. |
| **Engine Performance & State-Store Limits** | Argo Workflows hitting `etcd` size limits with large loops; heavyweight Java-based engines. | **Lightweight Kernel & Native Streaming.** GXO is a single Go binary. Its native data streaming plane passes data over efficient in-memory channels, not through the central state store, avoiding state-size bottlenecks. |
| **Opaque Error Handling & Debugging** | Generic `retry` or `ignore` flags; debugging requires parsing verbose engine logs. | **Structured Errors & Deep Observability.** GXO has a typed error system and is designed for OpenTelemetry tracing from day one, providing deep insight into the execution graph and failure modes. |

By learning from the successes and failures of its predecessors, GXO delivers the expressiveness of imperative workflows without sacrificing the readability, composability, or performance of a truly declarative automation platform.
