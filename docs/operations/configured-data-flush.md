# Configured Data Flush

Configured Data Flush clears accepted local-demo runtime and Workbench projection records without rebuilding the platform. It is a PostgreSQL transaction, not an environment cleanup command.

## Dry-run plan

The command is dry-run-only unless an exact reviewed `planId` is supplied:

```sh
CONFIGURED_DATA_FLUSH_POSTGRES_DSN='postgres://radiant:radiant@127.0.0.1:5432/radiant?sslmode=disable' \
  bun run configured-data-flush
```

The JSON plan reports the current and next Workbench generation, row counts for every target, all protected resource classes, and any active SimOps Run or Reactor Telemetry Worker Set blocker. Save or review the output before mutation. A changed generation, target count, or blocker set produces a different `planId`.

## Apply a reviewed plan

Apply only the exact identifier from the immediately preceding plan:

```sh
CONFIGURED_DATA_FLUSH_POSTGRES_DSN='postgres://radiant:radiant@127.0.0.1:5432/radiant?sslmode=disable' \
  bun run configured-data-flush -- --apply-plan cdf-REVIEWED_PLAN_ID
```

Apply re-inspects the database under a serializable transaction and rejects stale or blocked plans. The transaction clears Artifact Forge intent, outcome, and result-artifact eligibility records, SimOps event and telemetry records, measured and simulated projection rows, Twin State and Lineage, all pre-flush Twin publication recovery records, and removed Reactor Telemetry Worker Set control records. It advances `workbench_snapshot_generation` once in the same commit.

Active Runs and active or retryable Reactor Telemetry Worker Sets block mutation. Stop or reconcile them through their normal lifecycle before creating a new plan.

## Protected resources

The operation preserves:

- PostgreSQL schemas, hypertables, indexes, and constraints;
- Resident Source and tag declarations;
- SimOps Run, worker, spool-command, artifact, idempotency, and ingest-credential records;
- processed-message and consumer-offset recovery cursors, preventing retained broker records from replaying;
- required Redpanda topics and their retained records;
- Iceberg catalog metadata, object storage, and protected volumes;
- credentials, Compose wiring, and platform configuration.

The workflow never tears down the environment, prunes volumes, deletes whole volumes, or uses a volume-wide database shortcut.

## Recovery and verification

The serializable transaction either commits every target deletion with the next monotonic generation or rolls back to the prior generation. A Workbench Snapshot transaction therefore observes the complete pre-flush generation or the complete post-flush generation, never a field-wise mixture.

After apply, verify that `/api/simulator-workbench/state` reports the new generation and empty projected data. Start the next SimOps Run through the existing API; no schema migration, topic creation, Compose restart, or volume reprovisioning is required.
