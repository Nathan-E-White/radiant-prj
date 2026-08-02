# Radiant

Radiant is a public-safe engineering workbench for compute readiness, Simulation Ops, and Simulator Workbench demonstrations. Its language keeps data provenance explicit so measured observations, digital-twin estimates, and simulation results are not collapsed into one generic metric stream.

## Language

**Simulator Workbench**:
A top-level workbench surface that presents measured state, imputed state, simulated result state, and lineage together for public-safe review.
_Avoid_: SimOps tab, SCADA dashboard, control room

**Value Basis**:
The provenance class of a displayed value: measured, imputed, or simulated. It is part of the value's meaning, not a visual label that can be dropped.
_Avoid_: Metric type, category, display group

**Measured State**:
Direct sensor or SCADA stand-in observations from resident sources. In this repo it is always public-safe stand-in data, not real plant telemetry.
_Avoid_: Simulated reading, model output

**Imputed State**:
Digital-twin state inferred from measured inputs, model state, and lineage. It is model-derived state, not a raw observation.
Only the twin projector emits imputed state; Slurm/SimOps workers must never label their output as imputed.
_Avoid_: Measurement, sensor value

**Simulated Result State**:
Run-scoped scientific or compute result state produced by simulation workers and tied to a run, model, or artifact.
SimOps workers produce simulated result state separately from operational telemetry; the twin may consume it to create imputed state.
_Avoid_: Measurement, SCADA value

**Resident Source**:
A public-safe measured-source stand-in that exists independently of any single simulation run.
_Avoid_: Simulation worker, run worker

**Lineage**:
The source tags, model steps, simulation runs, and artifacts that explain why a displayed value exists.
_Avoid_: Log, trace, breadcrumb

**Simulation Health Summary**:
A compact trust summary for simulation result state, covering run completion and artifact disposition at a glance.
_Avoid_: Detailed health panel, SCADA health panel, infrastructure diagnostics

**Run-Scoped Simulation Worker**:
A worker that exists for a single Simulation Ops run and produces operational telemetry or simulated result state for that run.
_Avoid_: Resident source, data-plane service, permanent worker

**Run Connection Profile**:
The per-run launch contract that carries worker identity, ingest connectivity, runtime labels, cleanup policy, and credential boundaries for a run-scoped worker or trusted data-plane role.
_Avoid_: Docker config, job spec, environment blob

**Runtime Binding**:
The immutable SimOps Run association with one Runtime Adapter, selected when the Run is created and used for all later execution and reconciliation. A different container provider requires a new Run.
_Avoid_: Runtime migration, current process configuration, portable worker

**SimOps Runtime Adapter**:
The runtime-specific launcher that turns a Run Connection Profile into an external worker execution record while preserving the Simulation Ops run interface.
_Avoid_: Shell launcher, worker script, data-plane adapter

**Observed Worker Lifecycle**:
The runtime-resource state observed for a run-scoped worker, such as pending, active, succeeded, failed, missing, image-pull-failed, or stopped. Missing and image-pull-failed are initially uncertain and reconciliation-resolvable; each becomes a terminal Run failure input only after the bounded reconciliation policy is exhausted. Once the control plane durably establishes a terminal state, no later nonterminal observation may regress it; contradictory later adapter evidence opens or advances Terminal Observation Conflict without rewriting terminality.
It is not telemetry health, artifact disposition, data-plane health, or simulated result quality.

**Runtime Observation**:
The read-only adapter inquiry that returns an Observed Worker Lifecycle and its execution evidence. It may be repeated by reconciliation or verification without starting, stopping, deleting, or otherwise changing an external runtime resource. Runtime Cleanup is a separate explicit effect.
_Avoid_: Cleanup pass, compensating action, provider mutation, implicit force-stop
_Avoid_: Telemetry health, artifact health, infrastructure health

**Runtime Stop Request**:
The recorded request to halt one Run-Scoped Simulation Worker. It persists across a gateway restart and expresses desired execution state, distinct from the worker's later Observed Worker Lifecycle, which alone establishes that the worker stopped or reached another terminal state. A `runtime-limit` request is an internal, idempotent Lifecycle Reconciliation trigger when the Control-Plane Admission Clock reaches the shared Run deadline; it is not a distinct Run Lifecycle, a public module, or an adapter policy. If requested during launch uncertainty, Lifecycle Reconciliation first locates the worker by stable identity, then stops it if found; it neither launches a replacement nor treats local cancellation as proof of absence. Valid Gateway-Only Worker Ingest remains admitted while stop observation is pending; a stop request is not credential revocation. Post-stop frames add Active Execution Observed evidence but never clear unresolved stop, extend its policy, or retract the request. Repeated requests return the existing request; Lifecycle Reconciliation alone retries an external stop after a recorded failure or timeout within a bounded policy. If stop observation exhausts without proof, the Run remains visibly stopping and unresolved rather than being declared stopped or failed. A later missing resource remains missing even after a stop request; absence is not proof of a clean stop.
_Avoid_: Stopped worker, cancellation proof, terminal execution

**Runtime Cleanup Outcome**:
The recorded result of removing one worker's external runtime resource after terminal execution. It persists across a gateway restart, is distinct from execution terminality, and may be retried safely within a bounded policy without rewriting a terminal SimOps Run as failed. A policy-exhausted missing worker is cleanup-eligible so reconciliation can remove a possible orphan by its stable identity; cleanup remains unresolved until removal or idempotent absence is observed. Successful cleanup preserves the worker's established terminal Observed Worker Lifecycle; it does not turn the worker into an ambiguous missing resource. Runtime Observation is read-only; cleanup is the separate explicit Lifecycle Reconciliation effect. Cleanup is held while Terminal Observation Conflict is Queued or Verifying, and is eligible when it is Clear, Resolved, or Unresolved. A transition to Resolved or Unresolved durably records that eligibility before any cleanup attempt begins; cleanup and resulting absence do not corroborate or resolve the conflict. A Run-level cleanup state is derived from its worker outcomes and does not itself block later Runs; any admission restriction is explicit policy.
_Avoid_: Run failure, execution failure, artifact deletion

