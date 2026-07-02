# Objective Evidence Index

| Field | Value |
| --- | --- |
| Document ID | OEI-001 |
| Revision | 2.0 |
| Status | Draft for v2 review |
| Owner | Quality |
| Baseline | v2 candidate |

The evidence records below are synthetic and public-safe. They demonstrate traceability habits and reproducible engineering records.

| Evidence ID | Linked Run | Requirements | Artifacts | Limitation |
| --- | --- | --- | --- | --- |
| EP-TRN-001 | JOB-TRN-001 | SR-002, SW-001, SR-004 | `flux-profile.csv`, `transport-run.log`, `manifest.json` | Toy transport sweep only |
| EP-KDU-001 | JOB-THM-001 | SR-003, SR-004 | `thermal-margin.json`, `load-following.csv`, `thermal-run.log` | Lumped educational thermal model only |
| EP-FLT-001 | JOB-FLT-001 | SR-004, SW-003 | `telemetry-window.csv`, `anomaly-report.json`, `fleet-run.log` | Synthetic telemetry only |
| EP-HPC-404 | JOB-HPC-404 | SW-001, SW-002, SW-004 | `slurm-404.out`, `module-inventory.diff`, `triage-note.md` | Synthetic scheduler and module logs only |
| DOC-V2-001 | Documentation baseline | SR-005, SW-005 | `docs/quality/`, `docs/design/`, `docs/verification/`, `docs/release/` | Controlled documentation package only |
| REL-V2-001 | Release operation baseline | SR-006, SW-006 | `scripts/checkpoint-wip.sh`, `scripts/fold-v2-to-main.sh`, `scripts/checkpoint-v2.sh` | Local git release operation only |

## Generated Evidence

`bun run evidence:generate` creates `generated/evidence-index.json` from the controlled fixture set. Generated files are intentionally ignored by git so local evidence regeneration can be repeated without source churn.

## Artifact Hashing

The app fixtures use stable toy FNV-1a identifiers. The generation script uses SHA-256 prefixes for the local generated index. Neither hash set is a cryptographic guarantee for production use.

## Release Evidence

The v2 release package shall include completed release checklist, baseline record, approval record, test report, and dry-run output for checkpoint/fold/version scripts.
