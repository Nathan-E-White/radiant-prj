# Workbench Snapshot And Lifecycle: Primary-Source Research

## Scope and status

This note records the external constraints relevant to the accepted issue #135 target design. It is research, not implementation evidence: it does not claim that the current gateway already has the described lifecycle health, cancellation propagation, or read-only Snapshot behavior.

Primary sources used:

- Kubernetes, [Liveness, Readiness, and Startup Probes](https://kubernetes.io/docs/concepts/configuration/liveness-readiness-startup-probes/).
- Go, [package `context`](https://pkg.go.dev/context), [Canceling in-progress operations](https://go.dev/doc/database/cancel-operations), and [Executing transactions](https://go.dev/doc/database/execute-transactions).
- PostgreSQL, [Transaction Isolation](https://www.postgresql.org/docs/current/transaction-iso.html) and [`SET TRANSACTION`](https://www.postgresql.org/docs/current/sql-set-transaction.html).

## Health is not one condition

Kubernetes assigns different consequences to its probes. A failed liveness probe is a reason to restart a container; a failed readiness probe removes it from Service endpoints while the container continues running. Readiness is intended to cover initial work and temporary faults, and it is evaluated throughout the container lifecycle. [Kubernetes probes](https://kubernetes.io/docs/concepts/configuration/liveness-readiness-startup-probes/)

That supports the target split for the gateway:

| Target signal | Meaning | Consequence |
| --- | --- | --- |
| Liveness | The server process can make progress sufficiently to answer a minimal health check. | Do not turn an unfinished or failed reconciliation cycle alone into a restart signal. |
| Readiness | The gateway may receive ordinary traffic under the declared policy, including the lifecycle-reconciliation policy when enabled. | Return not-ready while the initial required cycle is incomplete or an applicable lifecycle obligation is degraded. |
| Lifecycle health | The named reconciliation obligations' own outcomes and startup state. | Feed readiness explicitly; retain each task's outcome for operators and metrics. |

This is an architectural application of the Kubernetes distinctions, not a prescription that every deployment must use Kubernetes probes.

## Cancellation is a control-flow contract

The Go `context` package says that a `Context` carries deadlines and cancellation across API boundaries, that the call chain should propagate it, and that derived contexts are canceled when their parent is canceled. It also directs callers to invoke the returned cancel function to release resources. [Go `context`](https://pkg.go.dev/context)

The Go database guidance makes the same connection for database work: a `Context` passed to a context-aware database operation can cancel work when its deadline expires or its parent is canceled, including when the original request is no longer needed. [Canceling in-progress operations](https://go.dev/doc/database/cancel-operations) The transaction guidance uses `DB.BeginTx` to create a `sql.Tx` for a single connection and says the transaction should end with either `Commit` or `Rollback`. [Executing transactions](https://go.dev/doc/database/execute-transactions)

Therefore the target scheduler should derive each cycle's deadline from the service-lifecycle context, pass that context to every context-aware reconciliation dependency and database operation, and classify the cause deliberately:

- service shutdown cancellation is expected termination, not an operational reconciliation failure;
- deadline expiry or an independent task error is an observable failed outcome for that named task;
- no helper should silently replace the lifecycle context with `context.Background()` for a task that must stop on shutdown.

The source does not say that every library operation is interruptible; adapters still need verification that their concrete driver and API methods honor the passed context.

## A Snapshot needs one database view and no writes

PostgreSQL documents that a `REPEATABLE READ` transaction sees rows committed before its snapshot and that successive `SELECT` statements in that transaction do not see concurrent commits made after the transaction began. [`SET TRANSACTION`](https://www.postgresql.org/docs/current/sql-set-transaction.html) describes the same practical distinction from `READ COMMITTED`, where each command can receive a new snapshot. This supports composing the Snapshot envelope's generation, measured frames, results, twin, and lineage from one read transaction rather than independently timed reads.

PostgreSQL also supports `READ ONLY` transaction mode. Its documentation says that a read-only transaction cannot alter non-temporary tables. [Transaction isolation and read-only transactions](https://www.postgresql.org/docs/current/transaction-iso.html) This provides a database-enforced backstop for the target rule that a public Snapshot read neither runs retention reconciliation nor otherwise changes persisted Workbench state.

Two limits matter:

- repeatable read gives one stable database snapshot; it does not by itself prove that the application selected semantically matching records or constructed a valid cross-runtime envelope;
- transaction options only constrain database writes. The controller boundary and tests must separately prove that a Snapshot request does not invoke in-memory retention mutation or any other side effect.

## Evidence implied by the sources

The sources narrow the useful proof rather than supplying it. The implementation evidence for this issue should show:

1. a lifecycle cycle begins before readiness becomes true when lifecycle reconciliation is enabled;
2. cancellation reaches every named task, and shutdown cancellation is distinguished from deadline/error outcomes;
3. one PostgreSQL Snapshot read uses a read-only repeatable-read transaction and yields one internally coherent envelope;
4. an expired record remains unchanged after Snapshot reads alone, then changes only after a scheduler-driven reconciliation cycle; and
5. liveness remains independently available while readiness reports the lifecycle policy state.

Those are proposed acceptance proofs. They remain pending until tests and operational evidence demonstrate them.
