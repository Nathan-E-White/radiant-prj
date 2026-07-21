# V3 SimOps Runtime Adapters And Connection Profiles Research

| Field | Value |
| --- | --- |
| Document ID | SIMOPS-RUNTIME-ADAPTERS-RESEARCH-001 |
| Status | Research note |
| Scope | GitHub issue #19, current checkout state, and primary-source runtime constraints |
| Last updated | 2026-07-10 |

## Source Boundary

Primary sources used:

- GitHub issue #19, "PRD: V3 SimOps runtime adapters and connection profiles", and its published child issue map comment: <https://github.com/Nathan-E-White/radiant-prj/issues/19> and <https://github.com/Nathan-E-White/radiant-prj/issues/19#issuecomment-4933153528>.
- Repo source, docs, schemas, scripts, deploy, and sandbox files cited inline with local paths and line numbers.
- Official Docker SDK / Engine docs for SDK-shaped Docker launch context: <https://docs.docker.com/reference/api/engine/sdk/examples/> and <https://docs.docker.com/reference/api/engine/version/v1.51/>.
- Official Kubernetes Jobs and TTL-after-finished docs for the future client-go Job adapter shape: <https://kubernetes.io/docs/concepts/workloads/controllers/job/> and <https://kubernetes.io/docs/concepts/workloads/controllers/ttlafterfinished/>.
- Official OpenTofu resource docs for substrate framing: <https://opentofu.org/docs/language/resources/>.
- Official Kind and OrbStack docs for local Kubernetes and Docker substrate context: <https://kind.sigs.k8s.io/docs/user/quick-start/> and <https://docs.orbstack.dev/docker/>.

## Problem Restatement

Issue #19 is open and describes a v3 PRD for deepening Radiant's existing Simulation Ops control plane rather than replacing it with a CRD, operator, Argo, Tekton, or other cluster-native workflow engine. Source: <https://github.com/Nathan-E-White/radiant-prj/issues/19>.

The problem it is solving is that Radiant already has a SimOps run interface, run-scoped workers, token-gated telemetry/result ingest, and Redpanda/Postgres/Timescale/WebTransport/Iceberg fanout, but the runtime launch surface needs a deeper module so Docker and Kubernetes execution can share SimOps semantics without leaking runtime details into handlers or ordinary workers. Source: <https://github.com/Nathan-E-White/radiant-prj/issues/19>.

The PRD's staged dependency map is: #20 `RunConnectionProfile`, #21 Docker SDK adapter, #22 runtime-neutral `SyncRun`, #23 Docker/OrbStack E2E proof, #24 Kubernetes client-go Job adapter, #25 Kind/client-go E2E proof, #26 OpenTofu substrate lane, and #27 docs/verification closeout. Source: <https://github.com/Nathan-E-White/radiant-prj/issues/19#issuecomment-4933153528>.

## Current SimOps State In The Checkout

### Public Run Interface

The public SimOps run API is still the main caller surface: `POST /api/simops/runs` creates a run, `GET /api/simops/runs/{runID}` reads it, `GET /api/simops/runs/{runID}/events` reads run events, and `POST /api/simops/runs/{runID}/stop` stops it. Source: `backend/slurm-gateway/internal/gateway/simops_handlers.go:9-105`.

The run request and response model carries scenario, source, work script, launch mode, worker kinds, runtime limit, idempotency key, lifecycle, workers, spool commands, artifacts, and MoQ subscription metadata. Source: `backend/slurm-gateway/internal/gateway/simops_types.go:36-60`.

The controller normalizes worker kinds, validates run requests, persists a planned launch, calls the injected SimOps spooler/runtime, saves launch records, moves the run to `streaming`, plans an artifact, and publishes a lifecycle event. Source: `backend/slurm-gateway/internal/gateway/simops_controller.go:126-204`.

### RunConnectionProfile

