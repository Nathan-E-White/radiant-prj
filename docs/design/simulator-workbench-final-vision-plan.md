# Status Workbench Final Vision Plan

| Field | Value |
| --- | --- |
| Document ID | SIMWB-PLAN-001 |
| Revision | 0.2 |
| Status | Planning document |
| Owner | Software |
| Baseline | Post-v3.0 planning input |

## Purpose

This planning document proposes the final architecture direction for the user-facing Status Workbench surface in the Kaleidos Compute Readiness Console. It covers project structure, additional files, frontend components, backend/data-plane changes, worker boundaries, documentation changes, and verification gates. Backend/API path names may still use `simulator-workbench` for controlled contract continuity.

This document is intentionally inert. It does not create source stubs, schemas, components, worker binaries, API handlers, database migrations, or deployment services. Those belong in later implementation threads after the current SimOps data-plane TODOs are finished.

## Current Repository State

The current branch is `redpanda-integration`. The worktree is dirty with active Simulation Ops data-plane changes. The dirty set is clustered around:

- `backend/slurm-gateway/` Go SimOps data-plane, Iceberg, Timescale, and MoQ changes.
- `deploy/slurm-gateway.compose.yml`, `deploy/postgres-init/001_simops.sql`, and `deploy/prometheus.yml`.
- `docs/design/`, `docs/requirements/`, README, fixture, and verification updates.
- Untracked SimOps data-plane files, including `docs/design/simops-data-plane-todo-stubs.md`.

The current checks pass:

- `bun run ci`
- `git diff --check`

Planning must preserve the existing dirty work and avoid pretending that the untracked data-plane TODOs are already baselined.

## Design Decisions

| Decision | Direction |
| --- | --- |
| Workbench placement | The user-facing workbench is Status Workbench. It replaces the separate Compute Workbench and SimOps Control top-level tabs while preserving Welcome and Evidence as the other top-level pages. |
| Simulation workers | Run-scoped Rust workers represent expensive scientific simulations or compute workloads launched dynamically by the backend. |
| Sensor/SCADA stand-ins | Resident Rust workers represent measured sensor/SCADA streams. They are not dynamically spawned per simulation run. |
| Digital twin | The twin consumes measured and simulated data. It does not generate source telemetry. |
| Value basis | Measured, imputed, and simulated values must remain distinguishable in schemas, storage, APIs, and UI. |
| Sensor/SCADA health | No dedicated sensor/SCADA health panel is planned. Lightweight data quality and freshness indicators are allowed. |
| Simulation health | Simulation/computation health remains in scope as the lower Status Workbench HPC status bay because expensive compute lifecycle, resource pressure, worker failures, and artifact production are central to the workbench. |
| Current TODOs | The Iceberg readback proof, WebTransport live-link probe, and Docker metadata/content preflight are closed in `docs/design/simops-data-plane-todo-stubs.md`; broader workbench implementation can build on that data-plane baseline. |

## Core Model

The workbench shall separate three classes of values.

| Class | Meaning | Examples | UI treatment |
| --- | --- | --- | --- |
| Measured state | Direct sensor/SCADA observations from resident source stand-ins | flux, temperature, pressure, valve position, pump state, breaker state | Display as observed values with tag identity, timestamp, unit, quality, and freshness. |
| Imputed state | Digital-twin estimates inferred from measured inputs and model state | unmeasured internal field estimates, local margins, interpolated spatial values, confidence bands | Display as model-derived values with basis, confidence, input lineage, and freshness. |
| Simulated result state | Run-scoped scientific computation outputs and scenario implications | forecasted margins, sensitivity results, computed unmeasurable quantities, predicted propagation | Display as simulation results tied to run id, model id, worker id, artifact lineage, and computation health. |

The dangerous move would be flattening all three into "metrics." That loses operational truth. The planned contract should carry an explicit `valueBasis` field, with initial values:

- `measured`
- `imputed`
- `simulated`

Future values such as `operatorEntered`, `replayed`, or `syntheticFixture` can be added only if the UI and storage semantics stay clear.

## Existing Backbone To Preserve

The current SimOps data-plane shape is the right backbone:

```text
-------------------------+       +------------------------+
| Run-scoped Rust worker | ----> | Go control plane ingest |
| simulation telemetry   | HTTP  | token gated validation  |
+-------------------------+       +-----------+------------+
                                             |
                                             v
                                      +-------------+
                                      | Redpanda    |
                                      | hot log     |
                                      +------+------+ 
                                             |
                +----------------------------+----------------------------+
                v                            v                            v
        +---------------+            +---------------+            +----------------+
        | MoQ track     |            | Timescale     |            | Iceberg-Go     |
        | router        |            | projection    |            | artifact/data  |
        +---------------+            +---------------+            +----------------+
```

