# Browser Workbench Snapshot Session

| Field | Value |
| --- | --- |
| Document ID | WORKBENCH-SNAPSHOT-SESSION-001 |
| Revision | 1.0 |
| Status | Implemented |
| Owner | Software |
| Baseline | Issue #88 candidate |

## Purpose

This design records the browser authority that carries one coherent Workbench Snapshot from the read-only HTTP adapter to the visible Status Workbench. It implements the Live Read Boundary in ADR-0007 without giving the browser ingest, broker, database, lake, container, runtime, or cluster credentials.

## Ownership Boundary

`src/domain/simulator-workbench/workbenchSnapshotSession.ts` is the sole owner of initial loading, refresh cadence, manual refresh, overlap cancellation, disposal, accepted generation, generation monotonicity, initial fixture fallback, stale retention, recovery, projection, selection reconciliation, and Simulation Health derivation.

`src/domain/simulator-workbench/liveWorkbench.ts` is an adapter. It performs one credential-free `GET /api/simulator-workbench/snapshot`, parses and validates the whole response, and projects that response into a candidate input or a typed read error. It does not export read-state transition or fallback policy.

React subscribes to the session result and forwards its render-ready projection and commands. React owns no refresh timer, accepted-state ref, fallback decision, selection reconciliation, or health animation.

## State Policy

| Input outcome | Session result |
| --- | --- |
| Complete first live Snapshot | Accept generation atomically; derive projection, selection, and health from it |
| Initial unavailable or empty read with fallback enabled | Accept the explicit whole-Snapshot fixture |
| Initial authentication, schema, generation, or partial failure | Visible error; no fixture |
| Failure after live acceptance | Retain the accepted live generation as stale |
| Older live generation | Reject and retain the newer accepted generation as stale |
| Complete newer live generation | Replace read state, projection, reconciled selection, and health in one publication |
| Overlap or disposal | Abort the superseded request and publish no late result |

Fixture health exists only while the accepted source is `fixture`. Live and stale-live health is always derived from the accepted live Snapshot timestamp and runs.

## Selection Policy

Unit, value, and commercial-basis commands enter through the session result. A valid selection survives refresh and recovery. When replacement removes a selected entity or value, the session deterministically selects the replacement Snapshot's valid unit and preferred imputed value. No publication combines selection or projection from different generations.

## Verification

The controlled verification record is `docs/verification/issue-88-workbench-snapshot-session.md`. Module and presentation tests cover session policy; Playwright covers assembled fallback, stale retention, recovery, selection replacement, credential absence, and unmount cancellation; React Chaos covers health-panel fault containment; Stryker measures whether policy faults are detected.