`RunConnectionProfile` is present and materially implemented: it carries run/scenario/worker identity, role, gateway ingest/result URLs and ingest token, worker image, manifest path, labels, Docker runtime fields, Kubernetes runtime fields, cleanup policy, and optional data-plane fields. Source: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:20-87`.

Ordinary worker profiles are built with role `ordinary-worker`; trusted profiles are restricted to stream gateway, projector, and Iceberg writer roles. Source: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:101-115`.

The profile builder derives ingest/result URLs from `SIMOPS_WORKER_INGEST_BASE_URL`, derives Docker container names, derives Kubernetes Job names, maps cleanup TTL/auto-remove, and only attaches data-plane fields when the caller asks for a trusted profile. Source: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:117-170`.

The profile test suite proves ordinary workers exclude data-plane refs, trusted roles include Redpanda/Postgres/Iceberg refs, Docker and Kubernetes runtime fields are rendered, cleanup TTL is mapped, and incomplete config is rejected. Source: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile_test.go:9-176`.

### Runtime Adapter And Spooler Shape

The current runtime seam is still `SimopsSpooler` / `SimopsRuntime` with `StartRun` and `StopRun`; there is no `SyncRun` method in that interface today. Source: `backend/slurm-gateway/internal/gateway/simops_adapters.go:12-28`.

The default controller uses a contract spooler for `SIMOPS_WORKER_RUNTIME=contract`; when `SIMOPS_WORKER_RUNTIME=docker`, the gateway core requires an injected runtime adapter rather than importing Docker directly. Source: `backend/slurm-gateway/internal/gateway/simops_controller.go:38-53`.

The server binary injects the concrete Docker SimOps adapter only when SimOps is enabled and `SIMOPS_WORKER_RUNTIME=docker`. Source: `backend/slurm-gateway/cmd/server/main.go:16-25`.

The gateway dependency check intentionally keeps `internal/gateway` free of Docker SDK and client-go dependencies by failing if `go list` finds `github.com/docker/docker` or `k8s.io/client-go` under `./internal/gateway`. Source: `scripts/check-go-gateway-deps.mjs:23-42`.

### Docker SDK Status

The Docker adapter is already SDK-backed in this checkout: `internal/simopsdocker` imports Docker container, filter, image, network, and client packages, defines a narrow fakeable `DockerClient`, and constructs the default client with Docker environment options and API version negotiation. Source: `backend/slurm-gateway/internal/simopsdocker/spooler.go:11-28`, `backend/slurm-gateway/internal/simopsdocker/spooler.go:70-80`.

The adapter starts workers by building a `RunConnectionProfile`, inspecting the configured image, calling `ContainerCreate`, calling `ContainerStart`, and returning worker/spool command records with container metadata. Source: `backend/slurm-gateway/internal/simopsdocker/spooler.go:82-130`, `backend/slurm-gateway/internal/simopsdocker/spooler.go:144-178`.

The adapter stops a run by listing all Docker containers with `label=simops.run_id=<runID>`, then stopping and force-removing each matching container. Source: `backend/slurm-gateway/internal/simopsdocker/spooler.go:180-208`.

The adapter maps the profile into container command arguments, environment variables, host/networking config, and labels including `simops.runtime=docker` and `simops.runtime_adapter=docker-sdk`. Source: `backend/slurm-gateway/internal/simopsdocker/spooler.go:217-291`.

The Docker adapter tests use a fake client to prove image inspect, container name/image/labels/env/cmd/network/auto-remove/start behavior, structured launch errors, and stop-by-run-label cleanup. Source: `backend/slurm-gateway/internal/simopsdocker/spooler_test.go:19-151`.

The module now depends on Docker SDK packages at the gateway module level, while the dependency check keeps those imports out of the runtime-neutral gateway package. Source: `backend/slurm-gateway/go.mod:5-14`, `scripts/check-go-gateway-deps.mjs:23-42`.

Official Docker docs are consistent with using SDK/API operations for this adapter shape: Docker publishes SDK/API examples and an Engine API reference for container create/list/start/stop/remove operations. Source: <https://docs.docker.com/reference/api/engine/sdk/examples/>, <https://docs.docker.com/reference/api/engine/version/v1.51/>.

### Lifecycle Sync Status

