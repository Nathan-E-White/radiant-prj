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
| Welcome | Public facts and milestones | Source-linked fact cards and boundaries | Fixture validation |
| Status Workbench | Fixture-backed fleet, measured, imputed, simulated, lineage, compute queue, SimOps orchestration, and HPC status examples | Value-basis grouped workbench, selected value lineage, queue-driven HPC status bay, and run orchestration subregions | Projection tests, frontend tests, and Simulator Workbench contract check |
| Evidence | Requirements, compute evidence packs, controlled evidence records, deployment checks | Traceability and evidence views | Verification matrix and generated evidence |

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
| `scripts/check-simulator-workbench-contract.mjs` | Simulator Workbench, SCADA stand-in, and digital twin schemas/examples | Pass/fail scaffold contract validation |
| `scripts/simops-smoke-json.mjs` | SimOps smoke JSON from API responses and Docker inspect | Pass/fail runtime proof parsing, gateway-ingest credential checks, and redacted evidence output |
| `scripts/simops-docker-orbstack-smoke.sh` | Local Docker/OrbStack compose platform and optional `SIMOPS_SMOKE_BUILD=auto\|always\|never` image build mode | Pass/fail SimOps Runtime Proof for Docker worker launch, gateway-only ingest, observed lifecycle, zero-TTL success cleanup, failed-run retention, and smoke-forced cleanup |
| `scripts/simulator-workbench-dataflow-smoke.sh` | Local Docker/OrbStack compose platform | Pass/fail backend dataflow proof for measured, telemetry, simulated, and imputed units |
| `scripts/hygiene-size.mjs` | Local repo, Git worktree, cache, and Docker/OrbStack storage inspection | Read-only size report with skipped optional sections |
| `scripts/check-hygiene-size.mjs` | Fake Git, Docker, Go, and cache fixtures | Pass/fail read-only size-report validation |
| `scripts/check-docker-storage-policy.mjs` | Controlled storage-policy documentation and report source | Pass/fail policy and read-only boundary validation |
| `bun run simops:generator:test` | `workers/simops-generator/Cargo.toml` | Rust tests with an external `/tmp/radiant-cargo-target/simops-generator` target directory by default |
| `bun run scada:standins:test` | `workers/scada-standins/Cargo.toml` | Rust tests with an external `/tmp/radiant-cargo-target/scada-standins` target directory by default |
| `scripts/checkpoint-wip.sh` | Git worktree state | WIP checkpoint commit and optional push |
| `scripts/fold-branch.sh` | Source and target branches | No-fast-forward merge into target branch |
| `scripts/checkpoint-version.sh` | Release candidate branch and version tag | Version checkpoint commit/tag and optional push |
| `scripts/cleanup-version-hygiene.sh` | Target branch, version tag, optional merged branch/worktree | Push/tag hygiene and optional local cleanup |
| v2 wrapper scripts | Historical v2 commands | Compatibility calls into generic release scripts |
| `scripts/create-local-gateway-certs.sh` | Local dev certificate request | Ignored `.local/certs/` CA, server, authorized client, and unauthorized client certificates plus smoke-readable `.local/compose-secrets/` copies for the local MoQ gateway |

## Backend Gateway Interface

