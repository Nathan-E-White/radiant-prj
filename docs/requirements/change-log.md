# Change Log

| Field | Value |
| --- | --- |
| Document ID | CLOG-001 |
| Revision | 2.1 |
| Status | Draft for v2.1 review |
| Owner | Quality |
| Baseline | v2.1 candidate |

| Change ID | Date | Summary | Affected Records | Verification |
| --- | --- | --- | --- | --- |
| CHG-001 | 2026-06-30 | Initial controlled fixture set for public facts, synthetic jobs, requirements, evidence packs, and deployment checks. | SR-001 through SR-004, SW-001 through SW-004 | `bun run validate:fixtures` |
| CHG-002 | 2026-06-30 | Added deterministic transport, thermal, fleet anomaly, and HPC diagnosis tests. | SR-002, SR-003, SW-001, SW-002 | `bun run test` |
| CHG-003 | 2026-06-30 | Added dry-run Docker, Terraform, Ansible, and CI artifacts. | SW-004 | `bun run infra:check` |
| CHG-004 | 2026-06-30 | Added React console with Kaleidos Brief, Compute Workbench, and Evidence Matrix tabs. | SW-003 | `bun run build` |
| CHG-005 | 2026-07-01 | Added v2 controlled quality, design, verification, and release documentation package. | SR-005, SW-005, QP-001 through QP-011, SDD-001, ICD-001, VVP-001, REL-001 through REL-005 | `bun run quality:check` |
| CHG-006 | 2026-07-01 | Added WIP checkpoint, fold-back, and v2 version checkpoint script plan and implementation. | SR-006, SW-006, REL-001, REL-002 | Script dry-runs |
| CHG-007 | 2026-07-02 | Closed the existing v2.0.0 tag record, added fixture-backed controlled evidence for SR-005/SR-006 and SW-005/SW-006, and generalized release tooling with v2 compatibility wrappers. | DOC-V2-001, REL-TOOL-001, SR-005, SR-006, SW-005, SW-006 | `bun run ci`, script dry-runs |

## Control Note

This log captures the demo's engineering-control story and release baseline evolution.