Lifecycle sync is only partially represented today: the controller records planned worker state, records adapter launch results, updates workers on ingest, and stops workers on explicit stop, but there is no runtime-neutral `SyncRun` method or Kubernetes/Docker lifecycle observation loop. Source: `backend/slurm-gateway/internal/gateway/simops_controller.go:170-187`, `backend/slurm-gateway/internal/gateway/simops_controller.go:236-268`, `backend/slurm-gateway/internal/gateway/simops_controller.go:290-327`, `backend/slurm-gateway/internal/gateway/simops_adapters.go:25-28`.

This means issue #22 remains future or partial in this checkout: lifecycle state is observable through stored run/worker records, but runtime process state is not yet synchronized through the adapter interface. Source: <https://github.com/Nathan-E-White/radiant-prj/issues/19#issuecomment-4933153528>, `backend/slurm-gateway/internal/gateway/simops_adapters.go:25-28`.

### Kubernetes / Kind / OpenTofu Status

The production SimOps gateway config currently accepts only `contract` and `docker` worker runtime values; there is no `kubernetes` runtime value in `SimopsConfig.Validate`. Source: `backend/slurm-gateway/internal/gateway/config.go:440-464`.

`RunConnectionProfile` renders Kubernetes namespace and Job name fields, but no production SimOps Kubernetes adapter exists under `backend/slurm-gateway`. The former Randiant Runtime sandbox was retired in issue #140; Git history retains that experiment, while active runtime evidence now lives in the Radiant backend adapters. Source: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:53-56`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:157-160`, <https://github.com/Nathan-E-White/radiant-prj/issues/140>.

The retired sandbox was a separate Docker, Kubernetes, surveillance, OpenTofu, and orchestrator experiment—not an integrated SimOps runtime. It must not be used as current verification or design evidence. Source: <https://github.com/Nathan-E-White/radiant-prj/issues/140>.

Official Kubernetes docs match the PRD's future Job-adapter direction: Jobs run one-off tasks to completion, create Pods, retry until successful completions, and support `restartPolicy: Never` in examples. Source: <https://kubernetes.io/docs/concepts/workloads/controllers/job/>.

Official Kubernetes docs also match the cleanup field already carried by `RunConnectionProfile`: finished Jobs can be cleaned up by setting `.spec.ttlSecondsAfterFinished`, with cascading deletion of dependent objects after the TTL expires. Source: <https://kubernetes.io/docs/concepts/workloads/controllers/ttlafterfinished/>.

The top-level `infra/main.tofu` is a placeholder custom Docker provider example, not a SimOps Kubernetes substrate lane. Source: `infra/main.tofu:3-29`.

OpenTofu's own language docs frame resources as managed infrastructure objects, which supports issue #19's substrate-only use for namespaces/RBAC/service accounts rather than per-run launch ownership. Source: <https://opentofu.org/docs/language/resources/>, <https://github.com/Nathan-E-White/radiant-prj/issues/19>.

Kind's official quick start says `kind create cluster` bootstraps a local Kubernetes cluster and can load Docker images into the cluster, which fits the PRD's local Kind proof lane after the Docker proof. Source: <https://kind.sigs.k8s.io/docs/user/quick-start/>, <https://github.com/Nathan-E-White/radiant-prj/issues/19>.

OrbStack's Docker docs say OrbStack creates a Docker context named `orbstack` and can create `/var/run/docker.sock` for compatibility, which matches the repo's Docker-socket-based local gateway wiring. Source: <https://docs.orbstack.dev/docker/>, `deploy/slurm-gateway.compose.yml:70-76`.

### Data-Plane Boundaries

Ordinary run-scoped workers use gateway ingest and result-ingest routes guarded by `X-Simops-Ingest-Token`; the HTTP handlers reject invalid ingest tokens before accepting telemetry or simulated result payloads. Source: `backend/slurm-gateway/internal/gateway/simops_handlers.go:108-170`.

The Docker worker command/env generated from `RunConnectionProfile` contains run identity, worker identity, ingest URLs, result ingest URLs, and the run ingest token, not direct Redpanda/Postgres/Iceberg credentials. Source: `backend/slurm-gateway/internal/simopsdocker/spooler.go:217-250`.