**Current Worker Record**:
The authoritative persisted current facts for one Run-Scoped Simulation Worker: desired stop state, Worker Launch Disposition, monotonic Observed Worker Lifecycle, Active Execution Observed, Runtime Cleanup Outcome, and bounded retry metadata. It supports recovery and lifecycle derivation without replaying an event history. Events may remain notifications or audit material, but they are not the source of truth for worker lifecycle.
_Avoid_: Event-sourced aggregate, append-only runtime history, transient adapter state

**Worker Launch Disposition**:
The derived control-plane account of whether a worker launch was unattempted, attempted but proved absent, launched, or unambiguously rejected. Unattempted and attempted-absent both satisfy a Runtime Stop Request without cleanup; a rejected launch contributes a failed Run only when no stop request supersedes it. It is distinct from Observed Worker Lifecycle, which describes an external resource only once one is observed.
_Avoid_: Provider status, worker execution outcome, generic missing state

**Worker Failure Stage**:
The derived evidence stage of a failed launched worker: failed without active observation, or failed after active observation. It is derived from Active Execution Observed. The former means the control plane did not establish active execution before failure; it does not claim the runtime physically never ran. Provider reason, message, exit evidence, and observation time remain the source facts.
_Avoid_: Provider failure taxonomy, physical root cause, inferred execution history

**Active Execution Observed**:
The monotonic worker fact that the control plane has observed active execution at least once. It is set by an active runtime observation or an accepted authenticated worker telemetry frame, and never cleared, preserving evidence across later terminal observations and restart. It does not prove uninterrupted execution or describe current worker state.
_Avoid_: Current active state, runtime uptime, inferred execution history

**Lifecycle Reconciliation Policy**:
The bounded control-plane policy for resuming one class of runtime work. Start reconciliation, stop observation, and cleanup each have a separate policy because their evidence, safety constraints, and acceptable delay differ; adapters report facts but do not choose the policy.
_Avoid_: Shared retry budget, adapter retry loop, provider default backoff

**Run Lifecycle**:
The exclusive, derived execution progression of a SimOps Run: starting, streaming, stopping, complete, failed, or stopped. It states the Run's durable execution meaning, not whether its current obligations are healthy or cleanup is complete.
_Avoid_: Health indicator, cleanup status, provider status

**Run Phase**:
The exclusive, derived substate that explains the current Run Lifecycle, such as dispatching, awaiting launch observation, executing, awaiting stop observation, stop unresolved, or stop settled. Exactly one Phase belongs to an active nonterminal Lifecycle and may explain a terminal stop outcome.
_Avoid_: Additional Run lifecycle, retry record, provider state

**Operational Condition**:
The derived confidence condition of a SimOps Run. Healthy means every currently applicable control-plane obligation is within its policy; Degraded means one or more explicit Degradation Reasons remain. A Run may execute or await stop while Healthy.
_Avoid_: Execution lifecycle, idle state, provider health

**Degradation Reason**:
The current, derived explanation for a Degraded Operational Condition, such as uncertain launch, uncertain image pull, missing observation, unresolved stop, unresolved cleanup, or late deadline enforcement. It is not a provider error string or an event-history record. A reason ceases to be current when its condition resolves; its material facts may remain as Resolved Degradation Evidence.
_Avoid_: Generic degraded state, runtime status, permanent audit event

**Resolved Degradation Evidence**:
The retained factual account of a Degradation Reason that is no longer current, including its cause and observed resolution. It preserves material operational history without keeping the Run's Operational Condition Degraded after the relevant condition resolves.
_Avoid_: Active Degradation Reason, erased incident, replay-based lifecycle authority

**Terminal Observation Conflict**:
The worker-local evidence condition in which a later adapter observation contradicts an established terminal Observed Worker Lifecycle. The first durable terminal outcome remains authoritative. A later nonterminal state is a nonterminal regression; a different terminal state is a terminal disagreement; a later identical terminal state is corroboration. Resource absence alone is an observation gap, not corroboration.
_Avoid_: Terminal lifecycle rewrite, generic provider error, proof from resource absence

**Terminal Observation Conflict Status**:
The nested status of Terminal Observation Conflict: NotApplicable before terminality; Clear when no contradiction is current; Queued while bounded verification is due; Verifying while reconciliation is obtaining evidence; Unresolved when that policy exhausts; and Resolved when the established terminal outcome is corroborated. Queued, Verifying, and Unresolved are active Degradation Reasons; Resolved retains Resolved Degradation Evidence. A new contradiction after resolution starts a new conflict episode.
_Avoid_: Run Lifecycle, adapter state, global conflict queue, permanent degraded state

**Terminal Observation Conflict Reconciliation Policy**:
The separate bounded policy that verifies a Terminal Observation Conflict. It may corroborate the established terminal outcome or expose an unresolved conflict, but it never changes that outcome. Its budget is independent of start recovery, Run Deadline Reconciliation, stop observation, and cleanup.
_Avoid_: Stop-observation retry budget, lifecycle rollback, unbounded provider polling

**Verification Status**:
The nested, policy-scoped status of a Terminal Observation Conflict verification episode: NotRequired when no conflict exists; Pending when verification may be claimed; Claimed while one reconciler holds a recoverable verification claim; RetryScheduled after an inconclusive attempt awaits its next eligible time; Corroborated when the established terminal outcome is observed again; and Exhausted when the bounded policy ends without corroboration. It verifies evidence only and never decides or rewrites worker terminality.
_Avoid_: Worker lifecycle, permanent in-progress state, provider outcome, generic job status

