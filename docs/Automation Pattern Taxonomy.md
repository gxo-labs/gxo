# **A Taxonomy of Automation Patterns for the Declarative Kernel**

**Document ID:** GXO-PATTERNS-V1
**Version:** 1.0
**Status:** Canonical Reference

## **Abstract**

This paper defines a universal taxonomy for automation patterns: recurring structures that define the execution lifecycle, state management, and control flow of automated processes. These patterns underlie and unify disparate domains such as CI/CD, SOAR, ETL, and configuration management. By mapping these patterns to the core primitives of the GXO Automation Kernel, we demonstrate that what were once domain-specific solutions can be reinterpreted as variants of a common, secure, and extensible execution substrate. This formalism not only provides clarity for developers and operators but also paves the way for a unified automation control plane that reduces toolchain fragmentation while enforcing consistent security and governance.

---

## **1. Introduction: From Domain-Specific Tools to Universal Patterns**

The history of automation has evolved from ad hoc scripting to a powerful but fragmented landscape of specialized orchestration systems. This separation has led to siloed toolsets with unique DSLs, execution models, and security policies. We propose that at their core, these tools embody recurring automation patterns: fundamental building blocks that express the essential semantics of automated tasks. Our thesis is simple: rather than viewing automation as domain-bound, we should recognize that it is pattern-based.

The GXO Automation Kernel serves as a reference implementation of a runtime that natively expresses these common patterns. By formalizing this taxonomy, we establish a framework that allows any automation task, regardless of its domain, to be mapped into a well-defined execution paradigm. This enables unified training, operational governance, and runtime security across an organization’s entire automation stack.

## **2. The Core Automation Patterns**

An automation pattern is a recurring architectural structure that defines an automated process's execution lifecycle, state management, control flow, and external interactions. For a system to qualify as an Automation Kernel, it must natively support these patterns without delegation to external processes or scripting envelopes.

The core patterns are:
1.  DAG-Based Task Execution
2.  Idempotent Reconciliation
3.  Reactive Event-Driven Workflows
4.  Scheduled & Periodic Automation
5.  Stateful Stream Processing
6.  Interactive Pause & Resume
7.  Compensating Transactions (The Saga Pattern)
8.  Resilient Execution (The Circuit Breaker Pattern)

## **3. Realizing Patterns in the GXO Automation Kernel**

The GXO Kernel is designed to be a minimal execution substrate, akin to an operating system kernel, but for automation. It provides a robust set of primitives that are composed to realize each automation pattern. The key is the `Workload`, GXO's atomic unit of automation, which fuses a `Process` (the *what*) with a `Lifecycle` (the *how* and *when*). By declaratively combining these primitives, any automation pattern can be expressed.

While the following sections describe each pattern individually for clarity, their true power emerges from their composability. A single GXO playbook can combine these patterns to create sophisticated, multi-paradigm automation. For example, a scheduled workflow might trigger a complex DAG, which in turn includes a pause/resume step for human approval and a compensating transaction to ensure a clean rollback on failure.

### **3.1. Pattern 1: DAG-Based Task Execution**

*   **Definition:** A discrete, non-recurring workflow that executes a Directed Acyclic Graph (DAG) of modules and terminates upon completion. This pattern is the foundation for all imperative, step-by-step logic, including complex parallel execution with synchronization points.
*   **Use Cases:** CI/CD pipelines (build, test, deploy), multi-step infrastructure provisioning, batch data processing jobs.
*   **GXO Realization:** This pattern is realized using `Workloads` with a `run_once` lifecycle. The Kernel's `DAGBuilder` automatically constructs the graph, enabling several control flows:
    *   **Sequential Flow:** A workload that depends on the registered result of another (e.g., `when: '{{ .step_A.status == "Completed" }}'`) creates a sequential dependency.
    *   **Fan-Out (Parallel Execution):** Multiple workloads with no dependencies on each other will be scheduled by the Kernel to run concurrently, up to the worker pool limit.
    *   **Fan-In (Synchronization):** The `control:barrier` module is used as a synchronization point. It takes multiple `stream_inputs` from parallel workloads and only completes after all of them have finished, allowing a subsequent stage to begin.

