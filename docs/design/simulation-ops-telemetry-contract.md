# Simulation Ops Telemetry Contract

| Field | Value |
| --- | --- |
| Document ID | SIMOPS-CONTRACT-001 |
| Revision | 0.1 |
| Status | Contract draft |
| Owner | Software |
| Baseline | v3.0 planning input |

## Purpose

This document defines the first Simulation Ops telemetry contract for the Kaleidos Compute Readiness Console. It gives local workers, backend collectors, chart panels, and evidence artifacts a shared event shape while the backend adds bounded run orchestration, MoQ/WebTransport subscription metadata, and durable telemetry handoff seams.

The telemetry envelope remains transport-stable. A frame that appears in the canonical NDJSON examples must also be usable over MoQ/WebTransport, WebSocket, backend-local TCP, HTTP batching, or a future container collector without changing the payload shape. The v1 live browser path is MoQ over WebTransport; RoQ is deferred.

## Run Lifecycle

| State | Meaning |
| --- | --- |
| `created` | A run manifest exists, but workers have not started. |
| `starting` | Workers are being launched or registered. |
| `streaming` | Workers are emitting telemetry frames. |
| `degraded` | The run is still active, but one or more stream-quality or scenario thresholds are exceeded. |
| `complete` | Runtime ended normally and summary artifacts are available. |
| `failed` | Runtime ended before a complete summary could be produced. |
| `stopped` | Runtime was intentionally stopped before natural completion. |

## Shared Event Envelope

Every telemetry frame uses a common envelope:

```json
{
  "schemaVersion": "simops.telemetry.v1",
  "runId": "RUN-HPC-404-001",
  "scenarioId": "scheduler-drift",
  "workerId": "scheduler-01",
  "workerKind": "scheduler",
  "sequence": 1,
  "emittedAt": "2026-07-04T18:00:00.000Z",
  "receivedAt": "2026-07-04T18:00:00.035Z",
  "payloadType": "schedulerCoScheduling",
  "streamQuality": {
    "quality": "good",
    "sourceLagMs": 12,
    "collectorLagMs": 35,
    "droppedFrameCount": 0
  },
  "payload": {}
}
```

Required fields are `schemaVersion`, `runId`, `scenarioId`, `workerId`, `workerKind`, `sequence`, `emittedAt`, `payloadType`, and `payload`.

`receivedAt` is optional because canonical NDJSON produced directly by a worker may not have passed through a collector yet. A backend collector may add it when it persists or forwards the frame.

`streamQuality` is optional but recommended for live streaming. It gives later transports enough metadata to show lag, dropped frames, replay gaps, or collector delay without making the UI infer transport behavior from chart values.

## Scenario Manifest

A run starts from a manifest with:

- `schemaVersion`: `simops.run-manifest.v1`.
- `runId`: stable identifier for a local run.
- `scenarioId`: one of `nominal`, `scheduler-drift`, `checkpoint-pressure`, `cloud-burst`, or `fabric-warning`.
- `lifecycle`: initial lifecycle state.
- `workbenchAnchor`: current app context, usually `JOB-HPC-404`, `EP-HPC-404`, and `SLURM-GATEWAY-001`.
- `runtimeLimitSec`: maximum local run duration.
- `transportBinding`: example or runtime transport name. Canonical examples use `ndjson`; live browser delivery uses `moq-webtransport`.
- `randomization`: scenario-level pressure curve, baseline ranges, couplings, events, and bounds.
- `workers`: worker definitions, payload type, emission rate, and panel target.
- `artifacts`: expected output artifacts.
- `provenance`: generated data status and storage format.

The manifest is not a command to Docker, Kubernetes, Slurm, or a browser. It is the shared input that a future backend orchestrator can translate into a bounded local deployment.

## Payload Families

| Payload type | Worker kind | Panel target | Purpose |
| --- | --- | --- | --- |
| `schedulerCoScheduling` | `scheduler` | Multiphysics Job Co-scheduler | Slurm-style allocation state, MPI rank distribution, queue Gantt lanes, barrier waits, and internode sync latency. |
| `checkpointStorage` | `storage` | IO and Checkpoint Burst Buffer Monitor | Parallel file system IOPS, burst-buffer throughput, NVMe-oF saturation, checkpoint progress, and storage target degradation. |
| `elasticBursting` | `burst` | Core Thermal Mesh Elastic Bursting Visualizer | Workload mesh pressure, hotspot trigger, local/cloud node state, EFA packet drops, spot cost, and topology graph. |
| `fabricProfiler` | `fabric` | Fabric Topology and MPI Profiler | Node-link fabric map, InfiniBand-style counters, message-size distribution, link utilization, and nonblocking communication overhead. |

