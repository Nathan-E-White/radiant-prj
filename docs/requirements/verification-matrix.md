# Verification Matrix

| Requirement | Verification Method | Verification Artifact | Automated Check |
| --- | --- | --- | --- |
| SR-001 | Inspection | Source-linked fact cards and claim-boundary fields | `bun run validate:fixtures` |
| SR-002 | Test | `JOB-TRN-001`, `EP-TRN-001`, transport unit tests | `bun run test` |
| SR-003 | Analysis | `JOB-THM-001`, `EP-KDU-001`, thermal unit tests | `bun run test` |
| SR-004 | Configuration audit | Evidence matrix, artifact hashes, objective evidence index | `bun run validate:fixtures`, `bun run evidence:generate` |
| SW-001 | Test | Scheduler state fixtures and logs | `bun run test` |
| SW-002 | Test | `JOB-HPC-404`, `EP-HPC-404`, diagnosis unit tests | `bun run test` |
| SW-003 | Demonstration | React console tabs and controlled fixture rendering | `bun run typecheck`, `bun run build` |
| SW-004 | Configuration audit | Docker Compose, Terraform locals/outputs, Ansible templates, CI workflow | `bun run infra:check` |

## Acceptance Scenario

1. Start the app with `bun run dev`.
2. Open the Kaleidos Brief and confirm public facts include source links and limitations.
3. Run the synthetic readiness bundle.
4. Open the Compute Workbench and select `JOB-HPC-404`.
5. Confirm the failed job has logs, diagnosis, next action, and preventative control.
6. Open the Evidence Matrix and confirm requirements link to jobs, evidence packs, hashes, and deployment checks.
