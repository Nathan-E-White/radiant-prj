# Commercial Fleet Strip Domain Research

| Field | Value |
| --- | --- |
| Document ID | SWB-COMMERCIAL-RESEARCH-001 |
| Revision | 0.1 |
| Status | Research note |
| Owner | Software |
| Scope | Presentational-only fleet strip and commercial display language; no billing, settlement, trading, control, or backend wiring |

## Purpose

This note records primary-source research for a public-safe commercial fleet strip for Radiant/Kaleidos units. It supports domain-modeling work only. It does not propose financial settlement, tariff calculation, energy trading, remote operation, alarm management, emergency response, safety logic, or a production control room.

The slice must preserve the existing repo terms `Kaleidos Unit`, `Kaleidos Fleet`, `Value Basis`, `Measured State`, `Imputed State`, `Simulated Result State`, `Lineage`, `Commercial Mode`, `Availability Phase`, and `Breaker-to-Breaker Run`. Source: [CONTEXT.md](../../CONTEXT.md).

## Source Set

Primary sources used:

- Radiant public site: <https://www.radiantnuclear.com/>
- NRC Kaleidos pre-application page: <https://www.nrc.gov/reactors/new-reactors/advanced/who-were-working-with/pre-application-activities/kaleidos>
- NRC outage glossary: <https://www.nrc.gov/reading-rm/basic-ref/glossary/outage>
- NRC forced outage glossary: <https://www.nrc.gov/reading-rm/basic-ref/glossary/outage-forced>
- NRC scheduled outage glossary: <https://www.nrc.gov/reading-rm/basic-ref/glossary/outage-scheduled>
- NRC net capacity factor glossary: <https://www.nrc.gov/reading-rm/basic-ref/glossary/capacity-factor-net>
- FERC PURPA Qualifying Facilities page: <https://www.ferc.gov/qf>
- PJM Capacity Market page: <https://www.pjm.com/markets-and-operations/rpm>
- PJM Energy Market page: <https://www.pjm.com/markets-and-operations/energy>

Local project sources used:

- [CONTEXT.md](../../CONTEXT.md)
- [Presentational Digital Twin Research](./presentational-digital-twin-research.md)
- [Simulator Workbench Final Vision Plan](./simulator-workbench-final-vision-plan.md)
- [Simulator Workbench Backend Dataflow Slice](./simulator-workbench-backend-dataflow-slice.md)

## High-Confidence Business Facts

Radiant describes Kaleidos as a compact reactor package for electricity generation, diesel replacement, resilient backup power, remote villages, hospitals, datacenters, and military installations. Radiant also says Kaleidos can provide 1 MW electric output and 1.9 MW of thermal power for facility heating or water desalination while generating electricity. Source: Radiant public site.

Radiant says Kaleidos units are assembled, fueled, and tested in the factory; hundreds of units can autonomously operate with data streaming back to Radiant's centralized 24/7 fleet monitoring system; units operate for five or more years before the container is shipped back for refueling; and customers can purchase through PPAs or direct unit sales. Source: Radiant public site.

The NRC describes Kaleidos as a transportable HTGR micro-reactor designed for 3 MWth and approximately 1 MWe, using TRISO fuel, helium gas coolant, and prismatic graphite blocks, fully contained in a single shipping container. Source: NRC Kaleidos pre-application page.

For this repo, that makes these commercial modes source-backed enough for a presentational fixture: `PPA electric`, `direct unit sale`, `facility heat`, `desalination heat`, and `resilience backup`. `Direct unit lease` is not directly supported by the Radiant page; if used, label it as a hypothetical commercial fixture, not a Radiant-public offering.

## Commercial Terms

