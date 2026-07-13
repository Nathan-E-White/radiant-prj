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
| SW-003 | Demonstration | React console pages and controlled fixture rendering | `bun run typecheck`, `bun run build` | VVR-001 |
| SW-004 | Configuration audit | Docker Compose, Terraform locals/outputs, Ansible templates, CI workflow | `bun run infra:check` | VVR-001 |
| SW-005 | Test | Documentation quality checker and controlled traceability records | `bun run quality:check` | VVR-001 |
| SW-006 | Configuration audit | Generic release operation scripts with v2 compatibility wrappers | Script dry-runs | REL-001 |
| SW-007 | Test | Handler tests for missing, empty, and unauthorized client certificate identities | `bun run backend:test` | SLURM-GATEWAY-001 |
| SW-008 | Test | Health, readiness, submit, status, and metrics handlers | `bun run backend:test` | SLURM-GATEWAY-001 |
| SW-009 | Test | Mock spooler and `sbatch` adapter tests, including output parsing and timeout behavior | `bun run backend:test` | SLURM-GATEWAY-001 |
| SW-010 | Test | SimOps run creation, idempotency, stop, and token-gated ingest handler tests | `bun run backend:test` | SIMOPS-BACKEND-001 |
| SW-011 | Test | WebTransport subscription metadata, Redpanda-backed lifecycle/telemetry/quality/artifact track routing tests, and the containerized WebTransport smoke probe | `bun run backend:test`, `bun run simops:contract:check`, `bun run simops:smoke:local` | SIMOPS-BACKEND-001 |
| SW-012 | Configuration audit | Compose, Timescale init SQL, Redpanda, MinIO, WebTransport gateway, Timescale writer, Iceberg-Go writer, and Docker metadata/content preflight artifacts | `bun run infra:check`, `bun run simops:smoke:local` | SIMOPS-BACKEND-001 |
| SW-013 | Test | Status Workbench value-basis validation, separate topics, Postgres projections, Iceberg tables, and read-only APIs | `bun run backend:test`, `bun run simops:contract:check`, `bun run simulator-workbench:contract:check`, `bun run simulator-workbench:dataflow:smoke` | WORKBENCH-DATAFLOW-001 |
| SW-014 | Test | Resident source declaration, measured-only SCADA frames, token-gated ingest, `scada_measured_frames`, and `scada.measured_frames` | `bun run scada:standins:test`, `bun run backend:test`, `bun run simulator-workbench:dataflow:smoke` | WORKBENCH-DATAFLOW-001 |
| SW-015 | Test | `simops.result.v1` contract, simulated-only result ingest, twin imputed state, and lineage materialization | `bun run simops:generator:test`, `bun run backend:test`, `bun run simulator-workbench:dataflow:smoke` | WORKBENCH-DATAFLOW-001 |
| SW-016 | Test | Status Workbench render test covers the three-page IA, preserved value-basis region, absorbed compute queue, SimOps orchestration subregion, and four-panel HPC status bay | `bun run typecheck`, `bun run test`, `bun run build-storybook` | VVR-001 |
| SW-017 | Test | Docker SDK SimOps adapter fake-client tests for profile consumption, create/start construction, label/env/network propagation, structured metadata/errors, and run/worker-scoped stop targeting | `go test ./internal/gateway ./internal/simopsdocker -run 'TestDefaultSimopsController|TestSpooler'`, `bun run backend:test`, `bun run backend:deps:check`, `bun run ci` | SIMOPS-DOCKER-SDK-001 |
| SW-018 | Test | SyncRun adapter contract, runtime-neutral observed worker states, Docker container-state mapping, missing-resource behavior, and telemetry/artifact/data-plane boundary tests | `go test ./internal/gateway ./internal/simopsdocker -run 'TestSimopsControllerSyncs|TestWorkerTelemetryDoesNotOverwrite|TestDataPlaneAndArtifactUpdatesDoNotMutate|TestSpoolerSyncRunProfiles'`, `bun run backend:test`, `bun run backend:deps:check`, `bun run ci` | SIMOPS-SYNCRUN-001 |
| SW-019 | Demonstration | Docker/OrbStack SimOps Runtime Proof for API-driven worker launch, gateway-only worker ingest, observed lifecycle sync, zero-TTL success cleanup, failed-run retention, and smoke-forced cleanup | `bun run simops:smoke:json:test`, `bun run simops:smoke:docker-orbstack` (`SIMOPS_SMOKE_BUILD=always` for forced image rebuild), `bun run infra:check`, `bun run ci` | SIMOPS-DOCKER-ORBSTACK-E2E-001 |
| DEV-HYGIENE-001 | Configuration audit | Docker/OrbStack storage policy, read-only size reporting, scoped cleanup guard, and protected-volume confirmation | `bun run docker:storage:check`, `bun run hygiene:size:check`, `bun run docker:prune:check` | DOP-001 |
| SW-020 | Test | client-go fake-client tests cover Kubernetes Job labels, gateway-only inputs, namespace, service account, TTL, create/delete errors, and Job/Pod lifecycle mapping | `go test ./internal/simopskubernetes`, `bun run backend:test`, `bun run backend:deps:check`, `bun run ci` | #24 |
| SW-021 | Demonstration | Kind/OrbStack proof covers API-driven Kubernetes Job launch, required labels, gateway-only worker inputs, frame ingest, runtime lifecycle sync, TTL, failure retention, and forced cleanup | `bun run simops:smoke:kind:check`, `bun run simops:smoke:json:test`, elevated `bun run simops:smoke:kind -- --timeout 300 --build auto` | SIMOPS-KIND-E2E-001 |
| SW-022 | Configuration audit | OpenTofu module and no-mutation preflight cover namespace, gateway/worker service accounts, scoped Job/Pod RBAC, runtime adapter ConfigMap values, and the explicit absence of per-run Jobs | `bun run simops:tofu:check`, `bun run simops:tofu:preflight` | SIMOPS-TOFU-SUBSTRATE-001 |
| SW-023 | Review and demonstration | Consolidated runtime docs match implemented RunConnectionProfile, Docker SDK, SyncRun, client-go/Kind, OpenTofu, credential and cleanup boundaries; final local commands cover both runtime smokes and no-mutation substrate plan | `bun run simops:runtime:closeout:check`, `bun run backend:test`, elevated `bun run simops:smoke:docker-orbstack`, elevated `bun run simops:smoke:kind -- --timeout 300 --build auto`, `bun run simops:tofu:preflight`, `bun run ci`, `bun run build` | SIMOPS-RUNTIME-CLOSEOUT-001 |