The plan should extend that backbone rather than create a second transport stack. The sensor/SCADA path should publish measured frames into the same durable data-plane family, but under a different topic, schema, and lifecycle semantics.

## Proposed Project Structure

### Workers

| Path | Purpose |
| --- | --- |
| `workers/simops-generator/` | Keep as the run-scoped scientific simulation telemetry generator. |
| `workers/scada-standins/` | Proposed Rust workspace for resident sensor/SCADA stand-ins. |
| `workers/scada-standins/src/sources/` | Proposed source modules for flux, temperature, pressure, actuator, and comms/tag streams. |
| `workers/scada-standins/src/output.rs` | Proposed output and ingest client for measured telemetry frames. |
| `workers/scada-standins/README.md` | Proposed operator-facing explanation of resident source behavior and non-goals. |

`workers/scada-standins/` should not reuse the simulation manifest as-is. It needs a resident source declaration, not a run manifest. A sensor source exists before and after a simulation run; a simulation worker exists because a run was requested.

### Backend

| Path | Purpose |
| --- | --- |
| `backend/slurm-gateway/internal/gateway/simops_*.go` | Keep current Simulation Ops control-plane and run orchestration while the backend root remains stable. |
| `backend/slurm-gateway/internal/gateway/scada_*.go` | Proposed resident-source ingest, validation, and projection logic. |
| `backend/slurm-gateway/internal/gateway/twin_*.go` | Proposed digital twin read-model API, lineage assembly, and state projection. |
| `backend/slurm-gateway/cmd/scada-standin-collector/` | Optional later collector process if resident workers should not post directly to the gateway. |
| `backend/slurm-gateway/cmd/twin-projector/` | Optional later process to consume measured and simulated streams and materialize twin state. |

The backend directory name is currently `slurm-gateway`, which is awkward but tolerable. Do not rename it during the first simulator workbench slice. A rename would touch too much surface area while the data plane is still settling.

### Frontend

| Path | Purpose |
| --- | --- |
| `src/api/simulatorWorkbench.ts` | Proposed API client for workbench state, twin state, simulation runs, and lineage. |
| `src/components/simulator-workbench/` | Proposed component folder for the top-level workbench surface. |
| `src/components/simulator-workbench/MeasuredStatePanel.tsx` | Proposed measured telemetry panel. |
| `src/components/simulator-workbench/TwinStatePanel.tsx` | Proposed measured-plus-imputed twin panel. |
| `src/components/simulator-workbench/SimulationResultsPanel.tsx` | Proposed simulation result panel. |
| `src/components/simulator-workbench/SimulationHealthPanel.tsx` | Existing compact health model/story fixture retained for coverage; user-facing detailed status now belongs in the Status Workbench HPC bay. |
| `src/components/simulator-workbench/LineagePanel.tsx` | Proposed value/source/run/model lineage panel. |
| `src/domain/simulator-workbench/` | Proposed UI domain mappers, value-basis helpers, and presentation models. |

The existing `SimOps Control` tab in `src/App.tsx` is absorbed as a Container Orchestration region inside Status Workbench. It should not return as a standalone top-level surface.

### Schemas And Examples

| Path | Purpose |
| --- | --- |
| `docs/schemas/simulator-workbench/` | Proposed shared workbench state and value-basis schemas. |
| `docs/schemas/scada/` | Proposed measured sensor/SCADA envelope and resident-source declaration schemas. |
| `docs/schemas/digital-twin/` | Proposed twin entity, twin state, imputed-value, and lineage schemas. |
| `examples/scada/` | Proposed measured telemetry examples. |
| `examples/digital-twin/` | Proposed twin state and lineage examples. |
| `examples/simulator-workbench/` | Proposed end-to-end example combining measured inputs, simulation outputs, and twin state. |

Do not overload `docs/schemas/simulation-ops/` with SCADA semantics. The current Simulation Ops schema is run-scoped and simulation-oriented. SCADA stand-ins need their own contract.

## Proposed Data Contracts

### Measured Telemetry Envelope

A measured frame should include:

| Field | Meaning |
| --- | --- |
| `schemaVersion` | Proposed value: `scada.telemetry.v1`. |
| `sourceId` | Resident source identifier. |
| `tagId` | SCADA tag or sensor channel identifier. |
| `assetId` | Workbench asset or twin entity associated with the tag. |
| `signalKind` | Domain category such as `flux`, `temperature`, `pressure`, `flow`, `actuatorState`, or `electricalState`. |
| `sampledAt` | Time the source claims the measurement was sampled. |
| `observedAt` | Time the collector received or accepted it. |
| `sequence` | Monotonic per-source or per-tag sequence. |
| `unit` | Physical unit. |
| `value` | Scalar, state, or compact structured value. |
| `quality` | Lightweight quality such as `good`, `stale`, `bad`, `missing`, or `estimated`. |
| `valueBasis` | Always `measured` for direct sensor/SCADA frames. |
| `syntheticStatus` | Explicit marker that this repo uses a public-safe stand-in, not real plant data. |

This is data qualification, not sensor diagnostics. It supports freshness and confidence decisions without introducing a SCADA maintenance console.

### Simulation Result Envelope

The existing Simulation Ops telemetry envelope should remain run-scoped, but later revisions should clarify:

| Field | Direction |
| --- | --- |
| `valueBasis` | `simulated` for scientific computation outputs. |
| `modelId` | Model or solver profile that produced the value. |
| `runId` | Required run identifier. |
| `workerId` | Required simulation worker identifier. |
| `artifactId` | Optional artifact or data-lake reference. |
| `inputWindow` | Optional measured/twin input time range used by the computation. |
| `confidence` | Optional value if the model produces confidence or uncertainty metadata. |

The worker may emit operational health and result frames, but result values remain simulated/imputed implications, not measurements.

### Twin State Envelope

The twin should materialize state for UI consumption:

| Field | Meaning |
| --- | --- |
| `schemaVersion` | Proposed value: `digital-twin.state.v1`. |
| `twinId` | Digital twin instance. |
| `entityId` | Asset, subsystem, channel, or spatial region. |
| `asOf` | State timestamp. |
| `values` | Collection of measured, imputed, and simulated values. |
| `lineage` | Source tags, simulation runs, model ids, artifact ids, and time windows. |
| `confidence` | Overall confidence or per-value confidence where available. |
| `freshness` | Age and stale/missing input summary. |

The twin is a consumer/projection. It should not be treated as a telemetry source. If a later implementation emits twin-state updates, those updates represent projected state, not raw measurement or simulation generation.

## Proposed Data Flow

```text
----------------------+       +--------------------------+
| Resident SCADA      | ----> | measured ingest / log    |
| stand-in workers    |       | scada.telemetry.v1       |
+----------------------+       +-------------+------------+
                                             |
                                             v
                                   +-------------------+
                                   | Redpanda hot log  |
                                   +---------+---------+
                                             |
                       +---------------------+---------------------+
                       v                                           v
             +-------------------+                       +----------------+
             | Timescale measured|                       | Data lake      |
             | projection        |                       | measured table |
             +---------+---------+                       +----------------+
                       |
                       v
+----------------------+       +--------------------------+
| Run-scoped           | ----> | simulation ingest / log  |
| simulation workers   |       | simops.telemetry.v1      |
+----------------------+       +-------------+------------+
                                             |
                                             v
                      +------------------------------------+
                      | Digital twin projector/read model  |
                      | consumes measured + simulated data |
                      +------------------+-----------------+
                                         |
                                         v
                            +--------------------------+
                            | Simulator Workbench UI   |
                            | measured / imputed /     |
                            | simulated views          |
                            +--------------------------+
```

## Frontend Surface Plan

Status Workbench should be a top-level tab or route. It should not be a landing page. It should open directly into a usable workbench.

### Recommended View Structure

| View | Responsibility |
| --- | --- |
| Overview | Current scenario, run state, measured-state summary, twin-state summary, active simulation result summary. |
| Measured State | Sensor/SCADA values grouped by subsystem or asset, with timestamp, unit, and data quality. |
| Digital Twin | State view that may combine measured and imputed values, with confidence and input lineage visible. |
| Simulation Results | Run-scoped computed values, projections, sensitivity outputs, and artifact references. |
| HPC Status Bay | Queue-driven worker lifecycle, run state, resource pressure, stream quality, checkpoint/artifact status, scheduler status, and failure causes. |
| Lineage | Explains where the selected displayed value came from: source tag, model, run, artifact, table, and timestamp window. |

### What Not To Build

- No SCADA maintenance dashboard.
- No sensor calibration or device diagnostic workflow.
- No alarm-management product.
- No direct browser access to Redpanda, Timescale/Postgres, MinIO, Docker, Slurm, or Iceberg credentials.
- No reactor control-room, safety-path, SCRAM, actuator-control, or validated-physics claims.