| Term | Source-backed meaning | Demo safety |
| --- | --- | --- |
| Power Purchase Agreement / PPA | Radiant says customers can purchase through PPAs; in this context the safe UI meaning is "contracted electric output context," not a full contract model. Source: Radiant public site. | Safe as `Commercial Mode: PPA electric`. Avoid price, counterparty, contract term, delivery point, imbalance, invoice, or settlement fields. |
| Direct unit sale | Radiant explicitly says direct unit sales are an option. Source: Radiant public site. | Safe as `Commercial Mode: direct unit sale`. Avoid ownership transfer, financing, warranty, tax, or revenue-recognition UI. |
| Direct unit lease | Not found in Radiant public page during this research. | Risky. Use only if product wants a hypothetical fixture; prefer `direct unit sale` or `reserved unit` unless a primary Radiant source appears. |
| Facility heat | Radiant says Kaleidos can provide thermal power for facility heating. FERC describes cogeneration as electricity plus useful thermal energy such as heat or steam. Sources: Radiant public site; FERC QF page. | Safe as `Commercial Mode: facility heat` and as thermal-output context. Avoid claiming PURPA/QF status. |
| Desalination heat | Radiant says Kaleidos can provide thermal power for water desalination. Source: Radiant public site. | Safe as `Commercial Mode: desalination heat`. Avoid water-contract pricing or plant process guarantees. |
| Resilience backup / backup power | Radiant uses resilient backup power language for hospitals, datacenters, and military installations. Source: Radiant public site. | Safe as `Commercial Mode: resilience backup` or `resilience retainer` if framed as a demo value basis. Avoid emergency panel, incident command, black-start, islanding, or remote-control claims. |
| Capacity / availability payment | PJM's capacity market secures future power supply resources for predicted demand. Source: PJM Capacity Market page. FERC QF language separately recognizes sales of energy and capacity. Source: FERC QF page. | Risky unless generic. Prefer `contracted availability` or `availability display basis`. Avoid `capacity payment`, `capacity credit`, or RPM-style language unless building a market integration. |
| Energy-only kWh sales | PJM's Energy Market page exposes day-ahead market data, bids, offers, uplift credits, and energy-market manuals. Source: PJM Energy Market page. | Risky for this slice. Use `delivered energy` as a physical display counter, not `energy settlement`, `LMP`, `day-ahead`, `real-time`, `bid`, or `offer`. |
| Avoided cost / customer value | FERC says QFs generally can sell to a utility at the utility's avoided cost or a negotiated rate. Source: FERC QF page. | Risky as a dollar basis. Safe only as `customer value estimate` with fixture assumptions. Avoid implying PURPA rate calculation, tariff calculation, or utility avoided-cost filing. |

## Revenue And Accrual Language

This slice should not display real revenue. Words like `revenue`, `invoice`, `settlement`, `billing`, `receivable`, `tariff`, `LMP`, `bid`, `offer`, `cleared`, and `market participant` drag the UI toward accounting, billing, or market operations. PJM exposes explicit Billing, Settlements & Credit surfaces and market-settlement data; that is exactly the category this presentational strip should not fake. Source: PJM Energy Market page; PJM Capacity Market page.

Safe display language:

- `Accrued display value`: fixture-derived commercial value for presentation only.
- `Delivered energy`: MWh or kWh counter derived from fixture output over elapsed time.
- `Delivered heat`: MWhth, MWth-hours, or similar thermal-service counter derived from fixture output over elapsed time.
- `Contracted availability`: display basis showing that a unit is in a commercial mode where readiness matters.
- `Availability display credit`: internal demo score, not a market credit or invoice credit.
- `Customer value estimate`: public-safe estimate using visible fixture assumptions.

Use-with-warning language:

- `Accrued value`: acceptable only with a visible `display estimate` label. Without that, it sounds like revenue accrual.
- `Availability credit`: risky because it can sound like capacity-market or settlement credit. Prefer `availability display credit`.
- `Capacity payment` and `availability payment`: real commercial/market terms; use only in source research or explanatory notes, not as a first-slice UI field.
- `Energy settlement`: avoid. It implies a settlement system.
- `Avoided cost`: avoid in UI unless the screen is explicitly a regulated-utility/PURPA explainer. FERC's definition is real and legalistic enough to make a casual demo field look like tariff work.

## Availability And Outage Language

NRC defines an outage as a period when a generating unit, transmission line, or other facility is out of service; outages may be forced or scheduled and may be full or partial. Source: NRC outage glossary.

NRC defines a forced outage as a shutdown for emergency reasons or an unanticipated breakdown that cannot reasonably be delayed beyond 48 hours, and says forced outages do not include scheduled outages for inspection, maintenance, or refueling. Source: NRC forced outage glossary.

NRC defines a scheduled outage as shutdown for inspection, maintenance, or refueling scheduled well in advance, and says scheduled outages do not include forced outages. Source: NRC scheduled outage glossary.

