# Software Requirements

| Field | Value |
| --- | --- |
| Document ID | REQ-002 |
| Revision | 2.0 |
| Status | Draft for v2 review |
| Owner | Software |
| Baseline | v2 candidate |

This document defines software and deployment requirements for the interview demonstration. The implementation is intentionally small, deterministic, locally reproducible, and controlled through the v2 quality documentation package.

| ID | Requirement | Rationale | Verification | Status |
| --- | --- | --- | --- | --- |
| SW-001 | The scheduler emulator shall expose queued, running, completed, failed, and held states with traceable logs. | The role emphasizes workload scheduling, job failure analysis, and cross-disciplinary triage. | Test | Verified |
| SW-002 | The diagnostic engine shall map environment failures to a root cause, next action, and preventative deployment control. | The demo should show Linux/HPC debugging and operational judgment. | Test | Verified |
| SW-003 | The frontend shall display source-linked public facts, compute jobs, evidence packs, and deployment checks from controlled fixtures. | The UI should be traceable to fixture records rather than freehand content. | Demonstration | Verified |
| SW-004 | The DevOps layer shall define dry-run-safe local container, Terraform, and Ansible artifacts for hybrid compute readiness. | The role calls for Linux, file systems, process management, networking, and infrastructure automation. | Configuration audit | Verified |
| SW-005 | The documentation quality check shall verify required v2 controlled document structure and metadata. | Release readiness needs enforceable documentation completeness checks. | Test | Draft |
| SW-006 | The WIP checkpoint, fold-back, and v2 checkpoint scripts shall support dry-run execution and conservative exclusions. | Release operations should be previewable before mutating git state. | Configuration audit | Draft |

## Interface Summary

- `bun run dev` starts the local console.
- `bun run test` runs deterministic solver and traceability tests.
- `bun run validate:fixtures` validates public facts, jobs, requirements, evidence packs, and deployment checks.
- `bun run evidence:generate` creates a generated evidence index in `generated/evidence-index.json`.
- `bun run infra:check` statically checks Docker, Terraform, and Ansible artifacts, and runs optional tool-native checks when those CLIs are present.
- `bun run quality:check` verifies the v2 controlled documentation package.

## Controlled Inputs

- `src/data/readiness-fixtures.json` is the source of public facts, synthetic compute jobs, requirements, evidence packs, milestones, and deployment checks.
- `infra/terraform/` declares infrastructure intent only.
- `infra/ansible/` targets a local dry-run root under `/tmp/kaleidos-readiness`.
- `docs/` contains controlled requirements, design, quality, verification, and release records.
