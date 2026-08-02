# SimOps worker credential verifier revocation research

## Question

At a terminal worker-ingest fence, should the control plane delete the opaque
Worker Ingest Credential verifier, retain a disabled verifier, or separate
authorization removal from audit? What must be transactional to make the
choice survive restart and concurrent frame admission?

## Source facts

### Invalidating an authenticator is a change to its binding

NIST defines authenticator invalidation (also called revocation or termination)
as removing the binding between an authenticator and the subscriber account. It
requires prompt invalidation when the account no longer exists, an
authenticator is compromised, or eligibility no longer holds, and notes that
the consequence of failing to invalidate a compromised authenticator is usually
greater than an erroneous invalidation's denial-of-service cost. Although a
worker capability is not a human authentication authenticator, this supports
the narrow security principle: when authority ends, the verifier must no longer
authorize the capability.

NIST also requires session secrets to be established from random input, passed
over an authenticated protected channel, unavailable to intermediaries, and
erased or invalidated when the session subject logs out. Those requirements are
not a direct specification for SimOps, but they support short-lived,
server-controlled capability authority rather than leaving a bearer secret
valid after its subject's work has ended.

Sources: [NIST SP 800-63B, authenticator invalidation](https://pages.nist.gov/800-63-4/sp800-63b.html#invalidation), [NIST SP 800-63B, session-secret requirements](https://pages.nist.gov/800-63-4/sp800-63b.html#session-bindings).

### A row lock or conditional write, not deletion alone, establishes ordering

PostgreSQL `FOR UPDATE` prevents other transactions from locking, modifying, or
deleting the selected row until the transaction ends. At the default `READ
COMMITTED` isolation level, an `UPDATE`, `DELETE`, or `SELECT FOR UPDATE` that
encounters a concurrent modification waits and then evaluates against the
updated row. Thus a terminal transition and frame admission can be ordered by
locking or conditionally updating the same worker-admission row in one short
transaction. A verifier deletion not paired with that decision is merely a
later cleanup operation: a handler that has already passed a separate read can
still publish a frame.

PostgreSQL also warns that locks are held to transaction end and that
inconsistent locking order can deadlock. The terminal-fence path and frame
admission path therefore need the same lock order (Run then worker, or worker
alone if that record contains all required authority facts) and must not wait
on a broker or runtime call while holding the transaction.

Sources: [PostgreSQL explicit locking](https://www.postgresql.org/docs/current/explicit-locking.html#LOCKING-ROWS), [PostgreSQL transaction isolation](https://www.postgresql.org/docs/current/transaction-iso.html#XACT-READ-COMMITTED).

### Revocation evidence does not require retaining a usable verifier

Neither NIST nor PostgreSQL requires a system to retain a credential verifier
after invalidation. The durable evidence needed to explain a decision is
different from material that can verify a presented bearer capability. NIST's
definition focuses on removal of the binding, while PostgreSQL supplies the
transactional mechanism to make that removal and its associated state change
commit together. The choice of an audit record's fields is consequently a
repository design decision, not an external standard.

## Repository facts

- SimOps presently keeps one plaintext `ingest_token` on `simops_runs` and
  compares it before decoding frames, publishing telemetry, and updating worker
  state. The check, publish, and updates are independent operations; there is
  no worker-scoped verifier or admission transaction. See
  `backend/slurm-gateway/internal/gateway/simops_controller.go` (lines
  312-348) and `deploy/postgres-init/001_simops.sql` (lines 3-37).
- The existing terminal-ingest research has already established that durable
  admission and terminal fencing must serialize on the same Run/worker facts,
  while broker delivery follows through an outbox or idempotent delivery path.
  See [terminal-ingest fence research](simops-ingest-fence-research.md).
- Reactor Telemetry is a useful but non-identical precedent. It records
  `CredentialsRevoked`, immediately makes authorization fail when removal
  begins, preserves that outcome through restart, and separately retries
  runtime cleanup. It uses signed credentials and a set-level lifecycle, so it
  is not an implementation template for the new worker-bound opaque
  capability. See
  `backend/slurm-gateway/internal/gateway/reactor_telemetry_worker_set.go`
  (lines 362-406) and
  `backend/slurm-gateway/internal/gateway/reactor_telemetry_postgres_integration_test.go`
  (lines 36-47).

## Repository proposal, not a source requirement

Use a persistent **Worker Ingest Admission** record for every planned worker.
It contains the worker identity, current admission state, fence generation,
and (only while admission is allowed) a server-side verifier of the opaque
credential. It also retains nonsecret terminal-fence evidence: fenced time,
reason, generation, and an admission/audit record identifier.

At terminal observation, or at the policy-defined unresolved-stop fence, one
transaction should:

1. lock or conditionally update that worker's admission record;
2. persist the terminal/fenced admission state and advance its fence generation;
3. **clear the verifier value** (`NULL` it or delete it from a distinct
   verifier table) in that same transaction; and
4. write the nonsecret audit/outbox evidence in that transaction.

Frame admission uses the same record in the same lock order. It may verify the
presented opaque credential only if the row is still admitting. A transaction
that obtains the record after fencing sees no verifier/admissible state and
rejects. One that commits before the fence is correctly recorded as
pre-fence—not retrospectively relabelled because its response or broker delivery
occurred later.

```mermaid
sequenceDiagram
    participant I as "Frame admission"
    participant A as "Worker admission row"
    participant F as "Terminal fence"
    participant O as "Outbox/audit row"

    alt "Admission commits first"
        I->>A: "lock; verify; admit generation N"
        I->>O: "record admitted frame"
        I->>A: "commit"
        F->>A: "lock; fence; generation N+1; clear verifier"
        F->>O: "record fence"
        F->>A: "commit"
    else "Fence commits first"
        F->>A: "lock; fence; generation N+1; clear verifier"
        F->>O: "record fence; commit"
        I->>A: "lock; find fenced / no verifier"
        I-->>I: "reject"
    end
```

Do **not** retain a disabled verifier merely to make audit convenient. It
extends the residence of material that can validate a bearer capability without
adding authorization value. Preserve the credential's opaque identifier (or a
non-reversible audit correlation chosen deliberately), worker and Run identity,
fence generation, reason, timestamps, and decision/outbox identifiers instead.
If an investigation has a genuine requirement to correlate a presented token
after revocation, define that separately with retention, access, and threat
model; it is not required for lifecycle recovery.

The resulting state is restart-safe: after a committed fence, reconciliation
finds a durable fenced state with no verifier and never recreates it. If the
transaction aborts, both authority and audit changes roll back together, so the
previous admissible state remains coherent. Runtime cleanup retries are
independent and must not restore ingest authority.

## Decision impact

The previous wording, “destroy the verifier atomically,” is sound if
**destroy** means clearing/deleting verification material in the same durable
admission transaction—not deleting a whole worker record and not issuing a
best-effort removal after a lifecycle update. Retain the admission record as a
fenced tombstone until ordinary Run/worker retention removes it; retain only
nonsecret evidence there. This gives the Lifecycle Reconciliation module a
single deep ownership point for authority, fencing, audit, and restart
recovery, without turning the event log into Event Sourcing.