## Randomization Blueprint

Simulation Ops randomization is correlated at the scenario level. Each run has a pressure value from `0.0` to `1.0` that changes over time. Payload generators derive local values from that pressure plus panel-specific jitter. This keeps the story coherent: scheduler delay, barrier waits, storage pressure, fabric stress, and cloud-burst pressure can rise together when the selected scenario calls for it.

Each scenario defines:

- `baseline`: normal metric ranges.
- `pressureCurve`: pressure at offsets from the run start.
- `couplings`: relationships that move metrics together.
- `events`: discrete transitions such as queue hold, checkpoint slowdown, burst activation, or fabric warning.
- `bounds`: hard min/max clamps for plausible chart values.

Manifests may include an optional `randomization.distributionProfile` for seeded stochastic augmentation. The profile samples emission, delay, burst-loss, and worker-sensitivity behavior conditionally on the shared pressure curve; it is not a general nested distribution language.

Repeatability is optional in this revision. `randomSeed` may appear in a manifest, but it is not required. Completed runs must record enough summary values to inspect what happened after the fact.

## Transport Binding

NDJSON is the canonical storage and example format for this slice. Each line is one telemetry envelope.

Live frontend delivery uses MoQ over WebTransport. The Go control plane returns short-lived subscription metadata for a run; the browser does not receive Redpanda credentials or bucket container authority.

The controlled track layout is:

| Track | Purpose |
| --- | --- |
| `lifecycle` | Run-level lifecycle transitions such as `starting`, `streaming`, `failed`, and `stopped`. |
| `workers/{worker_id}/telemetry` | Validated telemetry frames for one worker. |
| `workers/{worker_id}/quality` | Stream-quality observations, lag, replay gaps, and dropped-frame counts. |
| `artifacts` | Artifact and Iceberg commit references for completed or partially persisted runs. |

RoQ is not selected for v1 because the current plan needs browser-facing publish/subscribe telemetry tracks, not RTP media-session semantics. WebRTC is not assumed because the expected topology is worker-to-backend-to-browser, not peer-to-peer browser networking.

## Persistence and Artifact Handoff

The v1 deployment contract assigns Postgres to control-plane records for runs, workers, idempotency keys, spool commands, lifecycle state, artifact references, and local Iceberg SQL-catalog metadata. It assigns Redpanda to the hot durable telemetry log keyed by run and worker, MinIO to S3-compatible object storage, and Iceberg Rust to validated Parquet-backed table commits. The current checked-in Go slice keeps these as explicit adapter contracts while retaining a memory-backed local runtime for tests.

Iceberg manages analytic telemetry artifacts and table metadata. It does not replace Postgres for command, run, or authorization state.

## Contract Artifacts

| Artifact | Purpose |
| --- | --- |
| `docs/schemas/simulation-ops/simops-run-manifest.v1.schema.json` | Manifest schema. |
| `docs/schemas/simulation-ops/simops-telemetry-envelope.v1.schema.json` | Shared telemetry envelope schema. |
| `docs/schemas/simulation-ops/simops-run-summary.v1.schema.json` | Completed run summary schema. |
| `docs/schemas/simulation-ops/payload.scheduler-co-scheduling.v1.schema.json` | Scheduler co-scheduling payload schema. |
| `docs/schemas/simulation-ops/payload.checkpoint-storage.v1.schema.json` | Storage and checkpoint payload schema. |
| `docs/schemas/simulation-ops/payload.elastic-bursting.v1.schema.json` | Elastic bursting payload schema. |
| `docs/schemas/simulation-ops/payload.fabric-profiler.v1.schema.json` | Fabric profiler payload schema. |
| `examples/simulation-ops/run-manifest.scheduler-drift.json` | Coherent scheduler-drift manifest example. |
| `examples/simulation-ops/telemetry.scheduler-drift.ndjson` | Example telemetry stream. |
| `examples/simulation-ops/run-summary.scheduler-drift.json` | Example completed summary. |

## Verification

`bun run simops:contract:check` validates the example manifest, NDJSON telemetry frames, payload selection, per-worker sequence monotonicity, timestamp parsing, and summary references.