**Verification Claim**:
The time-limited durable right for one reconciler to perform one Verification Status attempt. Expiry returns the episode to Pending after a crash or abandoned attempt. It is an attempt to observe evidence, not proof that an external observation completed.
_Avoid_: Permanent ownership, completed verification, distributed lock without expiry

**Run Cleanup Aggregate**:
The derived cleanup disposition across a Run's eligible workers: not applicable, pending, complete, or unresolved. A fully not-launched Run is not applicable because no external resource was proved to exist; it is not a successful cleanup claim. The Aggregate does not alter the Run Lifecycle or itself block later Runs.
_Avoid_: Execution terminality, Run failure, admission policy

**Gateway-Only Worker Ingest**:
The rule that an ordinary Run-Scoped Simulation Worker sends operational telemetry and simulated result state through Simulation Ops gateway ingest URLs and its Worker Ingest Credential, without receiving direct Redpanda, Postgres, Iceberg, Docker, or Kubernetes credentials. Valid authenticated frames remain admitted during Stopping / AwaitingStopObservation as Post-Stop Evidence and become Active Execution Observed evidence. Admission ends on terminal worker observation or under the explicit bounded policy for unresolved stop observation.
Trusted data-plane roles may receive the credentials their role requires; ordinary run-scoped workers may not.
_Avoid_: Direct data-plane worker, credentialed simulation worker

**Worker Ingest Admission**:
The derived authorization disposition for one Run-Scoped Simulation Worker. It admits frames authenticated by that worker's Worker Ingest Credential while its execution is nonterminal, including Stopping / AwaitingStopObservation; it fences only that worker's frames on terminal worker observation or under an explicit bounded unresolved-stop policy. Fencing atomically advances the Ingest Fence Generation, clears verifier material, and records a Fenced Admission Tombstone. Admission and fencing are serialized durable control-plane decisions; they are not inferred from a Runtime Stop Request alone.
_Avoid_: Stop request, worker terminality, token lifetime alone

**Run Deadline**:
The immutable shared end of a SimOps Run's authorized execution window. It is derived from the accepted Run's creation time and runtime limit, before any runtime dispatch or active-execution observation. It bounds launch uncertainty as well as observed execution and cannot be extended by a late worker, a delayed adapter observation, or a retry. It does not rewrite an observed terminal worker outcome into a synthetic execution failure.
_Avoid_: Adapter dispatch time, first active observation, per-worker deadline, renewable execution lease

**Post-Deadline Runtime Observation**:
An Observed Worker Lifecycle received after the Run Deadline. It is execution evidence, not proof that the worker executed after the deadline: provider observation may itself be delayed. A terminal observation is preserved in the worker and derived Run outcome, while Deadline Enforcement Lag retains the control-plane delay evidence. An available Provider Terminal Time can qualify the evidence but cannot rewrite the outcome.
_Avoid_: Proven runtime overrun, authorization to ingest, synthetic failure

**Adapter Observation Time**:
The time a Runtime Adapter reports that it observed a worker lifecycle fact. It is diagnostic execution evidence and may be delayed, skewed, or absent relative to the control plane. It never orders Durable Ingest Admission, deadline enforcement, or derived Run lifecycle.
_Avoid_: Control-Plane Recorded Time, admission authority, durable ordering

**Control-Plane Recorded Time**:
The time at which the control plane durably records a worker lifecycle fact. In the Postgres-backed control plane it is the Control-Plane Admission Clock evaluated in the transaction that records the fact; an in-memory control plane uses an injected equivalent for deterministic tests. It orders control-plane state transitions and deadline-reconciliation dispositions. It is distinct from Adapter Observation Time and Provider Terminal Time.
_Avoid_: Provider timestamp, adapter clock, simulated event time

**Provider Terminal Time**:
An optional time supplied by a runtime provider for when it reports a worker reached terminal execution. An adapter supplies it only when one unambiguous terminal time can be attributed to the stable worker identity; otherwise it is absent. It is normalized as diagnostic evidence beside Adapter Observation Time and Control-Plane Recorded Time. It may distinguish a delayed observation from provider-reported late termination, but it is never clock authority for Worker Admission, Runtime Stop Requests, retries, or derived Run lifecycle.
_Avoid_: Control-Plane Admission Clock, authorization deadline, proof of exact execution time, lifecycle transition

**Run Deadline Reconciliation**:
The durable scheduled control-plane obligation that finds SimOps Runs whose Run Deadline has arrived. For a nonterminal worker it initiates the `runtime-limit` Runtime Stop Request and Worker Ingest Admission fence; for a worker already terminal when reconciliation arrives, it preserves the observed outcome and records no retroactive stop. It is independent of Run reads and worker traffic; an expired ingest attempt may encounter the same fence but is never the enforcement mechanism.
_Avoid_: Ingest-triggered expiry, read-triggered stop, adapter timer, worker watchdog

**SimOps Lifecycle Reconciliation**:
The control-plane authority that resumes and derives a SimOps Run's lifecycle obligations, including start recovery, Run Deadline Reconciliation, stop observation, and cleanup. It owns their distinct bounded policies and resulting Operational Conditions. Runtime adapters provide execution effects and observations to it, but do not choose timing, retry, or Run outcome.
_Avoid_: Reactor Telemetry reconciliation, HTTP handler, runtime adapter, one shared retry loop

