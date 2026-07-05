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
| SIMOPS-BACKEND-001 | Simulation Ops backend slice | SR-008, SW-010, SW-011, SW-012 | `backend/slurm-gateway/internal/gateway/simops_*.go`, `backend/slurm-gateway/cmd/simops-stream-gateway/`, `backend/slurm-gateway/cmd/simops-iceberg-writer/`, `deploy/postgres-init/001_simops.sql`, `deploy/slurm-gateway.compose.yml`, `workers/simops-generator/` | Contract-first local backend slice; production MoQ, Redpanda, and Iceberg client modules remain integration seams |

## Generated Evidence

`bun run evidence:generate` creates `generated/evidence-index.json` from the controlled fixture set, including compute evidence summaries and controlled process evidence summaries. Generated files are intentionally ignored by git so local evidence regeneration can be repeated without source churn.

## Artifact Hashing

The app fixtures use stable toy FNV-1a identifiers. The generation script uses SHA-256 prefixes for the local generated index. Neither hash set is a cryptographic guarantee for production use.

## Release Evidence

The v2 release package is recorded by the existing `v2.0.0` tag. The v3.0 release package shall include completed release checklist, baseline record, approval record, test report, Go backend test output, Simulation Ops contract output, infrastructure check output, and dry-run output for generic checkpoint/fold/version scripts.