## Acceptance Scenario

1. Start the app with `bun run dev`.
2. Open Welcome and confirm public facts include source links and limitations.
3. Run the synthetic readiness bundle.
4. Open Status Workbench and select `JOB-HPC-404` in the queue rail.
5. Confirm the selected job drives the HPC status bay, diagnosis terminal, SimOps orchestration region, and evidence-handoff messaging.
6. Open Evidence and confirm requirements link to jobs, evidence packs, hashes, and deployment checks.
7. Run `bun run backend:test` and confirm the Slurm gateway rejects unauthorized requests and records mock jobs.
8. Create a Simulation Ops run through `POST /api/simops/runs` and confirm the response contains WebTransport subscription metadata, worker tracks, and artifact references.
9. Run `bun run quality:check` and confirm the v3.0 controlled document and traceability package is complete.
10. Run `bun run simulator-workbench:dataflow:smoke` and confirm measured, telemetry, simulated, and imputed units reach Redpanda, Postgres, Iceberg, and the read-only Workbench APIs.
11. Run the Docker SDK SimOps adapter unit slice and confirm worker launch uses run connection profiles without Docker CLI shell-out.
12. Run the SyncRun lifecycle unit slice and confirm runtime observations remain separate from telemetry, artifacts, and data-plane health.
13. Run `bun run simops:smoke:docker-orbstack` and confirm the Docker/OrbStack runtime proof reports API launch, gateway-only ingest, observed Docker lifecycle, zero-TTL success cleanup, failed-run retention with logs, and forced cleanup. Use `SIMOPS_SMOKE_BUILD=always` when the evidence must include a fresh image rebuild.