**Deadline Enforcement Lag**:
The observed elapsed time between a Run Deadline and the first durable Run Deadline Reconciliation disposition: for a nonterminal worker, the action that fences admission and records the required `runtime-limit` Runtime Stop Request; for an already-terminal worker, the preserved terminal observation. If it exceeds the declared enforcement bound, it is an explicit Degradation Reason until the Run settles, then remains as Resolved Degradation Evidence. It measures control-plane enforcement, not completion of external runtime termination, which remains subject to stop observation. It never extends the Run Deadline, reopens a Worker Admission Window, or excuses the required stop request.
_Avoid_: Runtime extension, scheduler jitter hidden as success, provider-stop duration, a reason to skip stop

**Worker Admission Window**:
The half-open period during which one Worker's Ingest Credential may authorize Gateway-Only Worker Ingest. It begins when the credential is issued and ends exclusively at the shared Run Deadline: a frame is admissible only before that deadline, while the deadline itself belongs to the expired and fenced side. The Control-Plane Admission Clock decides the boundary. A Runtime Stop Request normally leaves the window open for Post-Stop Evidence; a `runtime-limit` request closes it at the shared deadline.
_Avoid_: Credential lifetime as an approximate duration, grace after expiry, Post-Stop Evidence entitlement

**Worker Ingest Admission Policy**:
The bounded control-plane policy governing how long Worker Ingest Admission can remain open after stop observation becomes unresolved. It is separate from the stop-observation policy because credential authority and runtime observation have different safety costs, but its deadline cannot outlive the visible unresolved-stop window.
_Avoid_: Stop-observation policy, token expiry only, indefinite grace period

**Ingest Fence Generation**:
The monotonically advancing Worker Ingest Admission epoch. A terminal worker observation or unresolved-stop fence advances that worker's generation; each accepted worker frame retains the generation under which it was admitted. It is evidence of admission order, not by itself the synchronization mechanism.
_Avoid_: Run-wide fence, lifecycle state, credential version alone, distributed lock

**Durable Ingest Admission**:
The atomic control-plane decision that either accepts one frame authenticated by a Worker Ingest Credential under that worker's current Ingest Fence Generation or fences it. It serializes frame admission and terminal fencing against the same Run and worker facts; delivery to an external event transport occurs only after durable admission and does not decide authority.
_Avoid_: Handler lifecycle read, broker acknowledgment, event-sourced authority

**Worker Ingest Credential**:
The immutable opaque gateway-ingest capability issued to one ordinary Run-Scoped Simulation Worker. The worker receives the capability only through runtime-secret injection while the control plane retains only a server-side verifier. It binds exactly one Run and Worker identity; it authorizes that worker's telemetry and simulated result frames but no sibling's frames or direct data-plane access. It is valid only within that worker's Worker Admission Window, whose exclusive end is the shared Run Deadline; expiry is an explicit fence reason decided by the Control-Plane Admission Clock. Fencing clears the verifier material, permanently ending its authority. It never appears in Run responses, event payloads, logs, or browser-visible configuration.
_Avoid_: Run-shared token, self-contained signed token, provider credential, data-plane credential

**Fenced Admission Tombstone**:
The nonsecret durable record that a Worker's Ingest Admission was fenced. It retains Run and Worker identity, fence reason and time, Ingest Fence Generation, and audit reference after verifier material is cleared. It never restores credential authority.
_Avoid_: Disabled credential verifier, secret audit record, cleanup recovery state

**Control-Plane Admission Clock**:
The single clock authority for Worker Ingest Credential expiry and Durable Ingest Admission. In the Postgres-backed control plane it is PostgreSQL `clock_timestamp()` evaluated inside the serialized admission transaction. Worker, Docker, and Kubernetes clocks are evidence only and cannot extend or decide admission authority.
_Avoid_: Gateway process clock, worker clock, provider clock, telemetry timestamp

**Post-Stop Evidence**:
Valid authenticated worker telemetry or result frames accepted after a Runtime Stop Request and before ingest is fenced. It is retained as execution evidence, but cannot independently complete a SimOps Run or make an artifact eligible; those outcomes remain derived from runtime observation and lifecycle policy.
_Avoid_: Completion signal, artifact acceptance, stop retraction

**SimOps Runtime Proof**:
A verification claim that a SimOps Runtime Adapter can launch, observe, and clean up Run-Scoped Simulation Workers through the existing Simulation Ops run interface while preserving Gateway-Only Worker Ingest.
It is narrower than full data-plane or lakehouse validation.
_Avoid_: Full lakehouse proof, browser UX proof, platform provisioning proof

**Kaleidos Unit**:
The single standardized reactor unit represented by the Simulator Workbench: a public-safe Kaleidos-style prismatic HTGR with TRISO/prismatic graphite core, helium primary loop, control drum, turbomachinery/cooling, reactor/shielding, vessel/container boundary, and heat/electric output context.
_Avoid_: Generic reactor, reactor zoo, pebble-bed variant

**Kaleidos Fleet**:
An ensemble view of identically produced, separately operated Kaleidos Units. Fleet views may summarize unit-level readiness, output, freshness, and lineage, but each unit remains its own reactor instance.
_Avoid_: One giant reactor, mixed reactor family, centralized control-room simulation

**Fleet Board**:
A playable Simulator Workbench board-game surface where abstract facilities, routes, service credits, and pressure pawns make fleet-scale readiness concepts tangible. It is a local demo game layered over Workbench projection state, not a real operations, design, dispatch, billing, or control surface.
_Avoid_: Fleet command, control console, operator game, real deployment planner

**Breaker-to-Breaker Run**:
The commercial operating interval for a Kaleidos Unit between grid/output connection and the next planned or unplanned breaker separation. An unplanned outage resets the run.
_Avoid_: Generic uptime, always-on status

