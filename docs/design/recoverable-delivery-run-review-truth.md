# Recoverable Delivery, Run, And Review Truth

| Field | Value |
| --- | --- |
| Document ID | RDRRT-001 |
| Revision | 1.0 |
| Status | Accepted architecture contract |
| Owner | Software |
| Baseline Reference | v3.0 candidate |
| Issue | #202 |

## Purpose

This contract governs the recovery and presentation rules that sit above
Iceberg lifecycle, Simulation Ops Run execution, and Status Workbench review.
It exists because Radiant now moves one logical fact through multiple stores,
runtimes, transports, and presentation contexts. A retry, a replay, or a
polished browser view must not make a stronger claim than the system actually
proved.

The governing model is **at-least-once with stable identity and reconciliation**.
No module may imply end-to-end exactly-once processing across the control-plane
store, Redpanda offsets, object storage, Iceberg catalogs, runtime adapters, and
browser presentation state.

## Controlling Separations

- **Delivery Assurance is not broker-offset completion.** A manifest write,
  external-command acknowledgement, disabled writer result, and Iceberg fresh
  readback are different proof strengths even when the artifact lifecycle
  reaches `committed`.
- **An Artifact Forge outcome is not Objective Evidence.** Artifact Forge owns
  game outcome eligibility and idempotent outcome application. It does not make
  operational telemetry or simulated results into public-safe evidence.
- **An Experience Scenario is not a Workbench Snapshot or a SimOps Run.**
  Experience Scenarios are design and test material. They may link to declared
  state, but they never become live Run truth or Snapshot truth.
- **Runtime adapters do not own Run truth.** Docker, Kubernetes, and local
  contract execution consume Run Connection Profiles and report observations.
  The gateway remains the authority for execution intent, lifecycle meaning,
  authorization, and role-scoped access.
- **A Workbench Snapshot remains one accepted generation.** Fixture fallback is
  whole-Snapshot, local-demo-only, and cannot be mixed field-by-field with live
  accepted data.

## Required Module Contracts

### Delivery Truth

The Delivery Attempt contract owns one stable delivery identity, one ordered
coordinate set, a pending/unknown/resolved state, Delivery Assurance, and
Verified Delivery Evidence. Batching, pacing, retry, reconciliation, and final
broker acknowledgement belong inside this contract.

An Unknown Commit must be reconciled before another logical append for the same
batch is attempted. Recovery uses the Delivery Attempt identity and expected
coordinates; it must not infer idempotency from an Iceberg snapshot ID, a
process exit code, an in-memory sequence counter, or an artifact status alone.

Iceberg Lifecycle owns catalog and table discovery, namespace/table creation,
schema compatibility handoff, append preparation, conflict recovery, and fresh
readback. Writer modes remain adapters at that seam.

### Publication Truth

A control-plane outbox records the domain fact and intended publication in the
same control-plane transaction, then publishes and reconciles outside that
transaction. This is Transactional Outbox-shaped work, not Event Sourcing.

Artifact Forge owns one idempotent outcome lifecycle: intent recovery,
eligibility, outcome mapping, and an atomic consumption marker. Operational
telemetry, incomplete artifacts, failed Runs, missing Lineage, and measured or
imputed values remain ineligible for game outcomes.

### Run Truth

The gateway alone accepts authenticated execution intent, selects permitted
runtime behavior, creates role-scoped Run Connection Profiles, and records Run
lifecycle state. Browser and Fleet Board callers select supported recipes, not
runtime images, broker endpoints, catalog details, database credentials, or
runtime-management credentials.

Runtime adapters start, stop, and observe external work from a Run Connection
Profile. Observed worker lifecycle is not telemetry health, artifact delivery,
data-plane health, or Snapshot truth.

Named Lifecycle Obligations own scheduling, cancellation, outcome recording,
and readiness policy for expiry and retention work. `/healthz` remains process
liveness. `/readyz` renders configured readiness policy; it must not claim that
a pure Workbench Snapshot read performed lifecycle reconciliation.

### Stream And Telemetry Truth

Run stream authorization happens before track selection and fan-out. Retained
track state is keyed by `(runID, track)` and is a latest-state materialized
view, not an event history.

Telemetry admission owns one durable disposition for `(run, worker, sequence)`:
first accept, duplicate, gap, or out-of-order. Publication and frame-count
effects happen only after that disposition is known.

A Run Observation Session combines bounded historical catch-up with the
authorized live Observer leg. It exposes catching-up, live, degraded, and
expired states without forcing browser callers to poll an unbounded event
history.

### Review Truth

Strict Workbench Snapshot acceptance validates provenance, generation,
completeness, source, and live/fixture state before projection. A Configured
Data Flush owns its plan, protected-resource inventory, generation switch,
recovery result, and write-admission rule. It is a compensating workflow, not a
global transaction.

The Workbench Review Context is presentation-only mediation. Every displayed
relationship declares its source and permitted link: live Snapshot, stale live
truth, fixture fallback, or Experience Scenario. It must not become a truth
store or perform field-wise mixing.

## Evidence Expectations

Each downstream slice must test external behavior at the highest existing seam:

- Unknown Commit reconciliation before a new append.
- one Delivery Attempt and one Verified Delivery Evidence record for a replayed
  batch.
- Delivery Assurance strings that match writer proof strength.
- one Artifact Forge outcome and one consumption marker for a replayed eligible
  artifact.
- lifecycle cancellation, distinct obligation outcomes, and truthful readiness.
- concurrent Runs with identically named tracks isolated by authorization and
  run-scoped retention.
- duplicate, gap, and reordered telemetry frames with explicit dispositions.
- bounded reconnect from catching-up to live without full-history polling.
- Configured Data Flush recovery to one coherent generation or preservation of
  the prior generation.
- live, stale, fixture, and Experience Scenario review contexts with declared
  provenance for every displayed relationship.

## Implementation Boundary

This document is the governing contract for Issue #202. It does not by itself
claim that every module above is implemented. Repository verification pins the
contract wording so later implementation slices have a stable target, and each
slice must add its own behavior evidence before being treated as complete.
