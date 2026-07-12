# Issue 23 Docker/OrbStack SimOps E2E Runtime Research

| Field | Value |
| --- | --- |
| Document ID | SIMOPS-DOCKER-ORBSTACK-E2E-RESEARCH-001 |
| Revision | 0.1 |
| Status | Research note |
| Owner | Software |
| Scope | Issue #23 research baseline; Docker/OrbStack SimOps runtime proof only |

## Purpose

This note records primary-source research for issue #23, "V3: Prove Docker/OrbStack SimOps E2E runtime path." The requested proof is deliberately narrower than full lakehouse validation: it must launch a SimOps run through the normal API path, start ordinary workers on Docker/OrbStack, verify worker ingest through the gateway, observe runtime lifecycle movement, and prove cleanup/retention behavior. Source: [issue #23](https://github.com/Nathan-E-White/radiant-prj/issues/23).

Issue #23 was blocked by #21 and #22 when this research started. The issue #23 branch is based on the closed #22 branch, so the Docker SDK adapter and runtime-neutral `SyncRun` lifecycle seam are now present locally and #23 can proceed against them. Source: [issue #23](https://github.com/Nathan-E-White/radiant-prj/issues/23), [issue #21](https://github.com/Nathan-E-White/radiant-prj/issues/21), [issue #22](https://github.com/Nathan-E-White/radiant-prj/issues/22), `backend/slurm-gateway/internal/gateway/simops_adapters.go:12-28`, `backend/slurm-gateway/internal/simopsdocker/spooler.go:153-224`.

## Source Set

Primary sources used:

- Radiant issue text: [issue #19](https://github.com/Nathan-E-White/radiant-prj/issues/19), [issue #21](https://github.com/Nathan-E-White/radiant-prj/issues/21), [issue #22](https://github.com/Nathan-E-White/radiant-prj/issues/22), [issue #23](https://github.com/Nathan-E-White/radiant-prj/issues/23)
- Docker Engine API and SDK docs: <https://docs.docker.com/reference/api/engine/version/v1.51/>, <https://docs.docker.com/reference/api/engine/sdk/>
- Docker Compose docs: <https://docs.docker.com/reference/cli/docker/compose/up/>, <https://docs.docker.com/reference/cli/docker/compose/down/>, <https://docs.docker.com/reference/compose-file/services/#depends_on>, <https://docs.docker.com/compose/how-tos/profiles/>
- Docker SDK/Moby source: <https://github.com/moby/moby/tree/v28.5.2/client>, <https://github.com/moby/moby/tree/v28.5.2/api/types>
- OrbStack Docker docs: <https://docs.orbstack.dev/docker/>
- Local code and docs cited inline below.

## Problem

The parent PRD frames v3 as a deepening of the existing SimOps control plane behind the current run interface, not a Kubernetes operator rewrite. It says ordinary workers should continue to use token-gated gateway ingest, while trusted data-plane roles keep Redpanda, Postgres, and Iceberg access. Source: [issue #19](https://github.com/Nathan-E-White/radiant-prj/issues/19).

Issue #23 is the Docker/OrbStack proof gate before the project takes on Kubernetes/Kind. Its acceptance criteria require launch through the normal API path, gateway-only worker ingest, lifecycle sync, successful-run cleanup, failed-run retention by default in local/dev, and forced cleanup in CI/smoke mode. Source: [issue #23](https://github.com/Nathan-E-White/radiant-prj/issues/23).

The sharp distinction for this issue is runtime launch semantics versus data-plane correctness. The existing smoke already waits for Redpanda-backed Timescale, MoQ/WebTransport, and Iceberg evidence after an API-created run, but #23 needs additional evidence about Docker worker launch, observed runtime states, and cleanup/retention. Source: `scripts/simops-local-smoke.sh:11-13`, `scripts/simops-local-smoke.sh:233-241`, `scripts/simops-local-smoke.sh:274-291`, [issue #23](https://github.com/Nathan-E-White/radiant-prj/issues/23).

## Current Repo Affordances

The repo already has a named smoke command, `simops:smoke:local`, and backend test commands under `package.json`. The full `ci` command includes backend tests, contract checks, generator tests, and related verification, while `ci:full` adds Compose smoke. Source: `package.json:18-27`, `package.json:40-41`.

The local smoke script starts the Docker/Compose platform, builds SimOps services, brings up Postgres/Redpanda/MinIO, starts the SimOps gateway and data-plane writers, posts `POST /api/simops/runs`, polls `GET /api/simops/runs/{run_id}`, checks public events as recovery evidence, and waits for Timescale/Iceberg/WebTransport proof. Source: `scripts/simops-local-smoke.sh:220-241`, `scripts/simops-local-smoke.sh:243-291`.

The Compose gateway is already configured for Docker workers: `SIMOPS_WORKER_RUNTIME=docker`, worker image `radiant-simops-generator:latest`, gateway ingest base URL `http://slurm-gateway:8080`, worker network `radiant-simops-local`, cleanup TTL `10m`, auto-remove `false` for local failed-worker inspection, and a mounted Docker socket. Source: `deploy/slurm-gateway.compose.yml:18-35`, `deploy/slurm-gateway.compose.yml:70-76`.

The Compose stack also wires trusted data-plane roles with Redpanda, Postgres, Iceberg, and MinIO connection settings. That is appropriate for gateway/writer roles, but issue #23 should verify ordinary worker containers do not require those credentials. Source: `deploy/slurm-gateway.compose.yml:20-25`, `deploy/slurm-gateway.compose.yml:36-54`, `deploy/slurm-gateway.compose.yml:99-109`, `deploy/slurm-gateway.compose.yml:134-144`, `deploy/slurm-gateway.compose.yml:170-197`, [issue #23](https://github.com/Nathan-E-White/radiant-prj/issues/23).

The public SimOps HTTP interface is already the right launch seam: `POST /api/simops/runs` creates runs, `GET /api/simops/runs/{id}` reads run state, `GET /events` reads recovery events, and `POST /stop` stops a run. Internal worker ingest endpoints are under `/internal/simops/runs/{id}/ingest` and `/results`. Source: `backend/slurm-gateway/internal/gateway/simops_handlers.go:9-40`, `backend/slurm-gateway/internal/gateway/simops_handlers.go:43-105`, `backend/slurm-gateway/internal/gateway/simops_handlers.go:108-170`.

The controller creates a run record, stores planned workers, calls the runtime spooler, updates the run to `streaming`, publishes a lifecycle event, and returns the run response. Worker telemetry ingest validates the run token, publishes worker telemetry, and updates worker frame counts/lifecycle through the store. Stop calls the runtime spooler and marks run/workers `stopped`. Source: `backend/slurm-gateway/internal/gateway/simops_controller.go:126-205`, `backend/slurm-gateway/internal/gateway/simops_controller.go:236-268`, `backend/slurm-gateway/internal/gateway/simops_controller.go:290-326`.

The current runtime interface exposes `StartRun`, `StopRun`, and `SyncRun`. Docker `SyncRun` lists ordinary worker containers by SimOps labels, inspects container state, and maps Docker state into runtime-neutral observed lifecycle values. Source: `backend/slurm-gateway/internal/gateway/simops_adapters.go:12-28`, `backend/slurm-gateway/internal/simopsdocker/spooler.go:153-224`, [issue #22](https://github.com/Nathan-E-White/radiant-prj/issues/22).

The current checkout contains a Docker SDK spooler under `internal/simopsdocker`. It defines a narrow Docker client, constructs a Docker client with `NewClientWithOpts(... FromEnv, WithAPIVersionNegotiation())`, creates/starts containers from `RunConnectionProfile`, records container IDs in worker labels, and stops/removes run containers by `simops.run_id` label. Source: `backend/slurm-gateway/internal/simopsdocker/spooler.go:21-34`, `backend/slurm-gateway/internal/simopsdocker/spooler.go:70-80`, `backend/slurm-gateway/internal/simopsdocker/spooler.go:82-130`, `backend/slurm-gateway/internal/simopsdocker/spooler.go:144-178`, `backend/slurm-gateway/internal/simopsdocker/spooler.go:180-208`.

`cmd/server` injects the Docker spooler only when SimOps is enabled and `SIMOPS_WORKER_RUNTIME=docker`; the default controller rejects docker runtime if no adapter is injected. Source: `backend/slurm-gateway/cmd/server/main.go:16-25`, `backend/slurm-gateway/internal/gateway/simops_controller.go:46-52`, `backend/slurm-gateway/internal/gateway/simops_runtime_injection_test.go:10-46`.

`RunConnectionProfile` is the credential-boundary seam. Ordinary worker profiles carry run identity, worker identity, gateway ingest URLs and token, worker image, labels, Docker network/container name, Kubernetes name placeholders, and cleanup policy. Trusted profiles can include Redpanda/Postgres/Iceberg data-plane refs; ordinary worker profiles do not include `DataPlane`. Source: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:20-61`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:101-115`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:137-170`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:217-237`.

The worker binary supports HTTP ingest and result ingest through explicit URLs/tokens, not direct Redpanda/Postgres/Iceberg clients. Its Cargo dependencies are `rand`, `rand_distr`, `serde`, `serde_json`, and `time`, and its output code posts JSON with `X-Simops-Ingest-Token` over local HTTP. Source: `workers/simops-generator/Cargo.toml:6-11`, `workers/simops-generator/src/cli.rs:48-67`, `workers/simops-generator/src/cli.rs:93-121`, `workers/simops-generator/src/output.rs:60-97`, `workers/simops-generator/src/output.rs:372-400`.

## External Runtime Facts

Docker's Engine API and SDK are the primary runtime sources for create/start/list/inspect/wait/remove behavior. The existing SDK adapter already uses the documented environment-backed client construction and API version negotiation pattern. Source: <https://docs.docker.com/reference/api/engine/version/v1.51/>, <https://docs.docker.com/reference/api/engine/sdk/>, <https://pkg.go.dev/github.com/docker/docker/client#NewClientWithOpts>, `backend/slurm-gateway/internal/simopsdocker/spooler.go:70-80`.

Docker Compose `depends_on` controls service startup/shutdown ordering, but short syntax starts dependencies without waiting for health; long syntax can require `service_healthy`. The current Compose file uses short `depends_on` plus the smoke script's explicit readiness polling, so #23 should keep explicit smoke evidence rather than assume Compose ordering proves readiness. Source: <https://docs.docker.com/reference/compose-file/services/#depends_on>, `deploy/slurm-gateway.compose.yml:78-81`, `deploy/slurm-gateway.compose.yml:115-116`, `deploy/slurm-gateway.compose.yml:150-152`, `deploy/slurm-gateway.compose.yml:203-206`, `scripts/simops-local-smoke.sh:179-207`, `scripts/simops-local-smoke.sh:233-241`.

Compose profiles are an official way to gate optional services, and the current smoke already builds with `--profile simops-buckets`. Source: <https://docs.docker.com/compose/how-tos/profiles/>, `scripts/simops-local-smoke.sh:233`.

OrbStack includes a Docker engine, Docker command-line tools including Compose/buildx, Compose compatibility through `docker compose`, a Docker context named `orbstack`, optional `/var/run/docker.sock` compatibility, and Docker-compatible networking/port-forwarding/bind-mount behavior. Source: <https://docs.orbstack.dev/docker/>.

The current smoke script prepends `$HOME/.orbstack/bin` to PATH, but `scripts/docker-up.sh` is documented as a Docker Desktop macOS helper. Issue #23 should document OrbStack preconditions explicitly instead of leaving them as path luck. Source: `scripts/simops-local-smoke.sh:69-70`, `scripts/simops-local-smoke.sh:220-224`, `scripts/docker-up.sh:4-20`, <https://docs.orbstack.dev/docker/>.

## Likely Implementation Seams For Issue 23

The external seam should remain the normal SimOps API path, not a bespoke smoke-only launcher. The smoke should create runs through `POST /api/simops/runs`, read state through `GET /api/simops/runs/{id}`, inspect events through `GET /events`, and stop through `POST /stop` when exercising stop/cleanup. Source: `backend/slurm-gateway/internal/gateway/simops_handlers.go:9-105`, `scripts/simops-local-smoke.sh:243-291`, [issue #23](https://github.com/Nathan-E-White/radiant-prj/issues/23).

The runtime seam should remain the runtime/spooler interface now that #22 extended it with sync. `SyncRun` can prove observed lifecycle states such as pending, active, succeeded, failed, missing, image-pull-failed, or stopped without making the smoke inspect private controller internals. Source: `backend/slurm-gateway/internal/gateway/simops_adapters.go:12-28`, `backend/slurm-gateway/internal/gateway/simops_types.go:20-39`, [issue #22](https://github.com/Nathan-E-White/radiant-prj/issues/22).

The credential-boundary seam should be `RunConnectionProfile`: ordinary worker containers should be inspected for gateway ingest URL/token fields and absence of direct Redpanda/Postgres/Iceberg fields, while trusted gateway/writer containers remain allowed to carry data-plane settings. Source: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:101-115`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:137-170`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:217-237`, `backend/slurm-gateway/internal/simopsdocker/spooler.go:217-250`.

The cleanup seam should be Docker labels plus runtime policy. The SDK spooler applies SimOps labels and stops/removes by `simops.run_id`; the profile carries `AutoRemove` and cleanup TTL. Source: `backend/slurm-gateway/internal/simopsdocker/spooler.go:180-208`, `backend/slurm-gateway/internal/simopsdocker/spooler.go:253-284`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:58-61`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:162-165`.

## Smoke Shape To Capture

Named commands: keep `bun run simops:smoke:local` as the broad local data-plane proof, and use `bun run simops:smoke:docker-orbstack` as the narrower #23 runtime proof. The issue explicitly asks for a named Docker/OrbStack smoke command and evidence-rich output. Source: `package.json`, [issue #23](https://github.com/Nathan-E-White/radiant-prj/issues/23).

Precondition output should include Docker/OrbStack identity and readiness: `docker context show`, `docker info`, `docker compose version`, the effective compose file, and whether the gateway can reach the mounted Docker socket. This follows OrbStack's documented `orbstack` context/socket behavior and Docker/Compose command model. Source: <https://docs.orbstack.dev/docker/>, <https://docs.docker.com/reference/cli/docker/compose/up/>, `deploy/slurm-gateway.compose.yml:75`, `scripts/simops-local-smoke.sh:69-70`.

Launch evidence should include the API request shape, run ID, worker IDs, container IDs, SimOps labels, container image, container network, and launch-mode values. The SDK adapter already records container ID in worker labels and command output, and labels include run, worker, role, launch mode, scenario, image, runtime, adapter, network, and namespace. Source: `backend/slurm-gateway/internal/simopsdocker/spooler.go:107-126`, `backend/slurm-gateway/internal/simopsdocker/spooler.go:275-290`.

Gateway-ingest evidence should verify the worker command/env point to `/internal/simops/runs/{run_id}/ingest` and `/results`, and that published events arrive via the gateway/store path. The worker CLI requires matching URLs and tokens when ingest flags are supplied. Source: `backend/slurm-gateway/internal/simopsdocker/spooler.go:217-250`, `workers/simops-generator/src/cli.rs:48-67`, `workers/simops-generator/src/output.rs:60-97`, `backend/slurm-gateway/internal/gateway/simops_controller.go:290-326`.

Lifecycle-sync evidence should wait for runtime-observed worker states after #22 lands. It should prove state movement through the runtime-neutral model requested by #22, and only then should it correlate those states with the public run response. Source: [issue #22](https://github.com/Nathan-E-White/radiant-prj/issues/22), `backend/slurm-gateway/internal/gateway/simops_types.go:8-18`, `backend/slurm-gateway/internal/gateway/simops_types.go:78-88`.

Cleanup evidence should assert no successful-run containers remain when cleanup/auto-remove policy says they should be removed. Failure-retention evidence should use a deliberate failed worker scenario with retention enabled and prove the failed container/logs remain inspectable by SimOps labels. CI/smoke force-cleanup evidence should then prove labeled failed containers are removed when the cleanup override is enabled. Source: `backend/slurm-gateway/internal/simopsdocker/spooler.go:180-208`, `backend/slurm-gateway/internal/simopsdocker/spooler.go:253-284`, `deploy/slurm-gateway.compose.yml:33-34`, [issue #23](https://github.com/Nathan-E-White/radiant-prj/issues/23).

## Cleanup And Retention Semantics To Verify

Successful run cleanup has two current mechanisms to verify: configured success cleanup TTL after observed Docker success, and explicit stop/remove by run label for forced cleanup. Compose keeps auto-remove false for local Docker workers so failed containers remain inspectable; the focused smoke sets `SIMOPS_WORKER_CLEANUP_TTL=0s` to prove success cleanup without waiting for the local-dev TTL. Source: `deploy/slurm-gateway.compose.yml:33-34`, `backend/slurm-gateway/internal/gateway/config.go:123-131`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:58-61`, `backend/slurm-gateway/internal/simopsdocker/spooler.go:253-260`.

Failed-run retention is not proven by the current smoke. The existing SDK spooler removes containers during explicit stop and partial-launch rollback, and auto-remove can remove exited containers automatically when enabled. Issue #23 therefore needs a separate retained-failure mode for local/dev, plus a smoke/CI override that removes retained containers after collecting evidence. Source: `backend/slurm-gateway/internal/simopsdocker/spooler.go:97-103`, `backend/slurm-gateway/internal/simopsdocker/spooler.go:173-175`, `backend/slurm-gateway/internal/simopsdocker/spooler.go:180-208`, [issue #23](https://github.com/Nathan-E-White/radiant-prj/issues/23).

The smoke output should redact tokens. Current profiles and Docker env/command include ingest tokens, so useful evidence must show presence/absence of fields without dumping secret values. Source: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:36-40`, `backend/slurm-gateway/internal/simopsdocker/spooler.go:217-250`.

## Explicit Non-Goals

Do not add Kubernetes/Kind validation in issue #23. Kubernetes client-go and Kind proof are #24/#25 territory. Source: [issue #23](https://github.com/Nathan-E-White/radiant-prj/issues/23), [issue #19](https://github.com/Nathan-E-White/radiant-prj/issues/19).

Do not add OpenTofu provisioning. The parent PRD assigns OpenTofu to mostly-static substrate work, and issue #23 explicitly excludes it. Source: [issue #19](https://github.com/Nathan-E-White/radiant-prj/issues/19), [issue #23](https://github.com/Nathan-E-White/radiant-prj/issues/23).

Do not redesign browser UX or change the frontend contract. Issue #21 explicitly excludes frontend contract changes, and issue #23 excludes browser UX changes. Source: [issue #21](https://github.com/Nathan-E-White/radiant-prj/issues/21), [issue #23](https://github.com/Nathan-E-White/radiant-prj/issues/23).

Do not add host-facing Redpanda listener support. The issue keeps the smoke focused on runtime launch semantics, and the current local compose path already keeps workers on gateway ingest while trusted services use in-network Redpanda. Source: [issue #23](https://github.com/Nathan-E-White/radiant-prj/issues/23), `deploy/slurm-gateway.compose.yml:20-30`.

Do not turn the smoke into full lakehouse validation. Existing data-plane evidence can remain as supporting proof, but #23's acceptance should be decided by launch, ingest boundary, lifecycle sync, and cleanup/retention evidence. Source: [issue #23](https://github.com/Nathan-E-White/radiant-prj/issues/23), `scripts/simops-local-smoke.sh:274-291`.

## Implementation-Relevant Conclusion

Issue #23 should be implemented on top of the closed #21 and #22 work. The current issue #23 branch contains the Docker SDK adapter and runtime-neutral lifecycle sync seam, so the remaining proof belongs in a focused Docker/OrbStack smoke at the normal SimOps API seam. Source: [issue #21](https://github.com/Nathan-E-White/radiant-prj/issues/21), [issue #22](https://github.com/Nathan-E-White/radiant-prj/issues/22), `backend/slurm-gateway/internal/simopsdocker/spooler.go:21-34`, `backend/slurm-gateway/internal/gateway/simops_adapters.go:12-28`.

When #23 proceeds, the best shape is a narrow Docker/OrbStack smoke at the normal SimOps API seam, backed by Docker-label inspection and runtime sync evidence. It should prove ordinary worker credential boundaries, capture enough launch/ingest/sync/cleanup evidence to debug failures, keep token values redacted, and leave Kubernetes, OpenTofu, browser UX, and host-facing Redpanda listeners out of the room where they belong. Source: [issue #23](https://github.com/Nathan-E-White/radiant-prj/issues/23).
