# Verification Matrix

| Field | Value |
| --- | --- |
| Document ID | VVM-001 |
| Revision | 2.1 |
| Status | Draft for v2.1 review |
| Owner | Quality |
| Baseline | v2.1 candidate |

| Requirement | Verification Method | Verification Artifact | Automated Check | Evidence Record |
| --- | --- | --- | --- | --- |
| SR-001 | Inspection | Source-linked fact cards and claim-boundary fields | `bun run validate:fixtures` | REQ-001, OEI-001 |
| SR-002 | Test | `JOB-TRN-001`, `EP-TRN-001`, transport unit tests | `bun run test` | VVR-001 |
| SR-003 | Analysis | `JOB-THM-001`, `EP-KDU-001`, thermal unit tests | `bun run test` | VVR-001 |
| SR-004 | Configuration audit | Evidence matrix, artifact hashes, objective evidence index | `bun run validate:fixtures`, `bun run evidence:generate` | OEI-001 |
| SR-005 | Configuration audit | `DOC-V2-001` controlled documentation evidence record | `bun run quality:check`, `bun run validate:fixtures` | QP-011, OEI-001 |
| SR-006 | Configuration audit | `REL-TOOL-001` version-aware release tooling evidence record | Script dry-runs, `bun run validate:fixtures` | REL-001, OEI-001 |
| SW-001 | Test | Scheduler state fixtures and logs | `bun run test` | VVR-001 |
| SW-002 | Test | `JOB-HPC-404`, `EP-HPC-404`, diagnosis unit tests | `bun run test` | VVR-001 |
| SW-003 | Demonstration | React console tabs and controlled fixture rendering | `bun run typecheck`, `bun run build` | VVR-001 |
| SW-004 | Configuration audit | Docker Compose, Terraform locals/outputs, Ansible templates, CI workflow | `bun run infra:check` | VVR-001 |
| SW-005 | Test | Documentation quality checker and controlled traceability records | `bun run quality:check` | VVR-001 |
| SW-006 | Configuration audit | Generic release operation scripts with v2 compatibility wrappers | Script dry-runs | REL-001 |

## Acceptance Scenario

1. Start the app with `bun run dev`.
2. Open the Kaleidos Brief and confirm public facts include source links and limitations.
3. Run the synthetic readiness bundle.
4. Open the Compute Workbench and select `JOB-HPC-404`.
5. Confirm the failed job has logs, diagnosis, next action, and preventative control.
6. Open the Evidence Matrix and confirm requirements link to jobs, evidence packs, hashes, and deployment checks.
7. Run `bun run quality:check` and confirm the v2.1 controlled document and traceability package is complete.