NRC defines net capacity factor as net electricity generated over a period divided by the energy that could have been generated at continuous full-power operation during that same period. Source: NRC net capacity factor glossary.

Recommendations for the fixture-backed strip:

| Existing term | Keep? | Rationale |
| --- | --- | --- |
| `online generation` | Yes | Public-safe commercial phase; does not imply control authority. |
| `ramping` | Yes | Useful display phase when output is changing. Avoid controls. |
| `cooldown` | Yes | Keeps post-shutdown thermal state distinct from `offline` or `standby`. |
| `planned maintenance outage` | Yes | Aligns with NRC scheduled outage language without collapsing refueling. |
| `unplanned maintenance outage` | Yes | Safer public label for forced/unanticipated unavailability. Avoid `emergency` or `SCRAM`. |
| `refueling outage` | Yes | Source-backed as a scheduled-outage purpose and important for Kaleidos because Radiant says the container is shipped back for refueling after five or more years. Sources: NRC scheduled outage glossary; Radiant public site. |
| `forced outage` | Use in research only | Source-backed, but too sharp for a public demo strip unless the UI is explicitly an availability research view. |
| `availability factor` | Defer | No primary-source definition was captured in this pass. Use `availability phase` instead. |
| `EFOR` / `GADS` | Avoid in UI | PJM publishes monthly Equivalent Forced Outage Rates as market data, but this slice should not imply NERC/GADS reporting or market qualification. Source: PJM Energy Market page. |
| `capacity factor` | Use only as optional summary | Source-backed, but it is a generation-performance metric, not a commercial phase. Do not combine it with billing dollars. |

`Breaker-to-Breaker Run` remains a useful local domain term, but this pass did not find a high-trust primary source for the exact phrase. Keep it defined locally as a repo term and show it as a presentational run counter, not as a regulated performance metric.

## Fleet Operations Language

Radiant supports a fleet view because it says hundreds of units can operate with data streaming back to centralized 24/7 fleet monitoring tracking reactor health. Source: Radiant public site.

That does not license a control room. For this slice, `Kaleidos Fleet` should mean "summary strip of separately operated Kaleidos Units" and not remote operation, dispatch, alarm acknowledgement, emergency response, or control action. Source boundary: [CONTEXT.md](../../CONTEXT.md); Radiant public site.

Safe fleet strip wording:

- `centralized fleet monitoring context`
- `unit-level summary`
- `selected Kaleidos Unit`
- `factory-made unit`
- `factory-fueled/tested unit`
- `fleet freshness`
- `commercial display basis`

Avoid:

- `operator console`
- `control room`
- `remote operation`
- `dispatch command`
- `alarm queue`
- `emergency status`
- `fleet command`
- `trading desk`

## Recommended Fleet Strip Fields

Use a compact row per `Kaleidos Unit`:

| Field | Example | Basis |
| --- | --- | --- |
| `unitId` | `KAL-003` | Fixture identity. |
| `siteContext` | `Hospital backup campus` | Public-safe customer/use context, not an address or contract. |
| `commercialMode` | `PPA electric`, `direct unit sale`, `facility heat`, `desalination heat`, `resilience backup` | Source-backed business context from Radiant public materials, except lease which should stay out unless explicitly hypothetical. |
| `availabilityPhase` | `online generation`, `ramping`, `cooldown`, `planned maintenance outage`, `unplanned maintenance outage`, `refueling outage` | NRC outage vocabulary mapped into public-safe phases. |
| `breakerToBreakerRun` | `42 days` | Local repo term. Presentational counter only. |
| `electricOutput` | `0.82 MWe` | Radiant/NRC support approximately 1 MWe unit scale. |
| `thermalOutput` | `1.4 MWth facility heat` | Radiant supports thermal power for facility heating/desalination and NRC supports 3 MWth design scale. |
| `deliveredEnergy` | `14.7 MWh display` | Fixture-derived physical counter. Not settlement. |
| `deliveredHeat` | `21.2 MWhth display` | Fixture-derived thermal counter. Not invoice. |
| `accruedDisplayValue` | `$18.4k display estimate` | Optional and risky; include only if the assumption basis is visible. |
| `freshness` | `fixture as of 10:42` | Reinforces non-live status. |
| `lineageRef` | `COMM-FIXTURE-KAL-003` | Links to display assumptions, not financial audit records. |