*   **Example: A CI/CD Pipeline with Parallel Stages**
    This playbook defines a pipeline with parallel build and test stages. The `build_backend` and `build_frontend` workloads run concurrently. The `integration_test_barrier` waits for both to complete successfully before the `deploy_to_staging` workload is allowed to run.

    ```yaml
    workloads:
      # Fan-Out: These two workloads have no dependencies and will run in parallel.
      - name: build_backend
        lifecycle: { policy: run_once }
        process:
          module: exec
          params: { command: "make build-backend" }

      - name: build_frontend
        lifecycle: { policy: run_once }
        process:
          module: exec
          params: { command: "make build-frontend" }

      # Fan-In: This barrier waits for both parallel build steps to complete.
      # It takes their names as stream_inputs, creating the synchronization point.
      - name: integration_test_barrier
        lifecycle: { policy: run_once }
        stream_inputs: [build_backend, build_frontend]
        process:
          module: control:barrier
    
      # Final Stage: This workload only runs after the barrier is complete.
      - name: deploy_to_staging
        lifecycle: { policy: run_once }
        when: '{{ ._gxo.workloads.integration_test_barrier.status == "Completed" }}'
        process:
          module: exec
          params: { command: "./scripts/deploy_staging.sh" }
    ```

### **3.2. Pattern 2: Idempotent Reconciliation**

*   **Definition:** A pattern where the system continually compares a system's current state against a declared desired state and applies the minimum necessary changes to converge the two.
*   **Use Cases:** Infrastructure as Code (IaC), continuous configuration management, self-healing systems.
*   **GXO Realization:** This pattern is achieved by composing idempotent modules (like `filesystem:manage` or `terraform:run`) with a `supervise` lifecycle. The `gxo daemon` continuously runs the `Workload`, and the idempotent module ensures that it only performs an action when a state drift is detected.

*   **Example: Managing a Configuration File**
    This `Workload` ensures a configuration file always exists with the correct content and permissions. The Kernel will run this process forever, and the `filesystem:manage` module will only report a change if the file is modified or deleted externally.

    ```yaml
    workloads:
      - name: enforce_ntp_config
        lifecycle:
          policy: supervise
          restart: on_failure
        process:
          module: filesystem:manage
          params:
            path: "/etc/ntp.conf"
            state: "present"
            content: |
              server time.google.com iburst
              server time.cloudflare.com iburst
            mode: "0644"
            owner: "root"
    ```

### **3.3. Pattern 3: Reactive Event-Driven Workflows**

*   **Definition:** Workflows that are triggered by asynchronous external or internal events (e.g., webhooks, monitoring alerts, messages from a queue) and respond dynamically by executing a pre-defined set of actions.
*   **Use Cases:** Security Orchestration, Automation, and Response (SOAR), real-time monitoring and alerting, serverless functions.
*   **GXO Realization:** This pattern uses two collaborating `Workloads`. A producer `Workload` (often with a `supervise` lifecycle) acts as the event source (e.g., using `connection:listen`). A consumer `Workload` with an `event_driven` lifecycle subscribes to that source. For each event emitted, the Kernel instantiates an ephemeral, isolated instance of the consumer's workflow.

*   **Example: A SOAR Webhook Handler**
    This playbook defines a webhook listener. For every incoming HTTP POST request, it runs a pipeline to parse the alert, enrich the data by looking up the IP address, and posts a message to Slack.

    ```yaml
    workloads:
      - name: soar_webhook_listener
        lifecycle: { policy: supervise, restart: always }
        process: { module: http:listen, params: { address: ":9090" } }

      - name: incident_response_pipeline
        lifecycle:
          policy: event_driven
          source: soar_webhook_listener # Subscribes to the listener's events
        process:
          module: stream:pipeline
          params:
            steps:
              - id: parse_alert
                uses: data:map
                with:
                  template: '{{ .body | fromJson }}' # Parse the incoming JSON body
              - id: enrich_ip
                uses: dns:query
                with:
                  name: '{{ .source_ip }}'
                register: dns_info
              - id: notify_slack
                uses: slack:post_message
                with:
                  message: "ALERT from {{ .source_ip }} (Host: {{ .dns_info.answers | first }}). Details: {{ .details }}"
    ```