### Lightweight Sensor Data Qualification

Measured panels may show:

- `quality`
- freshness age
- missing/stale input badge
- whether the twin accepted or rejected a measurement
- whether imputed values were degraded because inputs were stale

This is not a sensor health panel. It is the minimum truth needed to avoid displaying bad measured inputs as if they were good.

## HPC Status Bay Scope

Simulation health remains in scope because the simulation workers are expensive, resource-sensitive, and run-scoped. The Status Workbench HPC status bay should cover:

- run lifecycle
- selected scenario
- worker lifecycle
- queue or launch mode
- frame counts
- stream quality
- Redpanda topic/partition/offset progress
- Timescale projection progress
- Iceberg artifact status
- compute resource pressure, if available
- run errors and retry/disposition state

This panel answers whether the computation completed, whether results are trustworthy, and whether artifacts were committed.

## Data Lake, Database, And Twin Wiring

### Redpanda

Use separate topics for measured and simulation streams:

- `scada.telemetry.v1` for measured stand-in telemetry.
- `simops.telemetry.v1` for simulation worker telemetry.
- Optional future `digital-twin.state.v1` for projected twin updates if a streaming twin surface is needed.

### Timescale/Postgres

Postgres/Timescale should remain the local operational query store:

- control-plane runs, workers, commands, artifacts, idempotency, and ingest tokens
- measured telemetry projection tables
- simulation telemetry projection tables
- twin state materialization tables
- consumer offsets and processing status

The measured and simulation tables should not collapse into one untyped telemetry table unless `valueBasis`, source type, and lineage are indexed and impossible to ignore.

### Iceberg/Data Lake

Iceberg should remain the analytic artifact and data-lake boundary:

- append measured frames to measured telemetry tables
- append simulation frames to simulation result tables
- append twin snapshots or lineage records only if the downstream query use case is clear

Artifact status should only become `committed` after a readable Iceberg snapshot and data file exist. The current SimOps writer enforces this with fresh Iceberg-Go readback before commit.

### Digital Twin Projection

The twin projection should consume:

- recent measured frames
- accepted simulation result frames
- model metadata
- lineage mappings between tags, assets, model inputs, and simulated outputs

The twin projection should produce:

- current state for each twin entity
- per-value basis and confidence
- stale/missing input summaries
- references to contributing tags, runs, artifacts, and model ids

## Backend API Direction

The UI should talk to backend APIs, not directly to data-plane infrastructure.

Proposed API groups:

| API | Purpose |
| --- | --- |
| `GET /api/simulator-workbench/state` | Current top-level workbench state. |
| `GET /api/simulator-workbench/measured` | Measured state for selected assets/tags. |
| `GET /api/simulator-workbench/twin` | Current twin state and imputed values. |
| `GET /api/simulator-workbench/simulations` | Active and recent simulation runs. |
| `GET /api/simulator-workbench/lineage/{value_id}` | Source and artifact lineage for a displayed value. |
| existing `POST /api/simops/runs` | Keep for launching run-scoped simulations. |
| existing `GET /api/simops/runs/{run_id}` | Keep for run inspection. |
| existing `POST /api/simops/runs/{run_id}/stop` | Keep for controlled stop. |

The `/api/simops/runs/{run_id}/events` endpoint should remain recovery/inspection and should not become the final live telemetry stream.

## Documentation Changes Proposed

| Document | Proposed change |
| --- | --- |
| `README.md` | Add Simulator Workbench as a top-level surface and clarify measured/imputed/simulated value classes. |
| `docs/design/software-design-description.md` | Add Simulator Workbench architecture, resident SCADA stand-ins, twin projection, and value-basis constraints. |
| `docs/design/interface-control.md` | Add measured telemetry, twin state, and simulator workbench API interfaces. |
| `docs/design/simulation-ops-telemetry-contract.md` | Clarify that simulation worker outputs are simulated/imputed results, not measured state. |
| `docs/design/simops-data-plane-todo-stubs.md` | Keep as the pre-implementation gate closure record for Iceberg, WebTransport, and Docker metadata/content preflight. |
| `docs/requirements/software-requirements.md` | Add requirements for value-basis separation, resident measured-source ingestion, twin read model, and simulation health panels. |
| `docs/requirements/verification-matrix.md` | Add checks for measured/imputed/simulated delineation, no credential leakage, and twin lineage. |
| `docs/quality/document-index.md` | Add final controlled document entries once implementation docs are promoted beyond planning. |

## Implementation Sequence

### Phase 0: Finish Current Data-Plane TODOs

