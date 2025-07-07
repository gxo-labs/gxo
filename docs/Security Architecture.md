## **Appendix F: GXO Security Architecture & Threat Model**

**Document ID:** GXO-SEC-ARC
**Version:** GXO Security Model v1.0 Alpha
**Date:** 2025-07-02
**Status** Approved Future State
**Audience:** Security Architects, Penetration Testers, Security Operations Engineers

### **F.1. Core Security Philosophy: Defense in Depth & Zero Trust**

The GXO platform is designed with a "security-first" principle. We acknowledge that a powerful automation and service runtime presents a high-value target for attackers. Therefore, our security model is not an add-on but a foundational aspect of the architecture, built on three core philosophies:

1.  **Defense in Depth:** We assume that any single security control can fail. Security is achieved through multiple, layered, and independent controls. A compromise at one layer should be contained and detected by another.
2.  **Zero Trust Principles:** We strive to never trust implicitly. Communication between components must be authenticated and authorized based on strong, verifiable cryptographic identity. Workloads are assumed to be potentially hostile and must be isolated with the least privilege necessary to function.
3.  **Supply Chain Integrity:** We recognize that the security of the platform depends on the integrity of its components, including the GXO binary itself and all pluggable modules. Security must be considered throughout the entire build, distribution, and deployment lifecycle.

This document details the threat model for GXO and the specific controls implemented at each layer to ensure Confidentiality, Integrity, Availability, and Non-repudiation. For a cross-referenced mapping of threats to controls and residual risks, see the corresponding GXO Threat Matrix document.

### **F.2. Threat Model and Attack Surface Analysis**

We analyze the GXO ecosystem as a system with distinct components and interfaces, each representing a potential attack surface.

| Attack Surface | Primary Threat | Attacker Profile | CIA Impact |
| :--- | :--- | :--- | :--- |
| **1. Control Plane** | Unauthorized command execution, control plane takeover. | External Attacker with stolen credentials; Malicious Insider. | Confidentiality, Integrity, Availability |
| **2. `gxo daemon` Runtime**| Privilege escalation on the host, resource starvation (DoS). | Privileged Local User; Attacker who has gained code execution on a workload. | Confidentiality, Integrity, Availability |
| **3. Workload Execution**| Sandbox escape, cross-workload attacks. | Attacker who has compromised a single workload (e.g., via RCE in a web app). | Confidentiality, Integrity |
| **4. State & Data** | Secret exfiltration, state tampering, log tampering. | Malicious Playbook Author; Attacker who has compromised a workload or the host. | Confidentiality, Integrity |
| **5. Module Lifecycle** | Malicious module inserted into registry, compromised build pipeline. | Malicious Insider; Upstream Supply Chain Attacker. | Confidentiality, Integrity |

---

### **F.3. Control Implementation: A Layered Defense Strategy**

The following sections detail the specific security controls designed to mitigate the threats identified above.

#### **Layer 1: Securing the Control Plane (The "Front Door")**

The interface between `gxo ctl` and `gxo daemon` is the primary administrative attack surface.

*   **Threat: Unauthorized Access & Impersonation.**
    *   **Control: Mandatory Mutual TLS (mTLS).** All gRPC communication **must** use mTLS. The `gxo daemon` will be configured with a Server Certificate and a chosen trust model for clients.
    *   **Control: Pluggable Trust Models.** To accommodate diverse environments, the `gxo daemon` will support multiple modes for validating client certificates:
        1.  **`pki` Mode (Production Default):** The daemon is configured with the public certificate(s) of one or more trusted Certificate Authorities (CAs). It will only accept client certificates signed by one of these CAs.
        2.  **`allowlist` Mode (For Self-Signed & Ad-Hoc Environments):** The daemon is configured with a list of authorized certificate fingerprints. These fingerprints are computed over the DER-encoded SubjectPublicKeyInfo (SPKI), ensuring they remain valid across certificate re-issuance for the same key pair, and must be stored in a canonical format (e.g., base64-encoded SHA-256 digest).
    *   **Control: Certificate Subject-Based RBAC.** Authentication is not enough. The `gxo daemon` gRPC server **must** implement a Role-Based Access Control (RBAC) interceptor. The RBAC policy will match on the Subject Common Name (CN) and optionally on other certificate attributes (e.g., Organization Unit, SANs), enabling fine-grained, identity-based authorization.
    *   **Control: Credential Lifecycle Management.** GXO will advocate for short-lived client certificates. Operators are responsible for certificate rotation and revocation. The `gxo daemon` mTLS listener must be configured to check Certificate Revocation Lists (CRLs) or use OCSP stapling to ensure revoked certificates cannot be used in a PKI environment.