**Availability Phase**:
The public-safe fleet-strip phase for a Kaleidos Unit. Initial phases are online generation, ramping, cooldown, planned maintenance outage, unplanned maintenance outage, and refueling outage.
_Avoid_: Standby, offline, emergency, alarm state

**Planned Maintenance Outage**:
A scheduled non-refueling outage for inspection, maintenance, or service work. It interrupts breaker-to-breaker operation but is not an abnormal event.
_Avoid_: Offline, standby, emergency

**Unplanned Maintenance Outage**:
An unplanned outage bucket for forced maintenance, unplanned trips, special excursions, or other abnormal conditions that should not become alarm-management UI in this slice.
_Avoid_: Emergency panel, SCRAM workflow, incident command

**Refueling Outage**:
A scheduled outage specifically for fuel replacement or fuel-related maintenance. It is distinct from generic planned maintenance.
_Avoid_: Planned maintenance only, fuel status badge

**Trouble Pawn**:
The Fleet Board's visible event-pressure marker. It represents toy disruptions such as routing pressure, service delay, or short local outage pressure, and should stay playful and public-safe.
_Avoid_: Disaster response, emergency, incident command, sabotage, attack

**Simulation Container Token**:
A Fleet Board local-game resource representing simulated-job capacity installed on one reactor's Reactor Slot Rail. One token costs 2 Simulation Budget, and each rail holds at most two tokens. It is not real infrastructure capacity, cloud spend, project budget, or a live SimOps control.
_Avoid_: Real container quota, cloud budget, production capacity, live scheduler control

**Simulation Budget**:
A Fleet Board local-game resource used only to buy Simulation Container Tokens. A new game starts with 6 Simulation Budget; it is separate from cash and does not represent cloud spend, project funding, or infrastructure quota.
_Avoid_: Cash, cloud budget, compute credits, project budget

**Reactor Slot Rail**:
The two-slot local-game capacity display attached to one Fleet Board reactor. It shows whether each Simulation Container Token is idle, queued, or running a Simulation Job and has no relationship to a real scheduler, container runtime, or plant system.
_Avoid_: Container pool, scheduler queue, Kubernetes capacity, reactor control

**Simulation Job**:
A deterministic Fleet Board local-game lifecycle queued on one idle Simulation Container Token. It starts on the next day tick, completes after three advances, and remains local game state rather than a SimOps Run, Slurm job, backend artifact, or evidence record.
_Avoid_: SimOps Run, Slurm job, backend submission, objective evidence

**SimOps Run**:
A backend Simulation Ops execution lifecycle with its own run, worker, event, artifact, and Result Finality outcomes. Its Run Lifecycle, Run Phase, Operational Condition, Run Cleanup Aggregate, and Result Finality are separate derived facts. It is stopping after a Runtime Stop Request awaits worker observation; exhaustion of stop observation leaves it stopping, stop-unresolved, and degraded. It is complete only when every planned worker succeeds, and failed only when all planned workers are terminal and at least one failed or remains missing; a later stop request does not erase an established worker failure. A Degraded Operational Condition does not stop other workers or their Gateway-Only Worker Ingest. An eligible Fleet Board intent may request or associate a Run through Artifact Forge, but a local Simulation Job never becomes the Run.
_Avoid_: Simulation Job, game tick, local job state

**Result Finality**:
The derived disposition that determines whether a SimOps Run's materially committed artifacts and results may be published for downstream consumption. It is AwaitingTerminalOutcome, AwaitingMaterialization, Withheld, Eligible, PublicationClaimed, Finalized, Finalized Under Review, Superseded, or Ineligible. It requires the relevant successful terminal outcome and materialization, but an active Terminal Observation Conflict withholds finality without undoing material commitment, execution outcome, or cleanup. A conflict discovered after Finality Grant publication moves finality to Finalized Under Review without retracting that grant. Finality Review may maintain that state, supersede it with a Finality Revision, or declare future use Ineligible.
_Avoid_: Artifact commit status, Run Lifecycle, artifact deletion, provider observation

**Finality Grant**:
The durable idempotent authorization published by Result Finality for a named downstream consumer to use a Run's finalized result. It is issued only from Eligible through a recoverable publication claim and owns an immutable Grant Recipient Snapshot captured with publication. Consumers receive this grant rather than independently inferring lifecycle, artifact, and evidence eligibility. For each new governed use, a consumer resolves Finality Lineage and uses only its current consumable grant; a historical grant remains audit evidence, not ongoing authorization.
_Avoid_: Artifact-ready event, implicit consumer permission, raw terminal status

**Grant Recipient Snapshot**:
The immutable set of stable grantee identities recorded with one Finality Grant at publication. It fixes the recipients of any later Finality Correction Notice; role, subscription, endpoint, or configuration changes after publication do not alter that obligation. It retains identity rather than a frozen delivery endpoint: Finality Correction Delivery resolves the grantee's current endpoint at each attempt, and failed identity resolution is an explicit delivery failure. It is a Finality Grant value, not a separate lifecycle or module.
_Avoid_: Current role lookup, dynamic subscription, delivery endpoint, mutable recipient list

**Finality Correction Notice**:
The durable idempotent notice to a named Finality Grant recipient that newly discovered evidence puts the granted result under review. It does not retract or erase the grant or its past consumption; it exposes the changed confidence condition so the recipient can apply its own remediation policy. When relevant, it identifies only that recipient's unresolved Presented Consumption Authorization and exposure facts, without claiming to cancel it or exposing another recipient's authorization or outcome data. Each required notice is governed by Finality Correction Delivery. Recipient acknowledgment or remediation is a consumer-owned fact and may be retained only as an audit reference; it never proves delivery, resolves evidence, or changes Result Finality. Result Finality remains Finalized Under Review while any required correction is undelivered or unresolved.
_Avoid_: Grant revocation, deletion of consumed output, best-effort notification, silent confidence change

