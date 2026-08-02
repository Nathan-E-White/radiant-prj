# SimOps Runtime Retry-Policy Research

| Field | Value |
| --- | --- |
| Document ID | SIMOPS-RUNTIME-RETRY-POLICY-RESEARCH-001 |
| Revision | 0.1 |
| Status | Research note |
| Scope | Issue #129 retry ownership for lifecycle reconciliation; no runtime change |

## Question

Which runtime facts constrain a durable, bounded retry policy for starting,
observing, stopping, and cleaning up a SimOps worker, and where should that
policy live?

## Source facts

### A Go deadline ends the caller's wait, not necessarily the remote effect

- Go `Context` carries a cancellation signal and deadline through API
  boundaries. A derived context is canceled when its parent is canceled or its
  deadline expires. [`context`](https://pkg.go.dev/context).
- A `CancelFunc` tells an operation to abandon work, but does **not** wait for
  the work to stop. The `Done` channel may close asynchronously after the
  cancel function returns. A `context deadline exceeded` result is therefore a
  fact about the caller's local wait, not proof that an HTTP request was not
  received or acted on by Docker or Kubernetes.
  [`CancelFunc`](https://pkg.go.dev/context#CancelFunc),
  [`Context.Done`](https://pkg.go.dev/context#Context).
- `WithTimeout` is `WithDeadline` with a computed deadline. Go recommends
  calling the returned cancel function to release local resources even when the
  operation finishes early. The timeout belongs to one local attempt; it does
  not create a retry schedule or a durable operation record.
  [`WithTimeout`](https://pkg.go.dev/context#WithTimeout).

### Docker exposes separate effects and observations, not a durable retry protocol

- Docker Engine is an HTTP API and uses ordinary HTTP status codes for the
  outcome of an API call. Running a container comprises multiple API calls,
  rather than one all-or-nothing `run` action. [Docker Engine OpenAPI
  definition](https://raw.githubusercontent.com/moby/moby/master/api/swagger.yaml).
- The Engine creates, starts, inspects, stops, and removes containers as
  separate operations. The CLI documents `start` as starting stopped
  containers, `inspect` as displaying container information, and `stop` as
  sending a configured signal followed by a possible forced kill after its
  grace period. These are provider operations; the documentation does not
  specify a caller-supplied idempotency-key or a durable client retry policy.
  [Docker start](https://docs.docker.com/reference/cli/docker/container/start/),
  [Docker inspect](https://docs.docker.com/reference/cli/docker/container/inspect/),
  [Docker stop](https://docs.docker.com/reference/cli/docker/container/stop/).
- Docker's *container* restart policy is a runtime policy for a container that
  exits: its delay doubles from 100ms to prevent server flooding. That is not a
  control-plane retry of a failed `create`, `start`, `stop`, `inspect`, or
  `remove` call. [Docker Engine OpenAPI
  definition](https://raw.githubusercontent.com/moby/moby/master/api/swagger.yaml).

### Kubernetes has provider-owned retries, but they do not replace caller reconciliation

- A Kubernetes Job retries its Pods until the requested successful completions
  are reached. A failed or deleted Pod can cause the Job controller to create a
  new Pod. [Kubernetes Jobs](https://kubernetes.io/docs/concepts/workloads/controllers/job/).
- `.spec.backoffLimit` bounds Job Pod-failure retries (default six). The Job
  controller applies exponential delays of 10s, 20s, 40s, and so on, capped at
  six minutes. A Job becomes failed when the relevant failure calculation
  reaches the limit. [Job backoff policy](https://kubernetes.io/docs/concepts/workloads/controllers/job/#pod-backoff-failure-policy).
- A Job's `podFailurePolicy` can `FailJob`, `Ignore`, `Count`, or `FailIndex`
  selected Pod failures. Its result depends on Pod terminal state, exit codes,
  and conditions. A platform controller, not a SimOps adapter, owns this
  provider-specific decision. [Pod failure policy](https://kubernetes.io/docs/concepts/workloads/controllers/job/#pod-failure-policy).
- `activeDeadlineSeconds` can stop an otherwise retrying Job before its
  backoff limit. A Job that reaches its deadline or backoff limit has a
  permanent failed Job status; Kubernetes does not automatically restart that
  Job. [Job termination and cleanup](https://kubernetes.io/docs/concepts/workloads/controllers/job/#job-termination-and-cleanup).
- `FailureTarget` / `SuccessCriteriaMet` express that a Job has met a
  termination criterion, while current Kubernetes versions delay `Failed` /
  `Complete` terminal conditions until all Job Pods terminate. A controller
  that treats the earlier condition as its own terminal observation accepts a
  possible overlap with terminating Pods; that is a policy choice, not a
  generic retry fact. [Terminal Job conditions](https://kubernetes.io/docs/concepts/workloads/controllers/job/#terminal-job-conditions).

### Kubernetes control is reconciliation-shaped

- Kubernetes controllers work through a control loop: compare desired state
  with current cluster state and make changes to approach desired state.
  Kubernetes itself notes that the control loop is not necessarily a single
  read followed by one write; different controllers reconcile at different
  rates and observe partly independent state. [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/).
- Kubernetes object deletion can remain pending while finalizers complete.
  The object is deleted only after finalizers are empty. An accepted deletion
  request therefore cannot alone prove the underlying lifecycle is complete.
  [Finalizers](https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/).

## Repository proposals, not source facts

The following policy is proposed for Radiant. It is an inference from the
source constraints and the agreed Runtime Binding / worker state model, not a
Docker, Kubernetes, or Go guarantee.

| Lens | Finding |
| --- | --- |
| Conceptual model | One durable **Lifecycle Reconciliation** module owns retry eligibility and terminal interpretation. The Runtime Adapter translates one bounded attempt into provider commands and observations; it does not decide cross-provider retry budgets. |
| Algorithm / flow | Persist the effect kind, attempt number, start time, deadline, last error class, and next eligible time. Dispatch one bounded attempt. Classify its result as confirmed effect, confirmed rejection, or uncertain. Reconcile uncertain effects by the stored Runtime Binding and stable worker identity before another effect attempt. Stop when a terminal observation, confirmed rejection, or the policy's durable bound is reached. |
| Data structures | Current per-worker facts: desired stop state, latest observed lifecycle, cleanup outcome, plus per-effect retry metadata. A compact attempt receipt (effect, ordinal, time, outcome class, error summary) supplies auditability without making an append-only event log authoritative. |
| Pattern candidates | **State machine**: distinct desired, observed, and cleanup facts have legal transitions. **Retry loop**: persisted eligibility produces bounded re-attempts. **Reconciliation loop**: observation precedes retrial after ambiguity. This is not Event Sourcing; current state remains authoritative. It is not a Circuit Breaker unless a future policy intentionally opens a shared provider-wide failure gate. |
| Design consequence | The public Runtime Adapter interface should return provider facts, including unambiguous absence/terminality where available. The deep Lifecycle Reconciliation module owns time, counters, backoff, retryability classification, and aggregate Run derivation. This concentrates policy for leverage and locality instead of duplicating it across Docker and Kubernetes adapters. |

### Proposed ownership rule

Every retryable **control-plane effect**—start, observation after ambiguity,
stop, and cleanup—gets a policy record owned by Lifecycle Reconciliation. The
record provides one explicit bound: maximum attempts, an absolute deadline, or
both. A timeout is never itself a reason to issue an identical effect again;
the next step is reconciliation first whenever the original effect may have
reached the provider.

Provider-native retry remains intact but is deliberately narrower:

| Concern | Kubernetes Job controller | Docker Engine | Radiant Lifecycle Reconciliation |
| --- | --- | --- | --- |
| Retry a failed workload process | Yes, subject to Job configuration | Optional container restart policy | Observe and interpret; do not duplicate it |
| Retry a timed-out client request | No documented caller protocol | No documented caller protocol | Yes, after identity-based reconciliation and under one bound |
| Stop / cleanup retry | Provider processes each request | Provider processes each request | Own eligibility, timeout, audit facts, and unresolved outcome |
| Decide SimOps worker / Run terminality | Only Job/Pod facts | Only container facts | Yes, from persisted facts plus explicit policy |

### Consequences for the Runtime Adapter seam

The seam stays small and deep if it expresses only operations whose provider
meaning varies: start a deterministically identified worker, observe it, ask
it to stop, and request cleanup. The adapter may have private HTTP/client retry
mechanics necessary to finish *one* call, but its interface must not expose
per-provider retry knobs, timers, or state-machine decisions to callers.

Deleting the Lifecycle Reconciliation module should make the complex things
reappear across the controller and both adapters: deadline handling, unknown
start recovery, backoff, durable exhaustion, and Run derivation. That is the
deletion test showing that it earns its depth. Conversely, duplicating this
logic inside each adapter would make a Docker timeout and a Kubernetes timeout
mean different SimOps outcomes without a domain reason.

### Scenarios that should constrain later design and tests

| Scenario | Required policy result |
| --- | --- |
| Start call reaches its Go deadline; a labelled Docker container or Kubernetes Job is found | Mark the original start attempt reconciled; do not start a second worker. |
| Start deadline elapses and identity lookup proves absence | Schedule the next bounded start attempt; preserve the earlier uncertain receipt. |
| Kubernetes Job is retrying Pods below `backoffLimit` | Preserve the provider observation as nonterminal; do not create a replacement SimOps worker. |
| Kubernetes Job reports `FailureTarget` while Pods still terminate | Record the condition and apply the selected early-terminal policy explicitly; otherwise wait for `Failed`. |
| Stop request times out | Keep desired stop requested, reconcile observation, and retry the stop effect only under its persisted bound. |
| Cleanup observes a missing resource after a prior persisted terminal observation | Record cleanup success or idempotent absence without changing the worker's execution outcome. |
| Cleanup exhausts its policy after worker terminality | Keep the execution outcome terminal and expose cleanup as unresolved; do not retry forever or rewrite the Run as execution-failed. |

## Non-goals

This note does not choose retry counts, intervals, jitter, database schema,
HTTP client settings, Kubernetes Job `backoffLimit`, or a provider-wide outage
policy. It also does not introduce Event Sourcing: that is explicitly deferred;
current worker records and bounded retry metadata remain the proposed source of
truth for this issue.

## Source set

All external sources are first-party Go, Docker/Moby, or Kubernetes
documentation/source, checked 2026-08-01. The repository proposals are marked
separately and make no claim of current implementation.
