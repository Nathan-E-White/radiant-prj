# Verification Matrix

| Field | Value |
| --- | --- |
| Document ID | VVM-001 |
| Revision | 3.0 |
| Status | Draft for v3.0 review |
| Owner | Quality |
| Baseline | v3.0 candidate |

| Requirement | Verification Method | Verification Artifact | Automated Check | Evidence Record |
| --- | --- | --- | --- | --- |
| SR-001 | Inspection | Source-linked fact cards and claim-boundary fields | `bun run validate:fixtures` | REQ-001, OEI-001 |
| SR-002 | Test | `JOB-TRN-001`, `EP-TRN-001`, transport unit tests | `bun run test` | VVR-001 |
| SR-003 | Analysis | `JOB-THM-001`, `EP-KDU-001`, thermal unit tests | `bun run test` | VVR-001 |
| SR-004 | Configuration audit | Evidence matrix, artifact hashes, objective evidence index | `bun run validate:fixtures`, `bun run evidence:generate` | OEI-001 |
| SR-005 | Configuration audit | `DOC-V2-001` controlled documentation evidence record | `bun run quality:check`, `bun run validate:fixtures` | QP-011, OEI-001 |
| SR-006 | Configuration audit | `REL-TOOL-001` version-aware release tooling evidence record | Script dry-runs, `bun run validate:fixtures` | REL-001, OEI-001 |
| SR-007 | Configuration audit | `SLURM-GATEWAY-001` backend gateway evidence record | `bun run backend:test`, `bun run infra:check` | OEI-001 |
| SR-008 | Configuration audit | `SIMOPS-BACKEND-001` Simulation Ops backend evidence record | `bun run backend:test`, `bun run infra:check`, `bun run simops:contract:check` | OEI-001 |
| SW-001 | Test | Scheduler state fixtures and logs | `bun run test` | VVR-001 |
| SW-002 | Test | `JOB-HPC-404`, `EP-HPC-404`, diagnosis unit tests | `bun run test` | VVR-001 |
| SW-003 | Demonstration | React console tabs and controlled fixture rendering | `bun run typecheck`, `bun run build` | VVR-001 |
| SW-004 | Configuration audit | Docker Compose, Terraform locals/outputs, Ansible templates, CI workflow | `bun run infra:check` | VVR-001 |
| SW-005 | Test | Documentation quality checker and controlled traceability records | `bun run quality:check` | VVR-001 |
| SW-006 | Configuration audit | Generic release operation scripts with v2 compatibility wrappers | Script dry-runs | REL-001 |
| SW-007 | Test | Handler tests for missing, empty, and unauthorized client certificate identities | `bun run backend:test` | SLURM-GATEWAY-001 |
| SW-008 | Test | Health, readiness, submit, status, and metrics handlers | `bun run backend:test` | SLURM-GATEWAY-001 |
| SW-009 | Test | Mock spooler and `sbatch` adapter tests, including output parsing and timeout behavior | `bun run backend:test` | SLURM-GATEWAY-001 |
| SW-010 | Test | SimOps run creation, idempotency, stop, and token-gated ingest handler tests | `bun run backend:test` | SIMOPS-BACKEND-001 |
| SW-011 | Test | WebTransport subscription metadata, Redpanda-backed lifecycle/telemetry/quality/artifact track routing tests, and the containerized WebTransport smoke probe | `bun run backend:test`, `bun run simops:contract:check`, `bun run simops:smoke:local` | SIMOPS-BACKEND-001 |
| SW-012 | Configuration audit | Compose, Timescale init SQL, Redpanda, MinIO, WebTransport gateway, Timescale writer, Iceberg-Go writer, and Docker metadata/content preflight artifacts | `bun run infra:check`, `bun run simops:smoke:local` | SIMOPS-BACKEND-001 |
| SW-013 | Test | Workbench value-basis validation, separate topics, Postgres projections, Iceberg tables, and read-only APIs | `bun run backend:test`, `bun run simops:contract:check`, `bun run simulator-workbench:contract:check`, `bun run simulator-workbench:dataflow:smoke` | WORKBENCH-DATAFLOW-001 |
| SW-014 | Test | Resident source declaration, measured-only SCADA frames, token-gated ingest, `scada_measured_frames`, and `scada.measured_frames` | `bun run scada:standins:test`, `bun run backend:test`, `bun run simulator-workbench:dataflow:smoke` | WORKBENCH-DATAFLOW-001 |
| SW-015 | Test | `simops.result.v1` contract, simulated-only result ingest, twin imputed state, and lineage materialization | `bun run simops:generator:test`, `bun run backend:test`, `bun run simulator-workbench:dataflow:smoke` | WORKBENCH-DATAFLOW-001 |
| SW-016 | Test | Simulation Health stories and fixtures render from shared model contract with 4-card coverage scenarios and fixture-driven health ticking | `bun run typecheck`, `bun run test`, `bun run build-storybook` | VVR-001 |

## Acceptance Scenario

1. Start the app with `bun run dev`.
2. Open the Kaleidos Brief and confirm public facts include source links and limitations.
3. Run the synthetic readiness bundle.
4. Open the Compute Workbench and select `JOB-HPC-404`.
5. Confirm the failed job has logs, diagnosis, next action, and preventative control.
6. Open the Evidence Matrix and confirm requirements link to jobs, evidence packs, hashes, and deployment checks.
7. Run `bun run backend:test` and confirm the Slurm gateway rejects unauthorized requests and records mock jobs.
8. Create a Simulation Ops run through `POST /api/simops/runs` and confirm the response contains WebTransport subscription metadata, worker tracks, and artifact references.
9. Run `bun run quality:check` and confirm the v3.0 controlled document and traceability package is complete.
10. Run `bun run simulator-workbench:dataflow:smoke` and confirm measured, telemetry, simulated, and imputed units reach Redpanda, Postgres, Iceberg, and the read-only Workbench APIs.