| Handler | Method | Input | Output | Control |
| --- | --- | --- | --- | --- |
| `/healthz` | GET | None | `{"status":"ok"}` | Handler test |
| `/readyz` | GET | Runtime config | Ready status and mode | Handler test |
| `/metrics` | GET | In-memory counters | Prometheus text metrics | Handler test and infra check |
| `/api/jobs/submit` | POST | `script_name`, `partition`, `node_count`, optional `rank_count` | Queued job id, state, and mode | mTLS identity check, allowlists, Go tests |
| `/api/jobs/{job_id}` | GET | Job id path segment | Recorded job status | mTLS identity check and Go tests |
| `/api/simops/runs` | POST | Scenario id, optional work script, worker kinds, launch mode, runtime, idempotency key | Run id, lifecycle state, workers, spool commands, artifact refs, WebTransport subscription metadata | mTLS identity check, allowlists, idempotency tests, Go tests |
| `/api/simops/runs/{run_id}` | GET | Run id path segment | Run, worker, spool-command, lifecycle, observed runtime-worker lifecycle, and artifact-reference state | mTLS identity check and Go tests |
| `/api/simops/runs/{run_id}/events` | GET | Run id path segment | Persisted lifecycle, telemetry, and artifact-ready events | mTLS identity check, event store, and Go tests |
| `/api/simops/runs/{run_id}/stop` | POST | Run id path segment | Controlled stop lifecycle update | mTLS identity check and Go tests |
| `/internal/simops/runs/{run_id}/ingest` | POST | Token-gated telemetry frame batch | Validated ingest count and lifecycle/telemetry event append | Internal token validation and Go tests |
| `/internal/simops/runs/{run_id}/results` | POST | Token-gated simulated result batch | Validated simulated result count and Workbench event publication | Internal token validation and Go tests |
| `/internal/scada/sources` | POST | Workbench token-gated resident source declaration | Accepted source id and resident tag registration | Internal token validation and Go tests |
| `/internal/scada/telemetry` | POST | Workbench token-gated measured frame batch | Accepted measured frame count and Workbench event publication | Internal token validation and Go tests |
| `/api/simulator-workbench/state` | GET | Authorized client request | Compact Workbench state summary | mTLS identity check and Go tests |
| `/api/simulator-workbench/measured` | GET | Authorized client request | Latest measured frames | mTLS identity check and Go tests |
| `/api/simulator-workbench/twin` | GET | Authorized client request | Current digital twin state | mTLS identity check and Go tests |
| `/api/simulator-workbench/lineage/{value_id}` | GET | Authorized client request | Selected value lineage | mTLS identity check and Go tests |

Submit and status handlers require an authorized client certificate common name unless `SLURM_GATEWAY_REQUIRE_CLIENT_CERT=false` is explicitly set for local development. Default mode is `mock`; `sbatch` mode is enabled only through `SLURM_GATEWAY_MODE=sbatch`, `SLURM_GATEWAY_SCRIPT_ROOT`, and allowlist configuration.

Simulation Ops public handlers use the same backend trust boundary. Browser-local development may explicitly disable client certificate enforcement at the gateway while relying on the Vite/API proxy path; non-browser gateway use keeps mTLS as the controlled boundary. The frontend uses run/status endpoints for control and recovery inspection, receives short-lived WebTransport subscription metadata for live tracks, and never receives Redpanda, Timescale/Postgres, MinIO, Docker, or Iceberg catalog credentials.

## Simulation Ops Contract Interface

| Interface | Input | Output | Control |
| --- | --- | --- | --- |
| Run manifest | Scenario selection, workbench anchor, worker declarations, randomization blueprint | Bounded run setup record | `simops-run-manifest.v1` schema |
| Telemetry envelope | Worker frame with sequence, timestamps, payload type, and payload | Transport-agnostic telemetry frame | `simops-telemetry-envelope.v1` schema |
| Payload schemas | Scheduler, storage, elastic bursting, and fabric profiler metrics | Panel-ready metric structures | Payload schemas and example NDJSON |
| Run summary | Completed telemetry stream and scenario events | Reviewable run artifact for future evidence handoff | `simops-run-summary.v1` schema |
| Simulated result envelope | Run-scoped synthetic engineering result values | `simops.result.v1` frames with `valueBasis=simulated` | `simops-result-envelope.v1` schema |
| Runtime adapter sync | Run record and stored worker records | Runtime-neutral observed worker lifecycle states: `pending`, `active`, `succeeded`, `failed`, `missing`, `image-pull-failed`, `stopped` | Gateway runtime interface and Go tests |

The contract uses NDJSON as the canonical example and local fixture format. Live browser transport for v1 is WebTransport with a MoQ-compatible SimOps namespace/track envelope; `GET /api/simops/runs/{run_id}/events` is recovery/inspection only and is not the live telemetry stream.

Observed runtime-worker lifecycle is separate from telemetry stream health, artifact disposition, Redpanda status, Postgres status, and Iceberg write health. Docker sync maps container existence and inspected container state into the neutral state set. Kubernetes sync will use the same state set: Job `Complete` maps to `succeeded`, Job `Failed` maps to `failed`, Pod `Pending` maps to `pending`, Pod `Running` maps to `active`, Pod `Succeeded` maps to `succeeded`, Pod `Failed` maps to `failed`, `ErrImagePull`/`ImagePullBackOff` maps to `image-pull-failed`, and missing/deleted resources map to `missing` unless the run is already stopped.

Run inspection remains available when a runtime sync attempt fails; in that case the handler returns the stored run and worker records without fresh observed lifecycle updates.

Ordinary run-scoped worker containers are controlled by Gateway-Only Worker Ingest. Runtime proof may inspect worker container labels and environment keys, but evidence output must redact tokens and must fail if ordinary workers receive Redpanda, Postgres, Iceberg, Docker, Kubernetes, Workbench, or AWS credential-bearing environment variables.

