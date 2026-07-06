# Software Design Description

| Field | Value |
| --- | --- |
| Document ID | SDD-001 |
| Revision | 3.0 |
| Status | Draft for v3.0 review |
| Owner | Software |
| Baseline | v3.0 candidate |

## Purpose

This document describes the design of the Kaleidos Compute Readiness Console and its controlled boundaries.

## System Overview

The console is a local React and TypeScript application with an optional Go backend gateway. It presents source-linked public facts, synthetic compute jobs, deterministic toy calculations, infrastructure-readiness artifacts, and objective evidence records. The v3.0 backend gateway adds a controlled handler boundary for Slurm-style submission while keeping mock mode as the default public-safe path. The Simulation Ops backend slice adds bounded run orchestration, token-gated telemetry ingest, API-polled event state, Postgres-backed local control-plane persistence, Redpanda event publication, Docker-launched Rust workers, WebTransport delivery, and Iceberg artifact management. The Simulator Workbench backend dataflow slice promotes resident SCADA stand-ins, simulated result ingest, twin projection, Postgres read models, Iceberg tables, and read-only Workbench APIs while preserving the measured, simulated, and imputed value-basis boundary.

## Major Components

| Component | Location | Responsibility |
| --- | --- | --- |
| UI console | `src/App.tsx`, `src/styles.css` | Renders brief, workbench, and evidence views from controlled fixtures |
| Domain logic | `src/domain/readiness.ts` | Performs deterministic toy calculations, diagnosis, hashing, and traceability checks |
| Domain types | `src/domain/types.ts` | Defines controlled fixture and result shapes |
| Fixtures | `src/data/readiness-fixtures.json` | Source of public facts, synthetic jobs, requirements, compute evidence, controlled process evidence, and deployment checks |
| Evidence generation | `scripts/generate-evidence.mjs` | Generates reproducible local evidence index |
| Fixture validation | `scripts/validate-fixtures.mjs` | Enforces fixture integrity and traceability |
| Infrastructure checks | `scripts/check-infra.mjs` | Verifies local-safe infrastructure artifact completeness |
| Simulation Ops contract | `docs/design/simulation-ops-telemetry-contract.md`, `docs/schemas/simulation-ops/`, `examples/simulation-ops/` | Defines transport-agnostic scenario telemetry frames, payload schemas, and example run artifacts |
| Simulator Workbench contracts | `docs/design/simulator-workbench-stub-ledger.md`, `docs/design/simulator-workbench-backend-dataflow-slice.md`, `docs/schemas/scada/`, `docs/schemas/digital-twin/`, `docs/schemas/simulator-workbench/`, `examples/scada/`, `examples/digital-twin/`, `examples/simulator-workbench/` | Defines measured, imputed, simulated, lineage, workbench-state, and backend dataflow contracts |
| Simulator Workbench backend dataflow | `workers/scada-standins/`, `backend/slurm-gateway/internal/gateway/*workbench*.go`, `backend/slurm-gateway/cmd/workbench-projection-writer/`, `backend/slurm-gateway/cmd/twin-projector/`, `backend/slurm-gateway/cmd/workbench-iceberg-writer/` | Ingests resident measured SCADA frames and simulated result frames, projects read models, appends Iceberg tables, emits twin state, and serves read-only Workbench APIs |
| Slurm gateway | `backend/slurm-gateway/` | Provides health, readiness, metrics, submit, and status handlers with mTLS identity checks and mock/`sbatch` spooler modes |
| SimOps control plane | `backend/slurm-gateway/internal/gateway/simops_*.go` | Provides run creation, status, stop, event polling, token-gated ingest, idempotency, WebTransport subscription metadata, Postgres/Redpanda adapters, Docker worker launch, and artifact status transitions |
| SimOps and Workbench deployment | `deploy/slurm-gateway.compose.yml`, `deploy/postgres-init/001_simops.sql`, `deploy/scada-standins.Dockerfile` | Defines local Redpanda, Postgres, MinIO, stream-gateway, SimOps writers, Workbench writers, twin projector, Docker-launch access, and resident SCADA stand-ins |

