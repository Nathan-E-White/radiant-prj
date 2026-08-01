# Master Architecture Findings: Primary-Source Research Ledger

## Scope and status

This is the consolidated research ledger for every finding accepted in the
architecture counterfactual and ripple passes. It distinguishes three things
which are routinely—but unhelpfully—conflated:

1. **Current code evidence**: a fact directly observable in this checkout.
2. **External constraint**: a fact established by an owning specification or
   official documentation.
3. **Accepted proposal**: a future module or rule. It is not implementation
   evidence merely because it is included in this ledger.

The ledger preserves the terms in `CONTEXT.md`. In particular, a Workbench
Snapshot is not an Experience Scenario, an Artifact Forge outcome is not
Objective Evidence, and Delivery Assurance is not broker-offset completion.
No decision here authorizes implementation by itself.

## Primary sources

- Apache Iceberg, [specification](https://iceberg.apache.org/spec/) and
  [reliability guidance](https://iceberg.apache.org/docs/1.10.2/reliability/).
- Kubernetes, [container probes](https://kubernetes.io/docs/concepts/workloads/pods/probes/)
  and [Job lifecycle](https://kubernetes.io/docs/concepts/workloads/controllers/job/).
- Go, [`context`](https://pkg.go.dev/context) and [cancelling in-progress
  operations](https://go.dev/doc/database/cancel-operations).
- PostgreSQL, [transaction isolation](https://www.postgresql.org/docs/current/transaction-iso.html).
- IETF, [Media over QUIC transport draft](https://datatracker.ietf.org/doc/draft-ietf-moq-transport/)
  and [WebTransport over HTTP/3](https://www.ietf.org/archive/id/draft-ietf-webtrans-http3-14.html).
- CloudEvents, [core specification](https://github.com/cloudevents/spec/blob/main/cloudevents/spec.md)
  and [scope primer](https://github.com/cloudevents/spec/blob/main/cloudevents/primer.md).
- IETF, [HTTP semantics: idempotent methods](https://datatracker.ietf.org/doc/html/rfc9110#section-9.2.2).
- OpenTofu, [plan/apply workflow](https://opentofu.org/docs/cli/run/) and
  [refresh guidance](https://opentofu.org/docs/cli/commands/refresh/).

The MoQ documents are drafts, not a claim of protocol standardization or
interoperability. The repository itself describes its current transport as a
MoQ-compatible envelope, not a full relay/CDN implementation
([data-plane ledger](simops-data-plane-todo-stubs.md)).

## Consolidated findings

| # | Finding and current evidence | External constraint | Accepted proposal and research consequence |
| --- | --- | --- | --- |
| 1 | **Iceberg lifecycle concentration.** Writer selection and artifact lifecycle are currently co-located in `simops_iceberg_writer.go`; modes include manifest, external command, disabled, and Iceberg-Go. | Iceberg commits make a table metadata change atomically, using optimistic concurrency; they do not constitute a transaction with the broker or control-plane store. | Introduce one deep **Iceberg Lifecycle module** at the catalog/table seam. Its interface owns table discovery, creation, schema compatibility, append preparation, and fresh readback. Writer modes remain adapters. Do not call it a generic storage module. |
| 2 | **Delivery Assurance and crash recovery.** Current artifact records can reach the shared `committed` status after materially different writer paths. The existing research note records the Iceberg-Go readback check but distinguishes it from manifest and process acknowledgement. | Iceberg retry resolves metadata conflicts; a repeated logical append after an unknown outcome still needs an application-level identity and reconciliation rule. CloudEvents permits a resend to retain the same `source`/`id`, but explicitly does not supply delivery guarantees. | A **Delivery Attempt module** owns a stable batch identity, pending/unknown/resolved state, assurance claim, and verified delivery evidence. This is an at-least-once recovery state machine, not exactly-once processing or Event Sourcing. |
| 3 | **Delivery pacing and reconciliation.** `SimopsArtifactIntentProcessor` groups event work in memory before writing; delivery state and broker acknowledgement are separate effects. | Neither Iceberg nor WebTransport supplies a distributed commit across the catalog, broker offsets, and the control-plane record. RFC 9110 permits automatic retry only where intended effect is idempotent; a POST is not made safe merely by retrying it. | Keep pacing and reconciliation inside the Delivery Attempt implementation: bounded batches, one ordered coordinate set, and explicit unknown-commit recovery. A separate generic retry module would be shallow. |
| 4 | **Gateway-owned execution intent and access.** The control plane already owns Run lifecycle while `RunConnectionProfile` carries role-scoped configuration; ordinary worker tests assert exclusion of data-plane credentials. | Kubernetes Jobs create finite workload executions; credentials and authorization remain application policy, not something a Job controller can infer. | Make the gateway the sole module that accepts execution intent, selects allowed runtime behavior, and issues role-scoped access. Browser code and local game state must not select runtime/image/credential details. This deepens the established control-plane seam rather than adding another adapter. |
| 5 | **Profile-only runtime execution adapters.** Existing `RunConnectionProfile` construction and Docker/Kubernetes adapters show a real varying seam. `adr-0010.md` already assigns Run truth to the control plane. | Go `Context` is designed to carry cancellation and deadlines through calls; adapter operations must honor the propagated context to terminate coherently. | Keep runtime adapters profile-only: start, stop, and observe external resources; never own Run truth. The interface remains small because lifecycle policy and recovery stay in the control-plane implementation. |
| 6 | **Lifecycle obligations and readiness.** The server enables lifecycle health, while Fleet Board reconciliation exposes distinct expiry/retention task outcomes. | Kubernetes distinguishes liveness (restart-oriented) from readiness (traffic eligibility); readiness can change throughout lifetime. | Create a named **Lifecycle Obligation module** whose implementation schedules, cancels, records, and exposes each obligation. Readiness consumes its declared policy; a Workbench Snapshot read does not trigger it. This is a state machine, not a liveness probe wrapper. |
| 7 | **Cross-runtime SimOps contract corpus.** The checkout contains runtime, Snapshot, telemetry, and schema contracts, but their evidence is spread among source, fixtures, and checks. | Kubernetes observation states and Go cancellation semantics are runtime-specific facts that must be translated before callers can rely on a common Run meaning. | Maintain one contract corpus of executable vectors spanning profile construction, runtime observations, telemetry/result envelopes, Snapshot validity, and artifact evidence. This is a test corpus, not a new production abstraction. |
| 8 | **Run stream authorization and filtering.** The stream gateway sends the router Snapshot to each connected session; authorization policy cannot repair a retained value that has already been overwritten. | MoQ/WebTransport separates sessions and transport of media/data; it does not supply this product's Run authorization policy or retained-state semantics. | Introduce a Run-scoped subscription rule at the stream seam: authorize first, then select only that Run's retained tracks and future messages. Treat it as authorization plus filtered fan-out, not an access-control pattern claim beyond the actual policy. |
| 9 | **Control-plane outbox.** The controller currently has paths that publish an event and update stored facts as distinct effects. | Database transaction isolation gives one database transaction a coherent view; it does not atomically include an external broker publish. | Use a control-plane outbox implementation to record a domain fact and its intended publication atomically in the control-plane store, then publish/reconcile asynchronously. It is the Transactional Outbox pattern-shaped case; it does not turn the application into Event Sourcing. |
| 10 | **Strict Workbench Snapshot acceptance.** Frontend live-Snapshot validation checks generation and completeness; `workbenchSnapshotSession` already distinguishes live, fixture, stale, recovering, and error. | PostgreSQL repeatable-read supplies a stable database view for multiple reads in a transaction; it does not validate semantic cross-runtime coherence by itself. | Preserve one acceptance module that validates provenance, generation, completeness, and whole-Snapshot source before projection. Fixture fallback remains whole-Snapshot and local-demo-only. This is a validation gate at the existing read seam, not a cache. |
| 11 | **Artifact Forge settlement.** `adr-0007.md` requires an explicit intent, idempotency key, eligible artifact, atomic consumption marker, and at most one game outcome. | No listed platform source supplies this domain eligibility rule; it is a repository-owned invariant. | A settlement module owns request recovery, eligibility, outcome mapping, and the consumption marker. It is an idempotent state machine. It must not consume operational telemetry or fabricate Objective Evidence. |
| 12 | **Configured Data Flush generation transition.** The PostgreSQL flush implementation has an advisory lock and planning/apply stages; the glossary requires a monotonic new generation with no mixed Workbench Snapshot. | PostgreSQL transactions and locks coordinate the database portion only; they cannot make external runtime deletion or stream delivery atomic. OpenTofu likewise separates reviewable plan from apply and warns that refresh mutates state; that is a useful constraint against read-triggered authoritative mutation, not a runtime implementation choice. | A generation-transition module owns the configured plan, protected-resource inventory, switch state, recovery outcome, and admission rule. It is a compensating workflow, not a global transaction. |
| 13 | **Run-scoped track retention.** `SimopsMoQTrackRouter` retains values in `map[string]SimopsMoQTrackMessage`, while emitted lifecycle and artifact track names are shared; `Snapshot()` is then sent to every stream session. | The MoQ draft identifies objects with namespace, track name, group, and object identifiers. It does not define this product's Run mapping, so that mapping must be explicit before caching/filtering. | Replace global name retention with a Run-scoped materialized view keyed by `(runID, track)`. Fan-out is Observer-shaped. It is not Event Sourcing because only latest retained state is required. |
| 14 | **Run-scoped telemetry admission.** `validateTelemetryFrame` requires only a positive sequence; controller flow publishes and increments worker frame counts. Duplicate/reordered ingress has no durable disposition. | The broker and Iceberg sources do not provide a single identity authority for the gateway's telemetry ingress. | A telemetry-admission module owns `(run, worker, sequence)` disposition—first accept, duplicate, gap, or out-of-order—before downstream effects. This is an idempotent admission state machine, not a claim of exactly-once telemetry. |
| 15 | **Run Observation Session.** The browser refreshes events and the Run on an interval while the event listing is an unbounded append-copy read. | WebTransport permits bidirectional session transport but does not define a product cursor, history limit, reconnection rule, or authorization model. | One observation-session module combines bounded historical catch-up with the authorized live Observer leg and exposes live/catching-up/degraded/expired state. The current read model and live stream are CQRS-shaped; neither is sufficient alone. |
| 16 | **Workbench Review Context.** `App.tsx` and `SimulatorWorkbenchSurface.tsx` compose live Workbench input with fixture compute/job/scenario state. The glossary explicitly says an Experience Scenario is neither a live Workbench Snapshot nor a Run. | There is no external standard for the domain distinction; the controlling source is the repository glossary and ADR-0007 provenance rules. | Add a presentation-level review-context module that declares the source and permitted links for every rendered state. It is Mediator-shaped coordination only; it must not become another truth store or field-wise merge fixtures into a Snapshot. |

## The ripple dependency graph

The accepted work is not one large reliability initiative. It forms a small
chain of local truths. The first group makes storage and execution claims
honest; the second makes their publication and read-model effects recoverable;
the third prevents the browser from presenting a misleading composite.

```text
Iceberg lifecycle
  -> Delivery Attempt / assurance / recovery / pacing
  -> control-plane outbox
  -> Artifact Forge settlement

Gateway execution intent
  -> profile-only runtime adapters
  -> lifecycle obligations and readiness
  -> contract corpus

Run stream authorization
  -> run-scoped track retention
  -> telemetry admission
  -> Run Observation Session
  -> Workbench Review Context

Configured Data Flush generation transition
  -> strict Snapshot acceptance
  -> Workbench Review Context
```

The arrows mean “the downstream module must preserve the upstream invariant,”
not “implement in one deployment.” The `Configured Data Flush` and Workbench
Snapshot terms already require generation coherence; the third-order finding
is that presentation composition must preserve it as well.

## Research limits and decision tests

- Iceberg supports atomic table metadata commits, **not** an end-to-end
  exactly-once claim across Redpanda, the control-plane store, and the table.
- MoQ/WebTransport support transport mechanics, **not** product authorization,
  run identity, cursor recovery, or retention design.
- PostgreSQL can make its own transaction coherent, **not** turn unrelated
  external effects into a distributed transaction.
- Kubernetes readiness supports traffic-policy signaling, **not** a definition
  of Radiant lifecycle obligations.

Before implementation, each proposed module should pass the deletion test:
deleting it must cause its ordering, identity, recovery, and provenance rules
to reappear across callers. If not, retain direct data flow instead. The
highest test seam is the module interface: artifact delivery reconciliation,
Run admission, Snapshot acceptance, configured generation transition, and
review-context provenance—not the private storage shape of an adapter.

## Suggested evidence matrix

1. Force an Iceberg unknown commit and prove the same Delivery Attempt is
   reconciled before another logical append.
2. Replay an artifact intent and prove one settled Artifact Forge outcome and
   one consumption marker.
3. Stop a runtime adapter during a lifecycle cycle and prove cancellation,
   distinct obligation outcome, and truthful readiness.
4. Send duplicate, gap, and reordered telemetry and prove one explicit
   admission disposition for each identity.
5. Connect concurrent Runs with identically named tracks and prove each
   authorized subscriber gets only its own retained/latest data.
6. Reconnect an observation session after a bounded cursor and prove it enters
   catching-up then live without a full-history poll.
7. Interrupt a configured generation transition and prove recovery resumes one
   generation or rolls back to the prior one; no Snapshot mixes either.
8. Render live, stale, fixture, and Experience Scenario review contexts and
   prove every displayed relationship has declared provenance.