**Finality Correction Delivery**:
The bounded delivery lifecycle of one Finality Correction Notice: NotRequired, Pending, Claimed, RetryScheduled, Delivered, or Unresolved. Claimed is a recoverable time-limited delivery attempt; Unresolved ends automatic delivery and retains the Result Finality under review. Only a Correction Rearm by an authenticated control-plane operator returns an Unresolved notice to Pending, using the same notice identity.
_Avoid_: Best-effort event publish, permanent claim, unbounded automatic retry, new notice on recovery

**Finality Review**:
The privileged control-plane assessment of a Result Finality that remains under review because of unresolved evidence or correction delivery. Its decision is Maintain Under Review, Supersede, or Ineligible. A versioned Result Finality Policy selected when review opens determines disposition admissibility and is retained on the immutable decision; that version remains governing until the review is decided or abandoned, even when a newer policy becomes active. A Result Finality has at most one open review. A time-bounded durable Reviewer Claim grants exclusive temporary decision authority; its expiry returns the review to Open without deciding, abandoning, or changing it. That review may be abandoned and replaced by one under newer policy or later evidence, but the abandonment is immutable, actor-attributed, and reasoned; it never silently erases the old review or alters Result Finality. It may update the current Result Finality projection but never mutates a historical Finality Grant, correction notice, delivery outcome, or established worker lifecycle.
_Avoid_: Silent override, adapter decision, grant mutation, treating Unreachable as Delivered

**Reviewer Claim**:
The durable, time-bounded exclusive right for an authorized control-plane operator to decide one open Finality Review. It expires safely to Open, is recoverable after operator failure, and neither decides nor abandons the review on expiry.
_Avoid_: Permanent lock, browser session, decision itself, automatic abandonment

**Review Authorization**:
The Result Finality Policy's derived determination that an authenticated control-plane actor may Open, Claim, Decide, or Abandon a Finality Review. It owns action authority, claim validity, canonical actor identity, and optional separation of duties. Under dual control for irreversible decisions, the actor deciding Supersede or Ineligible must differ from both the review opener and current Reviewer Claim holder and must hold a distinct durable affirmative review of the cited evidence.
_Avoid_: Caller-owned role check, display-name comparison, adapter authorization, informal approval

**Review Approval**:
The distinct, durable, actor-attributed affirmative review of the Closed Evidence Snapshot required by a dual-control Result Finality Policy before a Supersede or Ineligible decision. It is bound to the exact evidence set, governing policy version, and proposed disposition; a change to any of them requires a new approval. It expires after the governing policy's defined period, requiring fresh assessment without rewriting its history. It records the approving actor and decision intent; it is not a generic or informal permission.
_Avoid_: Rubber stamp, chat approval, role membership, mutable comment

**Disposition Admissibility**:
The Result Finality Policy's derived determination that a proposed Finality Review disposition is Admissible, Requires Explicit Justification, or Prohibited for its cited Closed Evidence Snapshot. Material omissions may require decision-local justification for Supersede or Ineligible; loss of subject, prior-grant, or successor-material identity is always Prohibited.
_Avoid_: Caller-owned matrix, generic veto on incomplete evidence, free-form exception without evidence, adapter policy

**Finality Revision**:
A new immutable Finality Grant and current Result Finality projection issued by a Supersede decision. Supersede atomically consumes any required Review Approval, records the decision, creates the revision and successor grant, and updates Result Finality. It identifies both the prior grant and prior result material and supplies corrected or requalified result material. A later explicitly opened review may create it after an Ineligible disposition; nothing reopens automatically. The prior grant remains historical, and its correction obligations remain visible.
_Avoid_: Edited old grant, implicit replacement, erased consumption history

**Finality Lineage**:
The immutable singly linked chain of Finality Grants and Finality Revisions for one Finality Subject. Each revision has one predecessor grant and predecessor result material; each grant or material has zero or one direct successor. It derives the single current lineage head and the current consumable grant, if one exists. It returns a short-lived, integrity-protected Consumption Authorization for one bounded governed use rather than exposing a reusable raw grant. Failure to resolve it is No Consumable Finality for a new governed use; a consumer may not fall back to a cached historical grant.
_Avoid_: Mutable latest flag, branching corrected outputs, consumer-chosen revision, erased predecessor

**Consumption Authorization**:
The short-lived integrity-protected authorization returned by Finality Lineage for one bounded new governed use of current result material. It binds the consumer, current grant, result material, use scope, and mandatory caller-supplied idempotency key without becoming a reusable Finality Grant or a cached fallback. Presentation atomically re-checks that its grant remains the current consumable lineage head and marks the authorization Presented; this is the linearization point. A later revision blocks new use but does not retroactively interrupt the already presented bounded use; the predecessor grant's correction obligations remain visible. It also blocks every later retry under that authorization, even with the same idempotency key; a fresh attempt must resolve current Finality Lineage. It has a bounded outcome-reporting window; expiry without a report retains Presented as unresolved evidence and infers neither success nor failure. Only the canonical consumer bound to it may submit its outcome reference; a mismatched identity is rejected. The first accepted outcome reference is immutable; later reports are separately retained contradictory evidence, never an overwrite. A report after the window is retained as explicitly late evidence but leaves Presented unresolved. Its record and outcome reference remain retained at least through the corresponding Finality Lineage and correction-obligation retention period. Unresolved Presented is consumption observability, not automatic Result Finality or Run Operational Condition degradation. Contradictory or late outcome evidence is available to review policy and operators but never opens Finality Review automatically. A recorded consumer outcome reference is not proof of an external real-world side effect.
_Avoid_: Raw grant, permanent bearer token, generic session, cached authorization