### **3.4. Pattern 4: Scheduled & Periodic Automation**

*   **Definition:** Workloads that run on a recurring, time-based schedule, ensuring periodic execution of tasks like health checks, reports, or backups.
*   **Use Cases:** Daily build pipelines, routine health checks and inventory audits, data aggregation at fixed intervals.
*   **GXO Realization:** This pattern is implemented with a `scheduled` lifecycle, which uses a standard `cron` expression to define the execution interval. The `gxo daemon`'s scheduler computes the next activation time and launches a new workflow instance when the timer expires.

*   **Example: Daily Database Backup**
    This `Workload` will execute the backup script every day at 2:30 AM server time.

    ```yaml
    workloads:
      - name: nightly_database_backup
        lifecycle:
          policy: scheduled
          cron: "30 2 * * *" # Run every day at 2:30 AM
        process:
          module: exec
          params:
            command: "/opt/scripts/backup_database.sh"
            environment:
              - "PG_PASSWORD={{ secret 'db_backup_password' }}"
    ```

### **3.5. Pattern 5: Stateful Stream Processing**

*   **Definition:** Workloads that continuously process streams of data, performing stateful operations like joining, filtering, aggregating, or transforming inputs in real time.
*   **Use Cases:** ETL pipelines, log aggregation and analysis, real-time data enrichment.
*   **GXO Realization:** This pattern is a specialization of the DAG-based execution pattern, using `run_once` workloads that are connected via `stream_inputs`. It leverages GXO's native streaming data plane and stateful modules like `data:join` and `data:aggregate`.

*   **Example: Joining User Data with Login Events**
    This pipeline joins a stream of user records with a stream of login events to produce an enriched log entry.

    ```yaml
    workloads:
      - name: user_stream
        process: { module: database:query, params: { query: "SELECT user_id, name, team FROM users" } }
      
      - name: login_event_stream
        process: { module: some_log_ingestor } # Module that produces login events

      - name: enriched_log_producer
        lifecycle: { policy: run_once }
        stream_inputs: [user_stream, login_event_stream]
        process:
          module: data:join
          params:
            join_type: "left"
            on:
              - { stream: "user_stream", role: "build", field: "user_id" }
              - { stream: "login_event_stream", role: "probe", field: "user_id" }
            output:
              merge_strategy: "flat"
    ```

### **3.6. Pattern 6: Interactive Pause & Resume**

*   **Definition:** Workflows that deliberately pause execution at a defined checkpoint, awaiting external verification or data injection from a human operator or another system before resuming.
*   **Use Cases:** Change approvals in CI/CD pipelines, interactive incident investigations, manual verification for critical infrastructure changes.
*   **GXO Realization:** This pattern is enabled by the `Resume Context` Kernel primitive. A `Workload` uses the `control:wait_for_signal` module, which instructs the Kernel to pause execution, persist the workflow's state, and return a unique resume token. The workflow remains suspended until a `gxo ctl resume` command is issued with the correct token and an optional JSON payload.

*   **Example: Staged Deployment with Approval Checkpoint**
    This pipeline runs pre-deployment checks, then pauses and waits for an external approval before proceeding with the production deployment.

    ```yaml
    workloads:
      - name: run_staging_tests
        lifecycle: { policy: run_once }
        process: { module: exec, params: { command: "make test-staging" } }

      - name: wait_for_prod_approval
        lifecycle: { policy: run_once }
        when: '{{ eq ._gxo.workloads.run_staging_tests.status "Completed" }}'
        process:
          module: control:wait_for_signal
        register: approval_handle

      - name: deploy_to_production
        lifecycle: { policy: run_once }
        when: '{{ .approval_handle.resume_payload.approved == true }}'
        process:
          module: exec
          params: { command: "./scripts/deploy_prod.sh" }
    ```

### **3.7. Pattern 7: Compensating Transactions (The Saga Pattern)**

