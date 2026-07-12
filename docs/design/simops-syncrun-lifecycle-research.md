# SimOps SyncRun Lifecycle Research

| Field | Value |
| --- | --- |
| Document ID | SIMOPS-SYNCRUN-LIFECYCLE-RESEARCH-001 |
| Revision | 0.1 |
| Status | Research note |
| Owner | Software |
| Scope | Issue #22 research baseline for runtime-neutral SimOps lifecycle observation |

## Purpose

This note records primary-source research for Radiant issue #22, "V3: Add runtime-neutral SimOps SyncRun lifecycle observation." Issue #22 asks to deepen the SimOps runtime adapter contract from launch/stop into launch/stop/sync so the control plane can observe runtime lifecycle without confusing it with telemetry or data-plane health. Source: [issue #22](https://github.com/Nathan-E-White/radiant-prj/issues/22).

This is a research-only note. It does not implement `SyncRun`, add tests, change branches, or modify runtime behavior.

## Scope And Dependency Status

Dependency gate for this note:

- Parent PRD #19 is open, but the user explicitly said to ignore it as a parent PRD gate for this issue. Source: [issue #19](https://github.com/Nathan-E-White/radiant-prj/issues/19).
- Issue #20 is closed. Closeout says `RunConnectionProfile` centralizes ordinary worker and trusted data-plane profile construction, default `internal/gateway` builds exclude heavyweight concrete adapter dependencies, and final amended implementation is `f5b0281` tagged `v4.2.2`. Sources: [issue #20](https://github.com/Nathan-E-White/radiant-prj/issues/20), [issue #20 closeout comment](https://github.com/Nathan-E-White/radiant-prj/issues/20#issuecomment-4933959767), [issue #20 amended closeout comment](https://github.com/Nathan-E-White/radiant-prj/issues/20#issuecomment-4933980194).
- Issue #21 is closed. Closeout says PR #28 implemented the Docker SDK adapter on branch `codex/issue-21-docker-sdk-simops-launcher` at commit `9353041`, tagged `v4.2.3`, with `go test ./internal/gateway ./internal/simopsdocker -run 'TestDefaultSimopsController|TestSpooler'`, `bun run backend:test`, `bun run backend:deps:check`, `bun run quality:check`, and `bun run ci` passing. Sources: [issue #21](https://github.com/Nathan-E-White/radiant-prj/issues/21), [issue #21 closeout comment](https://github.com/Nathan-E-White/radiant-prj/issues/21#issuecomment-4934417115).
- Issue #22 is open and ready. Source: [issue #22](https://github.com/Nathan-E-White/radiant-prj/issues/22).

Issue #22 acceptance asks for an independent runtime lifecycle state, `SyncRun` observed worker states, explicit missing runtime resources, controller/store updates without runtime-specific branches, and state names documented in code or near the interface. Source: [issue #22](https://github.com/Nathan-E-White/radiant-prj/issues/22).

## Source Set

Primary sources used:

- Radiant issue text and closeout notes: [issue #19](https://github.com/Nathan-E-White/radiant-prj/issues/19), [issue #20](https://github.com/Nathan-E-White/radiant-prj/issues/20), [issue #21](https://github.com/Nathan-E-White/radiant-prj/issues/21), [issue #22](https://github.com/Nathan-E-White/radiant-prj/issues/22).
- Local Radiant context and source: `CONTEXT.md`, `backend/slurm-gateway/internal/gateway/simops_adapters.go`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go`, `backend/slurm-gateway/internal/gateway/simops_controller.go`, `backend/slurm-gateway/internal/gateway/simops_types.go`, `backend/slurm-gateway/internal/gateway/simops_store.go`, `backend/slurm-gateway/internal/gateway/simops_postgres_store.go`, `backend/slurm-gateway/internal/simopsdocker/spooler.go`, `backend/slurm-gateway/cmd/server/main.go`, and `docs/design/simops-docker-sdk-adapter-research.md`.
- Docker Engine API v1.51 reference and OpenAPI spec: <https://docs.docker.com/reference/api/engine/version/v1.51/> and <https://docs.docker.com/reference/api/engine/version/v1.51.yaml>.
- Moby/Docker SDK source for v28.5.2: <https://github.com/moby/moby/tree/v28.5.2/client> and <https://github.com/moby/moby/blob/v28.5.2/api/types/container/container.go>.
- Kubernetes docs and source: Pod lifecycle docs <https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/>, image pull docs <https://kubernetes.io/docs/concepts/containers/images/>, Job docs <https://kubernetes.io/docs/concepts/workloads/controllers/job/>, Kubernetes API reference for Pod and Job status <https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/> and <https://kubernetes.io/docs/reference/kubernetes-api/batch/job-v1/>, Kubernetes API source <https://github.com/kubernetes/api/blob/master/core/v1/types.go> and <https://github.com/kubernetes/api/blob/master/batch/v1/types.go>, kubelet image error source <https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/images/types.go>.
- client-go fake testing source for future Kubernetes adapter tests: <https://github.com/kubernetes/client-go/blob/master/kubernetes/fake/clientset_generated.go> and <https://github.com/kubernetes/client-go/blob/master/testing/fake.go>.

## Local Current-State Findings

Radiant's glossary already distinguishes `Run-Scoped Simulation Worker`, `Run Connection Profile`, and `SimOps Runtime Adapter`. A run-scoped worker produces operational telemetry or simulated result state for a single run; the profile carries worker identity, ingest connectivity, runtime labels, cleanup policy, and credential boundaries; the runtime adapter turns that profile into an external worker execution record while preserving the SimOps run interface. Source: `CONTEXT.md:41-51`.

The current runtime contract is still launch/stop-shaped. `SimopsSpooler` exposes `StartRun` and `StopRun`; profile-aware launch and stop are optional secondary interfaces; `SimopsRuntime` also only exposes `StartRun` and `StopRun`. Source: `backend/slurm-gateway/internal/gateway/simops_adapters.go:12-36`.

`ContractSimopsSpooler` returns worker records with `Lifecycle: SimopsStarting` and no sync behavior. Source: `backend/slurm-gateway/internal/gateway/simops_adapters.go:43-89`.

`RunConnectionProfile` is the right launch context for issue #22 because it already carries run identity, worker identity, worker kind, runtime labels, Docker container name, Kubernetes namespace/job name, cleanup policy, gateway ingest URLs, and optional data-plane refs. Source: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:20-87`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:137-170`.

Ordinary worker profiles deliberately exclude data-plane refs, while trusted roles can receive Redpanda, Postgres, and Iceberg fields. Source: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:101-115`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:217-236`.

The controller creates planned worker records before launch, delegates launch through either `RunConnectionProfileSpooler` or legacy `SimopsSpooler`, then moves the run to `SimopsStreaming` after launch records are saved. Source: `backend/slurm-gateway/internal/gateway/simops_controller.go:126-215`.

Stop currently updates the run lifecycle to `SimopsStopped` and marks each worker stopped through `UpdateWorkerFrames(..., SimopsStopped, 0)`. This is an explicit operator/control-plane stop path, not a runtime observation path. Source: `backend/slurm-gateway/internal/gateway/simops_controller.go:247-300`.

Telemetry ingest currently publishes `worker.telemetry` and updates worker frames plus worker lifecycle to `SimopsStreaming`. That means telemetry arrival can move worker lifecycle today, which issue #22 explicitly wants separated from runtime lifecycle observation. Source: `backend/slurm-gateway/internal/gateway/simops_controller.go:322-358`.

The response shape exposes run lifecycle, workers, spool commands, and artifacts together. Source: `backend/slurm-gateway/internal/gateway/simops_types.go:46-60`.

The current `SimopsLifecycle` values are product/control-plane lifecycle terms: `created`, `starting`, `streaming`, `degraded`, `complete`, `failed`, and `stopped`. Issue #22's neutral worker observations are a different vocabulary: `pending`, `active`, `succeeded`, `failed`, `missing`, `image-pull-failed`, and `stopped`. Source: `backend/slurm-gateway/internal/gateway/simops_types.go:8-18`, [issue #22](https://github.com/Nathan-E-White/radiant-prj/issues/22).

The in-memory and Postgres stores can update run lifecycle and worker lifecycle, but the worker update method is named `UpdateWorkerFrames` and couples lifecycle updates to frame-count deltas. Source: `backend/slurm-gateway/internal/gateway/simops_store.go:13-28`, `backend/slurm-gateway/internal/gateway/simops_store.go:177-193`, `backend/slurm-gateway/internal/gateway/simops_postgres_store.go:396-408`.

The Docker adapter from issue #21 is already in `internal/simopsdocker`, behind a narrow fakeable `DockerClient` with image inspect, create, start, list, stop, and remove methods. It currently does not expose `ContainerInspect`. Source: `backend/slurm-gateway/internal/simopsdocker/spooler.go:21-28`.

The Docker adapter starts worker containers from `RunConnectionProfile`, returns launch records in `SimopsStarting`, and stores container ID/name/image/worker metadata in labels or command metadata. Source: `backend/slurm-gateway/internal/simopsdocker/spooler.go:90-138`, `backend/slurm-gateway/internal/simopsdocker/spooler.go:320-347`.

The Docker adapter stops by listing containers with run/runtime/role labels, filtering by worker identity, then stopping and removing matches. Source: `backend/slurm-gateway/internal/simopsdocker/spooler.go:204-239`, `backend/slurm-gateway/internal/simopsdocker/spooler.go:384-404`.

The concrete server wires Docker runtime only in `cmd/server`, leaving the gateway core runtime-neutral. Source: `backend/slurm-gateway/cmd/server/main.go:16-25`.

The existing issue #21 research note already recommends keeping Docker SDK imports behind the adapter and keeping the runtime adapter's interface small. Source: `docs/design/simops-docker-sdk-adapter-research.md:149-153`.

## Recommended Neutral Model

Add a runtime-neutral observed worker lifecycle type near the runtime adapter interface, not in Docker or Kubernetes packages:

```go
type ObservedWorkerState string

const (
	ObservedWorkerPending         ObservedWorkerState = "pending"
	ObservedWorkerActive          ObservedWorkerState = "active"
	ObservedWorkerSucceeded       ObservedWorkerState = "succeeded"
	ObservedWorkerFailed          ObservedWorkerState = "failed"
	ObservedWorkerMissing         ObservedWorkerState = "missing"
	ObservedWorkerImagePullFailed ObservedWorkerState = "image-pull-failed"
	ObservedWorkerStopped         ObservedWorkerState = "stopped"
)
```

Recommended observation payload:

```go
type ObservedWorkerLifecycle struct {
	RunID      string
	WorkerID   string
	WorkerKind SimopsWorkerKind
	State      ObservedWorkerState
	Runtime    string
	RuntimeID  string
	Reason     string
	Message    string
	ExitCode   *int
	ObservedAt time.Time
	Labels     map[string]string
}
```

`SyncRun` should return this neutral payload, not Docker `container.Summary`, Docker `container.InspectResponse`, Kubernetes `Job`, or Kubernetes `Pod`. That keeps controller/store code from growing runtime-specific branches, which is an issue #22 acceptance criterion. Source: [issue #22](https://github.com/Nathan-E-White/radiant-prj/issues/22).

Recommended interface shape:

```go
type RunConnectionProfileSyncer interface {
	SyncRunProfiles(ctx context.Context, run SimopsRunRecord, profiles []RunConnectionProfile) ([]ObservedWorkerLifecycle, error)
}
```

The controller can rebuild profiles from stored workers the same way stop already does, call the sync interface if present, and pass neutral observations to the store. Source for existing stop/profile rebuild pattern: `backend/slurm-gateway/internal/gateway/simops_controller.go:282-300`.

## Docker State Mapping Recommendation

Docker Engine `GET /containers/json` returns a summary with machine `State` plus human-readable `Status`, and supports label filters plus `all=true`. Sources: <https://docs.docker.com/reference/api/engine/version/v1.51/#tag/Container/operation/ContainerList>, <https://docs.docker.com/reference/api/engine/version/v1.51.yaml>, `backend/slurm-gateway/internal/simopsdocker/spooler.go:204-217`.

Docker `ContainerInspect` returns `State` with `Status`, `Running`, `Paused`, `Restarting`, `OOMKilled`, `Dead`, `ExitCode`, `Error`, `StartedAt`, `FinishedAt`, and optional `Health`. Source: <https://github.com/moby/moby/blob/v28.5.2/api/types/container/container.go>.

The Docker adapter should add `ContainerInspect(ctx, containerID)` to its narrow `DockerClient`. It should list by labels to discover expected worker containers, then inspect matching containers for exit code, error, OOM, and exact status. Sources for current narrow client and list strategy: `backend/slurm-gateway/internal/simopsdocker/spooler.go:21-28`, `backend/slurm-gateway/internal/simopsdocker/spooler.go:204-217`; Moby inspect source: <https://github.com/moby/moby/blob/v28.5.2/client/container_inspect.go>.

Recommended Docker mapping:

| Docker observation | Neutral state | Notes |
| --- | --- | --- |
| Expected worker profile has no matching labeled container and no stop intent | `missing` | This is not `failed`; missing is explicit issue #22 behavior. Source: [issue #22](https://github.com/Nathan-E-White/radiant-prj/issues/22). |
| Expected worker profile has no matching labeled container after a controller stop intent | `stopped` | Stop removes containers today, so absence after requested stop should not be misreported as missing. Source: `backend/slurm-gateway/internal/simopsdocker/spooler.go:231-236`, `backend/slurm-gateway/internal/gateway/simops_controller.go:247-300`. |
| `State.Status == "created"` | `pending` | Container exists but has not reached running process state. Source: <https://github.com/moby/moby/blob/v28.5.2/api/types/container/container.go>. |
| `State.Status == "running"` | `active` | Worker process is running. Healthcheck status, if present, should stay detail only, not become data-plane health. Source: <https://github.com/moby/moby/blob/v28.5.2/api/types/container/container.go>. |
| `State.Status == "restarting"` | `active` with `Reason: "restarting"` | Docker says it has started and is under runtime management. The neutral model has no restart-specific state. Source: <https://github.com/moby/moby/blob/v28.5.2/api/types/container/container.go>. |
| `State.Status == "paused"` | `active` with `Reason: "paused"` | Paused is runtime-managed existence, not terminal success/failure. Source: <https://github.com/moby/moby/blob/v28.5.2/api/types/container/container.go>. |
| `State.Status == "exited"` and `ExitCode == 0` and no stop intent | `succeeded` | Worker process finished successfully. Source: <https://github.com/moby/moby/blob/v28.5.2/api/types/container/container.go>. |
| `State.Status == "exited"` and stop intent is present | `stopped` | Preserve user/operator stop semantics over raw exit code. Source: `backend/slurm-gateway/internal/gateway/simops_controller.go:247-300`. |
| `State.Status == "exited"` and `ExitCode != 0` | `failed` | Include `ExitCode`, `Error`, `OOMKilled`, and `FinishedAt` in details. Source: <https://github.com/moby/moby/blob/v28.5.2/api/types/container/container.go>. |
| `State.OOMKilled == true` | `failed` with `Reason: "oom-killed"` | OOM is a worker runtime failure, not telemetry health. Source: <https://github.com/moby/moby/blob/v28.5.2/api/types/container/container.go>. |
| `State.Status == "dead"` or `State.Dead == true` | `failed` | Docker marks dead containers distinctly from ordinary exits. Source: <https://github.com/moby/moby/blob/v28.5.2/api/types/container/container.go>. |
| `State.Status == "removing"` | `stopped` if stop intent exists, otherwise `missing` or `pending` with `Reason: "removing"` | This is transient. The controller's desired stop state must disambiguate cleanup from unexplained disappearance. |
| Launch/image preflight error from `ImageInspect`, future `ImagePull`, or create/start "no such image" | `image-pull-failed` | Current Docker launch uses `ImageInspect` before create/start. Docker Engine image create/pull progress can report `error` and `errorDetail`; create can fail when an image is not available. Sources: `backend/slurm-gateway/internal/simopsdocker/spooler.go:164-197`, <https://github.com/moby/moby/blob/v28.5.2/client/image_pull.go>, <https://docs.docker.com/reference/api/engine/version/v1.51.yaml>. |

Important Docker caveat: with the current SDK adapter, an image failure can happen before a container exists. `SyncRun` cannot infer `image-pull-failed` from Docker container state alone unless the adapter records launch-error evidence or repeats a cheap image inspection for expected missing containers. Source: `backend/slurm-gateway/internal/simopsdocker/spooler.go:164-197`.

## Kubernetes Job And Pod Mapping Preparation

Kubernetes Jobs are the planned runtime primitive for issue #24, but issue #22 should prepare the neutral state model now. Issue #19 says the Kubernetes path should use client-go Jobs and preserve the same SimOps run interface, while issue #22 asks to prepare the same state model for Job/Pod status mapping. Sources: [issue #19](https://github.com/Nathan-E-White/radiant-prj/issues/19), [issue #22](https://github.com/Nathan-E-White/radiant-prj/issues/22).

Kubernetes Jobs create one or more Pods and retry execution until the requested successful completions are reached; deleting a Job cleans up its Pods. Source: <https://kubernetes.io/docs/concepts/workloads/controllers/job/>.

Job status has terminal conditions: a Job that completes has condition type `Complete` with status true, a Job that fails has condition type `Failed` with status true, and a finished Job is in either terminal condition. It also has active, succeeded, failed, ready, terminating, completedIndexes, and failedIndexes counters. Sources: <https://kubernetes.io/docs/reference/kubernetes-api/batch/job-v1/>, <https://github.com/kubernetes/api/blob/master/batch/v1/types.go>.

Pod phase is a high-level summary and is not a comprehensive state machine. The official phases are `Pending`, `Running`, `Succeeded`, `Failed`, and `Unknown`. Sources: <https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/>, <https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/>, <https://github.com/kubernetes/api/blob/master/core/v1/types.go>.

Kubernetes also tracks container states: `Waiting`, `Running`, and `Terminated`; waiting includes a machine-readable reason, terminated includes reason, exit code, started/finished timestamps, and container ID. Sources: <https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/>, <https://github.com/kubernetes/api/blob/master/core/v1/types.go>.

Kubernetes image pull docs define `ImagePullBackOff` as a waiting state caused by Kubernetes being unable to pull a container image, with backoff retries. The kubelet source also defines `ErrImagePull` and `ImagePullBackOff` reason strings. Sources: <https://kubernetes.io/docs/concepts/containers/images/#imagepullbackoff>, <https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/images/types.go>.

Recommended Kubernetes mapping:

| Kubernetes observation | Neutral state | Notes |
| --- | --- | --- |
| Expected worker has no Job and no matching Pod, with no stop intent | `missing` | Missing Kubernetes resources are explicit and should not be folded into failed data-plane health. |
| Expected worker has no Job/Pod after controller stop intent | `stopped` | Deleting a Job cleans up Pods, so absence after requested stop is a stopped observation. Source: <https://kubernetes.io/docs/concepts/workloads/controllers/job/>. |
| Job exists but no terminal condition and no Pod has started | `pending` | Covers admission, scheduling, and initial image work. |
| Pod phase `Pending`, except image-pull reasons | `pending` | Pod phase pending includes time before scheduling and image download. Source: <https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/>. |
| Container `Waiting.Reason` is `ImagePullBackOff`, `ErrImagePull`, `InvalidImageName`, or `ErrImageNeverPull` | `image-pull-failed` | `ImagePullBackOff` is official docs; the other reason strings are kubelet source-owned. Sources: <https://kubernetes.io/docs/concepts/containers/images/#imagepullbackoff>, <https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/images/types.go>. |
| Job `status.active > 0` or Pod phase `Running` or container state `Running` | `active` | Pod running means at least one container is running or starting/restarting. Source: <https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/>. |
| Job condition `Complete=True` or Pod phase `Succeeded` | `succeeded` | Job and Pod sources both model success as terminal successful completion. Sources: <https://kubernetes.io/docs/reference/kubernetes-api/batch/job-v1/>, <https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/>. |
| Job condition `Failed=True`, Pod phase `Failed`, or terminated container exit code nonzero | `failed` | Preserve exit code and reason in details. Sources: <https://kubernetes.io/docs/reference/kubernetes-api/batch/job-v1/>, <https://github.com/kubernetes/api/blob/master/core/v1/types.go>. |
| Pod has `deletionTimestamp` or Job is deleting after stop intent | `stopped` | This is a control-plane stop/cleanup observation, not a worker failure. |
| Pod phase `Unknown` | `failed` with `Reason: "unknown"` or `missing` after timeout | Treat as a risk area. Kubernetes says `Unknown` typically means the state could not be obtained due to communication with the host. Source: <https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/>. |

Future Kubernetes tests can use `client-go` fake clients and reactors/watch reactors to drive Job/Pod status without a live cluster, reserving Kind proof for issue #25. Sources: <https://github.com/kubernetes/client-go/blob/master/kubernetes/fake/clientset_generated.go>, <https://github.com/kubernetes/client-go/blob/master/testing/fake.go>, [issue #25 context in issue #19 child map](https://github.com/Nathan-E-White/radiant-prj/issues/19#issuecomment-4933153528).

## Runtime Lifecycle Boundary

The neutral lifecycle model must answer only: "What does the external runtime currently observe about each expected worker execution resource?"

It must not answer:

- Did the worker send telemetry?
- Did Redpanda accept or retain an event?
- Did Postgres/Timescale project the event?
- Did WebTransport/MoQ stream to the UI?
- Did the Iceberg writer commit or fail an artifact?
- Did a simulated result or imputed state become available?

This boundary is required by issue #22 and matches the domain language. Simulated result state is separate from operational telemetry; imputed state belongs to the twin projector; simulation health summary covers run completion and artifact disposition, not infrastructure diagnostics. Sources: [issue #22](https://github.com/Nathan-E-White/radiant-prj/issues/22), `CONTEXT.md:19-27`, `CONTEXT.md:37-43`.

The current profile module already protects this boundary by omitting data-plane refs from ordinary worker profiles and reserving Redpanda/Postgres/Iceberg refs for trusted roles. Source: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:101-115`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:217-236`.

The current controller and config show why the boundary matters: telemetry log choice, Redpanda topic/brokers, Postgres control store, Iceberg writer, artifact planner, and runtime worker launch all live near each other, so a single "health" bucket would turn into mush fast. Sources: `backend/slurm-gateway/internal/gateway/config.go:37-72`, `backend/slurm-gateway/internal/gateway/simops_controller.go:55-77`, `backend/slurm-gateway/internal/gateway/simops_controller.go:189-202`.

Recommended wording near the interface:

> Observed worker lifecycle is runtime-resource state only. It is not telemetry health, artifact disposition, Redpanda/Postgres/Iceberg health, or simulated result quality.

## Suggested TDD Seams

Do not implement these tests in the research slice. These are suggested seams for issue #22 planning:

| Seam | Test intent | Why this seam |
| --- | --- | --- |
| Gateway runtime contract seam | Fake adapter returns `pending`, `active`, `succeeded`, `failed`, `missing`, `image-pull-failed`, and `stopped`; controller applies observations to workers without Docker/Kubernetes branches. | Exercises issue #22 acceptance through the caller-facing SimOps control plane. Source: `backend/slurm-gateway/internal/gateway/simops_controller.go:207-215`, [issue #22](https://github.com/Nathan-E-White/radiant-prj/issues/22). |
| Store lifecycle update seam | Store can update worker lifecycle/state without incrementing telemetry frame counts. | Current store method couples lifecycle and `framesDelta`. Source: `backend/slurm-gateway/internal/gateway/simops_store.go:177-193`, `backend/slurm-gateway/internal/gateway/simops_postgres_store.go:396-408`. |
| Docker adapter mapping seam | Fake Docker client list/inspect responses map to every neutral state, including not-found/missing, exit code 0, nonzero exit, OOM, dead, paused/restarting, and stop intent. | Fast, source-owned Docker state mapping without real Docker. Source: `backend/slurm-gateway/internal/simopsdocker/spooler.go:21-28`, <https://github.com/moby/moby/blob/v28.5.2/api/types/container/container.go>. |
| Docker image failure seam | Fake image inspect/create/pull failures produce `image-pull-failed` without requiring a container to exist. | Current Docker image failure can happen before a container resource exists. Source: `backend/slurm-gateway/internal/simopsdocker/spooler.go:164-197`. |
| Boundary seam | A worker telemetry ingest changes telemetry/frame state but does not overwrite runtime observed lifecycle; Redpanda/Postgres/Iceberg/artifact errors are not accepted as runtime lifecycle observations. | Guards the main issue #22 semantic boundary. Source: `backend/slurm-gateway/internal/gateway/simops_controller.go:322-358`, `backend/slurm-gateway/internal/gateway/simops_controller.go:189-202`, [issue #22](https://github.com/Nathan-E-White/radiant-prj/issues/22). |
| Future Kubernetes mapping seam | Fake client-go Job/Pod status maps to the same neutral states. | Prepares issue #24 without requiring client-go in gateway core. Sources: <https://github.com/kubernetes/client-go/blob/master/kubernetes/fake/clientset_generated.go>, <https://github.com/kubernetes/client-go/blob/master/testing/fake.go>. |

## Unknowns And Risks

1. Docker `image-pull-failed` is not always an inspectable container state. The current adapter preflights with `ImageInspect` and then `ContainerCreate`/`ContainerStart`; if the image is absent, there may be no container to sync. The implementation likely needs to retain launch-error evidence or re-check expected images during sync. Source: `backend/slurm-gateway/internal/simopsdocker/spooler.go:164-197`.

2. `stopped` versus `missing` requires desired-state context. A removed Docker container or deleted Kubernetes Job is indistinguishable from "never existed" unless the controller/store records stop intent or current run lifecycle. Source: `backend/slurm-gateway/internal/gateway/simops_controller.go:247-300`.

3. Current `SimopsLifecycle` is broader than runtime observation. Reusing it directly for `pending`/`active` would blur product/control-plane lifecycle, telemetry-derived lifecycle, and runtime-resource lifecycle. Source: `backend/slurm-gateway/internal/gateway/simops_types.go:8-18`, [issue #22](https://github.com/Nathan-E-White/radiant-prj/issues/22).

4. Store method naming and behavior are currently telemetry-shaped. `UpdateWorkerFrames` updates both frame counts and lifecycle; issue #22 will likely need a separate worker lifecycle update method or a neutral sync application method. Source: `backend/slurm-gateway/internal/gateway/simops_store.go:177-193`.

5. Docker `AutoRemove` can delete successful containers before sync observes `succeeded`. The current Docker adapter supports auto-remove through the profile and host config. Issue #22 may need launch command metadata, stop intent, or run-scoped observation timing to prevent success from collapsing into `missing`. Source: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:151-165`, `backend/slurm-gateway/internal/simopsdocker/spooler.go:298-305`.

6. Kubernetes Pod phase is intentionally coarse and not a full state machine. Mapping should use Job terminal conditions first, then Pod phase, then container waiting/running/terminated detail. Source: <https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/>, <https://kubernetes.io/docs/reference/kubernetes-api/batch/job-v1/>.

7. Avoid importing client-go into `internal/gateway` during issue #22. Issue #20 explicitly kept gateway core free of heavyweight concrete adapter deps; Kubernetes support belongs behind a concrete adapter package later. Sources: [issue #20 closeout comment](https://github.com/Nathan-E-White/radiant-prj/issues/20#issuecomment-4933980194), `docs/design/simops-docker-sdk-adapter-research.md:149-153`.

## Implementation-Relevant Conclusion

Issue #22 should introduce a small neutral `SyncRun` interface and observed worker state type near the SimOps runtime adapter contract. Docker and future Kubernetes adapters should translate their runtime-specific observations into that type internally. The controller/store should consume only neutral observations, preserving the existing SimOps run interface and keeping telemetry, artifact, Redpanda, Postgres, and Iceberg health out of runtime lifecycle. Sources: [issue #22](https://github.com/Nathan-E-White/radiant-prj/issues/22), `backend/slurm-gateway/internal/gateway/simops_adapters.go:12-36`, `backend/slurm-gateway/internal/gateway/simops_controller.go:207-215`, `CONTEXT.md:41-51`.
