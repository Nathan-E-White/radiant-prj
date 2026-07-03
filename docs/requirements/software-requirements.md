# Software Requirements

| Field | Value |
| --- | --- |
| Document ID | REQ-002 |
| Revision | 3.0 |
| Status | Draft for v3.0 review |
| Owner | Software |
| Baseline | v3.0 candidate |

This document defines software and deployment requirements for the interview demonstration. The implementation is intentionally small, deterministic, locally reproducible, and controlled through the v3.0 quality documentation, backend gateway, and traceability package.

| ID | Requirement | Rationale | Verification | Status |
| --- | --- | --- | --- | --- |
| SW-001 | The scheduler emulator shall expose queued, running, completed, failed, and held states with traceable logs. | The role emphasizes workload scheduling, job failure analysis, and cross-disciplinary triage. | Test | Verified |
| SW-002 | The diagnostic engine shall map environment failures to a root cause, next action, and preventative deployment control. | The demo should show Linux/HPC debugging and operational judgment. | Test | Verified |
| SW-003 | The frontend shall display source-linked public facts, compute jobs, evidence packs, and deployment checks from controlled fixtures. | The UI should be traceable to fixture records rather than freehand content. | Demonstration | Verified |
| SW-004 | The DevOps layer shall define dry-run-safe local container, Terraform, and Ansible artifacts for hybrid compute readiness. | The role calls for Linux, file systems, process management, networking, and infrastructure automation. | Configuration audit | Verified |
| SW-005 | The documentation quality check shall verify required controlled document structure, metadata, and v2.1 traceability content. | Release readiness needs enforceable documentation completeness checks. | Test | Verified |
| SW-006 | The release operation scripts shall support version-aware dry-run execution, signed checkpoints, conservative exclusions, and v2 compatibility wrappers. | Release operations should be previewable before mutating git state and should not depend on version-specific script copies. | Configuration audit | Verified |
| SW-007 | The Slurm gateway shall require authorized mTLS client identity for submit and status handlers. | Backend calls should have a clear trust boundary and no browser-held private keys. | Test | Verified |
| SW-008 | The Slurm gateway shall expose health, readiness, submit, job-status, and Prometheus-format metrics handlers. | Operations need a minimal handler surface before other backend processes are added. | Test | Verified |
| SW-009 | The Slurm gateway shall support mock mode by default and opt-in `sbatch` mode through validated configuration. | The demo must remain deterministic while preserving a real integration seam. | Test | Verified |

## Interface Summary

- `bun run dev` starts the local console.
- `bun run test` runs deterministic solver and traceability tests.
- `bun run validate:fixtures` validates public facts, jobs, requirements, evidence packs, and deployment checks.
- `bun run evidence:generate` creates a generated evidence index in `generated/evidence-index.json`.
- `bun run infra:check` statically checks Docker, Terraform, and Ansible artifacts, and runs optional tool-native checks when those CLIs are present.
- `bun run quality:check` verifies the v3.0 controlled documentation and traceability package.
- `bun run backend:test` runs the Go Slurm gateway handler and spooler tests.

## Controlled Inputs

- `src/data/readiness-fixtures.json` is the source of public facts, synthetic compute jobs, requirements, evidence packs, milestones, and deployment checks.
- `infra/terraform/` declares infrastructure intent only.
- `infra/ansible/` targets a local dry-run root under `/tmp/kaleidos-readiness`.
- `backend/slurm-gateway/` contains the mock-first Go Slurm gateway.
- `docs/` contains controlled requirements, design, quality, verification, and release records.