Trusted data-plane profiles can include Redpanda brokers/topic, Postgres DSN, and Iceberg catalog/warehouse/S3 fields, but ordinary worker profiles must have `DataPlane == nil`. Source: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:63-87`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:168-236`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile_test.go:43-45`.

The local compose data plane includes Redpanda, Postgres/Timescale, MinIO/Iceberg, MoQ gateway, Timescale writer, and Iceberg writer services, while the worker runtime is still configured as Docker. Source: `deploy/slurm-gateway.compose.yml:18-54`, `deploy/slurm-gateway.compose.yml:86-212`, `deploy/slurm-gateway.compose.yml:358-423`.

The Redpanda service advertises Kafka as `redpanda:9092` inside the compose network while publishing ports for local inspection; issue #19 still defers host-facing Redpanda listener complexity as a v3 runtime requirement. Source: `deploy/slurm-gateway.compose.yml:373-379`, <https://github.com/Nathan-E-White/radiant-prj/issues/19>.

### Value-Basis Boundary

The repo has an explicit value-basis schema with only `measured`, `imputed`, and `simulated`. Source: `docs/schemas/simulator-workbench/value-basis.v1.schema.json:1-7`.

SCADA/resident source validation requires measured value basis and public-safe stand-in status. Source: `backend/slurm-gateway/internal/gateway/workbench_controller.go:130-189`.

SimOps result validation requires simulated value basis and explicitly rejects imputed state as a worker output, reserving imputed state for the digital twin projector. Source: `backend/slurm-gateway/internal/gateway/workbench_controller.go:192-222`.

The SimOps result schema restricts `valueBasis` to `simulated`, while lineage inputs can carry measured, imputed, or simulated bases. Source: `docs/schemas/simulation-ops/simops-result-envelope.v1.schema.json:1-85`.

Workbench projection code materializes measured, simulated, and imputed values separately in twin state and lineage. Source: `backend/slurm-gateway/internal/gateway/workbench_consumers.go:330-395`.

## Verification Scripts And Evidence Gates

`package.json` exposes focused verification commands for backend tests, dependency boundaries, SimOps contract checks, generator tests, local SimOps smoke, and full CI. Source: `package.json:18-28`, `package.json:40-41`.

`scripts/check-go-gateway-deps.mjs` is the explicit boundary guard that keeps the runtime-neutral gateway package from importing Docker SDK or client-go. Source: `scripts/check-go-gateway-deps.mjs:23-42`.

`scripts/simops-local-smoke.sh` starts the local platform, builds worker/gateway services, creates one API-driven SimOps run, waits for Redpanda-backed Timescale/MoQ/Iceberg telemetry fanout, and treats REST event polling as recovery evidence. Source: `scripts/simops-local-smoke.sh:4-22`, `scripts/simops-local-smoke.sh:224-291`.

The local smoke script has image metadata/content preflights and cache-only/mirror override paths so Docker registry/base-image failure is separated from SimOps service failure. Source: `scripts/simops-local-smoke.sh:15-21`, `scripts/simops-local-smoke.sh:118-177`.

`scripts/check-infra.mjs` statically checks that compose carries SimOps worker runtime, ingest base URL, worker network, Kubernetes namespace placeholder, cleanup TTL, Docker socket, Redpanda/Postgres/MinIO/MoQ/Timescale/Iceberg wiring, and no-new-privileges labels. Source: `scripts/check-infra.mjs:78-139`.

## Implemented, Partial, And Remaining Slices

| Issue #19 slice | Checkout status | Evidence |
| --- | --- | --- |
| #20 RunConnectionProfile | Implemented enough to plan against. The module and tests exist, ordinary/trusted role boundaries are explicit, Docker/Kubernetes fields are rendered, and cleanup/data-plane fields are modeled. | <https://github.com/Nathan-E-White/radiant-prj/issues/19#issuecomment-4933153528>, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:20-170`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile_test.go:9-176` |
| #21 Docker SDK adapter | Implemented in current checkout. The adapter imports Docker SDK packages, uses a narrow fakeable client, creates/starts/stops/removes containers through SDK calls, and has fake-client tests. | <https://github.com/Nathan-E-White/radiant-prj/issues/19#issuecomment-4933153528>, `backend/slurm-gateway/internal/simopsdocker/spooler.go:11-337`, `backend/slurm-gateway/internal/simopsdocker/spooler_test.go:19-151` |
| #22 SyncRun lifecycle | Not implemented as specified. The interface has `StartRun` and `StopRun` only, and lifecycle is updated through create/ingest/stop flows rather than a runtime-neutral sync method. | <https://github.com/Nathan-E-White/radiant-prj/issues/19#issuecomment-4933153528>, `backend/slurm-gateway/internal/gateway/simops_adapters.go:25-28`, `backend/slurm-gateway/internal/gateway/simops_controller.go:170-187` |
| #23 Docker/OrbStack E2E proof | Partially present as local smoke infrastructure. The script creates a Docker-backed run and validates Timescale/MoQ/Iceberg fanout, but this note did not rerun the smoke. | <https://github.com/Nathan-E-White/radiant-prj/issues/19#issuecomment-4933153528>, `scripts/simops-local-smoke.sh:224-291`, `deploy/slurm-gateway.compose.yml:27-34` |
| #24 Kubernetes client-go Job adapter | Not implemented in production SimOps. The profile has Kubernetes names, but production config does not accept `kubernetes`, and no backend SimOps client-go Job adapter is present. | <https://github.com/Nathan-E-White/radiant-prj/issues/19#issuecomment-4933153528>, `backend/slurm-gateway/internal/gateway/config.go:440-464`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:53-56` |
| #25 Kind/client-go E2E proof | Not present for SimOps. The separate Randiant Runtime experiment was retired and is available only in Git history, not as active proof. | <https://github.com/Nathan-E-White/radiant-prj/issues/19#issuecomment-4933153528>, <https://github.com/Nathan-E-White/radiant-prj/issues/140> |
| #26 OpenTofu substrate lane | Not present as the PRD lane. Existing top-level Tofu is placeholder custom Docker infra; the retired sandbox does not establish static SimOps namespace/RBAC/service-account substrate evidence. | <https://github.com/Nathan-E-White/radiant-prj/issues/19#issuecomment-4933153528>, `infra/main.tofu:3-29`, <https://github.com/Nathan-E-White/radiant-prj/issues/140> |
| #27 Docs/verification closeout | Not complete. This research note contributes source mapping, but the runtime plan still lacks SyncRun, Kubernetes Job implementation/proof, OpenTofu substrate proof, and closeout evidence. | <https://github.com/Nathan-E-White/radiant-prj/issues/19#issuecomment-4933153528>, `package.json:18-28`, `package.json:40-41` |

