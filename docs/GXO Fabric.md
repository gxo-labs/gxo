# Appendix A – GXO Fabric (Federated Automation Kernel)

**Document ID:** GXO-FAB-FUTURE
**Date:** 2025-07-05
**Status:** Forward‑looking design proposal — targets **v1.0 Beta** after completion of the single‑node v1.0 Alpha roadmap.*

---

## 1 Purpose

The v1.0 Alpha release positions GXO as a **single‑node automation kernel** that unifies process supervision, event‑driven workflows, and streaming ETL under one signed binary.  This appendix explores a natural evolution — **GXO Fabric** — that federates multiple kernels into a fault‑tolerant, low‑latency mesh without adopting container semantics.  The goal is to extend availability and horizontal capacity *while preserving the micro‑kernel philosophy*.

## 2 Design Principles

|  ID  | Principle                              | Rationale                                                                                                                          |
| ---- | -------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
|  P‑1 | **Kernel‑per‑Host remains sacrosanct** | Local scheduling, seccomp isolation, and process supervision stay inside each daemon.  No global PID namespace.                    |
|  P‑2 | **Federation over orchestration**      | A lightweight control mesh distributes process manifests and replicates state; it never performs global resource negotiation.     |
|  P‑3 | **Data‑plane locality by default**     | Streams process where events arrive unless YAML explicitly requests `shard:` semantics.                                            |
|  P‑4 | **Asynchronous durability**            | State deltas replicate off‑node **after** local commit to minimise tail‑latency; critical listeners can opt‑in to synchronous ACK. |
|  P‑5 | **Secure by default**                  | mTLS mesh, signed process bundles, and per‑node RBAC with no downgrade path.                                                      |

## 3 High‑Level Architecture

```
          +-------------------+                
          |  gxocli / CI/CD   |
          +---------+---------+
                    | gRPC (mTLS, JWT)
     -------------- + ------------------------------
     |              |                              |
+----v----+   +-----v-----+   +-----v-----+   +-----v-----+
| Node A  |   | Node B    |   | Node C    |   | Node D    |
| gxo     |   | gxo       |   | gxo       |   | gxo       |
| daemon  |   | daemon    |   | daemon    |   | daemon    |
|======== |   |========   |   |========   |   |========   |
|  L0–6   |   |  L0–6     |   |  L0–6     |   |  L0–6     |
+----+----+   +-----+-----+   +-----+-----+   +-----+-----+
     |   Raft / gossip (state deltas, health, labels)
     +----------------------------------------------------+
                    |  (optional)
               +----v-----+
               | Object   |
               |  Storage |  (signed module blobs)
               +----------+
```

### 3.1 Control Mesh

* **Transport:** gRPC over mTLS (same certificates as daemon API).
* **Membership:** Hashicorp Serf‑style gossip or embedded k‑Raft heartbeat.
* **Metadata:** Node labels (zone, device‑class), process manifests, health metrics.

### 3.2 State Replication

* **Local store:** BoltDB continues as write‑ahead log (WAL).
* **Delta shipper:** Background goroutine batches page‑deltas → LZ4 → gRPC stream.
* **Quorum backend:** Etcd or Redis‑RAFT (pluggable).
* **Replay path:** On fail‑over, peer pulls latest snapshot then replays forward deltas.

### 3.3 Process Placement

| Directive           | Behaviour                                                             |
| ------------------- | --------------------------------------------------------------------- |
| *none*              | Manifest applied to **all** nodes matching label selectors.           |
| `placement: single` | Exactly one node runs the process (active/standby via leader token). |
| `shard: N`          | Hash‑partition stream inputs across *N* nodes that match selector.    |

## 4 Phased Roadmap (post‑Alpha)

| Phase | Milestone                            | Key Artifacts                                                                    |
| ----- | ------------------------------------ | -------------------------------------------------------------------------------- |
|  F‑1  | **Remote Runner API**                | `gxo ctl apply --target host=<node>`; process executed on remote node via gRPC. |
|  F‑2  | **Mesh Discovery & Health**          | Gossip heartbeat, node label registry, Prometheus scrape endpoints aggregated.   |
|  F‑3  | **State Delta Replication**          | Asynchronous WAL shipper; manual replay command for operator testing.            |
|  F‑4  | **Shard & Active/Standby Placement** | YAML schema update, scheduler extension, simple hash partitioner.                |
|  F‑5  | **Automatic Fail‑over**              | Node health watcher promotes standby or replays shard on peer.                   |
|  F‑6  | **Optional Quorum Backend Swap**     | Support etcd, Redis‑RAFT, or file‑based Litestream replicas.                     |

## 5 Performance Expectations (LAN, 3‑node quorum)

| Metric                    | Target          | Rationale                                                                               |
| ------------------------- | --------------- | --------------------------------------------------------------------------------------- |
| p95 task dispatch latency | ≤ 1 ms          | Local copy (2 ms) avoided by sharing snapshot; Raft commit adds ≤ 0.5 ms when batching. |
| Event throughput per node | ≥ 8 k tasks/sec | Assumes 64‑record batches, hybrid deep copy, 8‑core host.                               |
| Fail‑over RTO             | < 5 s           | Snapshot size ≤ 64 MB; delta replay parallelised.                                       |

## 6 Security Considerations

* **mTLS mesh** re‑uses daemon PKI; no plaintext node‑node traffic.
* **Signed manifests** (Cosign) verified before apply; signature includes placement rules to prevent downgrade.
* **Namespace isolation** unchanged — modules remain confined to host namespaces/cgroups.

## 7 Out‑of‑Scope for v1.0 Beta

* Global resource scheduling / bin‑packing.
* Cross‑node PID or cgroup management.
* OCI container lifecycle (remain a k8s‑adjacent tool).
* WAN latency optimisation (edge federation may follow in v2).

## 8 Open Questions

1. **Snapshot format** — stick with BoltDB pages or adopt Chunked CBOR?
2. **Quorum backend default** — etcd brings operational weight; Redis‑RAFT simpler but less battle‑tested.
3. **CLI UX** — hide clustering behind `--fabric` flag or auto‑detect peers?
4. **Security audit cadence** — additional threat vectors arise with mesh; schedule external pen‑test before GA.

## 9 Summary

GXO Fabric extends the Automation Kernel from a single binary into a **secure, low‑latency federation** that adds high availability and horizontal capacity without embracing container semantics.  Each node keeps the micro‑kernel virtues—local control, immutable state, least‑privilege modules—while the mesh replicates checkpoints and distributes manifests.  The result is a new category: *stream‑native, host‑level automation fabric* that complements Kubernetes rather than competes with it.