## Design Constraints

- Public facts shall remain source-linked and bounded.
- Synthetic outputs shall remain clearly separated from public facts.
- Evidence indexes shall be reproducible from controlled fixtures.
- Release scripts shall default to excluding generated output, build output, local environment files, and `JD.mhtml`.
- The application shall run locally with controlled fixtures and may run an optional backend gateway; mock mode remains the default and real `sbatch` mode is opt-in only.
- The frontend shall not hold client private keys for backend gateway authentication.
- The frontend shall not hold Redpanda, Postgres, MinIO, Docker, or Iceberg catalog credentials; it polls gateway run/event endpoints and receives short-lived WebTransport subscription metadata from the control plane.
- Iceberg artifact management shall not replace Postgres control-plane state; the deployment contract keeps Postgres as the run, spooler, idempotency, ingest-token, event, and artifact-reference source of truth while memory adapters remain available for deterministic local tests.
- Simulator Workbench values shall keep measured, imputed, and simulated basis visible in contracts, APIs, UI anchors, and examples.
- Resident SCADA stand-ins shall emit only `valueBasis=measured`; SimOps workers shall emit operational telemetry plus separate `valueBasis=simulated` result frames; only the twin projector shall emit `valueBasis=imputed`.
- Simulator Workbench scaffold artifacts shall not imply real plant telemetry, safety behavior, actuation, alarm management, or validated physics.

## Data Flow

1. Controlled fixtures define facts, requirements, jobs, compute evidence packs, controlled evidence records, milestones, and deployment checks.
2. Domain functions compute toy transport, thermal, fleet, diagnosis, evidence, and coverage outputs.
3. The UI renders controlled fixture records and derived outputs.
4. Validation scripts check fixture consistency and infrastructure artifact presence.
5. Evidence generation writes a reproducible derived index under `generated/`.
6. The optional Slurm gateway validates authorized client identity and request bounds before recording a synthetic mock job or delegating to configured `sbatch`.
7. Simulation Ops run requests enter the Go control plane through `POST /api/simops/runs`, which validates scenario, work-script, worker-kind, runtime, and idempotency bounds.
8. The control plane records run state, launches or plans worker spool commands, returns WebTransport subscription metadata, exposes persisted events at `/api/simops/runs/{run_id}/events` for recovery/inspection, and accepts token-gated telemetry at `/internal/simops/runs/{run_id}/ingest`.
9. Ingest validates frames, publishes them to Redpanda, and updates lightweight worker counters. Redpanda is the durable fanout source for `simops-moq-gateway`, `simops-timescale-writer`, and `simops-iceberg-writer`; Timescale/Postgres stores control state and hypertable projections, MinIO stores the Parquet-backed Iceberg warehouse, and Iceberg-Go commits `simops.telemetry_frames` only after fresh readback succeeds.
10. Resident SCADA stand-ins register a public-safe source declaration, post measured frames to `/internal/scada/telemetry`, and publish through Redpanda topic `scada.telemetry.v1` to Postgres projection `scada_measured_frames`, Iceberg table `scada.measured_frames`, and the twin projector.
11. SimOps workers continue to post operational telemetry to `/internal/simops/runs/{run_id}/ingest` for `simops.telemetry.v1`, and additionally post synthetic simulated result frames to `/internal/simops/runs/{run_id}/results` for `simops.results.v1`.
12. The twin projector consumes measured and simulated result streams, records lineage, emits `digital-twin.state.v1`, and the Workbench projection/Iceberg writers materialize `digital_twin_state_values` and `digital_twin.state_values`.
13. Read-only Workbench APIs expose `/api/simulator-workbench/state`, `/measured`, `/twin`, and `/lineage/{value_id}` for a later frontend-control slice.

## Design Outputs

Design outputs are source files, fixture records, test cases, infrastructure artifacts, quality documentation, release scripts, and generated evidence procedures.
