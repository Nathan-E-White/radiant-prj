# Change Log

| Field | Value |
| --- | --- |
| Document ID | CLOG-001 |
| Revision | 3.0 |
| Status | Draft for v3.0 review |
| Owner | Quality |
| Baseline | v3.0 candidate |

| Change ID | Date | Summary | Affected Records | Verification |
| --- | --- | --- | --- | --- |
| CHG-001 | 2026-06-30 | Initial controlled fixture set for public facts, synthetic jobs, requirements, evidence packs, and deployment checks. | SR-001 through SR-004, SW-001 through SW-004 | `bun run validate:fixtures` |
| CHG-002 | 2026-06-30 | Added deterministic transport, thermal, fleet anomaly, and HPC diagnosis tests. | SR-002, SR-003, SW-001, SW-002 | `bun run test` |
| CHG-003 | 2026-06-30 | Added dry-run Docker, Terraform, Ansible, and CI artifacts. | SW-004 | `bun run infra:check` |
| CHG-004 | 2026-06-30 | Added React console with Kaleidos Brief, Compute Workbench, and Evidence Matrix tabs. | SW-003 | `bun run build` |
| CHG-005 | 2026-07-01 | Added v2 controlled quality, design, verification, and release documentation package. | SR-005, SW-005, QP-001 through QP-011, SDD-001, ICD-001, VVP-001, REL-001 through REL-005 | `bun run quality:check` |
| CHG-006 | 2026-07-01 | Added WIP checkpoint, fold-back, and v2 version checkpoint script plan and implementation. | SR-006, SW-006, REL-001, REL-002 | Script dry-runs |
| CHG-007 | 2026-07-02 | Closed the existing v2.0.0 tag record, added fixture-backed controlled evidence for SR-005/SR-006 and SW-005/SW-006, and generalized release tooling with v2 compatibility wrappers. | DOC-V2-001, REL-TOOL-001, SR-005, SR-006, SW-005, SW-006 | `bun run ci`, script dry-runs |
| CHG-008 | 2026-07-03 | Rebuilt the v3.0 backend handler skeleton as a mock-first Go Slurm gateway with mTLS identity controls, status lookup, metrics, deploy artifacts, and secret hygiene. | SR-007, SW-007, SW-008, SW-009, SLURM-GATEWAY-001 | `bun run backend:test`, `bun run infra:check`, `bun run quality:check` |
| CHG-009 | 2026-07-04 | Added the Simulation Ops backend slice with bounded run control, MoQ/WebTransport subscription metadata, token-gated ingest, Redpanda/Postgres/MinIO/Iceberg deployment seams, stream-gateway and Iceberg-writer service boundaries, and Rust bucket container topology. | SR-008, SW-010, SW-011, SW-012, SIMOPS-BACKEND-001 | `bun run backend:test`, `bun run infra:check`, `bun run simops:contract:check`, `bun run simops:generator:test` |
| CHG-010 | 2026-07-05 | Reworked Simulation Ops telemetry into a Redpanda-backed data plane with Timescale hypertable projection, consumer offsets, MoQ track routing, and Iceberg-Go append mode for `simops.telemetry_frames`; documented `/events` as recovery/inspection rather than live telemetry. | SW-011, SW-012, SIMOPS-BACKEND-001 | `bun run backend:test`, `bun run infra:check`, `bun run simops:contract:check`, `bun run simops:generator:test`, Docker local smoke |
| CHG-011 | 2026-07-06 | Closed the SimOps data-plane pre-workbench gates with Docker image metadata/content preflight, WebTransport subscriber smoke probe, Compose-networked smoke workers, and Iceberg-Go append/readback verification before artifact commit. | SW-011, SW-012, SIMOPS-BACKEND-001 | `bun run backend:test`, `bun run infra:check`, `bun run simops:contract:check`, `bun run simops:smoke:local` |
| CHG-012 | 2026-07-06 | Added the Simulator Workbench backend dataflow slice with resident measured SCADA stand-ins, separate simulated result ingest, Workbench projection writers, twin imputed-state projection, Iceberg tables, read-only APIs, dataflow diagrams, and smoke evidence. | SW-013, SW-014, SW-015, WORKBENCH-DATAFLOW-001, ADR-0005, SWB-DATAFLOW-001 | `bun run backend:test`, `bun run simops:contract:check`, `bun run simulator-workbench:contract:check`, `bun run scada:standins:test`, `bun run simops:generator:test`, `bun run simulator-workbench:dataflow:smoke` |
| CHG-013 | 2026-07-07 | Refactored the app shell into readiness, simulator-workbench, and SimOps feature modules while preserving existing behavior and keeping fixture-backed workbench state selection. | SW-008, SW-009, SW-013 | `bun run typecheck`, `bun run test`, `bun run ci` |
| CHG-014 | 2026-07-07 | Added Simulation Health panel Storybook coverage backed by a shared module-level model builder/fixtures for nominal and all major degraded variants. | SW-016 | `bun run typecheck`, `bun run build-storybook` |
| CHG-015 | 2026-07-07 | Added fixture-driven 4-card Simulation Health projection logic, dynamic timer driver, and surface integration with cleanup-safe lifecycle. | SW-016 | `bun run typecheck`, `bun run test` |
| CHG-016 | 2026-07-08 | Consolidated the app IA into Welcome, Status Workbench, and Evidence; absorbed the compute queue and SimOps Control into Status Workbench with a queue-driven HPC status bay. | SW-003, SW-016, ADR-0006 | `bun run typecheck`, `bun run test`, `bun run build`, `bun run build-storybook`, `bun run ci` |
| CHG-017 | 2026-07-10 | Replaced the Simulation Ops Docker CLI worker launcher with a Docker SDK adapter behind the runtime seam, using run connection profiles, structured launch metadata/errors, and run/worker-scoped cleanup filters. | SW-017, SIMOPS-DOCKER-SDK-001 | `go test ./internal/gateway ./internal/simopsdocker -run 'TestDefaultSimopsController|TestSpooler'`, `bun run backend:test`, `bun run backend:deps:check`, `bun run ci` |
| CHG-018 | 2026-07-10 | Added runtime-neutral SyncRun lifecycle observation for Simulation Ops workers, including Docker state mapping, observed lifecycle persistence, and explicit telemetry/artifact/data-plane separation. | SW-018, SIMOPS-SYNCRUN-001, ICD-001, CI-SRC, CI-DOC, CI-INF | `go test ./internal/gateway ./internal/simopsdocker -run 'TestSimopsControllerSyncs|TestWorkerTelemetryDoesNotOverwrite|TestDataPlaneAndArtifactUpdatesDoNotMutate|TestSpoolerSyncRunProfiles'`, `bun run backend:test`, `bun run backend:deps:check`, `bun run ci` |

## Control Note

This log captures the demo's engineering-control story and release baseline evolution.