**Review Attention**:
The bounded recipient-scoped operational work item derived from late or contradictory Consumption Outcome evidence. It deduplicates exposure anomalies for operator action without changing Result Finality or opening Finality Review. Stronger exposure evidence creates a new linked attention and marks the prior attention Superseded, preserving history while keeping one active attention. It has an explicit resolution deadline and escalation policy; expiry escalates recipient-scoped operator urgency only, never Run health or Result Finality. An open attention adopts the newest escalation policy immediately. An operationally authorized actor may Resolved No Review with immutable actor-attributed rationale and cited evidence references; escalation records an explicitly opened Finality Review, whose stronger privileged authority policy remains separate.
_Avoid_: Automatic finality change, generic incident ticket, cross-recipient observability, unbounded inbox

**Closed Evidence Snapshot**:
The immutable evidence set cited by a Finality Review decision. It is closed at a declared scope version and source watermark, contains normalized stable references to the evidence considered, and has a SnapshotCompleteness result. Its declared review scope includes Consumption Authorizations and outcome references tied to the grant or result under review; they establish exposure context, not automatic correctness conclusions. Later evidence, source availability changes, or policy changes create a new snapshot rather than modifying this one.
_Avoid_: Live query result, mutable review attachment, copied provider log, implicit current-state reconstruction

**SnapshotCompleteness**:
The derived closure state of a Closed Evidence Snapshot. Collection moves from Not Started through Collecting and possibly Awaiting Sources to Complete or Incomplete; only Sealed Complete or Sealed Incomplete may support Finality Review. An Incomplete snapshot records every required omission, its reason, and retryability. A privileged Finality Review may Maintain Under Review, Supersede, or declare Ineligible from either sealed state; its decision retains the snapshot and omissions as immutable evidence. For an incomplete snapshot, the decision classifies every omission as Material or Nonmaterial and records its rationale. Sealed completeness never changes: later evidence requires a new snapshot at a later watermark.
_Avoid_: Boolean flag, review judgment, silently partial evidence, mutable sealed snapshot

**Correction Rearm**:
The explicit privileged control-plane action that resumes automatic delivery of one Unresolved Finality Correction Notice under its existing identity. It is authorized only for an authenticated control-plane operator. A Finality Grant recipient may acknowledge or remediate the notice, and a Runtime Adapter may provide evidence, but neither may rearm delivery policy.
_Avoid_: Recipient retry request, adapter retry loop, a new correction notice, automatic timer retry

**Artifact Forge**:
The server-side boundary that validates one explicit Fleet Board forge request, associates it with one SimOps Run, and may translate one eligible simulation artifact with Simulated Result State and complete Lineage into one versioned game outcome.
Operational telemetry, failed Runs, incomplete artifacts, and missing Lineage are ineligible.
_Avoid_: Telemetry reward, automatic backend launch, evidence generator

**Reactor Telemetry Worker Set**:
A bounded group of public-safe Resident Source workers associated with one player-added reactor and game session. It produces reactor-scoped Measured State through source-scoped, reactor-bound gateway ingest credentials and is not a Run-Scoped Simulation Worker set or production SCADA.
_Avoid_: Simulation worker pool, production telemetry, per-run worker set

**Configured Data Flush**:
A dry-run-first clearing of accepted local-demo runtime records that preserves schemas, source declarations, credentials, required topics, Compose wiring, platform configuration, and protected volumes while opening a new coherent data generation.
_Avoid_: Environment teardown, volume pruning, factory reset

**Workbench Snapshot**:
One coherent read generation of independently labeled Measured State, Simulated Result State, Twin State, and Lineage returned through the read-only Workbench interface. Reading it does not reconcile retention or otherwise mutate lifecycle state; live, stale, recovering, and fixture Snapshots must never be field-wise mixed.
_Avoid_: Best-effort aggregate, mixed-generation response, fixture patch

**Lifecycle Reconciliation**:
The scheduled fulfilment of distinct expiry and retention obligations for Reactor Telemetry Workers, Artifact Forge records, and dynamic Measured State. It is independent of a Workbench Snapshot read and retains each obligation's own outcome.
_Avoid_: Read-triggered cleanup, aggregate-only cleanup, request-path garbage collection

**Lifecycle Health**:
The operational truth of whether the configured Lifecycle Reconciliation obligations have completed successfully and remain within their required interval. It is distinct from process liveness and determines whether the gateway is ready to serve its declared lifecycle policy.
_Avoid_: Liveness, absence of requests, aggregate error string

**Trust Lens**:
A focused Simulator Workbench review mode that keeps the selected value's Value Basis, freshness, confidence, Workbench Snapshot generation, and Lineage visible together.
_Avoid_: Generic details panel, metric inspector, provenance tooltip

**Review Playback**:
A user-controlled replay of coherent Workbench Snapshots or local Fleet Board transitions for explaining what changed without merging generations or creating new Objective Evidence.
_Avoid_: Evidence recording, live telemetry stream, mixed-state timeline

**Board Navigator**:
The non-canvas representation of Fleet Board tiles, facilities, routes, pawns, and available local-game actions that stays synchronized with the rendered board and supports keyboard review.
_Avoid_: Screen-reader fallback, alternate game state, control console

**Experience Scenario**:
A coherent, named product state used to preview and verify a user journey across visual, semantic, and interaction conditions. It is test and design material, not a live Workbench Snapshot or a SimOps Run.
_Avoid_: Fixture blob, test case only, demo mode

