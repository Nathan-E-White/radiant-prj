# Objective Evidence Index

| Field | Value |
| --- | --- |
| Document ID | OEI-001 |
| Revision | 3.0 |
| Status | Draft for v3.0 review |
| Owner | Quality |
| Baseline | v3.0 candidate |

The evidence records below are synthetic and public-safe. They demonstrate traceability habits and reproducible engineering records.

| Evidence ID | Linked Run | Requirements | Artifacts | Limitation |
| --- | --- | --- | --- | --- |
| EP-TRN-001 | JOB-TRN-001 | SR-002, SW-001, SR-004 | `flux-profile.csv`, `transport-run.log`, `manifest.json` | Toy transport sweep only |
| EP-KDU-001 | JOB-THM-001 | SR-003, SR-004 | `thermal-margin.json`, `load-following.csv`, `thermal-run.log` | Lumped educational thermal model only |
| EP-FLT-001 | JOB-FLT-001 | SR-004, SW-003 | `telemetry-window.csv`, `anomaly-report.json`, `fleet-run.log` | Synthetic telemetry only |
| EP-HPC-404 | JOB-HPC-404 | SW-001, SW-002, SW-004 | `slurm-404.out`, `module-inventory.diff`, `triage-note.md` | Synthetic scheduler and module logs only |
| DOC-V2-001 | Documentation baseline | SR-005, SW-005 | `docs/quality/`, `docs/design/`, `docs/verification/`, `docs/release/`, `docs/requirements/` | Controlled documentation package only |
| REL-TOOL-001 | Version-aware release tooling baseline | SR-006, SW-006 | `scripts/checkpoint-version.sh`, `scripts/fold-branch.sh`, `scripts/cleanup-version-hygiene.sh`, v2 wrappers | Local git release operation only |
| SLURM-GATEWAY-001 | Backend gateway baseline | SR-007, SW-007, SW-008, SW-009 | `backend/slurm-gateway/`, `deploy/slurm-gateway.Dockerfile`, `deploy/slurm-gateway.compose.yml`, `scripts/create-local-gateway-certs.sh` | Controlled mock-first gateway only |
| SIMOPS-BACKEND-001 | Simulation Ops backend slice | SR-008, SW-010, SW-011, SW-012 | `backend/slurm-gateway/internal/gateway/simops_*.go`, `backend/slurm-gateway/cmd/simops-stream-gateway/`, `backend/slurm-gateway/cmd/simops-webtransport-probe/`, `backend/slurm-gateway/cmd/simops-timescale-writer/`, `backend/slurm-gateway/cmd/simops-iceberg-writer/`, `deploy/postgres-init/001_simops.sql`, `deploy/slurm-gateway.compose.yml`, `workers/simops-generator/` | Redpanda-backed local data-plane slice with WebTransport live tracks, Timescale projection, Iceberg-Go append/readback, and Docker metadata/content smoke preflight |
| SIMOPS-DOCKER-SDK-001 | Issue #21 Docker SDK launcher slice | SW-017 | `backend/slurm-gateway/internal/simopsdocker/`, `backend/slurm-gateway/internal/gateway/simops_runtime_injection_test.go`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go`, `deploy/postgres-init/001_simops.sql`, `docs/design/simops-docker-sdk-adapter-research.md` | Fake Docker client and backend unit evidence only; real Docker/OrbStack smoke remains a local compatibility gate |
| SIMOPS-SYNCRUN-001 | Issue #22 runtime lifecycle sync slice | SW-018 | `backend/slurm-gateway/internal/gateway/simops_runtime_sync_test.go`, `backend/slurm-gateway/internal/gateway/simops_adapters.go`, `backend/slurm-gateway/internal/gateway/simops_store.go`, `backend/slurm-gateway/internal/simopsdocker/spooler.go`, `deploy/postgres-init/001_simops.sql`, `docs/design/simops-syncrun-lifecycle-research.md` | Runtime-resource observation only; no reconciliation loop, frontend redesign, or data-plane health aggregation |
| SIMOPS-KIND-E2E-001 | Issue #25 Kind/client-go Kubernetes runtime proof | SW-021 | `scripts/simops-kind-smoke.sh`, `scripts/check-simops-kind-smoke.mjs`, `scripts/simops-smoke-json.mjs`, `backend/slurm-gateway/internal/simopskubernetes/`, `docs/design/issue-25-kind-client-go-simops-e2e.md` | Local Kind/OrbStack compatibility proof only; no production hardening, operator, workflow engine, or full lakehouse performance claim |
| WORKBENCH-DATAFLOW-001 | Simulator Workbench backend dataflow slice | SW-013, SW-014, SW-015 | `docs/design/simulator-workbench-backend-dataflow-slice.md`, `backend/slurm-gateway/internal/gateway/*workbench*.go`, `backend/slurm-gateway/cmd/workbench-projection-writer/`, `backend/slurm-gateway/cmd/twin-projector/`, `backend/slurm-gateway/cmd/workbench-iceberg-writer/`, `workers/scada-standins/`, `workers/simops-generator/`, `deploy/postgres-init/001_simops.sql`, `scripts/simulator-workbench-dataflow-smoke.sh` | Backend-only local proof; public-safe stand-ins and synthetic simulated result state only |
| DOP-001 | Docker/OrbStack storage policy | DEV-HYGIENE-001 | `docs/design/docker-orbstack-storage-policy.md`, `scripts/hygiene-size.mjs`, `scripts/docker-prune-hygiene.sh`, `scripts/check-docker-storage-policy.mjs` | Local Docker reporting and scoped cleanup guard; no live prune execution |

## Generated Evidence

`bun run evidence:generate` creates `generated/evidence-index.json` from the controlled fixture set, including compute evidence summaries and controlled process evidence summaries. Generated files are intentionally ignored by git so local evidence regeneration can be repeated without source churn.

## Artifact Hashing

The app fixtures use stable toy FNV-1a identifiers. The generation script uses SHA-256 prefixes for the local generated index. Neither hash set is a cryptographic guarantee for production use.

## Release Evidence

The v2 release package is recorded by the existing `v2.0.0` tag. The v3.0 release package shall include completed release checklist, baseline record, approval record, test report, Go backend test output, Simulation Ops contract output, Workbench dataflow smoke output, infrastructure check output, and dry-run output for generic checkpoint/fold/version scripts.
