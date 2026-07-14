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

**SimOps Runtime Adapter**:
The runtime-specific launcher that turns a Run Connection Profile into an external worker execution record while preserving the Simulation Ops run interface.
_Avoid_: Shell launcher, worker script, data-plane adapter

**Observed Worker Lifecycle**:
The runtime-resource state observed for a run-scoped worker, such as pending, active, succeeded, failed, missing, image-pull-failed, or stopped.
It is not telemetry health, artifact disposition, data-plane health, or simulated result quality.
_Avoid_: Telemetry health, artifact health, infrastructure health

**Gateway-Only Worker Ingest**:
The rule that an ordinary Run-Scoped Simulation Worker sends operational telemetry and simulated result state through Simulation Ops gateway ingest URLs and tokens, without receiving direct Redpanda, Postgres, Iceberg, Docker, or Kubernetes credentials.
Trusted data-plane roles may receive the credentials their role requires; ordinary run-scoped workers may not.
_Avoid_: Direct data-plane worker, credentialed simulation worker

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
A backend Simulation Ops execution lifecycle with its own run, worker, event, and artifact outcomes. An eligible Fleet Board intent may request or associate a Run through Artifact Forge, but a local Simulation Job never becomes the Run.
_Avoid_: Simulation Job, game tick, local job state

**Artifact Forge**:
The server-side boundary that validates one explicit Fleet Board forge request, associates it with one SimOps Run, and may translate one eligible simulation artifact with Simulated Result State and complete Lineage into one versioned game outcome.
Operational telemetry, failed Runs, incomplete artifacts, and missing Lineage are ineligible.
_Avoid_: Telemetry reward, automatic backend launch, evidence generator

**Reactor Telemetry Worker Set**:
A bounded group of public-safe Resident Source workers associated with one player-added reactor and game session. It produces reactor-scoped Measured State through source-scoped Gateway-Only Worker Ingest and is not a Run-Scoped Simulation Worker set or production SCADA.
_Avoid_: Simulation worker pool, production telemetry, per-run worker set

**Configured Data Flush**:
A dry-run-first clearing of accepted local-demo runtime records that preserves schemas, source declarations, credentials, required topics, Compose wiring, platform configuration, and protected volumes while opening a new coherent data generation.
_Avoid_: Environment teardown, volume pruning, factory reset

**Workbench Snapshot**:
One coherent read generation of independently labeled Measured State, Simulated Result State, Twin State, and Lineage returned through the read-only Workbench interface. Live, stale, recovering, and fixture Snapshots must never be field-wise mixed.
_Avoid_: Best-effort aggregate, mixed-generation response, fixture patch

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
