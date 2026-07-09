# Fleet Operations Primary-Source Research

| Field | Value |
| --- | --- |
| Document ID | FLEET-OPS-RESEARCH-001 |
| Revision | 0.1 |
| Status | Research note |
| Owner | Software |
| Scope | Primary-source grounding for public-safe Radiant/Kaleidos Fleet Operations language |

## Purpose

This note records primary-source research for what `Fleet Operations` should mean in Radiant's public-safe Simulator Workbench and Status Workbench language. It supports domain modeling only. It does not propose a control room, remote reactor operation, dispatch system, alarm-management workflow, emergency-response workflow, SCADA maintenance console, billing engine, settlement engine, energy-market integration, or trading desk.

The slice must preserve the existing repo terms `Kaleidos Unit`, `Kaleidos Fleet`, `Value Basis`, `Measured State`, `Imputed State`, `Simulated Result State`, `Lineage`, `Availability Phase`, `Commercial Mode`, and `Commercial Display Basis`. Source: [CONTEXT.md](../../CONTEXT.md).

## Source Set

Primary sources used:

- Radiant public site: <https://www.radiantnuclear.com/>
- NRC Kaleidos pre-application page: <https://www.nrc.gov/reactors/new-reactors/advanced/who-were-working-with/pre-application-activities/kaleidos>
- NRC outage glossary: <https://www.nrc.gov/reading-rm/basic-ref/glossary/outage>
- NRC forced outage glossary: <https://www.nrc.gov/reading-rm/basic-ref/glossary/outage-forced>
- NRC scheduled outage glossary: <https://www.nrc.gov/reading-rm/basic-ref/glossary/outage-scheduled>
- FERC PURPA Qualifying Facilities page: <https://www.ferc.gov/qf>
- PJM Capacity Market page: <https://www.pjm.com/markets-and-operations/rpm>
- PJM Energy Market page: <https://www.pjm.com/markets-and-operations/energy>

Local project sources used:

- [CONTEXT.md](../../CONTEXT.md)
- [Commercial Fleet Strip Domain Research](./commercial-fleet-strip-domain-research.md)
- [Presentational Digital Twin Research](./presentational-digital-twin-research.md)
- [Presentational Digital Twin Slice Implementation Plan](./presentational-digital-twin-slice-plan.md)
- [Simulator Workbench Stub Ledger](./simulator-workbench-stub-ledger.md)

## High-Confidence Source Facts

Radiant describes Kaleidos as a compact nuclear reactor package with all equipment necessary to generate electricity in a mass-producible and transportable package. Source: Radiant public site.

Radiant describes target use contexts including replacement of diesel generators, remote villages, hospitals, datacenters, military installations, resilient microgrids, facility heating, and water desalination. Source: Radiant public site.

Radiant states that Kaleidos can provide 1 MW electric output and 1.9 MW of thermal power for facility heating or water desalination while generating electricity. Source: Radiant public site.

Radiant says Kaleidos units are assembled, fueled, and tested in the factory. Source: Radiant public site.

Radiant gives the key fleet source phrase: hundreds of units can autonomously operate while data streams back to Radiant's centralized 24/7 fleet monitoring system tracking the health of each reactor. Source: Radiant public site.

Radiant says that after five or more years of operation, the fuel is depleted and the entire container can be shipped back for refueling; back at the factory, Kaleidos can be fueled four total times for a 20-year product lifetime. Source: Radiant public site.

Radiant says customers can purchase through Power Purchase Agreements or direct unit sales. Source: Radiant public site.

The NRC describes Kaleidos as a transportable HTGR micro-reactor designed to generate 3 MWth and approximately 1 MWe, using TRISO fuel, helium gas coolant, and prismatic graphite blocks. Source: NRC Kaleidos pre-application page.

The NRC says each Kaleidos micro-reactor will be fully contained in a single shipping container. Source: NRC Kaleidos pre-application page.

