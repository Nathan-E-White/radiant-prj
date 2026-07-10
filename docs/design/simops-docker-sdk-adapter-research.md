# SimOps Docker SDK Adapter Research

| Field | Value |
| --- | --- |
| Document ID | SIMOPS-DOCKER-SDK-RESEARCH-001 |
| Revision | 0.1 |
| Status | Research note |
| Owner | Software |
| Scope | Issue #21 research baseline and adapter design notes |

## Purpose

This note records primary-source research for Radiant issue #21: replacing the current Docker CLI shell-out SimOps launcher with a Docker SDK-backed runtime adapter while preserving the existing SimOps run interface and the `RunConnectionProfile` launch contract. Sources: [issue #21](https://github.com/Nathan-E-White/radiant-prj/issues/21), [issue #20](https://github.com/Nathan-E-White/radiant-prj/issues/20), [issue #19](https://github.com/Nathan-E-White/radiant-prj/issues/19).

This note does not itself implement the adapter; it captures the source-grounded baseline and design constraints used by the issue #21 implementation. Source boundary: [issue #21](https://github.com/Nathan-E-White/radiant-prj/issues/21).

## Source Set

Primary sources used:

- Docker Engine API v1.51 reference and OpenAPI spec: <https://docs.docker.com/reference/api/engine/version/v1.51/> and <https://docs.docker.com/reference/api/engine/version/v1.51.yaml>
- Docker SDK/Moby source for v28.5.2 client calls: <https://github.com/moby/moby/tree/v28.5.2/client>
- Docker SDK/Moby source for v28.5.2 container, network, and filter types: <https://github.com/moby/moby/tree/v28.5.2/api/types>
- OrbStack Docker docs: <https://docs.orbstack.dev/docker/>
- Radiant issue text: [issue #19](https://github.com/Nathan-E-White/radiant-prj/issues/19), [issue #20](https://github.com/Nathan-E-White/radiant-prj/issues/20), [issue #21](https://github.com/Nathan-E-White/radiant-prj/issues/21)
- Local code: `backend/slurm-gateway/internal/gateway/simops_adapters.go`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go`, `backend/slurm-gateway/internal/gateway/config.go`, `backend/slurm-gateway/internal/gateway/simops_controller.go`, `deploy/slurm-gateway.compose.yml`

## Repo Baseline

Issue #21 asks only for the Docker path: add a Docker SDK adapter behind the existing SimOps runtime/spooler boundary, consume `RunConnectionProfile`, preserve image/command/label/network/auto-remove/run/worker behavior, remove command string assembly from normal Docker startup, return structured launch metadata/errors, and stay compatible with local OrbStack-backed Docker. Source: [issue #21](https://github.com/Nathan-E-White/radiant-prj/issues/21).

Issue #20 established the `RunConnectionProfile` seam and explicitly kept Docker SDK, Kubernetes client-go, Kafka/Redpanda, Postgres/pgx, and Iceberg/Arrow concrete dependencies out of default `internal/gateway` core builds. Source: [issue #20](https://github.com/Nathan-E-White/radiant-prj/issues/20).

The pre-issue-21 runtime selector used `ContractSimopsSpooler` by default and `NewDockerSimopsSpooler(cfg)` when `SIMOPS_WORKER_RUNTIME=docker`. Source: pre-change `backend/slurm-gateway/internal/gateway/simops_controller.go:34-45`.

The pre-issue-21 `DockerSimopsSpooler` shelled out through `CmdRunner`: it called `docker image inspect`, built `docker run -d ...` args, listed containers with `docker ps -a --filter label=simops.run_id=...`, and stopped/removed with `docker stop` plus `docker rm --force`. Source: pre-change `backend/slurm-gateway/internal/gateway/simops_adapters.go:93-309`.

`RunConnectionProfile` already carries the SDK adapter's launch inputs: run/scenario/worker identity, gateway ingest URLs and token, worker image, manifest path, labels, Docker network/container name/auto-remove, Kubernetes namespace/job name, cleanup TTL, and optional trusted data-plane refs. Source: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:20-87`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:137-170`.

Ordinary worker profiles exclude direct data-plane refs, while trusted profiles can include Redpanda, Postgres, and Iceberg fields. Source: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:101-115`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:168-236`.

The local compose path runs SimOps worker runtime as Docker, uses image `radiant-simops-generator:latest`, sets `SIMOPS_WORKER_NETWORK=radiant-simops-local`, enables `SIMOPS_WORKER_AUTO_REMOVE=true`, and mounts `/var/run/docker.sock` into the gateway container. Source: `deploy/slurm-gateway.compose.yml:26-34`, `deploy/slurm-gateway.compose.yml:70-76`.

## Fakeable Docker Client Methods

Do not fake Docker's full SDK `APIClient`; the v28.5.2 `ContainerAPIClient` includes many methods beyond this slice, and `ImageAPIClient` includes broad image operations beyond image inspect. Source: <https://github.com/moby/moby/blob/v28.5.2/client/client_interfaces.go>.

Use a repo-local narrow interface around the exact calls issue #21 needs: image inspect, container create, container start, container list, container stop, and container remove. Sources: <https://github.com/moby/moby/blob/v28.5.2/client/image_inspect.go>, <https://github.com/moby/moby/blob/v28.5.2/client/container_create.go>, <https://github.com/moby/moby/blob/v28.5.2/client/container_start.go>, <https://github.com/moby/moby/blob/v28.5.2/client/container_list.go>, <https://github.com/moby/moby/blob/v28.5.2/client/container_stop.go>, <https://github.com/moby/moby/blob/v28.5.2/client/container_remove.go>.

Recommended adapter-facing seam for v28.5.2-style imports:

```go
type dockerWorkerClient interface {
	ImageInspect(ctx context.Context, image string) (image.InspectResponse, error)
	ContainerCreate(ctx context.Context, cfg *container.Config, host *container.HostConfig, net *network.NetworkingConfig, platform *ocispec.Platform, name string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, id string, opts container.StartOptions) error
	ContainerList(ctx context.Context, opts container.ListOptions) ([]container.Summary, error)
	ContainerStop(ctx context.Context, id string, opts container.StopOptions) error
	ContainerRemove(ctx context.Context, id string, opts container.RemoveOptions) error
}
```

`client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())` is the v28.5.2 source's documented construction path for environment-backed configuration and API version negotiation. Source: <https://github.com/moby/moby/blob/v28.5.2/client/client.go>, <https://github.com/moby/moby/blob/v28.5.2/client/options.go>.

## Create And Start Shape

Docker Engine `POST /containers/create` accepts a request body composed from container config plus `HostConfig` and `NetworkingConfig`, and it accepts a `name` query parameter for the container name. Source: <https://docs.docker.com/reference/api/engine/version/v1.51/#tag/Container/operation/ContainerCreate>, <https://docs.docker.com/reference/api/engine/version/v1.51.yaml>.

The v28.5.2 SDK `ContainerCreate` method maps those Engine inputs to `config *container.Config`, `hostConfig *container.HostConfig`, `networkingConfig *network.NetworkingConfig`, optional platform, and `containerName`, then posts `container.CreateRequest` to `/containers/create`. Source: <https://github.com/moby/moby/blob/v28.5.2/client/container_create.go>.

Map `RunConnectionProfile.WorkerImage` to `container.Config.Image`, current post-image worker arguments to `container.Config.Cmd`, and `RunConnectionProfile.Labels` to `container.Config.Labels`. Sources: `backend/slurm-gateway/internal/gateway/simops_adapters.go:213-240`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:137-167`, <https://github.com/moby/moby/blob/v28.5.2/api/types/container/config.go>.