| MoQ Namespace | Track | Payload |
| --- | --- | --- |
| `radiant/simops/{run_id}` | `lifecycle` | Run lifecycle and control-plane status updates |
| `radiant/simops/{run_id}` | `workers/{worker_id}/telemetry` | Worker telemetry envelope frames |
| `radiant/simops/{run_id}` | `workers/{worker_id}/quality` | Worker health, validation, and quality observations |
| `radiant/simops/{run_id}` | `artifacts` | Artifact and Iceberg commit references |

## Simulation Ops Persistence Interface

| Layer | Controlled Responsibility | Local Artifact |
| --- | --- | --- |
| Timescale/Postgres | Runs, workers, observed runtime-worker lifecycle, spool commands, idempotency keys, launch records, artifact refs, telemetry hypertable projection, consumer offsets, and Iceberg SQL-catalog metadata | `deploy/postgres-init/001_simops.sql` |
| Redpanda | Hot durable telemetry and lifecycle log keyed by run and worker | `deploy/slurm-gateway.compose.yml` |
| MinIO | S3-compatible object storage for local Parquet-backed Iceberg table data | `deploy/slurm-gateway.compose.yml` |
| Timescale writer | Redpanda consumer projecting telemetry frames into `simops_telemetry_frames` idempotently | `backend/slurm-gateway/cmd/simops-timescale-writer` |
| Iceberg-Go writer | Redpanda consumer appending telemetry frames to `simops.telemetry_frames` and updating artifact status | `backend/slurm-gateway/cmd/simops-iceberg-writer` |
| WebTransport gateway | Redpanda consumer routing lifecycle, telemetry, quality, and artifact tracks to actual WebTransport subscribers | `backend/slurm-gateway/cmd/simops-stream-gateway`, `backend/slurm-gateway/cmd/simops-webtransport-probe` |
| Workbench projection writer | Redpanda consumer projecting `scada.telemetry.v1`, `simops.results.v1`, and `digital-twin.state.v1` into Workbench read models | `backend/slurm-gateway/cmd/workbench-projection-writer` |
| Twin projector | Redpanda consumer producing imputed twin state and lineage from measured and simulated streams | `backend/slurm-gateway/cmd/twin-projector` |
| Workbench Iceberg writer | Redpanda consumer appending `scada.measured_frames`, `simops.simulated_results`, and `digital_twin.state_values` | `backend/slurm-gateway/cmd/workbench-iceberg-writer` |

## Simulator Workbench Backend Dataflow Interface

| Interface | Input | Output | Control |
| --- | --- | --- | --- |
| Value basis | Displayed value basis | `measured`, `imputed`, or `simulated` | `value-basis.v1` schema and contract check |
| Resident source declaration | Mixed public-safe source set | Declared measured tags for flux, temperature, pressure, actuator, electrical, and comms stand-ins | SCADA source schema and `workers/scada-standins` tests |
| Measured telemetry | Resident stand-in frame | `scada.telemetry.v1` measured frame | SCADA telemetry schema and contract check |
| Simulated result | Synthetic worker result batch | `simops.result.v1` values with `valueBasis=simulated` | Result schema, contract check, and backend tests |
| Digital twin state | Measured tags, simulated result state, imputed model state, and simulation references | `digital-twin.state.v1` values with lineage ids | Digital twin schema, backend tests, and dataflow smoke |
| Workbench state | Contract refs and panel summaries | `simulator-workbench.state.v1` state summary | Workbench schema and contract check |
| Workbench projection | Workbench state, measured frames, twin state, lineage records, selected queue context, and SimOps status context | Grouped measured/imputed/simulated values, selected lineage, queue-driven HPC status bay, and run orchestration subregions | TypeScript projection and render tests |

The user-facing frontend surface is now Status Workbench. The backend dataflow slice keeps read-only `/api/simulator-workbench/*` APIs for controlled route compatibility. The lower Status Workbench regions present queue, orchestration, and synthetic HPC status without exposing browser credentials for Redpanda, Timescale, Iceberg, Docker, or WebTransport internals.

## Infrastructure Interface

Docker, Terraform, Ansible, Slurm gateway compose, SimOps compose services, Prometheus, and local certificate helper files describe local-safe infrastructure intent. They are validated by static checks and optional tool-native checks when the relevant tools are installed.

## External Source Interface

External public-source links are controlled through fixture fields and shall use HTTPS URLs.