*   **Definition:** A pattern for managing long-running, distributed transactions that involve multiple independent services. To maintain data consistency without using locks, a failure in any step of the forward transaction triggers a corresponding series of "compensating" (or "undo") actions that revert the changes made by the successful preceding steps.
*   **Use Cases:** User signup processes that touch multiple microservices (authentication, billing, email); complex e-commerce orders that must reserve inventory and process payments.
*   **GXO Realization:** This advanced workflow pattern can be realized by extending a `Workload` with a declarative `on_failure` block. When a `Workload` fails and is not configured with `ignore_errors: true`, the Kernel's scheduler, instead of halting, would schedule the compensating `Workloads` defined in this block, passing them the state from the failed attempt for context.

*   **Example: Multi-Service User Creation with Rollback**
    If the `setup_billing` workload fails, the Kernel will execute the `rollback_user_creation` workload to ensure no orphaned user record is left in the database.

    ```yaml
    workloads:
      - name: create_user_in_db
        lifecycle: { policy: run_once }
        process:
          module: database:query
          params:
            query: "INSERT INTO users (username) VALUES ('new_user') RETURNING id;"
        register: new_user
        on_failure: # This would be a no-op, but demonstrates the structure.
          - process: { module: control:identity }

      - name: setup_billing
        lifecycle: { policy: run_once }
        when: '{{ ._gxo.workloads.create_user_in_db.status == "Completed" }}'
        process:
          module: http:request
          params:
            url: "https://billing.service/api/v1/customers"
            method: "POST"
            body: '{ "user_id": {{ .new_user.rows.0.id }} }'
        on_failure: # If billing fails, roll back the user creation.
          - name: rollback_user_creation
            process:
              module: database:query
              params:
                query: "DELETE FROM users WHERE id = {{ .new_user.rows.0.id }};"
    ```

### **3.8. Pattern 8: Resilient Execution (The Circuit Breaker Pattern)**

*   **Definition:** A stateful resilience pattern that prevents an application from repeatedly attempting to execute an operation that is likely to fail. After a configured number of failures, the "circuit opens," and subsequent calls fail immediately without executing. After a timeout, the circuit enters a "half-open" state, allowing a limited number of test requests. If they succeed, the circuit closes; otherwise, it trips open again.
*   **Use Cases:** Calling a flaky or temporarily overloaded downstream microservice; preventing cascading failures in a complex system.
*   **GXO Realization:** This pattern is a sophisticated enhancement to the existing `retry` mechanism. It can be implemented by adding a `circuit_breaker` policy to a `Workload`'s `retry` block. The Kernel's `WorkloadRunner` would maintain the state of the circuit (`CLOSED`, `OPEN`, `HALF-OPEN`) in its persistent store, making decisions to execute, fail-fast, or test recovery based on this state.

*   **Example: Resiliently Calling a Flaky API**
    This `Workload` will attempt to call a service. After five consecutive failures, it will stop trying for 60 seconds. After that, it will allow one request through. If that succeeds, normal operation resumes; if it fails, the 60-second wait begins again.

    ```yaml
    workloads:
      - name: call_downstream_api
        lifecycle: { policy: run_once }
        process:
          module: http:request
          params:
            url: "https://api.flaky-service.com/data"
            timeout: "5s"
        retry:
          attempts: 3 # Per-run attempts before marking a single run as failed.
          circuit_breaker:
            failure_threshold: 5      # Open circuit after 5 consecutive failed runs.
            open_duration: "60s"      # Stay open (fail-fast) for 60 seconds.
            success_threshold: 1      # Require 1 success in half-open to close.
    ```

## **4. Conclusion**

Automation is not inherently tied to any specific domain. Rather, it is an expression of common patterns that, when abstracted properly, reveal a unified control plane. The GXO Automation Kernel embodies this unification, providing a reference implementation that maps well-understood OS paradigms to the realm of automation. By embracing a pattern-based approach, organizations can consolidate training, streamline operations, and enforce a single, coherent set of security and governance policies, replacing a patchwork of disparate tools with a cohesive, programmable automation substrate. This taxonomy provides a foundation for reasoning about automation and is itself extensible as new high-level patterns emerge from the composition of these core primitives.
