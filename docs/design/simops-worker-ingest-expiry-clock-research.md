# SimOps worker-ingest expiry clock research

## Question

Which clock decides whether a Worker Ingest Credential has expired? How can the
control plane observe clock skew without letting a worker, Docker, or Kubernetes
clock decide worker-ingest admission?

## Source facts

### A remotely supplied time is not an authority clock

NTP exists to discipline independently drifting system clocks toward a common
timebase; it does not make their readings identical or make one host's reading
authoritative for another host's authorization decision. The NTP reference
documentation describes a feedback algorithm that adjusts the local system
clock from offset samples. Therefore a worker- or runtime-provided timestamp is
useful diagnostic evidence, but it is not a sound input for accepting a
credential that the gateway must revoke on its own deadline.

Sources: [NTP overview](https://www.ntp.org/reflib/exec/), [NTP clock discipline
algorithm](https://www.ntp.org/documentation/4.2.8-series/discipline/).

### Process-local monotonic time does not survive a durable decision

Go distinguishes wall-clock time from process-local monotonic time. The latter
is appropriate for measuring an interval within one process, but is omitted
when a `time.Time` is serialized and has no meaning outside that process. A
persisted credential expiry must consequently be a UTC wall-clock instant; a
process may use its monotonic clock only for local timeouts, not to compare a
stored deadline across restart or between gateway replicas.

Source: [Go `time` package, monotonic clocks](https://pkg.go.dev/time#hdr-Monotonic_Clocks).

### PostgreSQL can evaluate time and serialize the admission decision together

PostgreSQL distinguishes transaction-start time (`CURRENT_TIMESTAMP`, `now()`)
from actual current time (`clock_timestamp()`), which changes even during one
statement. It also supplies row locks and transaction isolation that can order
conflicting updates to one worker-admission row. This supports an expiry check
inside the same short transaction that decides frame admission or fencing;
there is no comparable guarantee from a worker or provider clock.

For an admission transaction that might wait behind a fence, `now()` is a poor
choice: it can be older than the lock wait because it is fixed at transaction
start. `clock_timestamp()` is the relevant database expression when the policy
means "expired at the instant the durable admission decision is made."

Sources: [PostgreSQL date/time functions](https://www.postgresql.org/docs/current/functions-datetime.html#FUNCTIONS-DATETIME-CURRENT), [PostgreSQL explicit row locking](https://www.postgresql.org/docs/current/explicit-locking.html#LOCKING-ROWS), [PostgreSQL Read Committed behavior](https://www.postgresql.org/docs/current/transaction-iso.html#XACT-READ-COMMITTED).

## Repository facts

- The accepted glossary direction already gives every Worker Ingest Credential
  an absolute expiry at the shared Run deadline and says worker, Docker, and
  Kubernetes clocks do not decide ingest fencing. See `CONTEXT.md` under
  **Worker Ingest Credential**.
- The current SimOps store has no worker-scoped admission record and uses
  process `time.Now().UTC()` in several persistence paths, including lifecycle
  and worker updates. `simops_runs` currently holds a run-wide plaintext
  `ingest_token`. See `backend/slurm-gateway/internal/gateway/simops_postgres_store.go`
  (lines 460-488) and `deploy/postgres-init/001_simops.sql` (lines 3-37).
- The current lifecycle and both runtime adapters accept injected `Now`
  functions for deterministic tests: `SimopsRunLifecyclePolicy.SetNow`,
  `simopsdocker.Spooler.Now`, and `simopskubernetes.Spooler.Now`. That is useful
  implementation testability, but it is not yet one durable admission clock.

## Repository proposal, not a source requirement

Name the authority **Control-Plane Admission Clock**. It is one clock source
inside the gateway's durable admission path, not a clock reported by a worker
or Runtime Adapter. Persist the Run Deadline as a UTC instant when the Run is
created. On every admission attempt, serialize on the worker-admission record
and make the expiry comparison in the same transaction that records the
admission or fence.

For a horizontally replicated gateway backed by one PostgreSQL authority, use
PostgreSQL `clock_timestamp()` in that conditional update. This makes the
decision independent of which gateway process received the frame and prevents a
transaction-start timestamp from extending an expired credential after waiting
on a row lock. The gateway still owns the policy; PostgreSQL supplies the clock
and serializes its durable decision.

```mermaid
sequenceDiagram
    participant W as "Worker clock (evidence only)"
    participant G as "Gateway"
    participant A as "Worker admission row"
    participant P as "Postgres control-plane clock"

    W->>G: "frame + optional occurred-at"
    G->>A: "lock worker admission row"
    A->>P: "clock_timestamp() >= run_deadline?"
    alt "before deadline and admitting"
        A->>A: "record admitted frame at fence generation N"
        A-->>G: "accept"
    else "expired or already fenced"
        A->>A: "fence; clear verifier; tombstone reason expiry"
        A-->>G: "reject"
    end
```

The worker's reported timestamp may be retained as frame evidence. Alongside
it, record the gateway receive time and, where operationally useful, the
difference between them as an observed skew/latency measurement. Such a value
can drive metrics and diagnostics (for example, unusually large absolute
offset, missing timestamp, or sudden offset change); it must neither extend the
Run Deadline nor reject an otherwise authenticated frame by itself. Docker and
Kubernetes observation timestamps have the same evidence-only role.

Do not use application-process `time.Now()` as the final expiry predicate in a
multi-replica deployment unless the architecture explicitly provides a single
disciplined shared clock and tests its failure mode. Otherwise two gateway
replicas can make different edge decisions around the deadline. Go's injected
clock remains appropriate at the module's internal seam for deterministic unit
tests; PostgreSQL integration tests should prove the actual conditional
admission/fence ordering around the durable clock.

## Decision impact

The prior rule needs one precision: “the gateway's clock” should mean the
**Control-Plane Admission Clock**, not each gateway process's local wall clock.
For the proposed PostgreSQL-backed worker-admission record, that is
`clock_timestamp()` evaluated in the serialized admission transaction. The
remaining product decision is whether this database-clock choice is accepted
as the concrete meaning of gateway authority, or whether Radiant intends to
operate a distinct shared clock authority.