Complete before implementing this plan:

1. Iceberg-Go readback proof for appended telemetry rows and Parquet data files.
2. WebTransport live link with a MoQ-compatible namespace/track envelope.
3. Docker registry/base-image metadata/content preflight hardening for local smoke.

Exit criteria:

- Docker-dependent smoke reaches data-plane assertions.
- Iceberg artifact status reflects real readable data.
- Live telemetry claims match real WebTransport behavior.

### Phase 1: Contract And Design Baseline

Create docs and schemas only:

1. Measured telemetry contract.
2. Resident source declaration contract.
3. Digital twin state contract.
4. Workbench value-basis model.
5. Example measured, simulated, and twin state artifacts.

Exit criteria:

- Contract check validates example measured and twin artifacts.
- Existing `bun run ci` remains green.
- Docs explicitly forbid SCADA health scope creep.

### Phase 2: Resident Source Stand-Ins

Implement the resident Rust worker family:

1. Resident source declarations.
2. Measured telemetry generation.
3. Token-gated ingest path.
4. Separate Redpanda topic and Timescale projection.
5. Smoke path that proves measured frames land in the operational store.

Exit criteria:

- Resident worker is not launched as a simulation run.
- Measured frames carry `valueBasis=measured`.
- No sensor health panel or maintenance semantics are introduced.

### Phase 3: Twin Read Model

Implement the twin consumer/projection:

1. Consume measured and simulation streams.
2. Materialize per-entity twin state.
3. Preserve lineage and value basis.
4. Expose backend API for current twin state and selected value lineage.

Exit criteria:

- Twin state includes measured and imputed values.
- Simulated results are linked by run/model/artifact.
- UI can show why a value exists and whether it is measured, imputed, or simulated.

### Phase 4: Status Workbench UI

Implement the top-level surface:

1. Route or tab for Status Workbench.
2. Measured State panel.
3. Digital Twin panel.
4. Simulation Results panel.
5. Lineage panel.
6. Container Orchestration region.
7. Queue-driven HPC Status Bay.

Exit criteria:

- Measured, imputed, and simulated values cannot be confused visually.
- Queue-driven simulation/HPC status is visible.
- Sensor/SCADA health remains out of scope except data quality/freshness.

### Phase 5: Evidence And Verification

Update requirements, verification, evidence, and quality docs:

1. Add requirements for value-basis separation and twin lineage.
2. Add verification matrix rows for measured/twin/simulation API behavior.
3. Add fixture or generated evidence only where implementation exists.
4. Add browser and backend tests around the final workbench API and UI.

Exit criteria:

- `bun run ci` passes.
- Docker-dependent smoke passes where required.
- Evidence can explain workbench claims without implying real plant data.

## Acceptance Criteria For Final Vision

- Status Workbench is a top-level surface.
- Compute Workbench and SimOps Control are not standalone top-level tabs.
- Scientific simulation workers remain run-scoped and dynamically launched.
- Sensor/SCADA stand-ins are resident measured-source abstractions.
- Digital twin consumes measured and simulation data and materializes state.
- The UI clearly distinguishes measured, imputed, and simulated values.
- Simulation/computation health is first-class in the queue-driven HPC status bay.
- Sensor/SCADA health is out of scope, except lightweight quality/freshness on measured values.
- Data lake, Timescale/Postgres, Redpanda, and backend APIs preserve lineage and value basis.
- No frontend credential leakage is introduced.
- No real plant, safety, control-room, or validated-physics claim is implied.

## Open Questions

1. Should the first resident source set be organized by plant-like subsystems, compute/fleet infrastructure analogs, or a small mixed set?
2. Should digital twin state be primarily polled through HTTP first, with streaming added later, or should it be designed around the same WebTransport track model now that the SimOps data-plane gate is closed?
3. Should measured telemetry and simulation telemetry use separate Iceberg namespaces, or one namespace with separate tables?
4. What additional evidence handoff records should the Status Workbench terminal name once the queue-driven HPC bay grows beyond static synthetic status?
5. What is the smallest demo scenario that proves the value-basis split: measured flux/temp/pressure plus imputed internal state plus simulation result?

## Non-Goals

- Building a SCADA maintenance console.
- Building sensor calibration, fieldbus diagnostics, PLC/RTU status, historian administration, or alarm-management workflows.
- Creating source-code stubs in this planning thread.
- Renaming `backend/slurm-gateway/` before the runtime architecture stabilizes.
- Treating debug endpoints as live-stream proof.
- Claiming measured plant data, reactor control behavior, safety-path behavior, or validated reactor physics.
