# SimOps Runtime State-Machine Research

| Field | Value |
| --- | --- |
| Document ID | SIMOPS-RUNTIME-STATE-MACHINE-RESEARCH-001 |
| Revision | 0.1 |
| Status | Research note |
| Scope | Issue #129 state-machine semantics; no runtime change |

## Question

What runtime facts constrain the distinction between an attempted launch, an
uncertain observation, a terminal worker outcome, a stop request, and cleanup
for the Docker Engine and Kubernetes Job adapters?

## Source facts

### Observation is not a complete state machine

- Docker's Engine API is a REST interface to the daemon; its inspect operation
  returns low-level container information, while its `wait` operation can wait
  for `not-running`, `next-exit`, or `removed`. An inspect/list result is an
  observation, not an acknowledgement of a caller's intended state transition.
  [Docker Engine API](https://docs.docker.com/reference/api/engine/),
  [Engine API container reference](https://docs.docker.com/reference/api/engine/version/v1.47/).
- Kubernetes expressly says a Pod phase is a high-level summary and **not** a
  comprehensive state machine. `Pending` includes both scheduling and image
  download; `Unknown` means the state could not be obtained. A Pod is
  `Succeeded` only after every container succeeds, and `Failed` only after all
  containers terminate and at least one fails.
  [Pod lifecycle](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/).
- A Kubernetes container has `Waiting`, `Running`, or `Terminated` state.
  `Waiting` covers startup work including image pulling; its reason is only a
  summary. `Terminated` carries a reason, exit code, and start/finish times.
  [Pod lifecycle](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/),
  [Pod API reference](https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/).

### Image-pull observations are waiting evidence, not terminal proof

- `ImagePullBackOff` is a waiting state: Kubernetes was unable to pull the
  image and keeps retrying with an increasing delay, capped at 300 seconds.
  Kubernetes documents invalid image naming and private-registry credentials as
  possible causes, not as facts inferable from that reason alone.
  [Images: ImagePullBackOff](https://kubernetes.io/docs/concepts/containers/images/#imagepullbackoff).
- Kubernetes' Job failure-policy guidance uses a Pending Pod in
  `ImagePullBackOff` as an example of a potentially transient issue: the image
  may subsequently pull. Therefore an adapter may record this observation and
  retain the reason/message, but it cannot treat it as Kubernetes-confirmed
  terminal execution failure.
  [Pod failure policy](https://kubernetes.io/docs/tasks/job/pod-failure-policy/).

### Job conditions establish terminal Kubernetes execution

- A Job's terminal conditions are `Complete` (succeeded) and `Failed`. Failure
  includes a reached `backoffLimit` or `activeDeadlineSeconds`, applicable
  indexed-job limits, or a `FailJob` pod-failure-policy result. A Job succeeds
  when it reaches its completions or success policy.
  [Job terminal conditions](https://kubernetes.io/docs/concepts/workloads/controllers/job/#terminal-job-conditions).
- In Kubernetes v1.31+, the Job controller adds `Complete` / `Failed` only once
  every Job Pod terminates. `SuccessCriteriaMet` and `FailureTarget` expose the
  earlier point at which success or failure criteria are met; acting there can
  overlap a replacement with terminating Pods. The external state model must
  decide explicitly whether that overlap is acceptable.
  [Job termination and cleanup](https://kubernetes.io/docs/concepts/workloads/controllers/job/#termination-of-job-pods).
- A Job retries failed Pods according to its backoff configuration. Once the
  limit is reached it is marked failed; Job retry does not restart an already
  failed Job. This is distinct from a control plane starting a new run or a
  new delivery attempt.
  [Job backoff policy](https://kubernetes.io/docs/concepts/workloads/controllers/job/#pod-backoff-failure-policy).

### Stop and deletion are requests with asynchronous effects

- Docker `stop` sends the main process `SIGTERM` (or configured stop signal),
  then `SIGKILL` after the grace period. `rm --force` kills a running container
  before removal. Thus a successful request must not be presented as proof of
  a clean, observed worker stop.
  [Docker stop](https://docs.docker.com/reference/cli/docker/container/stop/),
  [Docker remove](https://docs.docker.com/reference/cli/docker/container/rm/).
- Kubernetes deletion records a grace period and asks the kubelet/container
  runtime to stop containers asynchronously. A force deletion removes the API
  object without waiting for confirmation that the resource has ended; the
  resource can continue to run. Deletion and observed termination must remain
  separate facts.
  [Pod termination and forced deletion](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-termination-flow).
- Deleting a Job deletes its Pods. Background cascading deletion removes the
  owner first and dependents later; foreground deletion keeps the owner in a
  deletion-in-progress state while relevant dependents are removed.
  [Jobs](https://kubernetes.io/docs/concepts/workloads/controllers/job/#job-termination-and-cleanup),
  [cascading deletion](https://kubernetes.io/docs/concepts/architecture/garbage-collection/#cascading-deletion).

### Cleanup can erase runtime evidence

- `ttlSecondsAfterFinished` starts only when a Job condition becomes `Complete`
  or `Failed`; expiry makes the Job eligible for cascading removal with its
  dependent objects. TTL zero makes it eligible immediately. Kubernetes does
  not guarantee retention when an already-expired TTL is extended.
  [TTL-after-finished](https://kubernetes.io/docs/concepts/workloads/controllers/ttlafterfinished/).
- Consequently, a later `NotFound` can mean pre-creation absence, external
  deletion, intentional cleanup, or TTL cleanup. It cannot alone prove that a
  worker never started or stopped cleanly.

### Caller cancellation does not establish external outcome

- Go `Context` conveys a cancellation signal; cancellation indicates that work
  should stop. It propagates to derived contexts, and `Done` can close
  asynchronously. `CancelFunc` cancels the context and releases local resources;
  it does not wait for an external API's side effect to finish.
  [Go context package](https://pkg.go.dev/context).
- A timed-out or canceled create/delete call is therefore an incomplete local
  observation. It cannot distinguish “request never reached the runtime” from
  “runtime performed the operation but the reply was lost.”

## Repository proposals, not source facts

The following is a proposed control-plane model derived from the facts above.
It is not a Kubernetes, Docker, or Go guarantee.

| Lens | Proposal |
| --- | --- |
| Conceptual model | A durable controller records intent and stable external identity; adapters report observations. A worker reaches a terminal control-plane outcome only from a policy that evaluates observations and reconciliation history. |
| Algorithm / flow | Record the start attempt and stable identity before or atomically with submission where possible. If submission returns an unambiguous rejection, record confirmed launch failure. If it times out or cancels after dispatch, record `unknown` and reconcile by identity before any retry. |
| Data structures | Keep append-only attempt/request receipts, latest observation with timestamp/reason, bounded reconciliation counters/deadlines, and a separate cleanup outcome. Do not collapse these into a single status string. |
| Pattern candidates | This is state-machine-shaped with an at-least-once command/reconciliation loop. It is not a Saga: there is no distributed compensating transaction that restores a completed worker. |
| Design consequence | The Runtime Adapter seam should expose small commands to start, observe, stop, and clean up; the deeper Lifecycle Reconciliation module owns retries, uncertainty expiry, and terminal interpretation. |

### Proposed transition rules

1. A **start attempt** has three outcomes: confirmed launch, confirmed terminal
   rejection, or unknown. Unknown must reconcile by the stable runtime identity
   before another attempt, so a lost response cannot create duplicate workers.
2. `ErrImagePull` / `ImagePullBackOff` is an **uncertain image-pull
   observation**. It becomes a terminal run failure input only if the Job later
   reports a terminal `Failed` condition, or the explicit bounded reconciliation
   policy expires without recovery.
3. A **Runtime Stop Request** records desired execution state. Its successful
   dispatch does not establish `stopped`; only a later runtime observation or
   bounded unresolved policy does that.
4. A confirmed Job `Complete` / `Failed` condition is a terminal runtime
   observation. Preserve reason, message, observation time, and runtime
   identity before asynchronous TTL cleanup can erase them.
5. **Cleanup** is an independent obligation after terminal observation.
   Cleanup success must not rewrite an established terminal worker result;
   cleanup failure remains visible and retryable under its own bound. It is not
   an implicit force-stop after a mere stop request.
6. `missing` remains uncertain unless durable records prove an intentional
   cleanup after a terminal observation, or the reconciliation policy exhausts.

### Locked repository state model

This is a repository decision for Issue #129, not an external-runtime claim.
The worker has separate persisted current facts rather than a single overloaded
status:

| State machine | Authoritative fact | Terminal / unresolved rule |
| --- | --- | --- |
| Desired stop | no request or Runtime Stop Request | A request remains desired intent; exhausting stop observation leaves it unresolved. |
| Observed execution | latest adapter observation and reconciliation history | `succeeded`, `failed`, `stopped`, or policy-exhausted `missing` are terminal control-plane inputs. |
| Cleanup | eligibility and Runtime Cleanup Outcome | Cleanup starts only after a terminal observed execution state, including policy-exhausted `missing`; it uses stable identity to remove a possible orphan and succeeds only on observed removal or idempotent absence. It must not be an implicit force-stop. |

The Run is a derived hierarchical projection: an exclusive Run Lifecycle with
one nested Run Phase, plus orthogonal Operational Condition and Run Cleanup
Aggregate regions. `Degraded` is an Operational Condition with explicit
current reasons, not a peer lifecycle state. A Run whose stop-observation
policy exhausts remains `stopping / stop-unresolved / degraded`; it is neither
fabricated `stopped` nor rewritten as execution `failed`. A separately
approved future force-termination policy may introduce an escalation, but it
is not cleanup and is outside this decision. The complete model is in
[SimOps Hierarchical State-Machine Research](simops-hierarchical-state-machine-research.md).

### Scenarios to test the interface

| Scenario | Required result |
| --- | --- |
| Create request times out; a later lookup finds the labelled Job | Preserve the original attempt; observe it and do not create a duplicate. |
| Pod reports `ImagePullBackOff`; a later pull succeeds | Move from uncertain image-pull observation back to pending/active, without a terminal failure. |
| Job reaches `FailureTarget` while Pods terminate | Do not call it terminal unless the selected policy explicitly permits early replacement overlap. |
| Stop request succeeds; API object disappears after force deletion | Record the stop request and unresolved/missing observation separately; do not claim clean stop. |
| Zero-TTL Job is gone before the next sync | Use persisted terminal observation if present; otherwise retain ambiguity and reconcile evidence rather than infer success. |

## Source set

All external sources above are first-party Docker, Kubernetes, or Go
documentation, checked 2026-08-01. This note intentionally makes no claim that
the proposals are already implemented.
