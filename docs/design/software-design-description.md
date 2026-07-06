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

The console is a local React and TypeScript application with an optional Go backend gateway. It presents source-linked public facts, synthetic compute jobs, deterministic toy calculations, infrastructure-readiness artifacts, and objective evidence records. The v3.0 backend gateway adds a controlled handler boundary for Slurm-style submission while keeping mock mode as the default public-safe path. The Simulation Ops backend slice adds bounded run orchestration, token-gated telemetry ingest, API-polled event state, Postgres-backed local control-plane persistence, Redpanda event publication, Docker-launched Rust workers, and local manifest artifact management. The Simulator Workbench scaffold adds inert contracts and compile-safe anchors for resident measured stand-ins, digital twin state, simulated result references, and lineage without adding runtime wiring.

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
| Simulator Workbench scaffold | `docs/design/simulator-workbench-stub-ledger.md`, `docs/schemas/scada/`, `docs/schemas/digital-twin/`, `docs/schemas/simulator-workbench/`, `examples/scada/`, `examples/digital-twin/`, `examples/simulator-workbench/` | Defines inert measured, imputed, simulated, lineage, and workbench-state contracts |
| Simulator Workbench source anchors | `src/api/simulatorWorkbench.ts`, `src/domain/simulator-workbench/`, `src/components/simulator-workbench/`, `workers/scada-standins/`, `backend/slurm-gateway/internal/gateway/*workbench*.go` | Provides compile-safe placeholders without mounting a frontend surface or registering backend routes |
| Slurm gateway | `backend/slurm-gateway/` | Provides health, readiness, metrics, submit, and status handlers with mTLS identity checks and mock/`sbatch` spooler modes |
| SimOps control plane | `backend/slurm-gateway/internal/gateway/simops_*.go` | Provides run creation, status, stop, event polling, token-gated ingest, idempotency, WebTransport subscription metadata, Postgres/Redpanda adapters, Docker worker launch, and artifact status transitions |
| SimOps deployment | `deploy/slurm-gateway.compose.yml`, `deploy/postgres-init/001_simops.sql` | Defines local Redpanda, Postgres, MinIO, stream-gateway, Iceberg-writer, Docker-launch access, and Rust bucket smoke services |

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
10. The Simulator Workbench scaffold validates example measured frames, twin state, lineage, and workbench state through `scripts/check-simulator-workbench-contract.mjs`; it does not add runtime data flow yet.

## Design Outputs

Design outputs are source files, fixture records, test cases, infrastructure artifacts, quality documentation, release scripts, and generated evidence procedures.
