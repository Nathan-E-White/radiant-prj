# Iceberg Delivery Assurance And Recovery: Primary-Source Research

## Scope and status

This note clarifies the two proposed SimOps enhancements: an explicit delivery-assurance profile and crash-recoverable run-scoped artifact delivery. It is research, not implementation evidence. It does not claim that the current gateway provides durable checkpoints, exactly-once delivery, or a particular Iceberg catalog guarantee.

Primary sources used:

- Apache Iceberg, [Specification: goals, optimistic concurrency, sequence numbers, and commit conflict resolution](https://iceberg.apache.org/spec/).
- Apache Iceberg, [Reliability](https://iceberg.apache.org/docs/1.10.2/reliability/) and [configuration](https://iceberg.apache.org/docs/latest/configuration/).
- Apache Iceberg Go, [concurrent writes](https://go.iceberg.apache.org/concurrent-writes.html) and [table package API](https://pkg.go.dev/github.com/apache/iceberg-go/table).
- The repository pins `github.com/apache/iceberg-go v0.6.0`; its [release tag](https://apache.googlesource.com/iceberg-go/+/refs/tags/v0.6.0) is the applicable upstream source revision.

## Terms that need separating

The current artifact status `committed` is too broad for the four writer modes. The useful terms are:

- **Delivery Attempt**: one identified request to deliver one ordered batch for one SimOps Run. Its identity must be stable across retry and restart; an Iceberg snapshot ID is an outcome of that attempt, not its identity.
- **Delivery Assurance**: the narrow claim demonstrated by a completed attempt.
- **Verified Delivery Evidence**: the recorded proof for an attempt: its stable identity, the exact expected Redpanda coordinates, the achieved assurance, and—when Iceberg is used—the observed snapshot ID and readback result.
- **Unknown Commit**: a transport or process failure after an Iceberg commit may have reached the catalog but before the caller learned the result. It must be reconciled before a new append is attempted.

These terms do not alter ADR-0005. They describe the operational-telemetry storage path; they do not turn operational telemetry into Measured State, Simulated Result State, Imputed State, or Lineage.

## What each successful writer result can honestly claim

| Writer result | Supported Delivery Assurance | It does **not** establish |
| --- | --- | --- |
| Local manifest write | The gateway received no error while creating the local directory and writing the deterministic manifest payload. | An Iceberg catalog commit, readable Iceberg data files, remote durability, or broker-offset completion. The current path uses `os.WriteFile`; it does not fsync or atomically rename the file. |
| External command exit 0 | The invoked program returned success for the supplied payload. | What that program persisted, which catalog/table/snapshot it affected, whether a later process can read the data, or broker-offset completion. A process exit is not a storage proof. |
| Iceberg append plus a fresh filtered scan | At the time of the fresh catalog load and scan, the catalog exposed a committed table snapshot from which readable data files supplied every expected `(topic, partition, offset)` for the run. | Exactly-once ingestion, permanent retention, cross-store atomicity with artifact status/event publication, or consumer-offset completion. A later commit can replace the current snapshot, and a scan cannot prove an earlier offset commit did not happen. |

Iceberg's guarantee is deliberately table-scoped: readers see committed snapshots, and a write makes its file additions/removals visible atomically through the metadata commit. The specification does not provide a distributed transaction spanning Redpanda, the control-plane store, and the catalog. [Iceberg specification](https://iceberg.apache.org/spec/)

The current Iceberg-Go path does the strongest of the four checks: it appends, loads the table again, plans a `run_id` scan, reads the files, and verifies the expected Redpanda coordinates. The current manifest, external, and disabled paths can all still lead to the same control-plane `committed` status. The proposed assurance profile should expose that difference rather than decorate it with a more optimistic noun.

## Commit retries and snapshot identity

Iceberg writes are optimistic. A writer commits by replacing the catalog's metadata pointer only if the base metadata is still current; after a conflicting commit it reloads and re-applies an update when the operation's validation permits it. Appends have no file-removal preconditions and are normally retryable. [Iceberg specification](https://iceberg.apache.org/spec/) [Iceberg reliability](https://iceberg.apache.org/docs/1.10.2/reliability/)

That helps with concurrent writers, but it is not idempotency. A repeated append after an **unknown commit** can add the same logical records twice unless the delivery layer first proves that the identified batch is already present. Snapshot sequence numbers are assigned only to successful commits and can be reassigned on retry, so neither a sequence number nor a snapshot ID is a durable request key. [Iceberg specification](https://iceberg.apache.org/spec/)

The repository's pinned `iceberg-go` version supplies `AppendTable`, `CurrentSnapshot`, and scan APIs. Its official concurrency guide says commit retry is opt-in for the Go implementation and that retries reload metadata and re-run validation. [Iceberg Go concurrent writes](https://go.iceberg.apache.org/concurrent-writes.html) This must be verified against the actual catalog mode in use; a retry property on a newly created table is not evidence that an existing table has the property.

## Safe retry and restart boundary

The smallest defensible recovery protocol is a checkpointed, at-least-once pipeline with explicit deduplication evidence:

1. Before external delivery, durably record a **pending Delivery Attempt** keyed by a deterministic delivery ID and containing the exact ordered Redpanda coordinates, run, target table, and batch payload identity.
2. Append with that delivery ID in the snapshot summary/properties, then capture the returned or freshly loaded current snapshot ID. Iceberg snapshot summaries are string maps and the Go append API accepts snapshot properties, so this is a suitable audit correlation mechanism—not a uniqueness constraint. [Iceberg specification](https://iceberg.apache.org/spec/) [iceberg-go table API](https://pkg.go.dev/github.com/apache/iceberg-go/table)
3. On normal return, and especially after restart or an unknown result, load fresh catalog metadata and resolve the pending attempt by its stable ID plus the expected coordinate set. Treat "present and readable" as delivered; treat "absent" as eligible to append; treat an ambiguous or partial result as a failed/held attempt, not an automatic second append.
4. Persist Verified Delivery Evidence and the artifact's operational status only after that resolution. Commit the corresponding broker offsets last. If offset commit fails afterward, replay is expected; recovery must find the already-delivered attempt and avoid producing a second logical delivery.

This gives at-least-once broker processing with a deliberate idempotency check. It does **not** claim exactly-once end-to-end delivery: that would require a transaction or a durable deduplication authority spanning the broker-offset and catalog/control-plane boundaries. Iceberg's atomic metadata swap is valuable, but it stops at the table.

## Consequences for the two enhancements

1. **Delivery Assurance Profile** should distinguish `manifest-written`, `external-command-acknowledged`, `iceberg-readback-verified`, and `delivery-disabled`; `committed` should remain an artifact lifecycle state only if its accompanying assurance states what was actually proved.
2. **Crash-Recoverable Artifact Delivery** should checkpoint an identified batch before it is sent, record the observed Iceberg snapshot ID as evidence after reconciliation, and make broker offset acknowledgement depend on resolved evidence. A restart must not reconstruct identity from only in-memory sequence counters or infer delivery from artifact status alone.

## Acceptance evidence suggested by the sources

- Simulate a catalog conflict and show a retry refreshes metadata without changing the Delivery Attempt identity.
- Simulate a timeout/connection loss after catalog commit; restart and show reconciliation finds the expected coordinates before any new append.
- Simulate a crash after verified delivery but before broker-offset acknowledgement; replay and show one logical Delivery Attempt/evidence record rather than a duplicate logical batch.
- For each writer mode, assert the emitted Delivery Assurance string and reject a claim stronger than the mode's demonstrated proof.
- Demonstrate that the accepted protocol does not move or merge the four ADR-0005 streams.

The uncomfortable but useful conclusion is that “the command returned zero” and “a fresh Iceberg scan contains these coordinates” are not neighbouring strengths of the same claim. One is an acknowledgement; the other is table-readability evidence. Treating both as `committed` is how an otherwise sensible system acquires a rather misleading moustache.