The current command arguments to preserve are `--manifest`, `--worker`, `--run-id`, `--ingest-url`, `--ingest-token`, `--result-ingest-url`, `--result-ingest-token`, `--output -`, and optional `--frames`. Source: `backend/slurm-gateway/internal/gateway/simops_adapters.go:227-230`.

Call `ContainerStart(ctx, createResp.ID, container.StartOptions{})` after successful create; the SDK start method posts to `/containers/{id}/start`, and the Engine API returns 204 on success, 304 for already started, and 404 for missing container. Sources: <https://github.com/moby/moby/blob/v28.5.2/client/container_start.go>, <https://docs.docker.com/reference/api/engine/version/v1.51/#tag/Container/operation/ContainerStart>, <https://docs.docker.com/reference/api/engine/version/v1.51.yaml>.

## Labels, Env, And Network Propagation

The profile label map includes `simops.run_id`, `simops.worker_id`, `simops.role`, `simops.launch_mode`, `simops.scenario_id`, `simops.worker_image`, and `simops.worker_kind` when a worker kind exists. Source: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:202-215`.

Docker Engine container config supports user-defined key/value labels and `Env` entries as `VAR=value` strings. Sources: <https://docs.docker.com/reference/api/engine/version/v1.51/#tag/Container/operation/ContainerCreate>, <https://docs.docker.com/reference/api/engine/version/v1.51.yaml>, <https://github.com/moby/moby/blob/v28.5.2/api/types/container/config.go>.

Issue #21 asks for label/env/network propagation tests, but the pre-change Docker launcher passed ingest token and URLs as command arguments rather than environment variables. Sources: [issue #21](https://github.com/Nathan-E-White/radiant-prj/issues/21), pre-change `backend/slurm-gateway/internal/gateway/simops_adapters.go:227-230`.

Implementation implication: preserve command-argument parity first; add environment variables only if the adapter deliberately defines them, and then test their exact `KEY=value` rendering from `RunConnectionProfile`. Sources: `backend/slurm-gateway/internal/gateway/simops_adapters.go:227-230`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:20-87`, <https://github.com/moby/moby/blob/v28.5.2/api/types/container/config.go>.