## Constraints And Risks

Ordinary workers must remain gateway-ingest-only; direct Redpanda, Postgres, Iceberg, Docker, and Kubernetes credentials are reserved for trusted roles or runtime/platform code. Source: <https://github.com/Nathan-E-White/radiant-prj/issues/19>, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile_test.go:82-100`, `backend/slurm-gateway/internal/simopsdocker/spooler.go:217-250`.

Trusted data-plane fields must stay role-scoped because `BuildTrustedRunConnectionProfile` only allows stream gateway, projector, and Iceberg writer roles to receive data-plane refs. Source: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:108-115`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:217-237`.

Measured, imputed, and simulated values are already enforced in schemas and validators, so runtime-adapter work must not collapse those bases into a generic worker metric stream. Source: `docs/schemas/simulator-workbench/value-basis.v1.schema.json:1-7`, `backend/slurm-gateway/internal/gateway/workbench_controller.go:144-149`, `backend/slurm-gateway/internal/gateway/workbench_controller.go:214-219`.

Local failed-artifact retention versus successful cleanup needs design attention because Docker currently maps profile cleanup to `HostConfig.AutoRemove` and explicit stop/remove, while Kubernetes Jobs use TTL cleanup after completion or failure. Source: `backend/slurm-gateway/internal/simopsdocker/spooler.go:253-260`, `backend/slurm-gateway/internal/simopsdocker/spooler.go:180-208`, <https://kubernetes.io/docs/concepts/workloads/controllers/ttlafterfinished/>.

