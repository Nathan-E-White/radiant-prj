# SimOps worker activity evidence research

## Question

Can Docker or Kubernetes observation, after a controller restart or an external
cleanup, reliably establish that a particular worker was previously active? What
does that imply for a control-plane distinction between **active observed** and
**not observed active**?

## Source facts

### Docker Engine

Docker Engine's container-inspect response is a snapshot of a container that
still exists. Its `State.Status` describes the present state (`created`,
`running`, `paused`, `restarting`, `removing`, `exited`, or `dead`); it also has
`StartedAt` and `FinishedAt`, described respectively as the times the container
was *last* started and *last* exited. The documentation therefore supports
inferring prior start for an extant container with a meaningful `StartedAt`; it
does not describe an append-only history of all runs or observations.

Source: [Moby Engine API OpenAPI definition, `ContainerState`](https://raw.githubusercontent.com/moby/moby/master/api/swagger.yaml).

An inspect response is unavailable once the container itself has been removed.
This follows from the API shape: `ContainerState` is part of the object returned
by inspect, not a separately retained execution-history resource. Thus a later
not-found result cannot distinguish a resource that never existed from a
resource that existed, ran, and was then removed. That conclusion is an
inference from the API model, not a claim of a Docker history-retention
guarantee.

### Kubernetes Pods and Jobs

Kubernetes records the *current* state of each container in a Pod as one of
`Waiting`, `Running`, or `Terminated`. A current `Running` state exposes the
last (re-)start time. A current `Terminated` state includes the prior execution's
start and finish times, exit code, reason, message, signal, and container ID.
These fields can support a post-restart inference that a still-retained Pod's
container started, even when it is no longer running.

Sources: [Kubernetes Pod lifecycle: container states](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#container-states); [Kubernetes Pod API reference: `ContainerStateRunning` and `ContainerStateTerminated`](https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/#ContainerState).

That evidence is neither an unbounded history nor durable beyond object
retention. Pods are described as relatively ephemeral. For finished Jobs, the
TTL-after-finished controller may delete the finished Job *and its dependent
objects* after its terminal condition and configured TTL. Kubernetes explicitly
notes that retaining a finished Job is useful for determining whether it
succeeded or failed. Once the relevant object has been deleted, its
`containerStatuses` are no longer available from the ordinary object API.

Sources: [Kubernetes Pod lifetime](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-lifetime); [Kubernetes TTL-after-finished cleanup](https://kubernetes.io/docs/concepts/workloads/controllers/ttlafterfinished/).

### Meaning of absence

Neither provider's ordinary object inspection turns later absence into a fact
that a worker never ran. At most, stable-identity lookup can establish that no
matching resource is presently retained. Provider-side status can strengthen a
positive claim while the resource remains, but it cannot repair an observation
gap after deletion, control-plane downtime, or a missed watch event.

This is an inference from the source facts above. It is deliberately narrower
than claims about physical execution: a controller can only assert what it
observed or what the provider's retained status proves.

## Repository proposal

Persist on each SimOps worker a monotonic control-plane fact:

```text
active_execution_observed: false -> true
first_active_observed_at: optional timestamp
active_observation_source: optional provider observation reference
```

Set it only when the runtime adapter returns qualifying positive evidence: a
current Docker `running` state, a retained Docker start timestamp accepted by
the adapter's evidence rules, a Kubernetes `Running` container state, or a
retained Kubernetes termination record with a meaningful prior `startedAt`.
The exact admissible provider evidence belongs inside the adapter
implementation; the control-plane fact has one provider-neutral meaning:
**SimOps established active execution at least once.**

Do not set the fact from a successful launch request, a timeout, a canceled
request, a stable-identity lookup that merely finds a resource, or a later
absence result. Those establish different facts.

Derive Worker Failure Stage from this monotonic fact:

| Derived stage | Rule |
| --- | --- |
| `FailedWithoutActiveObservation` | Worker terminally failed and `active_execution_observed` is false. |
| `FailedAfterActiveObservation` | Worker terminally failed and `active_execution_observed` is true. |

The first label means the control plane never established activity; it does not
assert that activity was physically impossible. Raw provider reason, message,
exit evidence, and timestamps remain the evidence for diagnosis.

## Consequence

This field is a compact durable fact, not Event Sourcing and not a provider
history mirror. It makes the derived failure stage restart-safe and independent
of provider cleanup, while preserving the honest epistemic distinction between
"not observed active" and "observed active." It should be authoritative only
for SimOps' observation claim; provider-specific details remain attached as
evidence rather than collapsed into a universal failure taxonomy.
