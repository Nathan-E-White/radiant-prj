# Fleet Operations Domain Guide

| Field | Value |
| --- | --- |
| Document ID | FLEET-OPS-DOMAIN-001 |
| Revision | 0.1 |
| Status | Domain guide |
| Owner | Software |
| Scope | Public-safe Fleet Operations vocabulary for Radiant/Kaleidos demos |

## Purpose

This guide pins down what `Fleet Operations` means in Radiant's Simulator Workbench and Status Workbench language. The short version: Fleet Operations is the read-only coordination layer for many separately operated Kaleidos Units. It is not a control room, a SCADA alarm console, a trading desk, a billing system, or a reactor-operations simulator.

## Primary-Source Anchors

Detailed source notes are captured in [Fleet Operations Primary-Source Research](./fleet-operations-primary-source-research.md).

Radiant describes Kaleidos as a compact, transportable reactor package that can replace diesel generation, support remote villages, hospitals, datacenters, military installations, and provide both electric and thermal output. Source: <https://www.radiantnuclear.com/>.

Radiant explicitly supports a fleet-scale view: it says hundreds of units can autonomously operate while data streams back to Radiant's centralized 24/7 fleet monitoring system tracking reactor health. Source: <https://www.radiantnuclear.com/>.

Radiant says Kaleidos units are assembled, fueled, and tested in the factory; after five or more years of operation, the container can be shipped back for refueling; and customers can purchase through PPAs or direct unit sales. Source: <https://www.radiantnuclear.com/>.

The NRC describes Kaleidos as a transportable HTGR micro-reactor designed for 3 MWth and approximately 1 MWe, using TRISO fuel, helium gas coolant, and prismatic graphite blocks, with each micro-reactor contained in a single shipping container. Source: <https://www.nrc.gov/reactors/new-reactors/advanced/who-were-working-with/pre-application-activities/kaleidos>.

The NRC Kaleidos pre-application page lists a non-public proprietary `Kaleidos Fleet Operator Training White Paper` under review. That proves fleet-operator language exists in the regulatory engagement surface, but the public page does not expose enough content to model operator training details. Source: <https://www.nrc.gov/reactors/new-reactors/advanced/who-were-working-with/pre-application-activities/kaleidos>.

The NRC defines outage vocabulary that should constrain public availability phases: an outage is a period when a generating unit or similar facility is out of service; forced outages are emergency or unanticipated unavailable conditions; scheduled outages are inspection, maintenance, or refueling shutdowns scheduled well in advance. Sources: <https://www.nrc.gov/reading-rm/basic-ref/glossary/outage>, <https://www.nrc.gov/reading-rm/basic-ref/glossary/outage-forced>, <https://www.nrc.gov/reading-rm/basic-ref/glossary/outage-scheduled>.

## Canonical Meaning

`Fleet Operations` is the public-safe coordination context for a `Kaleidos Fleet`.

It covers:

- unit identity and selected-unit context;
- reactor-health monitoring context, expressed as public-safe readiness/freshness summaries;
- availability phase, including online generation, ramping, cooldown, planned maintenance outage, unplanned maintenance outage, and refueling outage;
- factory/refueling/service context at a summary level;
- electric output, useful thermal output, residual heat context, and commercial display context;
- lineage that explains where a displayed value came from and whether it is measured, imputed, or simulated;
- synthetic fleet-screen analysis, such as anomaly flags or readiness summaries, when clearly marked as fixture/demo data.

It does not cover:

- remote reactor operation or control actions;
- setpoints, actuator commands, dispatch commands, or start/stop workflows;
- alarm acknowledgement, incident command, emergency response, SCRAM workflow, or safety-path UI;
- production SCADA, historian, calibration, PLC/RTU diagnostics, or device-maintenance workflow;
- billing, settlement, tariff calculation, LMP, bids/offers, invoices, receivables, revenue recognition, capacity-market products, or trading;
- real Kaleidos telemetry or real Radiant infrastructure claims.

## Relationship To Existing Terms

`Kaleidos Fleet` is the ensemble: many identically produced, separately operated Kaleidos Units. `Fleet Operations` is the coordination context around that ensemble.

`Fleet Strip` is a UI pattern. It is a compact row of fleet cards, not the whole domain.

`Commercial Mode` explains why a unit's output matters in a presentational business context. It does not create billing logic.

`Commercial Display Basis` explains value-like counters in the rail. It is the guard against financial overclaiming: if a card shows `$18.4k (est)`, the rail must explain the fixture assumptions and exclusions.

