# SimOps runtime-limit admission-boundary research

## Question

At the shared Run Deadline, what exact rule decides whether a Worker Ingest
Credential can admit a frame, and how must concurrent fencing and admission be
ordered?

## Source facts

### `clock_timestamp()` is the relevant expiry clock

PostgreSQL documents `CURRENT_TIMESTAMP`, `transaction_timestamp()`, and
`now()` as transaction-start time. It documents `clock_timestamp()` as actual
current time, which can change during one SQL statement. An admission
transaction can wait on the Worker admission row; using transaction-start time
after that wait could admit a frame after the deadline. The expiry predicate
therefore needs `clock_timestamp()` at the durable decision, not `now()`.

Source: [PostgreSQL current date/time functions](https://www.postgresql.org/docs/current/functions-datetime.html#FUNCTIONS-DATETIME-CURRENT).

### One durable row must serialize admission and fencing

Under PostgreSQL's default Read Committed isolation, a concurrent `UPDATE` or
`SELECT FOR UPDATE` waits for an updater of the same row. If that updater
commits, PostgreSQL re-evaluates the waiting command's `WHERE` clause against
the updated row. `FOR UPDATE` likewise blocks competing writers and lockers
until transaction end. Thus, a conditional update on the worker-admission row,
or an explicit row lock followed by its guarded write, can make acceptance or
expiry fencing one serialized decision.

Sources: [PostgreSQL Read Committed behavior](https://www.postgresql.org/docs/current/transaction-iso.html#XACT-READ-COMMITTED), [PostgreSQL row-level locking](https://www.postgresql.org/docs/current/explicit-locking.html#LOCKING-ROWS).

`UPDATE ... RETURNING` returns only rows actually updated, while an update of
zero rows is not an error. The gateway can therefore treat a missing returned
row as a rejected admission rather than relying on an earlier read.

Source: [PostgreSQL UPDATE](https://www.postgresql.org/docs/current/sql-update.html).

## Repository proposal

Define the Worker Ingest Credential's admissible interval as the half-open
interval `[issued_at, run_deadline)`. A frame is admitted only if the
Control-Plane Admission Clock satisfies:

```text
clock_timestamp() < run_deadline
```

At equality and afterwards (`clock_timestamp() >= run_deadline`), the
credential is expired. The same serialized durable operation must reject the
frame and, if not already done, fence the worker admission record with reason
`runtime-limit`. The deadline-trigger reconciliation then records its
idempotent `Runtime Stop Request(reason=runtime-limit)` and derives
`Stopping / DispatchingStop`; neither action implies that the runtime has
stopped.

The strict boundary is a repository policy, not a PostgreSQL-specified
credential convention. It is the recommended policy because equality otherwise
has no unambiguous side: choosing `<=` would grant an additional admission at
the exact instant at which the Run's absolute deadline claims to have elapsed.

The shape can be a short transaction using a guarded mutation over the same
worker-admission record used by terminal and explicit-unresolved fences:

```sql
UPDATE simops_worker_admission
SET admitted_generation = ingest_fence_generation
WHERE run_id = $1
  AND worker_id = $2
  AND verifier_matches($3)
  AND admission_state = 'admissible'
  AND clock_timestamp() < run_deadline
RETURNING ingest_fence_generation;
```

An implementation need not use this literal schema or function. It must retain
the single-decision properties: authenticate the worker-bound credential,
evaluate the strict deadline predicate, persist accepted evidence/outbox work
at the returned generation, or atomically persist/reuse the existing fence.
If a fence wins first, the blocked admission rechecks and affects no row. If
admission commits first, it is pre-fence evidence even when the HTTP response
arrives later.

```mermaid
sequenceDiagram
    participant I as "Ingest admission"
    participant A as "Worker admission record"
    participant L as "Deadline reconciliation"

    I->>A: "conditional admission: clock < deadline"
    alt "admission commits first"
        A-->>I: "accept at generation N"
        L->>A: "fence reason runtime-limit; generation N+1"
    else "deadline fence commits first"
        L->>A: "fence reason runtime-limit; generation N+1"
        I->>A: "recheck predicate after wait"
        A-->>I: "reject"
    end
```

## Decision impact

The proposed answer to the open grilling question is **yes**: use strict
`< run_deadline` admission; `>= run_deadline` is expired and on the fence side.
This refines the existing Control-Plane Admission Clock decision without
creating a new lifecycle state or delegating deadline authority to a Runtime
Adapter.
