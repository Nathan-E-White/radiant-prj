# SimOps worker-ingest authority scope research

## Question

The present SimOps credential is stored on the Run, while terminal observation
and ingest admission are worker-specific. What must be scoped to a worker, what
can remain Run-scoped, and how can terminal fencing serialize correctly without
fencing healthy sibling workers?

## Source facts

### PostgreSQL can serialize one worker decision

`SELECT FOR UPDATE` prevents other transactions from locking, modifying, or
deleting the selected row until the transaction ends. Row locks block writers
and lockers, and are released at transaction end. PostgreSQL's default `READ
COMMITTED` isolation gives each command a new snapshot; an ordinary
read-then-write check therefore is not one atomic decision.

Sources: [PostgreSQL explicit locking](https://www.postgresql.org/docs/current/explicit-locking.html#LOCKING-ROWS), [PostgreSQL Read Committed isolation](https://www.postgresql.org/docs/current/transaction-iso.html#XACT-READ-COMMITTED).

An `UPDATE ... WHERE` changes only rows satisfying its predicate, and
`RETURNING` reports rows actually changed. If it encounters a concurrently
updated/locked row in `READ COMMITTED`, PostgreSQL waits and re-evaluates the
predicate against the committed version. A conditional update of one worker
row can consequently be the admission-or-fence decision for that worker.

Source: [PostgreSQL UPDATE](https://www.postgresql.org/docs/current/sql-update.html).

When a transaction must lock more than one row, PostgreSQL can detect a
deadlock and abort one participant. Its documentation recommends acquiring
locks on multiple objects in a consistent order and retrying a deadlock
abort.

Source: [PostgreSQL deadlocks](https://www.postgresql.org/docs/current/explicit-locking.html#LOCKING-DEADLOCKS).

These are database-concurrency facts. PostgreSQL does not prescribe the
meaning or scope of a worker credential or ingest fence.

## Repository facts

- `simops_runs.ingest_token` is one non-null opaque token per Run; the schema
  has no worker credential, worker admission state, or fence generation.
  `simops_workers` is keyed by `(run_id, worker_id)`. See
  `deploy/postgres-init/001_simops.sql` (lines 3-40).
- A `RunConnectionProfile` is built per worker and includes its `WorkerID` and
  `WorkerKind`, but all ordinary-worker profiles copy `run.IngestToken` into
  the gateway connection. See
  `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go`
  (lines 120-131, 155-190).
- Telemetry and result requests authenticate against that Run token, then
  validate that the frame's Run and scenario match the Run and that its worker
  ID is nonempty and kind is supported. They do not establish that the token
  holder is the named worker, nor that the named worker exists in the Run.
  Telemetry publishes before `UpdateWorkerFrames` discovers a nonexistent
  worker. See `simops_controller.go` (lines 312-351, 511-538) and
  `workbench_controller.go` (lines 100-128, 221-241).
- Worker identity is already meaningful downstream: event routing uses
  `workers/<worker_id>/...`, and telemetry persistence indexes `(run_id,
  worker_id)`. See `simops_moq_tracks.go` (lines 62-95) and
  `deploy/postgres-init/001_simops.sql` (lines 84-113).
- Current `getRecordForIngest` grants or rejects all ingest from the Run
  lifecycle; it cannot keep one healthy sibling open when another worker
  becomes terminal. See `simops_controller.go` (lines 365-374).

## Design analysis

A **Run membership credential** answers only “may this principal submit
something associated with this Run?” It cannot by itself establish the
worker-specific authority required to fence one worker while siblings remain
admissible. With the present shared token, a compromised or erroneous worker
can name a sibling in a frame. That is not merely a fence-granularity problem:
it is a provenance problem.

A **Worker Ingest Credential** is an immutable, opaque worker-bound capability
issued through runtime-secret injection in the worker's Run Connection Profile.
The worker receives the capability while the control plane retains only a
server-side verifier; it must not appear in Run responses, event payloads,
adapter logs, or browser-visible configuration. The handler authenticates it,
verifies it binds the frame's named worker, and then uses the `(run_id,
worker_id)` worker record as the durable admission authority. A separate
Run-wide credential is unnecessary for ordinary-worker ingest unless another
trusted Run-scoped producer is introduced.

**Worker Ingest Admission** and **Ingest Fence Generation** should therefore
both be worker-scoped. One worker's terminal observation atomically fences
only that worker's credential/admission record. A healthy sibling retains its
own current generation and continues to submit frames. An accepted frame
records the generation of *its worker* as admission evidence.

The Run remains the aggregate owner, not the authorization authority. When a
worker transition may make the Run terminal, one transaction should lock the
affected worker row, derive its worker admission/fence result, and update the
Run aggregate. If derivation needs all worker records, lock them by stable
`worker_id` order (and the Run row in one documented order) before deriving.
The aggregate transition must not itself rewrite or fence siblings: terminal
Run state is a consequence of every worker's independent terminal fence.

```mermaid
sequenceDiagram
    participant A as Worker A frame
    participant WA as "Worker A admission row"
    participant WB as "Worker B admission row"
    participant T as "Worker A terminal observation"
    participant R as "Run aggregate"

    A->>WA: authenticate capability for A; conditional admit
    T->>WA: lock; persist A terminal and fence A; generation + 1
    T->>R: derive aggregate
    Note over WB: no lock, no fence; Worker B remains admissible
```

This is a repository proposal, not an assertion that Docker or Kubernetes
provide worker-ingest credentials. Their runtime identity remains separate
from gateway authority.

## Consequences and decision remaining

The deep **Lifecycle Reconciliation module** should own this policy behind a
small interface: it admits a frame for a named worker, records the resulting
facts, and applies a terminal observation. Its implementation hides credential
verification, conditional writes, worker-generation increments, aggregate
derivation, and outbox delivery. Runtime Adapters do not participate. This
gives callers a single admission decision and keeps concurrency locality in
one module.

The remaining product/security decision is whether ordinary workers receive
distinct worker-bound credentials (recommended) or retain one shared Run token
and accept that it proves only Run membership. The latter cannot prevent a
sibling from impersonating another sibling and does not support worker-scoped
fencing honestly.

## Verification implications

- A valid credential for Worker A cannot admit a Worker B telemetry or result
  frame.
- Fencing Worker A rejects later A frames while a concurrent valid Worker B
  frame is admitted.
- A worker-terminal transaction and a same-worker admission race serialize:
  the later decision observes the former committed fact.
- An all-worker terminal derivation cannot deadlock when it locks rows in the
  documented stable order.
- A frame admitted before its worker's fence retains that worker's prior
  generation; one admitted after the fence affects no admission row and emits
  no outbox record.