Docker/OrbStack proof should remain first because the compose path is already Docker-backed and the local smoke script is Docker/compose oriented. Source: <https://github.com/Nathan-E-White/radiant-prj/issues/19>, `deploy/slurm-gateway.compose.yml:27-34`, `scripts/simops-local-smoke.sh:224-291`.

Kind/Kubernetes should follow Docker because the production gateway does not yet accept a Kubernetes runtime; the retired sandbox is not current reference material. Source: `backend/slurm-gateway/internal/gateway/config.go:440-464`, <https://github.com/Nathan-E-White/radiant-prj/issues/140>.

OpenTofu should remain substrate-only because issue #19 excludes per-run OpenTofu applies, and the current repo has no finished SimOps namespace/RBAC/service-account substrate lane. The retired sandbox's per-run experiment is not evidence for this direction. Source: <https://github.com/Nathan-E-White/radiant-prj/issues/19>, `infra/main.tofu:3-29`, <https://github.com/Nathan-E-White/radiant-prj/issues/140>.

CRD/operator, Argo, Tekton, and host-facing Redpanda listener support remain out of scope for v3 runtime adapter planning. Source: <https://github.com/Nathan-E-White/radiant-prj/issues/19>.

The retired sandbox must not create a naming or expectation risk: it demonstrated an experiment, not the SimOps client-go Job adapter required by issue #19. Git history is its archive. Source: <https://github.com/Nathan-E-White/radiant-prj/issues/140>, <https://github.com/Nathan-E-White/radiant-prj/issues/19>.

There is a verification-risk split between "script exists" and "proof recently passed": this note confirms scripts and wiring, but it did not run Docker/OrbStack smoke, Kind smoke, or OpenTofu validation. Source: `scripts/simops-local-smoke.sh:224-291`, `package.json:22-28`.

## Implications For Planning

The next planning step should not restart from an empty PRD: `RunConnectionProfile` and the Docker SDK adapter already exist, so planning should start by confirming whether #20 and #21 can be closed or need hardening evidence. Source: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:20-170`, `backend/slurm-gateway/internal/simopsdocker/spooler.go:11-337`, <https://github.com/Nathan-E-White/radiant-prj/issues/19#issuecomment-4933153528>.

The highest-value next design/TDD slice is likely `SyncRun`, because it is the missing seam between the already-working Docker launch path and the future Kubernetes Job lifecycle observer. Source: `backend/slurm-gateway/internal/gateway/simops_adapters.go:25-28`, `backend/slurm-gateway/internal/gateway/simops_controller.go:236-268`, <https://github.com/Nathan-E-White/radiant-prj/issues/19>.

Kubernetes planning should use the active Radiant backend adapters and design a production SimOps adapter around native Jobs, labels, ConfigMap/Secret references, service account, restart policy, and cleanup TTL from `RunConnectionProfile`. The sandbox is retired. Source: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:53-60`, <https://github.com/Nathan-E-White/radiant-prj/issues/140>, <https://kubernetes.io/docs/concepts/workloads/controllers/job/>.

The OpenTofu lane should be planned as substrate evidence, not runtime launch code, because the PRD excludes per-run OpenTofu Jobs and the OpenTofu resource model is infrastructure-object oriented. Source: <https://github.com/Nathan-E-White/radiant-prj/issues/19>, <https://opentofu.org/docs/language/resources/>.

Every future runtime slice should preserve the data-plane credential boundary and the measured/imputed/simulated value-basis distinction as acceptance criteria, because both are already present in code and schemas rather than just in prose. Source: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile_test.go:82-100`, `docs/schemas/simulator-workbench/value-basis.v1.schema.json:1-7`, `backend/slurm-gateway/internal/gateway/workbench_controller.go:192-222`.
