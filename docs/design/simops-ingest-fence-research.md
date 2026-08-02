# SimOps terminal-ingest fence research

## Question

When a terminal worker observation fences SimOps worker ingest, what must be
atomic so that a concurrent telemetry or result frame cannot be accepted after
the fence? Does a persisted ingest-fence generation solve that race by itself?

## Source facts

### PostgreSQL makes a conditional write the concurrency decision

PostgreSQL's default `READ COMMITTED` isolation gives each ordinary `SELECT` a
snapshot as of the beginning of that statement. Two separate statements can
therefore observe different committed facts. An ingest handler that first reads
an admissible lifecycle and later inserts an event has a time-of-check to
time-of-use gap.

Under the same isolation level, an `UPDATE`, `DELETE`, or `SELECT FOR UPDATE`
that finds a row already updated or locked by a concurrent transaction waits;
after the other transaction commits, PostgreSQL re-evaluates the command's
`WHERE` condition against the updated row. A conditional update can therefore
be the single admission decision, rather than a prior read followed by an
unconditional write.

Sources: [PostgreSQL transaction isolation](https://www.postgresql.org/docs/current/transaction-iso.html), [PostgreSQL explicit locking](https://www.postgresql.org/docs/current/explicit-locking.html).

`UPDATE ... RETURNING` returns values from rows actually updated, while an
update count of zero is not itself an error. This permits an application to
treat a no-row result as a rejected admission decision. `INSERT ... ON
CONFLICT DO UPDATE` similarly has an atomic insert-or-update outcome, but that
does not make independent statements before or after it part of the same
decision.

Sources: [PostgreSQL UPDATE](https://www.postgresql.org/docs/current/sql-update.html), [PostgreSQL INSERT](https://www.postgresql.org/docs/current/sql-insert.html).

### A generation is a version fact, not a lock

The PostgreSQL sources describe transactional row locking and conditional row
updates; they do not assign special concurrency semantics to an application
column called a generation. Such a column is useful as a monotonically changing
version or as evidence of which admission epoch accepted a frame. It does not
close a race unless the terminal transition and every admission write compare
or lock the same durable row in one transaction. This conclusion is an
application-design inference from the documented semantics above.

## Repository facts

- `SimopsController.Ingest` reads the Run with `getRecordForIngest`, validates
  its opaque token and frames, then separately publishes each `worker.telemetry`
  event and calls `UpdateWorkerFrames`. It has no transaction spanning that
  check and the acceptance side effects. See
  `backend/slurm-gateway/internal/gateway/simops_controller.go` (lines
  312-351).
- `getRecordForIngest` makes its decision from the Run lifecycle fetched by
  `GetRun`. In the current model, only `starting`, `streaming`, and `degraded`
  admit ingest. See `simops_controller.go` (lines 365-374).
- `StopRun` invokes the runtime, then separately updates the Run and each
  worker to `stopped`; that is both an incorrect lifecycle claim for the new
  design and a multi-statement fence. See `simops_controller.go` (lines
  221-254).
- The Postgres store implements `UpdateRunLifecycle`, `SaveEvent`, and
  `UpdateWorkerFrames` as independent database calls. `simops_runs` currently
  has no fence/admission column, and `simops_events` has no admission epoch.
  See `backend/slurm-gateway/internal/gateway/simops_postgres_store.go` (lines
  453-486, 531-548) and `deploy/postgres-init/001_simops.sql` (lines 3-72).
- When Redpanda is configured, the controller publishes through an event-log
  adapter rather than the local Postgres event table. Thus a database
  transaction cannot atomically include arbitrary broker publication in the
  current shape. This is a repository fact from
  `NewDefaultSimopsController` and `Ingest`, not a claim about Redpanda
  transactional capabilities.

## Repository proposal, not a PostgreSQL feature

Make **durable admission** a method of the deep Lifecycle Reconciliation
module, not a lifecycle read in an HTTP handler. Its Postgres implementation
should make one transaction decide either terminal fencing or worker-frame
admission using the same Run/worker rows.

Persist at least a run-scoped `ingest_fence_generation` and an explicit
admission state (or an equivalent terminal fact from which admission is
derived). Terminal observation atomically changes that state and increments
the generation. An accepted frame records the generation it was admitted
under, updates the worker's durable evidence/frame state, and creates the
durable local event/outbox record in the same transaction.

The admission transaction must either lock the relevant Run row with `SELECT
... FOR UPDATE`, or execute a conditional `UPDATE`/`INSERT` whose predicate
requires the Run to remain ingest-admissible at the presented/current
generation. The terminal transition must use that same row. PostgreSQL then
serializes the conflict:

```mermaid
sequenceDiagram
    participant I as Ingest transaction
    participant R as simops_runs row
    participant T as Terminal-observation transaction

    I->>R: lock/conditional admission check
    alt ingest wins
        I->>R: persist frame at generation N; commit
        T->>R: fence; generation N+1; commit
    else terminal transition wins
        T->>R: fence; generation N+1; commit
        I->>R: recheck predicate; reject admission
    end
```

If an ingest transaction commits before the terminal-fence transaction, it was
accepted before the durable fence even if its HTTP response is delayed. That is
the strongest meaningful ordering available: do not relabel it as post-fence.
If the fence transaction commits first, the later conditional admission must
affect no row and return a conflict. A bare generation increment with the
existing check-then-publish flow does **not** provide this property.

For the Redpanda path, publish after durable admission through a transactional
outbox/reconciler (or an equivalent idempotent delivery mechanism); do not make
broker publish the authority that decides whether a frame crossed the fence.
This is a repository proposal. It preserves the agreed non-event-sourced model:
the current worker and Run facts are authoritative, while events are
notifications/audit records.

## State-machine consequence

`Stopping / AwaitingStopObservation` is ingest-admissible under its bounded
policy. A terminal worker observation changes the admission epoch atomically
with the worker fact. `Stopping / StopUnresolved` may have a separate bounded
admission policy, but when that policy fences ingest it must use the same
transactional admission mechanism. The generation explains *which authority
epoch* accepted a record; it is not a second lifecycle state.