*   **Threat: Eavesdropping and Man-in-the-Middle (MITM) Attacks.**
    *   **Control: TLS 1.3 Encryption.** The mTLS implementation will enforce modern TLS versions and strong cipher suites.
    *   **Control: Server Identity Verification.** The `gxo ctl` client **must** always validate the `gxo daemon`'s server certificate. Self-signed certificates **must** be explicitly pinned via the `--server-ca-cert` flag and will not be accepted without prior trust configuration. No insecure fallback to automatic acceptance is permitted.
*   **Threat: Control Plane Denial of Service.**
    *   **Control: gRPC Resource Management.** The `gxo daemon` gRPC server will implement strict limits on message size, connection rate, and concurrent streams. These limits can be applied globally and, in future versions, optionally per authenticated client identity.

#### **Layer 2: Hardening the `gxo daemon` Runtime**

The daemon process itself must be hardened against local attacks on the host.

*   **Threat: Privilege Escalation from the Daemon.**
    *   **Control: Least-Privileged User Execution.** The GXO installation process and documentation **must** mandate that the `gxo daemon` runs as a dedicated, unprivileged user.
    *   **Control: Minimal File Permissions.** The daemon's configuration files, its persistent state file, and its binary must have strict file permissions (e.g., `0600`), owned by the daemon's user.
*   **Threat: Host Resource Starvation by the Daemon.**
    *   **Control: Self-Imposed Cgroup Limits.** The recommended `systemd` unit file for running `gxo daemon` will include directives to place the daemon process itself into a dedicated system slice with defined CPU and memory limits.
*   **Threat: Subversion via Malicious Module or Playbook.**
    *   **Control: Module Integrity Verification.** Until a formal module signing mechanism is implemented, operators are responsible for validating module sources through external controls. A future version **must** implement a module signing policy where the `gxo daemon` can be configured to only execute workloads that use modules with a valid cryptographic signature (e.g., via `cosign`). If module signature verification is enabled and fails, the default policy **must** be to reject execution (**fail-closed**). Operators may explicitly configure `allow_unsigned_modules=true` for trusted, air-gapped environments. The verification process will support an allowlist of trusted public keys to facilitate key rotation without downtime.

#### **Layer 3: Workload Isolation & Sandboxing (Containing the Blast Radius)**

This is the most critical layer for preventing a single compromised application from taking over the system.

*   **Threat: Filesystem-based Breakout.**
    *   **Control: Mount Namespace and `pivot_root`.** When a `security_context.root_filesystem` is defined, GXO **must** create a new mount namespace for the workload process (`CLONE_NEWNS`) and use `pivot_root` to jail the process. The `root_filesystem` may optionally be mounted read-only to further restrict workload capabilities.
*   **Threat: Resource Exhaustion by a Workload.**
    *   **Control: Cgroup Enforcement.** When `security_context.resources` are defined, GXO **must** programmatically create a dedicated cgroup slice for the workload and write the specified resource limits to the control files.
*   **Threat: Kernel Exploit via Malicious System Calls.**
    *   **Control: Seccomp-BPF Filtering.** GXO **must** ship with a set of default, restrictive `seccomp` profiles. A `default-minimal` profile, which permits a minimal set of syscalls necessary for standard file and network I/O but denies privilege escalation primitives, will be applied to any workload that does not specify a `security_context`. Custom profiles, specified as a JSON-formatted seccomp filter policy compatible with `libseccomp`, may be provided via the `security_context.seccomp_profile_path`.
*   **Threat: Inter-Process Interference.**
    *   **Control: PID and IPC Namespaces.** GXO **must** also place supervised workloads in new PID namespaces (`CLONE_NEWPID`) and IPC namespaces (`CLONE_NEWIPC`).
*   **Threat: Unintended Privilege.**
    *   **Control: Unambiguous Privilege Model.** GXO **will not** support workloads requiring root privileges running directly under the daemon. Any operation requiring root must be delegated to a well-defined module that uses `sudo` with a specific, narrow command configuration, or be executed within a container.