## Commercial Display Basis Fields

If the strip shows any value-like counter, include a visible basis object:

| Field | Purpose |
| --- | --- |
| `basisLabel` | e.g. `presentation estimate`, `fixture display value`, or `customer value estimate`. |
| `commercialMode` | Explains whether the row is electric PPA context, direct sale context, heat service context, desalination context, or resilience context. |
| `outputWindow` | Elapsed fixture interval used for delivered energy/heat counters. |
| `electricOutputBasis` | Fixture MWe value and whether it is measured, imputed, or simulated in the existing `Value Basis` sense. |
| `thermalOutputBasis` | Fixture MWth value and its `Value Basis`. |
| `rateAssumptionLabel` | Human-readable assumption name only, e.g. `demo flat value`, not a tariff ID. |
| `exclusions` | Explicitly say `not billing`, `not settlement`, `not tariff`, `not market-cleared`, and `not dispatch`. |
| `lineage` | Link back to fixture assumptions and selected Unit state. |

Do not add fields for tariff class, node, LMP, bid, offer, cleared MW, settlement interval, invoice number, receivable, counterparty credit, imbalance charge, ancillary service award, capacity market delivery year, or emergency status. Those fields imply a billing engine, trading desk, energy-market integration, emergency panel, or control room.

## Recommended Glossary Additions For CONTEXT.md

Do not edit `CONTEXT.md` from this research note. Proposed text:

**Commercial Display Basis**:
The visible fixture assumptions used to explain a commercial fleet-strip value, including commercial mode, output window, electric/thermal output basis, rate-assumption label, freshness, and lineage. It is presentation context only, not billing, settlement, tariff calculation, or market participation.
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

**Resilience Backup**:
A commercial mode where a Kaleidos Unit is presented as supporting backup or resilience value for a facility type such as a hospital, datacenter, military installation, or remote site. It is not an emergency management panel or black-start control workflow.
_Avoid_: Emergency mode, incident command, black-start dispatch, alarm state

**Facility Heat**:
A commercial mode where thermal output is presented as useful heat for facility heating. It is display context only and does not claim cogeneration qualification, process guarantee, or heat-sale billing.
_Avoid_: Cogeneration certification, steam contract, heat tariff

**Desalination Heat**:
A commercial mode where thermal output is presented as useful heat for water desalination. It is display context only and does not model a desalination plant contract or water-service billing.
_Avoid_: Water sale, desalination contract engine, process guarantee

## Risk Register

| Term | Risk | Recommendation |
| --- | --- | --- |
| `revenue` | Accounting/billing implication. | Do not use. |
| `accrued value` | Can imply revenue accrual. | Use `accrued display value` and visibly mark as fixture estimate. |
| `availability credit` | Can imply settlement or capacity-market credit. | Use `availability display credit`. |
| `capacity payment` | Market/commercial product implication. | Keep out of UI; mention only in research. |
| `energy settlement` | Directly implies settlement system. | Ban from first-slice UI. |
| `avoided cost` | Regulated PURPA/utility-rate term. | Use `customer value estimate` unless explicitly building a PURPA explainer. |
| `lease` | Not supported by Radiant public page in this pass. | Prefer `direct unit sale`; only use lease as hypothetical. |
| `forced outage` | Source-backed but operationally sharp. | Map public UI to `unplanned maintenance outage`. |
| `EFOR` / `GADS` | Reliability-reporting/market data implication. | Avoid in public-safe fleet strip. |
| `centralized fleet monitoring` | Radiant source-backed, but can drift into control room. | Use as context only; no controls, alarms, remote ops, or commands. |

## Recommendation

The fleet strip should be fixture-backed and presentational. Use `Commercial Mode` as a contextual label and `Commercial Display Basis` as the explanation rail for any value-like number. Show `delivered energy`, `delivered heat`, `contracted availability`, and optionally `accrued display value`; keep `revenue`, `settlement`, `invoice`, `tariff`, `LMP`, `bid`, `offer`, `capacity payment`, and emergency/control terms out of the UI.

For availability, keep the existing repo phases and map them to NRC outage vocabulary only at the display-language level: online/ramping/cooldown for active non-outage states, planned maintenance/refueling for scheduled outage concepts, and unplanned maintenance for forced/unanticipated outage concepts. Do not turn this into alarm management.