Docker Engine `HostConfig.NetworkMode` accepts standard network modes and custom network names, and `NetworkingConfig.EndpointsConfig` maps network names to endpoint configuration. Sources: <https://docs.docker.com/reference/api/engine/version/v1.51.yaml>, <https://github.com/moby/moby/blob/v28.5.2/api/types/container/hostconfig.go>, <https://github.com/moby/moby/blob/v28.5.2/api/types/network/network.go>, <https://github.com/moby/moby/blob/v28.5.2/api/types/network/endpoint.go>.

Implementation implication: for the current one-network case, map `profile.Runtime.Docker.Network` to `container.HostConfig.NetworkMode` and, when non-empty, also provide a `network.NetworkingConfig{EndpointsConfig: map[string]*network.EndpointSettings{networkName: {}}}` so tests can prove network propagation structurally instead of by string search. Sources: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:151-156`, <https://github.com/moby/moby/blob/v28.5.2/api/types/container/hostconfig.go>, <https://github.com/moby/moby/blob/v28.5.2/api/types/network/network.go>.

## Auto-Remove And Cleanup

The pre-change CLI path mapped profile auto-remove to `docker run --rm`. Source: pre-change `backend/slurm-gateway/internal/gateway/simops_adapters.go:224-226`.

Docker Engine exposes the same behavior as `HostConfig.AutoRemove`, described as automatically removing the container when the container process exits and as ineffective when restart policy is set. Sources: <https://docs.docker.com/reference/api/engine/version/v1.51.yaml>, <https://github.com/moby/moby/blob/v28.5.2/api/types/container/hostconfig.go>.

Do not make `AutoRemove` the only cleanup path: issue #21 requires stop behavior that can target launched worker containers reliably, and the current code already lists by run label and then stops/removes matching containers. Sources: [issue #21](https://github.com/Nathan-E-White/radiant-prj/issues/21), `backend/slurm-gateway/internal/gateway/simops_adapters.go:257-300`.

Use `container.RemoveOptions{Force: true}` for rollback/explicit stop cleanup; the SDK remove method maps `Force`, `RemoveVolumes`, and `RemoveLinks` to query parameters before calling `DELETE /containers/{id}`. Sources: <https://github.com/moby/moby/blob/v28.5.2/client/container_remove.go>, <https://github.com/moby/moby/blob/v28.5.2/api/types/container/options.go>, <https://docs.docker.com/reference/api/engine/version/v1.51/#tag/Container/operation/ContainerDelete>.

## Stop-By-Label Strategy

Docker Engine `GET /containers/json` supports `all=true` to include stopped containers and supports label filters such as `label=key` or `label="key=value"`. Sources: <https://docs.docker.com/reference/api/engine/version/v1.51/#tag/Container/operation/ContainerList>, <https://docs.docker.com/reference/api/engine/version/v1.51.yaml>.

The SDK list method maps `container.ListOptions.All` and `container.ListOptions.Filters` to the Engine list request. Sources: <https://github.com/moby/moby/blob/v28.5.2/client/container_list.go>, <https://github.com/moby/moby/blob/v28.5.2/api/types/container/options.go>.

The Docker filter package builds multi-value filter args with `filters.NewArgs(filters.Arg(...))`, so the run stop query can use `filters.NewArgs(filters.Arg("label", "simops.run_id="+runID))` and add `simops.worker_id` or `simops.worker_kind` filters for narrower operations later. Sources: <https://github.com/moby/moby/blob/v28.5.2/api/types/filters/parse.go>, <https://docs.docker.com/reference/api/engine/version/v1.51.yaml>.

Stop each listed container with `ContainerStop(ctx, id, container.StopOptions{Timeout: &seconds})` and then remove it with `ContainerRemove(ctx, id, container.RemoveOptions{Force: true})`; the SDK stop source documents the timeout semantics and maps the timeout to the `t` query parameter. Sources: <https://github.com/moby/moby/blob/v28.5.2/client/container_stop.go>, <https://github.com/moby/moby/blob/v28.5.2/api/types/container/config.go>, <https://github.com/moby/moby/blob/v28.5.2/client/container_remove.go>.

## Error Mapping

Image preflight should replace `docker image inspect` with `ImageInspect(ctx, profile.WorkerImage)` because the current shell path fails before launch when the configured image is missing. Sources: `backend/slurm-gateway/internal/gateway/simops_adapters.go:243-255`, <https://github.com/moby/moby/blob/v28.5.2/client/image_inspect.go>.

Map Docker Engine 404 from create as `image_not_found` when the missing object is the configured image, because the Engine create operation documents 404 as "no such image". Sources: <https://docs.docker.com/reference/api/engine/version/v1.51/#tag/Container/operation/ContainerCreate>, <https://docs.docker.com/reference/api/engine/version/v1.51.yaml>.

Map create 409 as a launch conflict, and if the conflict is a generated container-name conflict, clean it up only when the existing container carries matching SimOps labels. Sources: <https://docs.docker.com/reference/api/engine/version/v1.51/#tag/Container/operation/ContainerCreate>, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:239-241`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:202-215`.

If create succeeds and start fails, remove the created container before returning the launch error, because issue #21 asks for structured launch errors and reliable cleanup while the SDK separates create and start into two operations. Sources: [issue #21](https://github.com/Nathan-E-White/radiant-prj/issues/21), <https://github.com/moby/moby/blob/v28.5.2/client/container_create.go>, <https://github.com/moby/moby/blob/v28.5.2/client/container_start.go>, <https://github.com/moby/moby/blob/v28.5.2/client/container_remove.go>.

Treat 404/not-found during stop/remove as idempotent cleanup success, but do not treat 404 during create/start as success; the Engine API documents 404 for missing image/container on create/start/stop/remove, and the current stop path is explicitly cleanup-oriented. Sources: <https://docs.docker.com/reference/api/engine/version/v1.51.yaml>, `backend/slurm-gateway/internal/gateway/simops_adapters.go:257-300`.

Map start 304 as `container_already_started` and stop 304 as an idempotent already-stopped condition, because Engine start/stop document 304 for those states. Sources: <https://docs.docker.com/reference/api/engine/version/v1.51/#tag/Container/operation/ContainerStart>, <https://docs.docker.com/reference/api/engine/version/v1.51/#tag/Container/operation/ContainerStop>, <https://docs.docker.com/reference/api/engine/version/v1.51.yaml>.

## OrbStack And Local Docker Compatibility

OrbStack includes a Docker engine, creates a Docker context named `orbstack`, can create `/var/run/docker.sock` as a compatibility symlink, and describes its Docker environment as highly compatible with Docker Desktop-style tooling. Source: <https://docs.orbstack.dev/docker/>.

The v28.5.2 SDK default Docker host on non-Windows is `unix:///var/run/docker.sock`, and `client.FromEnv` reads `DOCKER_HOST`, `DOCKER_API_VERSION`, Docker TLS env vars, and related version settings. Sources: <https://github.com/moby/moby/blob/v28.5.2/client/client_unix.go>, <https://github.com/moby/moby/blob/v28.5.2/client/options.go>, <https://github.com/moby/moby/blob/v28.5.2/client/envvars.go>.

