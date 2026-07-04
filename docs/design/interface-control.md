# Interface Control Document

| Field | Value |
| --- | --- |
| Document ID | ICD-001 |
| Revision | 3.0 |
| Status | Draft for v3.0 review |
| Owner | Software |
| Baseline | v3.0 candidate |

## Purpose

This document identifies internal and operational interfaces that are controlled for the v3.0 baseline.

## User Interface

| Interface | Inputs | Outputs | Control |
| --- | --- | --- | --- |
| Kaleidos Brief | Public facts and milestones | Source-linked fact cards and boundaries | Fixture validation |
| Compute Workbench | Synthetic compute jobs | Job status, logs, outputs, diagnosis | Unit tests and fixture validation |
| Evidence Matrix | Requirements, compute evidence packs, controlled evidence records, deployment checks | Traceability and evidence views | Verification matrix and generated evidence |

## Fixture Interface

`src/data/readiness-fixtures.json` shall conform to `src/domain/types.ts`. The fixture set includes public facts, milestones, requirements, compute jobs, compute evidence packs, controlled evidence records, and deployment checks.

## Script Interface

| Script | Inputs | Outputs |
| --- | --- | --- |
| `scripts/validate-fixtures.mjs` | Controlled fixtures | Pass/fail fixture validation |
| `scripts/generate-evidence.mjs` | Controlled fixtures | `generated/evidence-index.json` |
| `scripts/check-infra.mjs` | Infrastructure files | Pass/fail static and optional native checks |
| `scripts/check-quality-docs.mjs` | Controlled markdown docs | Pass/fail documentation structure check |
| `scripts/check-simops-contract.mjs` | Simulation Ops schemas and examples | Pass/fail contract example validation |
| `scripts/checkpoint-wip.sh` | Git worktree state | WIP checkpoint commit and optional push |
| `scripts/fold-branch.sh` | Source and target branches | No-fast-forward merge into target branch |
| `scripts/checkpoint-version.sh` | Release candidate branch and version tag | Version checkpoint commit/tag and optional push |
| `scripts/cleanup-version-hygiene.sh` | Target branch, version tag, optional merged branch/worktree | Push/tag hygiene and optional local cleanup |
| v2 wrapper scripts | Historical v2 commands | Compatibility calls into generic release scripts |
| `scripts/create-local-gateway-certs.sh` | Local dev certificate request | Ignored `.local/certs/` CA, server, authorized client, and unauthorized client certificates |

## Backend Gateway Interface

| Handler | Method | Input | Output | Control |
| --- | --- | --- | --- | --- |
| `/healthz` | GET | None | `{"status":"ok"}` | Handler test |
| `/readyz` | GET | Runtime config | Ready status and mode | Handler test |
| `/metrics` | GET | In-memory counters | Prometheus text metrics | Handler test and infra check |
| `/api/jobs/submit` | POST | `script_name`, `partition`, `node_count`, optional `rank_count` | Queued job id, state, and mode | mTLS identity check, allowlists, Go tests |
| `/api/jobs/{job_id}` | GET | Job id path segment | Recorded job status | mTLS identity check and Go tests |
| `/api/simops/runs` | POST | Scenario id, optional work script, worker kinds, launch mode, runtime, idempotency key | Run id, lifecycle state, workers, spool commands, artifact refs, MoQ/WebTransport subscription metadata | mTLS identity check, allowlists, idempotency tests, Go tests |
| `/api/simops/runs/{run_id}` | GET | Run id path segment | Run, worker, spool-command, lifecycle, and artifact-reference state | mTLS identity check and Go tests |
| `/api/simops/runs/{run_id}/stop` | POST | Run id path segment | Controlled stop lifecycle update | mTLS identity check and Go tests |
| `/internal/simops/runs/{run_id}/ingest` | POST | Token-gated telemetry frame batch | Validated ingest count and lifecycle/telemetry event append | Internal token validation and Go tests |

Submit and status handlers require an authorized client certificate common name unless `SLURM_GATEWAY_REQUIRE_CLIENT_CERT=false` is explicitly set for local development. Default mode is `mock`; `sbatch` mode is enabled only through `SLURM_GATEWAY_MODE=sbatch`, `SLURM_GATEWAY_SCRIPT_ROOT`, and allowlist configuration.

Simulation Ops public handlers use the same backend trust boundary. The frontend receives short-lived MoQ/WebTransport subscription metadata, not Redpanda, Postgres, MinIO, Docker, or Iceberg catalog credentials.

## Simulation Ops Contract Interface

| Interface | Input | Output | Control |
| --- | --- | --- | --- |
| Run manifest | Scenario selection, workbench anchor, worker declarations, randomization blueprint | Bounded run setup record | `simops-run-manifest.v1` schema |
| Telemetry envelope | Worker frame with sequence, timestamps, payload type, and payload | Transport-agnostic telemetry frame | `simops-telemetry-envelope.v1` schema |
| Payload schemas | Scheduler, storage, elastic bursting, and fabric profiler metrics | Panel-ready metric structures | Payload schemas and example NDJSON |
| Run summary | Completed telemetry stream and scenario events | Reviewable run artifact for future evidence handoff | `simops-run-summary.v1` schema |

The contract uses NDJSON as the canonical example and local fixture format. Live browser transport for v1 is MoQ over WebTransport; the SSE events endpoint is not part of the controlled v1 interface.

| MoQ Namespace | Track | Payload |
| --- | --- | --- |
| `radiant/simops/{run_id}` | `lifecycle` | Run lifecycle and control-plane status updates |
| `radiant/simops/{run_id}` | `workers/{worker_id}/telemetry` | Worker telemetry envelope frames |
| `radiant/simops/{run_id}` | `workers/{worker_id}/quality` | Worker health, validation, and quality observations |
| `radiant/simops/{run_id}` | `artifacts` | Artifact and Iceberg commit references |

## Simulation Ops Persistence Interface

| Layer | Controlled Responsibility | Local Artifact |
| --- | --- | --- |
| Postgres | Runs, workers, spool commands, idempotency keys, launch records, artifact refs, and Iceberg SQL-catalog metadata | `deploy/postgres-init/001_simops.sql` |
| Redpanda | Hot durable telemetry and lifecycle log keyed by run and worker | `deploy/slurm-gateway.compose.yml` |
| MinIO | S3-compatible object storage for local Parquet-backed Iceberg table data | `deploy/slurm-gateway.compose.yml` |
| Iceberg Rust writer boundary | Micro-batch validation, Parquet write planning, Iceberg commit handoff, and artifact-reference registration | `backend/slurm-gateway/cmd/simops-iceberg-writer` |
| MoQ stream gateway boundary | Redpanda consumption and MoQ/WebTransport track publication | `backend/slurm-gateway/cmd/simops-stream-gateway` |

## Infrastructure Interface

Docker, Terraform, Ansible, Slurm gateway compose, SimOps compose services, Prometheus, and local certificate helper files describe local-safe infrastructure intent. They are validated by static checks and optional tool-native checks when the relevant tools are installed.

## External Source Interface

External public-source links are controlled through fixture fields and shall use HTTPS URLs.
