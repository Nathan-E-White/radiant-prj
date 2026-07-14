# Verification Plan

| Field | Value |
| --- | --- |
| Document ID | VVP-001 |
| Revision | 3.0 |
| Status | Draft for v3.0 review |
| Owner | Quality |
| Baseline | v3.0 candidate |

## Purpose

This plan defines verification activities for the v3.0 baseline.

## Verification Scope

The scope includes application type checking, frontend/domain unit tests, Go backend gateway tests, fixture validation, generated evidence, infrastructure artifact checks, quality documentation checks, Simulation Ops contract checks, Status Workbench contract checks, resident SCADA stand-in tests, SimOps generator tests, Status Workbench backend dataflow smoke, and production build.

## Verification Activities

| Activity | Command | Evidence |
| --- | --- | --- |
| Type checking | `bun run typecheck` | Command output |
| Unit tests | `bun run test` | Vitest output |
| Backend gateway tests | `bun run backend:test` | Go test output |
| Fixture validation | `bun run validate:fixtures` | Validation output |
| Evidence generation | `bun run evidence:generate` | Generated index under `generated/` |
| Infrastructure checks | `bun run infra:check` | Static and optional native check output |
| Quality documentation checks | `bun run quality:check` | Documentation check output |
| Simulation Ops contract checks | `bun run simops:contract:check` | Contract validation output |
| SimOps smoke JSON helper tests | `bun run simops:smoke:json:test` | Node test output for runtime proof parsing and redaction |
| Status Workbench contract checks | `bun run simulator-workbench:contract:check` | Contract validation output |
| Resident SCADA stand-in tests | `bun run scada:standins:test` | Rust test output |
| Reactor Telemetry Worker Set contract | `bun run backend:test`, `bun run backend:snapshot:postgres:test` | Caps, idempotency, restart recovery, source/reactor credential binding, retention, and cleanup tests |
| Reactor Telemetry Docker tracer bullet | Build `deploy/scada-standins.Dockerfile`, then run `go test -tags dockerintegration ./internal/gateway -run '^TestDockerReactorTelemetryWorkerSetPublishesMeasuredStateAndCleansUp$'` with `DOCKER_HOST` and `REACTOR_TELEMETRY_TEST_IMAGE` set | One three-worker set, six reactor-scoped Measured State tags, gateway-only credentials, and zero labeled containers after removal |
| Configured Data Flush | `bun run configured-data-flush:check`, `bun run backend:snapshot:postgres:test` | Dry-run completeness, stale-plan and active-resource rejection, protected-resource preservation, transaction rollback, coherent next generation, and subsequent Run startup |
| Artifact Forge | `bun run artifact-forge:check`, `bun run backend:test`, `bun run backend:snapshot:postgres:test` | Distinct local-job/Run identities, durable retry association, simulated-result artifact and Lineage eligibility, idempotent versioned outcome, and explicit no-outcome decisions |
| Live Workbench read boundary | `bun run test`, `bun run test:e2e -- tests/e2e/workbench-live-read.spec.ts`, `bun run backend:test` | Atomic generation acceptance, Value Basis separation, explicit stale/recovery/error states, development-only whole-Snapshot fixture fallback, and credential-free browser reads |
| SimOps generator tests | `bun run simops:generator:test` | Rust test output |
| Docker/OrbStack SimOps runtime smoke | `bun run simops:smoke:docker-orbstack` (`SIMOPS_SMOKE_BUILD=always` for forced image rebuild) | Docker/OrbStack launch, gateway-ingest, lifecycle sync, zero-TTL success cleanup, failed-worker retention/log evidence, and smoke-forced cleanup output |
| Kind/client-go SimOps runtime smoke | `bun run simops:smoke:kind -- --timeout 300 --build auto` | Kind context, namespace, Job names, run IDs, gateway-only worker inputs, frame ingest, success/image-pull lifecycle, TTL, retention, and cleanup output |
| OpenTofu substrate preflight | `bun run simops:tofu:preflight` | Fmt/init/validate and `6 to add, 0 to change, 0 to destroy` no-mutation plan with adapter configuration evidence |
| Runtime closeout documentation | `bun run simops:runtime:closeout:check` | Implemented runtime lanes, credential/cleanup boundaries, commands, and deferred items remain explicit |
| Status Workbench backend dataflow smoke | `bun run simulator-workbench:dataflow:smoke` | Docker smoke output for Redpanda, Postgres, Iceberg, and read APIs |
| Production build | `bun run build` | Build output |
| Full local CI | `bun run ci` | Combined command output |
| Docker/OrbStack storage policy | `bun run docker:storage:check`, `bun run hygiene:size:check`, `bun run docker:prune:check` | Read-only report and scoped cleanup guard output |

## Acceptance Criteria

- Required commands complete successfully.
- Requirements and evidence references remain traceable.
- Generated evidence can be recreated from controlled fixtures.
- Slurm gateway handlers reject missing or unauthorized certificates and validate job requests before spooling.
- Simulation Ops contract examples validate against the documented envelope, payload, manifest, and summary schemas.
- Docker/OrbStack runtime proof launches workers through the SimOps API, verifies gateway-only worker ingest, observes runtime lifecycle, removes succeeded workers through the configured zero-TTL cleanup policy, retains failed-worker evidence before forced cleanup, and removes labeled failed-worker containers after smoke cleanup; fresh-image verification sets `SIMOPS_SMOKE_BUILD=always`.
- Kind runtime proof launches Jobs through the same SimOps API, verifies labels and Gateway-Only Worker Ingest, observes successful and image-pull-failed lifecycle states, records frames and runtime identifiers, and cleans the cluster after evidence capture.
- OpenTofu preflight plans only static namespace, service-account, RBAC, and ConfigMap substrate and never applies per-run Jobs.
- CRD/operator, Argo, Tekton, host-facing Redpanda listeners, and production hardening remain deferred.
- Status Workbench backend dataflow proves measured SCADA frames, SimOps telemetry, simulated results, and imputed twin state through Redpanda, Postgres, Iceberg, and read-only APIs.
- Reactor Telemetry runtime proof creates no more than three Resident Source workers for one reactor, projects six reactor-scoped `valueBasis=measured` tags, exposes no ingest credential to the browser response, and removes every set-labeled container.
- Configured Data Flush defaults to a readable plan, requires its exact reviewed identifier for mutation, blocks active runtime resources, preserves protected platform resources, and atomically exposes either the prior or next coherent Workbench generation.
- Artifact Forge launches only from an explicit completed-job intent, keeps the local Simulation Job distinct from its SimOps Run, and applies one idempotent outcome only from a committed allowlisted simulated-result artifact with complete association Lineage; telemetry and every incomplete or failed path remain visible and reward-free.
- Release scripts pass dry-run checks.
- No blocking findings remain open in release records.
