# Change Log

| Change ID | Date | Summary | Affected Records | Verification |
| --- | --- | --- | --- | --- |
| CHG-001 | 2026-06-30 | Initial controlled fixture set for public facts, synthetic jobs, requirements, evidence packs, and deployment checks. | SR-001 through SR-004, SW-001 through SW-004 | `bun run validate:fixtures` |
| CHG-002 | 2026-06-30 | Added deterministic transport, thermal, fleet anomaly, and HPC diagnosis tests. | SR-002, SR-003, SW-001, SW-002 | `bun run test` |
| CHG-003 | 2026-06-30 | Added dry-run Docker, Terraform, Ansible, and CI artifacts. | SW-004 | `bun run infra:check` |
| CHG-004 | 2026-06-30 | Added React console with Kaleidos Brief, Compute Workbench, and Evidence Matrix tabs. | SW-003 | `bun run build` |

## Control Note

This log captures the demo's engineering-control story. It is not a regulatory, procurement, or production configuration-control record.
