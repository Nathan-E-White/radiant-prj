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

## Control Note

This log captures the demo's engineering-control story and release baseline evolution.