*   **Interaction with Container Runtimes:** When using the `container` module, GXO **must** defer all namespace, seccomp, and cgroup enforcement to the container runtime itself and will not apply additional OS-level sandboxing.

#### **Layer 4: Data Security (Confidentiality & Integrity)**

This layer focuses on protecting the data that GXO handles, both at rest and in motion.

*   **Threat: Secret Exposure in State or Logs.**
    *   **Control: The "Taint and Redact" System.** Secrets resolved via the `secret` function are "tainted" on a per-process basis. Any attempt to write a tainted value to the state store or log it as part of an error message will result in the value being replaced with `[REDACTED_SECRET]`.
    *   **Control: Proactive Memory Zeroing.** Upon process termination, all memory buffers within the `SecretTracker` containing tainted secrets **must** be proactively zeroed. While GXO will implement this best-effort zeroing, it must be acknowledged that due to Go runtime behavior (e.g., garbage collector copies of immutable strings), complete memory zeroing cannot be guaranteed. For highly sensitive environments, operators are advised to restrict memory dump access and enforce host-level protections.
    *   **Control: Debug Logging Caveats.** In debug logging modes, operators are advised that secrets may be inadvertently logged as part of raw parameter inputs. Production deployments **must** avoid debug-level logging on sensitive workloads.
*   **Threat: Unauthorized Secret Access by Workloads.**
    *   **Control: Scoped Secret Provider Integration & Failure Policy.** The integration with secrets backends like HashiCorp Vault **must** be role-based. A playbook will specify a `vault_role`. The `gxo daemon` will use this role to request a temporary, scoped token from Vault. The system will support a configurable failure policy (`fail-closed` vs. `fail_open`) for handling unavailable secret backends, with `fail-closed` as the default.
*   **Threat: State Tampering and Exposure at Rest.**
    *   **Control: State Encryption at Rest.** The persistent state file used by `gxo daemon` will support optional encryption at rest using an AEAD cipher (e.g., AES-GCM). The encryption key must be provided by the operator through a secure mechanism. Key rotation will require a managed, offline re-encryption of the state file, which will require a brief downtime window for the `gxo daemon`.

#### **Layer 5: Non-Repudiation and Auditing**

This layer ensures that actions can be definitively and reliably traced back to their origin.

*   **Control: Cryptographic Identity in Auditing.** Every gRPC request to the `gxo daemon` that modifies state **must** generate a structured audit log entry. This entry will include the **full Subject Distinguished Name (DN) and Serial Number** of the client certificate that initiated the action and the source IP address.
*   **Control: Trace Propagation.** The OpenTelemetry `trace_id` initiated by an action will be propagated through every log entry and every subsequent GXO task.
*   **Control: Time Synchronization.** To ensure the integrity of timestamps in audit logs and traces, operational guidance **must** require that all `gxo daemon` hosts are synchronized to a reliable time authority. Operators are strongly encouraged to deploy authenticated NTP (e.g., NTS - Network Time Security) to prevent tampering with clock drift. Future versions may implement an optional configuration to enforce monotonic time verification before `gxo daemon` startup.
*   **Control: Log Integrity.** GXO itself does not guarantee log integrity after an entry is written. For high-security environments, operators are responsible for configuring the `gxo daemon`'s host to forward structured logs immediately to a remote, write-once, or append-only log aggregation system. Future versions may incorporate an append-only local log with hash chaining (e.g., leveraging Merkle tree-based techniques similar to Sigstore Rekor) to provide cryptographic tamper evidence prior to external shipping.

### **F.6. Conclusion: A Defensible, Production-Ready Security Posture**

No system is impenetrable. The GXO security model is designed to be defensible, auditable, and to make compromise difficult, costly for an attacker, and loud from a detection standpoint. It achieves this not by inventing new security technologies, but by providing a robust, declarative interface to orchestrate powerful, industry-standard security primitives.

The security of a GXO deployment is a shared responsibility, but GXO provides the tools, the secure-by-default configurations, and the explicit declarative controls necessary for an operator to build and maintain a hardened, least-privilege automation environment. By addressing these concerns at an architectural level, GXO aims to be a platform that security professionals can trust and advocate for.