The NRC Kaleidos pre-application page lists a `Kaleidos Fleet Operator Training White Paper` as non-public proprietary and under review. This supports the existence of fleet-operator language in the public regulatory engagement surface, but it does not provide public details for training content, operator roles, control authority, procedures, or staffing model. Source: NRC Kaleidos pre-application page.

NRC outage language supports careful availability terminology: an outage is a period when a generating unit or similar facility is out of service; forced outages cover emergency or unanticipated unavailable conditions; scheduled outages cover inspection, maintenance, or refueling shutdowns scheduled well in advance. Sources: NRC outage glossary; NRC forced outage glossary; NRC scheduled outage glossary.

## What Fleet Operations Means Here

For this repo, `Fleet Operations` should mean public-safe coordination and explanation across a `Kaleidos Fleet`:

- unit-level identity and selected-unit context;
- centralized fleet monitoring context;
- reactor-health/readiness summaries;
- measured, imputed, and simulated value lineage;
- data freshness and degraded freshness;
- availability phase and outage/refueling context;
- electric output, useful thermal output, residual heat, and commercial display context;
- synthetic fleet-screen anomaly/readiness output when explicitly fixture-backed.

The phrase should not imply that the Simulator Workbench can operate reactors. Radiant's public wording says units can autonomously operate and stream data back to centralized monitoring; it does not say a browser UI dispatches units, changes reactor setpoints, acknowledges alarms, or runs emergency response. Source: Radiant public site.

The NRC public page proves there is a fleet-operator-training topic in the regulatory engagement record, but the white paper is non-public proprietary. That means this project should not infer operator-training details, crew workflows, staffing rules, procedure steps, or control authority from the public listing. Source: NRC Kaleidos pre-application page.

## Safe Domain Boundary

| Concept | Safe interpretation | Boundary |
| --- | --- | --- |
| `Fleet Operations` | Read-only coordination context across separately operated Kaleidos Units. | Not a control system. |
| `centralized fleet monitoring context` | Source-backed language for receiving data and tracking reactor health at fleet scale. | Not remote command authority. |
| `unit-level readiness` | Public-safe summary of selected unit state, freshness, availability phase, and lineage. | Not a safety declaration. |
| `availability phase` | Public-safe phase mapping to online/ramping/cooldown/planned maintenance/unplanned maintenance/refueling. | Not alarm state. |
| `fleet freshness` | Display of fixture age or degraded data status. | Not a production telemetry SLA. |
| `fleet anomaly flag` | Synthetic engineering-review cue for fixture-backed demos. | Not alarm acknowledgement or incident command. |
| `commercial display context` | Fixture-backed explanation of why output matters commercially. | Not billing, settlement, tariff, dispatch, or trading. |

## Recommended Fleet Operations Fields

| Field | Example | Source basis |
| --- | --- | --- |
| `unitId` | `KAL-03` | Local fixture identity for one Kaleidos Unit. |
| `displayName` | `Kaleidos Unit 03` | Local fixture display label. |
| `availabilityPhase` | `online generation`, `cooldown`, `refueling outage` | NRC outage vocabulary plus local public-safe phase mapping. |
| `monitoringContext` | `centralized fleet monitoring context` | Radiant public fleet-monitoring language. |
| `freshness` | `fresh`, `late 4m`, `stale 18m` | Local fixture freshness semantics; aligns with monitoring context without live claims. |
| `commercialMode` | `PPA electric`, `direct unit sale`, `facility heat`, `desalination heat`, `resilience backup` | Radiant public commercial/use-context language. |
| `electricOutputMwe` | `0.94 MWe` | Radiant and NRC support approximately 1 MWe scale. |
| `usefulThermalOutputMwt` | `2.6 MWt` | Radiant supports facility heat and desalination heat context. |
| `residualHeatMwth` | `0.18 MWth residual heat` | Local cooldown reactor-state context. |
| `commercialBasisId` | `CDB-KAL-03-DESALINATION` | Local link to display-only assumptions and exclusions. |
| `lineageRef` | `COMM-FIXTURE-KAL-003` | Local fixture lineage, not financial audit or plant log. |

## Availability And Maintenance Language

Recommended first-slice availability phases:

| Phase | Keep? | Rationale |
| --- | --- | --- |
| `online generation` | Yes | Public-safe active generation state. |
| `ramping` | Yes | Public-safe output-changing state; do not expose control commands. |
| `cooldown` | Yes | Keeps residual reactor-state context distinct from standby/offline language. |
| `planned maintenance outage` | Yes | Aligns with scheduled outage language for inspection or maintenance. |
| `unplanned maintenance outage` | Yes | Safer UI label for forced/unanticipated unavailability. |
| `refueling outage` | Yes | Source-backed by NRC scheduled-outage purpose and Radiant's shipped-back refueling model. |
| `forced outage` | Research only | Source-backed but sharp; map UI to `unplanned maintenance outage`. |
| `emergency` | No | Would imply emergency response or alarm handling. |
| `SCRAM` | No | Would imply reactor protection/control workflow. |
| `standby` | Avoid | Too vague and can imply restart/control availability. |

## Commercial And Market Boundary

Radiant supports PPAs, direct unit sales, facility heat, desalination heat, and resilience backup as public commercial/use context. Source: Radiant public site.

FERC and PJM sources show why this slice should avoid casual market and settlement language: FERC's QF materials discuss regulated sales context, while PJM exposes real energy and capacity market surfaces. Sources: FERC PURPA Qualifying Facilities page; PJM Capacity Market page; PJM Energy Market page.

Safe language:

- `Commercial Mode`
- `Commercial Display Basis`
- `delivered energy`
- `delivered heat`
- `contracted availability`
- `accrued display value`
- `display estimate`

Avoid:

- `revenue`
- `invoice`
- `settlement`
- `tariff`
- `LMP`
- `bid`
- `offer`
- `market-cleared`
- `capacity payment`
- `dispatch`
- `trading`

## Risk Register

| Term or feature | Risk | Recommendation |
| --- | --- | --- |
| `Fleet Ops Console` | Sounds like a control-room or operator workstation. | Prefer `Fleet Operations` for domain vocabulary and `Kaleidos Fleet` or `Status Workbench` for UI labels. |
| `operator console` | Implies licensed/operator workflows. | Avoid unless a future public source and product scope explicitly support it. |
| `remote operation` | Implies control authority. | Ban from public-safe demo UI. |
| `alarm queue` | Implies alarm management and acknowledgement. | Use `engineering review` or `fleet anomaly flag` only for synthetic data. |
| `emergency status` | Implies emergency response. | Avoid; use availability phase and quiet engineering review wording. |
| `dispatch` | Implies power-market or control command behavior. | Avoid in UI and fixtures. |
| `fleet command` | Implies control authority over multiple reactors. | Ban. |
| `fleet operator training` | Real regulatory phrase but public details are unavailable. | Mention only as source boundary; do not model training. |
| `revenue` | Accounting implication. | Use `Accrued Display Value` with visible display-only basis. |
| `settlement` | Energy-market/billing implication. | Ban from first-slice UI. |

## Recommended Glossary Addition

Captured in [CONTEXT.md](../../CONTEXT.md):

**Fleet Operations**:
The public-safe coordination context for a Kaleidos Fleet: unit-level monitoring, readiness, availability phase, maintenance/refueling context, commercial display context, freshness, and lineage across separately operated Kaleidos Units. In this repo it is read-only/demo context, not remote reactor operation, dispatch, alarm acknowledgement, emergency response, billing, settlement, or trading.
_Avoid_: Fleet command, control room, remote operation, operator console, alarm desk, dispatch desk, trading desk

## Recommendation

Use `Fleet Operations` as an internal domain term for read-only fleet-scale monitoring, readiness, availability, freshness, commercial display context, and lineage across standardized Kaleidos Units. Use `Kaleidos Fleet` for the ensemble and `Fleet Strip` for the compact UI pattern.

Do not build or imply a remote-operations console. The public source-backed story is strong enough for fleet monitoring and readiness context, and not strong enough for commands, alarms, operator procedures, emergency workflows, settlement, dispatch, or trading.