**Insight Token**:
A reactor-scoped Fleet Board local-game reward produced when a Simulation Job completes. One token automatically absorbs one Inspector or Trouble non-refueling outage for its reactor; fuel-driven refueling never spends it. This is a game rule, not a safety claim, operating recommendation, or backend simulation result.
_Avoid_: Safety credit, validated result, operational recommendation, backend artifact

**Cooldown**:
A post-shutdown phase where the unit is not commercially generating but still has active thermal/reactor-state work to represent. Cooldown is not standby and does not imply immediate restart availability.
_Avoid_: Standby, offline, idle

**Cooldown Heat**:
The residual heat-generation context shown for a unit in cooldown. It is usually Imputed State unless the fixture represents a direct measured thermal tag. It is reactor-state context, not commercial thermal output, delivered heat, or outage economics.
_Avoid_: Commercial output, heat sale, lost-generation cost

**Core Power Distribution Estimate**:
A coarse Imputed State value derived from multiple public-safe neutron flux stand-ins plus reactor configuration context. It may support a simple axial/radial overlay, but it is not validated neutronics, an in-core detector map, or safety analysis.
_Avoid_: Core power shape proof, validated flux map, safety limit

**Multi-Zone Flux Stand-In**:
A public-safe measured stand-in for relative neutron flux at a coarse core zone. Several such stand-ins are required before the UI may display a Core Power Distribution Estimate.
_Avoid_: Single probe power shape, real in-core detector, safety instrumentation

**Commercial Mode**:
The business context used by the presentational fleet strip to explain why a Kaleidos Unit's output matters. Initial source-backed fixture modes are PPA electric, direct unit sale, facility heat, desalination heat, and resilience backup. This is display context, not billing logic.
_Avoid_: Direct unit lease, billing engine, tariff model, financial settlement

**Commercial Display Basis**:
The visible fixture assumptions used to explain a commercial fleet-strip value, including commercial mode, output window, electric or thermal output basis, rate-assumption label, freshness, and lineage. It is presentation context only.
_Avoid_: Invoice basis, settlement basis, tariff model, market position

**Accrued Display Value**:
A fixture-derived estimate used to show that a unit's delivered energy, delivered heat, or availability context has commercial relevance. It is not recognized revenue, an invoice amount, a receivable, or a settlement result.
_Avoid_: Revenue, accrued revenue, bill, settlement, receivable

**Delivered Energy**:
The fixture-backed electric energy displayed for a Kaleidos Unit over a visible output window. It is a physical/presentational counter and must not imply market settlement or metered billing.
_Avoid_: Energy settlement, cleared energy, invoice kWh

**Delivered Heat**:
The fixture-backed thermal energy displayed for facility heating or desalination context over a visible output window. It is a presentation counter and not a process-heat contract guarantee.
_Avoid_: Heat invoice, guaranteed thermal delivery, tariff heat credit

**Contracted Availability**:
The presentational idea that a unit's availability matters in a commercial mode. It should be displayed as context, not as a capacity-market payment, capacity accreditation, or settlement credit.
_Avoid_: Capacity payment, capacity credit, RPM credit, market availability charge

**Availability Display Credit**:
A local demo score or display contribution for a unit being in an availability-supporting phase. It is not a market credit, billing credit, or reliability-product settlement.
_Avoid_: Availability credit, capacity credit, performance credit

**Outage Economics**:
Out of scope for the presentational Simulator Workbench slice. Real operators may track lost generation opportunity cost and additional operations or maintenance cost during outages, cooldown, and refueling, but this project shall not model those costs.
_Avoid_: Lost revenue counter, outage cost model, maintenance expense tracker

**Resilience Backup**:
A commercial mode where a Kaleidos Unit is presented as supporting backup or resilience value for a facility type such as a hospital, datacenter, military installation, or remote site. It is not an emergency management panel or black-start control workflow.
_Avoid_: Emergency mode, incident command, black-start dispatch, alarm state

**Facility Heat**:
A commercial mode where thermal output is presented as useful heat for facility heating. It is display context only and does not claim cogeneration qualification, process guarantee, or heat-sale billing.
_Avoid_: Cogeneration certification, steam contract, heat tariff

**Desalination Heat**:
A commercial mode where thermal output is presented as useful heat for water desalination. It is display context only and does not model a desalination plant contract or water-service billing.
_Avoid_: Water sale, desalination contract engine, process guarantee

**Pebble-Bed Reactor**:
Out of scope for Radiant. Pebble-bed designs may appear in external HTGR background research, but the Simulator Workbench shall not include pebble-bed topology or pebble-bed comparison modes.
_Avoid_: Pebble-bed toggle, VHTR comparator mode, alternative fuel-form view

**Nuclear Thermal Propulsion**:
Out of scope for Radiant. The Simulator Workbench shall not include rocket propulsion, propellant tank, turbopump, nozzle, or NTP test-article topology.
_Avoid_: Propulsion analogue, NTP comparison mode, hydrogen nozzle loop

### Public Writing

**Public-Facing Document**:
An artifact created for readers outside the project team, including resumes, presentation narratives, public bios, and externally shared project summaries. It should explain value and evidence without private implementation sprawl, compliance overclaiming, or demo hype.
_Avoid_: Internal note, marketing copy, private work log

**Resume**:
A public-facing evidence document that compresses a person's relevant work into concrete roles, outcomes, and proof points. It is not a biography, sales sheet, or exhaustive project inventory.
_Avoid_: Bio, CV dump, hype sheet, work diary

**Hemingway-Star Style**:
The local writing baseline adapted from The Kansas City Star copy rules associated with Ernest Hemingway: short openings, vigorous concrete language, positive construction, and deletion of superfluous words. It is a public-document discipline, not imitation of Hemingway's fiction.
_Avoid_: Literary voice, macho minimalism, marketing punch-up