`Value Basis` still rules everything. Fleet Operations must not flatten measured observations, imputed twin estimates, and simulated results into one generic metric pile.

## Source-Backed First-Slice Fields

| Field | Meaning | Boundary |
| --- | --- | --- |
| `unitId` | Fixture identity for a Kaleidos Unit. | Not a real asset registry. |
| `availabilityPhase` | Public-safe operating/outage phase. | Not alarm state or emergency workflow. |
| `commercialMode` | Presentational business context such as PPA electric, direct unit sale, facility heat, desalination heat, or resilience backup. | Not a contract engine. |
| `breakerToBreakerLabel` | Local display counter for current commercial operating interval. | Not a regulated performance metric. |
| `electricOutputMwe` | Fixture electric output. | Not metered settlement data. |
| `usefulThermalOutputMwt` | Fixture thermal-output context. | Not a heat-sale guarantee. |
| `residualHeatMwth` | Cooldown reactor-state context. | Not commercial thermal output. |
| `freshness` | Display freshness or degradation marker. | Not live telemetry SLA. |
| `commercialBasisId` | Link to display assumptions and exclusions. | Not an audit, invoice, or settlement record. |
| `lineage` | Explanation of measured/imputed/simulated source chain. | Not an operational event log. |

## Domain Scenarios

### KAL-03 Online In Desalination Heat Mode

The fleet card may show online generation, electric output, useful thermal output, desalination heat, and a compact display estimate. Selecting the value should open Commercial Display Basis with output window, delivered energy, delivered heat, fixture freshness, and exclusions. It should not expose water-sale billing, tariff rules, customer invoice state, or market settlement.

### KAL-04 In Cooldown

The fleet card may show cooldown, breaker-to-breaker reset, residual heat, stale freshness, and no commercial output. Residual heat belongs to reactor-state context and usually has `valueBasis=imputed`; it must not be converted into delivered heat, lost revenue, outage economics, or a commercial heat-sale claim.

### KAL-05 In Refueling Outage

The fleet card may show refueling outage and direct unit sale context. That is source-aligned because Radiant describes shipping the container back for refueling after five or more years. The UI should not invent fuel-handling procedures, factory work orders, regulatory training content, or shipment logistics.

### Fleet Anomaly Flag

A synthetic fleet-screen worker may flag one unit or channel for engineering review if the evidence and lineage are fixture-backed. The label should be `fleet telemetry anomaly` or `engineering review`, not `alarm`, `trip`, `SCRAM`, `emergency`, or `operator action required`.

## Safe Language

- `Fleet Operations`
- `centralized fleet monitoring context`
- `unit-level readiness`
- `selected Kaleidos Unit`
- `fleet freshness`
- `availability phase`
- `commercial display basis`
- `engineering review`
- `display estimate`

## Banned Or Risky Language

- `fleet command`
- `remote operation`
- `control room`
- `operator console`
- `alarm queue`
- `emergency status`
- `SCRAM workflow`
- `dispatch command`
- `trading desk`
- `revenue`
- `invoice`
- `settlement`
- `tariff`
- `LMP`
- `bid`
- `offer`
- `capacity payment`
- `lost-generation cost`

## Guide For Product And UI Work

Use `Fleet Operations` when the screen is explaining fleet-scale readiness, monitoring context, availability phase, freshness, lineage, and display-only commercial output across Kaleidos Units.

Use `Kaleidos Fleet` when naming the ensemble itself.

Use `Fleet Strip` when talking about the horizontal card UI.

Use `Commercial Display Basis` whenever a fleet card includes a money-like or value-like number.

Use `Status Workbench` or `Simulator Workbench` for the actual product surface. Do not name the surface `Fleet Ops Console`; that wording invites the wrong product boundary.

## Open Questions

1. Should `Fleet Operations` become a visible UI label, or should it stay as internal domain vocabulary behind `Kaleidos Fleet` and `Status Workbench`?
2. Should synthetic fleet-screen anomaly output live under engineering lineage, Simulation Health Summary, or a future read-only Fleet Operations summary?
3. Should the repo keep `Breaker-to-Breaker Run` as the commercial interval term, or eventually replace it with a more source-backed generation/outage metric if a primary source appears?

## Recommendation

Treat Fleet Operations as a read-only coordination and explanation layer. It should help a reviewer understand how a fleet of standardized Kaleidos Units is doing, why a selected unit matters, whether its displayed values are fresh, and what basis those values rest on. The moment the UI starts implying remote control, alarm handling, settlement, dispatch, or real plant telemetry, it has wandered into a different product and should be pulled back.
