# Workbench Snapshot Read And Lifecycle Policy

| Field | Value |
| --- | --- |
| Document ID | SWB-SNAPSHOT-LIFECYCLE-001 |
| Revision | 1.0 |
| Status | Accepted design; implementation evidence pending |
| Owner | Software |
| Governing decision | [ADR-0013](../adr/adr-0013.md) |

## Purpose

This record defines the Workbench Snapshot read interface, separates it from internal projection queries, and records lifecycle reconciliation as an independently observable operational concern. It applies ADR-0007's coherent Live Read Boundary to public routes, storage adapters, Browser behavior, retention, readiness, cancellation, and delivery evidence.

## Read Module

The Browser crosses one public seam:

```text
Browser → GET /api/simulator-workbench/snapshot → Workbench Snapshot
```

The response contains one generation-bound State, Measured State, Simulated Result State, Twin State, and Lineage set. A Browser accepts or rejects the whole response; it never combines fields from separate responses or uses a field route as fallback.

The `liveWorkbench` adapter owns transport, parsing, envelope validation, and typed read errors. The Snapshot session owns refresh, cancellation, accepted-generation monotonicity, stale-live retention, fixture fallback, selection reconciliation, and presentation projection. Shared Workbench type declarations remain available to rendering and fixtures, but the stale field-read functions and their test are removed.

### Snapshot contract

The canonical envelope schema and shared vectors are authoritative for cross-runtime shape. The schema composes the existing State, SCADA, Twin, Lineage, and simulated-result contracts. Semantic checks enforce what schema alone cannot express:

- `snapshot.generation` equals `snapshot.state.snapshotGeneration`;
- every constituent is present before acceptance;
- measured, simulated, and imputed values keep their declared Value Basis;
- invalid, partial, and generation-mismatched vectors are rejected as whole responses.

Go handler/adapter tests prove that real serialized Snapshots obey these rules. Browser tests prove that the same valid vector is accepted and invalid vectors receive the expected typed error. The contract is a test surface, not a generated client or another runtime module.

## Storage Interfaces

```text
Workbench
  ├─ Snapshot reader → InMemory adapter | Postgres adapter
  └─ Twin Projector hydration → InMemory adapter | Postgres adapter
```

The public Snapshot reader has one operation: `Snapshot() → WorkbenchSnapshot`. InMemory captures all returned fields under one read lock and returns owned values. Postgres captures generation and all returned projections in one read-only repeatable-read transaction. Both adapters return one immutable logical generation.

Twin Projector hydration is a distinct internal interface. It reads bounded latest measured frames and results to reconstruct the projector's private working state after restart. Its ordering, limits, and independent consistency are intentional and must not leak through the public Snapshot interface. Current Twin State and selected Lineage lookup have no independent production caller and are retired with the public field-read routes.

## Read-Only Retention Policy

`Snapshot()` is a query: it performs no retention mutation, does not advance generation, and remains available when cleanup fails. Dynamic measured retention is a command:

```text
ReconcileDynamicMeasuredRetention(cutoff)
  → prune eligible dynamic reactor-scoped frames
  → preserve resident source declarations
  → advance generation once only when rows changed
```

InMemory performs the command under its write lock. Postgres performs it in its own write transaction. An explicit retention pass may change a later Snapshot; reading a Snapshot may not invoke the pass.

## Lifecycle Reconciliation And Health

Each lifecycle cycle produces independent outcomes for Reactor expiry, Artifact Forge expiry, and measured retention. A normal task error records that task's failure and does not prevent independent tasks from running. A context deadline or cancellation stops later task starts; application shutdown cancellation is not an operational failure.

The lifecycle health state is `disabled`, `starting`, `ready`, `degraded`, or `not_ready`.

| State | Meaning | `/healthz` | `/readyz` |
| --- | --- | --- | --- |
| `disabled` | No lifecycle task is configured. | 200 | Does not wait for reconciliation. |
| `starting` | A configured scheduler has not completed its initial full cycle. | 200 | 503 |
| `ready` | Required tasks succeeded within their deadlines. | 200 | 200 |
| `degraded` | A task failed but its last success remains within policy. | 200 | Policy-defined ready response and alert. |
| `not_ready` | A required task has never succeeded or is overdue. | 200 | 503 |

The scheduler runs one full initial cycle, then serialized periodic cycles. It passes its context through Reactor cleanup, Artifact Forge expiry, and measured retention. `/healthz` remains process liveness. `/readyz` is a pure rendering of static configuration and lifecycle health. `/metrics` remains scrapeable while readiness is degraded and exposes fixed-cardinality, task-labelled success, failure, age, and affected-count metrics. Error text, session identities, and reactor identities are not metric labels.

## Delivery Evidence

Operational evidence distinguishes intermediate materialization from the public read result:

1. Redpanda, Postgres, and Iceberg checks prove the dataflow reached each materialization target.
2. One Snapshot check proves those visible Workbench constituents belong to one generation.
3. Scheduler expiry evidence seeds expired dynamic data, starts lifecycle reconciliation without a Browser read, records the named retention outcome, and verifies the subsequent Snapshot observes the post-expiry generation.

The Docker Reactor Telemetry proof polls only Snapshot and verifies its measured constituent contains the expected reactor-scoped, measured frames and matching generation. It does not require unrelated simulated, Twin, or Lineage constituents to be non-empty.

Smoke scripts that need a settled gateway use `/readyz`. Container liveness checks use `/healthz`; Prometheus continues to scrape `/metrics` regardless of readiness.

## Required Verification

- Snapshot read tests prove coherence and read-only behavior separately.
- InMemory and Postgres adapters prove matching logical Snapshot semantics, generation, ordering, and immutable returned values.
- Contract tests cover valid, partial, generation-mismatched, and Value Basis-invalid Snapshot vectors across Go and Browser tests.
- Lifecycle tests prove initial state, initial reconciliation, serialized ticks, independent task recovery, cancellation propagation, liveness/readiness policy, and metrics visibility.
- Delivery tests prove scheduler-driven expiry without Snapshot-triggered cleanup and use Snapshot as the final public observation.