The repo's compose-local gateway mounts `/var/run/docker.sock` into the container, so the SDK default socket path matches the existing compose wiring when that socket is present. Sources: `deploy/slurm-gateway.compose.yml:70-76`, <https://github.com/moby/moby/blob/v28.5.2/client/client_unix.go>.

Implementation implication: keep adapter configuration daemon-agnostic by using the standard SDK client construction path and reserve real OrbStack compatibility for smoke coverage after issue #21; issue #21 itself only requires unit-level SDK adapter behavior plus compatibility with the local OrbStack-backed Docker path. Sources: [issue #21](https://github.com/Nathan-E-White/radiant-prj/issues/21), <https://docs.orbstack.dev/docker/>, <https://github.com/moby/moby/blob/v28.5.2/client/client.go>.

## Recommended Test Matrix For Issue #21

Use fake-client tests for fast unit coverage and keep real Docker/OrbStack proof for a later smoke gate, because issue #21 explicitly says to use a fake Docker client if practical and reserve real Docker for smoke coverage. Source: [issue #21](https://github.com/Nathan-E-White/radiant-prj/issues/21).

| Test | Behavior Proven | Sources |
| --- | --- | --- |
| Create request construction | `RunConnectionProfile` becomes `container.Config`, `container.HostConfig`, `network.NetworkingConfig`, and name | [issue #21](https://github.com/Nathan-E-White/radiant-prj/issues/21), `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:20-87`, <https://github.com/moby/moby/blob/v28.5.2/client/container_create.go> |
| Label propagation | `simops.run_id`, `simops.worker_id`, `simops.worker_kind`, role, launch mode, scenario, and image labels are present | `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:202-215`, <https://github.com/moby/moby/blob/v28.5.2/api/types/container/config.go> |
| Command/env propagation | Current CLI args are preserved in `Config.Cmd`; env is rendered only if deliberately added | `backend/slurm-gateway/internal/gateway/simops_adapters.go:227-230`, <https://github.com/moby/moby/blob/v28.5.2/api/types/container/config.go> |
| Network propagation | Docker network becomes structured host/networking config, not a shell string | `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:151-156`, <https://github.com/moby/moby/blob/v28.5.2/api/types/container/hostconfig.go>, <https://github.com/moby/moby/blob/v28.5.2/api/types/network/network.go> |
| Launch error mapping | image inspect, create, start, conflict, not-found, and cleanup failures return structured launch errors | [issue #21](https://github.com/Nathan-E-White/radiant-prj/issues/21), <https://docs.docker.com/reference/api/engine/version/v1.51.yaml> |
| Partial launch rollback | created or previously launched run containers are stopped/removed by label when later worker launch fails | `backend/slurm-gateway/internal/gateway/simops_adapters.go:140-149`, `backend/slurm-gateway/internal/gateway/simops_adapters.go:257-300`, <https://github.com/moby/moby/blob/v28.5.2/client/container_list.go> |
| Stop by identity | `StopRun` lists all containers with the run label and stops/removes matches idempotently | [issue #21](https://github.com/Nathan-E-White/radiant-prj/issues/21), <https://docs.docker.com/reference/api/engine/version/v1.51/#tag/Container/operation/ContainerList>, <https://github.com/moby/moby/blob/v28.5.2/api/types/filters/parse.go> |

## Implementation-Relevant Conclusion

The replacement should be a concrete Docker runtime adapter behind the existing SimOps runtime/spooler seam, not a change to the browser-facing SimOps contract or the profile module. Sources: [issue #21](https://github.com/Nathan-E-White/radiant-prj/issues/21), `backend/slurm-gateway/internal/gateway/simops_controller.go:34-45`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:20-87`.

The adapter's deep module interface should stay small: accept `RunConnectionProfile`, use a narrow fakeable Docker client, return structured container launch metadata/errors, and keep Docker SDK imports out of the runtime-neutral profile/controller core. Sources: [issue #20](https://github.com/Nathan-E-White/radiant-prj/issues/20), [issue #21](https://github.com/Nathan-E-White/radiant-prj/issues/21), <https://github.com/moby/moby/blob/v28.5.2/client/client_interfaces.go>.
