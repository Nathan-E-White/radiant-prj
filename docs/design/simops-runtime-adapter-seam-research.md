# SimOps Runtime Adapter Seam Research

| Field | Value |
| --- | --- |
| Document ID | SIMOPS-RUNTIME-ADAPTER-SEAM-RESEARCH-001 |
| Revision | 0.1 |
| Status | Research note |
| Scope | External-runtime facts that constrain Issue #129 |

## Purpose and method

This note separates what Docker Engine, Kubernetes, and Go specify from the
repository decisions proposed for Issue #129. It uses only first-party
documentation and upstream source. It does not define an implementation.

Radiant terms retain their glossary meanings: a **Run Connection Profile** is
the per-run launch contract; a **SimOps Runtime Adapter** turns that profile
into an external execution record; the control plane remains authoritative for
the SimOps Run and its outcome. [ADR-0010](../adr/adr-0010.md) is the
repository decision that establishes that ownership split.

## Source set

- [Docker Engine API v1.51 reference](https://docs.docker.com/reference/api/engine/version/v1.51/)
- [Docker Engine API OpenAPI specification v1.51](https://docs.docker.com/reference/api/engine/version/v1.51.yaml)
- [Moby client package source](https://github.com/moby/moby/tree/v28.5.2/client)
- [Kubernetes Jobs](https://kubernetes.io/docs/concepts/workloads/controllers/job/)
- [Kubernetes Pod lifecycle](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/)
- [Kubernetes cascading deletion](https://kubernetes.io/docs/tasks/administer-cluster/use-cascading-deletion/)
- [Kubernetes Job v1 API reference](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/job-v1/)
- [Go `context` package](https://pkg.go.dev/context)
- [Go Concurrency Patterns: Context](https://go.dev/blog/context)

## Source-backed facts

### Start: resource creation and identity are runtime-specific

| Runtime | Source-backed fact | Constraint on a common adapter interface |
| --- | --- | --- |
| Docker Engine | Creating a container and starting a container are distinct Engine operations; Docker identifies the created resource with a container ID, while a caller may also supply a name. The Engine supports labels on the created container. [Engine API](https://docs.docker.com/reference/api/engine/version/v1.51/), [OpenAPI spec](https://docs.docker.com/reference/api/engine/version/v1.51.yaml) | A launch result must preserve an opaque external execution identity and stable labels. It cannot make a Docker container name the universal identity. |
| Kubernetes | A Job creates Pods until it meets a success or failure condition. The Job's object identity and its Pods are distinct; Job status reports active, succeeded, failed, and conditions. [Jobs](https://kubernetes.io/docs/concepts/workloads/controllers/job/), [Job v1 API](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/job-v1/) | A launch result must permit one runtime execution record to represent a controller resource whose work appears in subordinate resources. |
| Go | A `Context` carries deadlines, cancellation signals, and request-scoped values across call boundaries; code should pass it explicitly and must not retain it in a struct. [context package](https://pkg.go.dev/context), [Go blog](https://go.dev/blog/context) | Start must accept a caller-owned context. An adapter must not invent unbounded work after the caller's deadline or cancellation signal. |

### Observe: state must be evidence, not inferred domain outcome

| Runtime | Source-backed fact | Constraint on observation and result mapping |
| --- | --- | --- |
| Docker Engine | Container inspection exposes runtime state such as running, paused, restarting, dead, exit code, error, and finish time. Listing can filter containers by labels, which makes labels a discovery mechanism rather than an outcome model. [Engine API](https://docs.docker.com/reference/api/engine/version/v1.51/), [OpenAPI spec](https://docs.docker.com/reference/api/engine/version/v1.51.yaml) | The adapter can report observed resource facts and a runtime reason, but cannot collapse a Docker exit code into the SimOps Run's authoritative domain outcome. |
| Kubernetes | Pod phase is a high-level summary and is deliberately not a comprehensive state machine. Pod conditions and container waiting/terminated states carry additional evidence; Job conditions and counters describe Job progress and terminal result. [Pod lifecycle](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/), [Jobs](https://kubernetes.io/docs/concepts/workloads/controllers/job/) | An adapter must inspect the appropriate runtime evidence (Job plus Pods for Kubernetes) before mapping to Radiant's `Observed Worker Lifecycle`. A single generic `status` field is insufficient. |
| Kubernetes | The Job controller can replace failed or terminating Pods, and completion is determined by the Job's configured completion semantics. [Jobs](https://kubernetes.io/docs/concepts/workloads/controllers/job/) | Observation must be keyed to the Run Connection Profile's stable run/worker labels and external execution record, not to an assumed one-Pod-per-worker identity. |

### Stop, cancel, and cleanup: deletion is asynchronous and not equivalent to completion

| Runtime | Source-backed fact | Constraint on a common adapter interface |
| --- | --- | --- |
| Docker Engine | The Engine exposes distinct stop and delete operations. Stop can use a timeout before a forced kill; delete can force removal. [Engine API](https://docs.docker.com/reference/api/engine/version/v1.51/), [OpenAPI spec](https://docs.docker.com/reference/api/engine/version/v1.51.yaml) | `stop` and `cleanup` have different mechanics even when a local policy calls them together. A runtime adapter should make its returned facts distinguish requested stop, missing resource, and delete failure. |
| Kubernetes | Deleting an owner can use background, foreground, or orphan propagation. Background deletion removes the owner immediately and cleans dependents in the background; foreground waits for dependents; orphan leaves dependents. [Cascading deletion](https://kubernetes.io/docs/tasks/administer-cluster/use-cascading-deletion/) | Stop/cleanup cannot promise synchronous disappearance of Job Pods. The adapter must report an observation after a delete request rather than claim that a delete request proves cleanup completed. |
| Kubernetes | `ttlSecondsAfterFinished` applies after a Job is finished; it is not the mechanism to cancel an active Job. [Jobs](https://kubernetes.io/docs/concepts/workloads/controllers/job/) | Cleanup TTL belongs in the profile as runtime configuration, but explicit lifecycle compensation must use a stop/delete operation and not wait for TTL cleanup. |
| Go | `WithCancel`, `WithDeadline`, and `WithTimeout` derive a child context; calling the returned cancel function releases associated resources. `WithoutCancel` returns a derived context that is not canceled when its parent is canceled. [context package](https://pkg.go.dev/context) | Bounded recovery may deliberately use a detached, timeout-bounded context, but this is lifecycle policy owned by the control plane. The adapter must honor the context passed to each call. |

### Idempotency and errors: normalize only what the sources make comparable

| Fact | Constraint |
| --- | --- |
| Docker's create can conflict when a requested name is already in use; Kubernetes object creation can return an already-exists conflict. Their resource names and conflict meanings are runtime-specific. [Docker Engine API](https://docs.docker.com/reference/api/engine/version/v1.51/), [Kubernetes API concepts](https://kubernetes.io/docs/concepts/overview/working-with-objects/names/) | Do not equate a name conflict with a successful launch. The adapter needs enough external identity and labels for the control plane to reconcile whether the extant resource belongs to this Run. |
| Kubernetes deletion is asynchronous under cascading propagation, and a deleted or already-absent object has different operational evidence from a failed delete request. [Cascading deletion](https://kubernetes.io/docs/tasks/administer-cluster/use-cascading-deletion/) | Cleanup should be idempotent at the lifecycle-policy level: an absent resource can satisfy desired cleanup, while authorization, transport, and invalid-request failures remain errors. The adapter should retain a runtime reason/cause for the policy to record. |
| Go recommends checking `ctx.Err()` when a context is done; cancellation and deadline expiration are distinguishable sentinels. [context package](https://pkg.go.dev/context) | Do not translate caller cancellation or deadline expiration into a fabricated worker failure. Preserve it as an operation error, then let the control plane decide Run disposition. |

## Repository evidence (not external-source claims)

The current checkout has three overlapping interfaces in
`backend/slurm-gateway/internal/gateway/simops_adapters.go`:
`SimopsSpooler`, `RunConnectionProfileSpooler`, and
`RunConnectionProfileStopper`. In
`simops_run_lifecycle.go`, `startWorkers` and `compensate` probe those optional
profile interfaces and fall back to legacy `StartRun` and `StopRun` calls.

`RunConnectionProfile` already centralizes run and worker identity, gateway
ingest information, labels, Docker/Kubernetes execution hints, and cleanup
policy in `simops_run_connection_profile.go`. The lifecycle policy owns
durable Run records, lifecycle transitions, compensation, recovery, artifacts,
and event publication. Those facts agree with ADR-0010's ownership decision.

## Repository proposals for Issue #129

These are design proposals, not claims made by Docker, Kubernetes, or Go.

1. Replace the overlapping launch/stop contracts with one Runtime Adapter
   module whose interface accepts control-plane-built Run Connection Profiles
   for start, observation, stop, and cleanup. Keep any legacy compatibility
   implementation behind that one interface only during a bounded migration.
2. Make the adapter return stable, runtime-neutral observations plus opaque
   external execution identity and runtime reason. The lifecycle policy maps
   those facts into durable SimOps Run and worker outcomes; the adapter never
   becomes the authority for domain outcome.
3. Keep `BuildRunWorkerConnectionProfiles` in the control-plane
   implementation. Concrete Docker and Kubernetes adapters consume profiles;
   neither rebuilds one or obtains profile data from browser input.
4. Treat requested stop, observed absence, terminal success/failure, and
   cleanup error as separate facts. The policy may define an idempotent absent
   resource as successful cleanup, while retaining authorization and transport
   failure evidence.
5. Preserve Gateway-Only Worker Ingest. Runtime adapters receive only the
   profile appropriate to the role; ordinary Run-Scoped Simulation Workers do
   not gain direct data-plane or runtime-management credentials.

## Test implications

- A common adapter contract test should prove profile-driven start, label-based
  lookup, observation mapping, caller-context cancellation, stop, and repeated
  cleanup for the contract, Docker, and Kubernetes adapters.
- Docker tests should distinguish container create conflict, start failure,
  inspect/list observation, stop timeout/error, and force-delete failure.
- Kubernetes tests should distinguish Job terminal conditions from Pod-level
  image-pull/termination evidence; delete request from observed disappearance;
  and normal background cleanup from explicit retained-debug behavior.
- Lifecycle-policy tests should prove that cancellation/deadline errors remain
  operation errors and that durable Run outcome remains control-plane-owned.

## Non-goals

This research does not select a Docker client version, Kubernetes `client-go`
version, delete propagation default, retry policy, or an exact Go interface.
Those are repository implementation decisions to make under ADR-0010, after
the profile-driven seam and ownership constraints are accepted.
