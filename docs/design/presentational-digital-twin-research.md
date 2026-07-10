# Presentational Digital Twin Research

| Field | Value |
| --- | --- |
| Document ID | SWB-DT-RESEARCH-001 |
| Revision | 0.1 |
| Status | Research note |
| Owner | Software |
| Scope | Presentational-only frontend slice; no backend wiring |

## Purpose

This note records primary-source research for a complete presentational-only Simulator Workbench digital-twin slice. It is not an implementation plan for backend ingest, live telemetry, SCADA maintenance, reactor control, safety functions, validated physics, or production plant monitoring.

The slice should preserve Radiant's existing language: the Simulator Workbench presents measured state, imputed state, simulated result state, and lineage; a Value Basis is part of a displayed value's meaning; a Resident Source is a public-safe measured-source stand-in; and the digital twin is a consumer/projection/read model, not a telemetry source. Source: [CONTEXT.md](../../CONTEXT.md), [Simulator Workbench Final Vision Plan](./simulator-workbench-final-vision-plan.md).

## Source Set

Primary sources used:

- NRC Kaleidos pre-application page: `https://www.nrc.gov/reactors/new-reactors/advanced/who-were-working-with/pre-application-activities/kaleidos`
- Generation IV International Forum VHTR page: `https://www.gen-4.org/gif/jcms/c_9362/vhtr`
- JAEA HTGR overview: `https://www.jaea.go.jp/04/o-arai/nhc/en/faq/index.html`
- JAEA HTGR structure page: `https://www.jaea.go.jp/04/o-arai/nhc/en/faq/htgr_struc.html`
- JAEA HTTR outline: `https://www.jaea.go.jp/04/o-arai/nhc/en/faq/httr.html`
- JAEA HTGR design page: `https://www.jaea.go.jp/04/o-arai/nhc/en/research/htgr/index.html`
- DOE Office of Nuclear Energy TRISO overview: `https://www.energy.gov/ne/articles/triso-particles-most-robust-nuclear-fuel-earth`
- Radiant Nuclear homepage: `https://www.radiantnuclear.com/`
- OPC UA Part 4 MonitoredItem model: `https://reference.opcfoundation.org/specs/OPC-10000-4/5.13.1`
- OPC UA Part 8 Data Access model: `https://reference.opcfoundation.org/specs/OPC-10000-8/5`
- NRC outage glossary: `https://www.nrc.gov/reading-rm/basic-ref/glossary/outage`
- NRC forced outage glossary: `https://www.nrc.gov/reading-rm/basic-ref/glossary/outage-forced`
- NRC scheduled outage glossary: `https://www.nrc.gov/reading-rm/basic-ref/glossary/outage-scheduled`
- MDN SVG: `https://developer.mozilla.org/en-US/docs/Web/SVG`
- MDN Canvas API: `https://developer.mozilla.org/en-US/docs/Web/API/Canvas_API`
- PixiJS official site: `https://pixijs.com/`
- three.js official docs: `https://threejs.org/docs/index.html`

Local project sources used:

- [CONTEXT.md](../../CONTEXT.md)
- [Simulator Workbench Final Vision Plan](./simulator-workbench-final-vision-plan.md)
- [Simulator Workbench Backend Dataflow Slice](./simulator-workbench-backend-dataflow-slice.md)
- [Simulation Ops Integrated Layout Mockup](./simulation-ops-integrated-layout-mockup.md)
- [ADR 0000](../adr/adr-0000.md)
- [package.json](../../package.json)

## Plant And Reactor Topology For UI

Radiant-adjacent microreactor topology should start from Kaleidos and the HTGR/VHTR family, not from a generic pressurized-water plant and not from a nuclear rocket. The NRC describes Kaleidos as a transportable microreactor designed for 3 MWth and about 1 MWe, and as an HTGR using TRISO fuel, helium gas coolant, and prismatic graphite blocks. The NRC page image caption identifies primary heat exchangers, helium circulators, a control drum, the core, and the vessel shell as visible design elements. Source: NRC Kaleidos page.

The Generation IV International Forum describes VHTR as graphite moderated, helium cooled, thermal-spectrum reactor technology that can use prismatic block cores or pebble-bed cores, with TRISO coated-particle fuel, graphite core structure, helium coolant, lower power density, and high outlet temperatures. This is background only: this project excludes pebble-bed topology and shall model only the Kaleidos-style prismatic unit. Source: Generation IV International Forum VHTR page; [CONTEXT.md](../../CONTEXT.md).

JAEA's HTGR material reinforces the same topology: HTGRs use helium gas coolant, ceramic materials mainly graphite for core structures, high-temperature heat supply, coated fuel particles, graphite core structure, and helium coolant. Its HTTR outline gives a concrete prismatic test-reactor reference: 30 MW thermal power, helium coolant, 395 / 850 or 950 deg C outlet/inlet coolant temperature as written on the JAEA page, 4 MPa primary coolant pressure, graphite core material, 2.9 m core height, 2.3 m core diameter, coated-particle low-enriched UO2 fuel dispersed in graphite, and a steel reactor pressure vessel. Source: JAEA HTGR overview; JAEA HTGR structure page; JAEA HTTR outline.

For the frontend, the primary presentational topology should therefore include these static zones:

- Reactor vessel shell and core block region, with TRISO/prismatic graphite called out only at a public conceptual level. Source: NRC Kaleidos page.
- Control drum ring or side elements, because the NRC page explicitly identifies a control drum for Kaleidos. Source: NRC Kaleidos page.
- Helium primary loop, helium circulators, and primary heat exchangers, because those are identified in the NRC Kaleidos page image caption and project overview. Source: NRC Kaleidos page.
- Shipping-container/test-unit boundary, because the NRC says each microreactor is fully contained in a single shipping container. Source: NRC Kaleidos page.
- Fuel form callout: prismatic graphite blocks and TRISO fuel only. Pebble-bed fuel-form views are out of scope. Source: NRC Kaleidos page; [CONTEXT.md](../../CONTEXT.md).
- Primary helium path: cold helium return, circulator/compressor element, core inlet/plenum concept, core heated region, hot helium outlet, cross-duct or hot-leg concept, primary heat exchanger, and cooled helium return. Source: NRC Kaleidos page; JAEA HTTR outline; Radiant Nuclear homepage.
- Secondary heat-use path: facility heating or water desalination heat-use context plus electric output context, shown as downstream consumers rather than a separate plant design. Source: Radiant Nuclear homepage.
- Passive heat-removal/context layer: vessel shell, graphite thermal mass, container/test boundary, and environmental heat dissipation shown as contextual read-model state, not as a safety claim. Source: JAEA HTGR overview; JAEA HTGR structure page.

TRISO should appear as a nested material/fuel callout, not as a separate subsystem. DOE describes TRISO particles as uranium, carbon, and oxygen kernels encapsulated by carbon- and ceramic-based layers, fabricated into cylindrical pellets or billiard-ball-sized pebbles for high-temperature gas or molten-salt reactors; DOE also says each particle acts as its own containment system and can withstand temperatures beyond current nuclear fuel thresholds. Source: DOE TRISO overview.

Nuclear Thermal Propulsion topology is out of scope for Radiant. The Simulator Workbench should not include propellant tanks, turbopumps, hydrogen flow, nozzles, test-article propulsion modes, or propulsion analogue comparison panels. Source: [CONTEXT.md](../../CONTEXT.md).

Pebble-bed topology is out of scope for Radiant. The Simulator Workbench should not include pebble-bed core geometry, pebble-bed toggles, or VHTR comparator modes. Source: [CONTEXT.md](../../CONTEXT.md).

The only allowed expansion beyond a single reactor is a fleet ensemble of identically produced, separately operated Kaleidos Units. Radiant's homepage describes Kaleidos as a mass-producible package, says units are assembled, fueled, and tested in the factory, and says hundreds of units can autonomously operate with data streaming back to centralized 24/7 fleet monitoring. Source: Radiant Nuclear homepage.

Fleet availability language should use reactor/outage phases, not vague `standby` or `offline` buckets. NRC defines an outage as a period when a generating unit or similar facility is out of service, and says outages can be forced or scheduled. NRC describes scheduled outages as shutdowns for inspection, maintenance, or refueling scheduled well in advance, while forced outages cover emergency or unanticipated unavailable conditions and exclude scheduled inspection, maintenance, or refueling. Source: NRC outage glossary; NRC forced outage glossary; NRC scheduled outage glossary.

## Measured Inputs

The UI should model measured values as direct observations from resident sources, with `valueBasis=measured`. Source: [CONTEXT.md](../../CONTEXT.md), [Simulator Workbench Final Vision Plan](./simulator-workbench-final-vision-plan.md).

Representative measured plant/process values:

| Group | Example tags | Units | Why plausible |
| --- | --- | --- | --- |
| Reactor/core | multi-zone relative neutron flux stand-ins, relative power, core inlet/outlet temperature, core axial zone temperature, control drum position | percent power or detector units, K or deg C, degrees | Kaleidos uses a core and control drum; VHTR/HTGR references identify graphite core structure, helium coolant, high outlet temperature, and HTTR provides concrete inlet/outlet temperature and pressure examples. Multiple coarse flux stand-ins are required before the UI may display a Core Power Distribution Estimate. Sources: NRC Kaleidos page; Generation IV International Forum VHTR page; JAEA HTTR outline; [CONTEXT.md](../../CONTEXT.md). |
| Fuel/core material context | TRISO fuel form, graphite moderator/core structure state, prismatic block state | enum/state, deg C for displayed material-temperature proxy | TRISO coated-particle fuel, graphite core structure, helium coolant, and prismatic graphite blocks are the relevant Kaleidos-style facts; pebble-bed modes are out of scope. Sources: NRC Kaleidos page; JAEA HTGR structure page; DOE TRISO overview; [CONTEXT.md](../../CONTEXT.md). |
| Helium primary loop | helium pressure, helium inlet temperature, helium outlet temperature, helium mass-flow proxy, circulator speed/state | MPa/kPa, K or deg C, kg/s or percent flow, rpm or state | Kaleidos uses helium gas coolant and helium circulators; JAEA HTTR gives 4 MPa primary coolant pressure and helium coolant temperature points. Sources: NRC Kaleidos page; JAEA HTTR outline. |
| Heat exchange / process heat | primary heat exchanger hot-side inlet/outlet temperature, facility-heat output, desalination heat output, exchanger delta-T | K or deg C, kg/s or percent flow, K | Kaleidos rendering identifies primary heat exchangers, and Radiant says Kaleidos can provide thermal power for facility heating or water desalination. Sources: NRC Kaleidos page; Radiant Nuclear homepage. |
| Power conversion / load following | turbomachinery/cooling state, generator output, electric demand, heat-to-process split, load demand | percent, MWe/kW, state, MWt or percent | Radiant describes high-speed turbomachinery operating from 30% to 100% power or 1 MW electric, plus electric and heat output that adjusts to demand. Source: Radiant Nuclear homepage. |
| Passive/context state | vessel shell temperature proxy, container boundary temperature proxy, environmental heat rejection proxy | K or deg C, percent margin/proxy | JAEA describes graphite heat capacity and heat dissipation from the reactor pressure vessel during loss-of-coolant contexts; this belongs as context/read-model state, not a safety-path display. Source: JAEA HTGR overview. |

Representative diagnostics:

| Group | Example tags | Units | Basis |
| --- | --- | --- | --- |
| Data quality | source timestamp, server/observed timestamp, status code, stale/missing flag | timestamp, enum | OPC UA DataValue includes source/server timestamp concepts, and MonitoredItems report data changes and status. Source: OPC UA Part 4. |
| Sampling/freshness | requested interval, revised interval, publishing interval, freshness age | ms or s | OPC UA defines a sampling interval as the fastest rate at which the server should sample its source; the assigned interval is best effort; servers may revise unsupported intervals; underlying update cycles can be slower than OPC sampling. Source: OPC UA Part 4. |
| Engineering metadata | engineering units, instrument range, normal engineering range | unit code/display, range | OPC UA Data Access defines EngineeringUnits, InstrumentRange, and EURange; it states that units are essential for uniform measurement use. Source: OPC UA Part 8. |
| Asset diagnostics | pump/circulator vibration, bearing temperature, line or valve state, source alive heartbeat | mm/s or g, K or deg C, enum, timestamp | These should be shown as diagnostics about equipment/source condition, not as plant state itself; this distinction follows the repo's boundary between data qualification and a SCADA health panel. Source: [Simulator Workbench Final Vision Plan](./simulator-workbench-final-vision-plan.md). |

Freshness expectations should be displayed as negotiated or configured expectations, not hard-coded truth. OPC UA supports client-requested sampling intervals, server-revised intervals, exception-based data, and slower underlying source updates. Source: OPC UA Part 4. For the presentational slice, use display bands like `fresh <= expectedInterval * 2`, `late`, and `stale` only as UI fixture semantics, with the configured expected interval visible in lineage.

First-slice measured stand-ins are intentionally capped:

- `flux axial low`
- `flux axial mid`
- `flux axial high`
- `flux radial north`
- `flux radial south`
- `core inlet helium temp`
- `core outlet helium temp`
- `primary helium pressure`
- `primary helium flow or circulator speed`
- `control drum position`
- `heat exchanger hot-side temp`
- `heat exchanger cold-side temp`
- `vessel/container boundary temp`
- `electric output`
- `useful thermal output`
- `source freshness / quality`

## Imputed And Model-Derived Values

Imputed values should be emitted by the twin projection/read model only. SimOps workers should not label their own output as imputed. Source: [CONTEXT.md](../../CONTEXT.md), [Simulator Workbench Backend Dataflow Slice](./simulator-workbench-backend-dataflow-slice.md).

Reasonable imputed values for a presentational twin:

| Imputed value | Inputs | Why reasonable | Display requirements |
| --- | --- | --- | --- |
| Core Power Distribution Estimate | multiple coarse relative flux stand-ins, helium inlet/outlet temperature, control drum position, core-zone fixture geometry | Kaleidos identifies core/control drum; VHTR/HTGR architecture centers on graphite core structure, helium coolant, TRISO fuel, and high outlet temperature. A single flux stand-in is not enough to display a distribution estimate; without multiple zones, downgrade the display to power level or hot-region thermal estimate. Sources: NRC Kaleidos page; Generation IV International Forum VHTR page; JAEA HTGR structure page; [CONTEXT.md](../../CONTEXT.md). | Label `valueBasis=imputed`; show confidence and contributing measured tags; avoid validated neutronics or safety-analysis claims. |
| Unmeasured fuel/block temperature estimate | boundary temperature tags, helium flow proxy, relative power, prismatic-block fixture model | TRISO fuel and graphite block structure are the material basis, but a presentational UI should not pretend every internal fuel or graphite temperature is directly sensed. Sources: NRC Kaleidos page; JAEA HTGR structure page; DOE TRISO overview. | Display as estimate, not sensor value. |
| Helium-loop heat balance | helium inlet/outlet temperature, pressure, circulator state, heat-exchanger state | HTGR/VHTR systems transfer heat by helium coolant through core and heat-exchanger/power-conversion paths. Sources: NRC Kaleidos page; Generation IV International Forum VHTR page. | Show model id and input window in lineage. |
| Heat-exchanger/process-heat duty proxy | hot-side helium temperatures, facility-heat/desalination context, flow proxy | Kaleidos includes primary heat exchangers and can provide thermal power for facility heating or water desalination. Sources: NRC Kaleidos page; Radiant Nuclear homepage. | Keep as public-safe operational estimate, not a plant guarantee. |
| Circulator operating margin | helium pressure, circulator speed/state, loop flow proxy, configured envelope | Kaleidos identifies helium circulators and Radiant describes turbomachinery/cooling as part of the unit. Sources: NRC Kaleidos page; Radiant Nuclear homepage. | Keep as imputed operational margin, not a safety limit. |
| Passive heat soak / thermal inertia proxy | vessel/container temperature proxy, graphite/core heat estimate, stale input summary | JAEA emphasizes graphite heat capacity and slow core temperature variation in abnormal contexts; a presentation can show a degraded read-model heat-soak estimate without claiming safety analysis. Source: JAEA HTGR overview. | Label as conceptual imputed state; include caveat in lineage. |
| Data confidence / stale-input degradation | source timestamps, revised sampling interval, status code, missing tags | OPC UA explicitly separates sampling, publishing, source update timing, and data/status reporting. Source: OPC UA Part 4. | Show lineage and age; this is data qualification, not sensor maintenance. |

First-slice imputed values are intentionally capped:

- `Core Power Distribution Estimate`
- `Unmeasured Fuel/Block Temperature Estimate`
- `Local Thermal Margin`
- `Helium-Loop Heat Balance`
- `Heat-Exchanger Duty Proxy`
- `Circulator Operating Margin`
- `Cooldown Heat`
- `Imputed Confidence`

Simulated result state should stay run-scoped. A synthetic scenario worker can produce `valueBasis=simulated` values such as sensitivity outputs, forecasted margins, or scenario projections; the twin can consume those results later to produce imputed state. Source: [CONTEXT.md](../../CONTEXT.md), [Simulator Workbench Backend Dataflow Slice](./simulator-workbench-backend-dataflow-slice.md).

## Frontend Rendering Choices

Current repo constraints:

- The app is React 19, Vite, TypeScript, and lucide-react; `package.json` does not list Phaser, PixiJS, Three.js, D3, or a canvas abstraction dependency. Source: [package.json](../../package.json).
- The existing app already contains inline SVG for a cutaway-style visual. Source: [src/App.tsx](../../src/App.tsx).
- Older ADRs accepted Phaser/Pixi/WebGL for high-frequency rendering, but newer design mockups explicitly say not to add Phaser/WebGL until review establishes a measured rendering need. Sources: [ADR 0000](../adr/adr-0000.md), [Simulation Ops Integrated Layout Mockup](./simulation-ops-integrated-layout-mockup.md).

Evaluation:

| Option | Fit | Reason |
| --- | --- | --- |
| Plain React + SVG | Best first choice | SVG is a web vector graphics format suitable for diagrams and interactive schematic regions. The current repo already uses React and SVG, and a presentational-only slice needs selectable topology, labels, status badges, and lineage overlays more than a high-frequency render loop. Sources: MDN SVG; [package.json](../../package.json); [src/App.tsx](../../src/App.tsx). |
| React + Canvas 2D | Acceptable only for dense heat-map layers | Canvas provides scriptable bitmap drawing. It is useful if the heat map becomes a dense raster field, but it makes semantic hit regions, layout, and accessibility more manual than SVG. Source: MDN Canvas API. |
| PixiJS | Defer | PixiJS is a fast 2D WebGL renderer, but it adds a dependency and a rendering lifecycle that the presentational slice does not yet need. Source: PixiJS official site; [package.json](../../package.json). |
| Phaser | Defer harder | Phaser is game-oriented and the older ADR's high-frequency premise does not match a no-backend presentational slice. It would also fight dense dashboard text and controls, a drawback already called out in the ADR. Source: [ADR 0000](../adr/adr-0000.md). |
| Three.js | Not first slice | Three.js is appropriate when a true 3D scene is the primary experience; this slice needs inspectable P&ID/cutaway topology, value-basis labels, and lineage. The current repo has no Three dependency. Sources: three.js docs; [package.json](../../package.json). |

Recommended layout:

- Use React components for panels, selection state, and tables.
- Keep the first Simulator Workbench slice as one dense integrated screen rather than internal tabs. It should present fixture-backed simulation/workbench state in one place: commercial fleet strip, selected Kaleidos Unit twin viewport, measured state, imputed state, simulated result state, compact summaries, and bottom explanation rail.
- Include compact simulation result monitoring only. The Workbench may show active/recent scenario name, simulated result count, latest result status, artifact/result reference, and one or two key simulated outputs influencing the selected unit. Do not include worker logs, Redpanda offsets, Timescale/Iceberg details, WebTransport tracks, Slurm lifecycle controls, or run launch/stop controls in this presentational slice.
- Superseded by ADR-0006: SimOps Control is absorbed below the Status Workbench value-basis region as Container Orchestration. The value-basis region may reference fixture-backed Simulated Result State without collapsing run controls, operational telemetry, measured state, and imputed twin state into one metric stream.
- Use one SVG schematic as the main twin canvas, with stable subsystem IDs for `core`, `controlDrums`, `primaryLoop`, `heatExchangers`, `circulators`, `vessel`, `shielding`, `containerBoundary`, `secondaryHeatUse`, and `powerConversion`.
- Use SVG groups for measured/imputed/simulated overlays so the UI can visibly encode Value Basis without changing transport or backend contracts.
- Include a lightweight commercial fleet output strip for identical Kaleidos Units. The strip may show unit id, availability phase, breaker-to-breaker run counter, electric output, thermal output, source-backed commercial mode, compact accrued display value such as `$18.4k (est)`, degraded freshness only, and selected state, and selecting a unit may swap the fixture-backed single-unit twin. It is not a centralized ops console, control room, emergency panel, or backend fleet-monitoring implementation.
- Use simplified notation on fleet cards whenever possible. Show compact electric/thermal output and compact accrued display value on the card; put delivered energy, delivered heat, rate-assumption label, freshness timestamp, exclusions, and detailed Commercial Display Basis in the bottom explanation rail. On the card itself, show freshness only when it is late or stale.
- Use source-backed first-slice commercial modes: PPA electric, direct unit sale, facility heat, desalination heat, and resilience backup. Do not use direct unit lease unless a primary Radiant source appears.
- Use these first-slice availability phases: online generation, ramping, cooldown, planned maintenance outage, unplanned maintenance outage, and refueling outage. Do not use standby, offline, emergency, SCRAM, or alarm-management language.
- Include multiple fixture-backed Kaleidos Units in the first slice, not placeholder cards. Initial set: `KAL-01` online generation / PPA electric, `KAL-02` ramping / facility heat, `KAL-03` online generation / desalination heat, `KAL-04` cooldown / resilience backup with residual heat, and `KAL-05` planned maintenance outage or refueling outage. At most one card should use unplanned maintenance outage in the first slice.
- Use one shared beauty plate for all fleet units. Unit selection changes overlays, values, selected-unit label, commercial display basis, site/use context, and explanation rail content, but not the base reactor design. Different beauty plates would imply product variants and should be avoided.
- Treat selected fleet unit as the full workbench context. Selecting a unit updates measured values, imputed values, simulated result values, SVG overlay highlights, the commercial fleet strip selected state, and the bottom explanation rail.
- For cooldown cards, show residual heat generation or cooldown heat as reactor-state context, not as delivered heat, commercial thermal output, or outage economics. Treat cooldown heat as `valueBasis=imputed` unless the fixture explicitly represents a direct measured thermal tag. Keep the compact card label simple; show the Value Basis and input lineage in the bottom explanation rail.
- Show lightweight commercial display basis for any accrued display value counter: electric output fixture, thermal output fixture, commercial mode fixture, elapsed operating window, rate-assumption label, and freshness timestamp. This is not invoicing-grade financial audit, tariff modeling, or settlement logic.
- Do not model outage economics in this slice. Non-generating phases may show availability phase, breaker-to-breaker reset state, and explanatory context, but not lost-generation opportunity cost, outage cost, or added operations/maintenance expense.
- Reuse the bottom explanation rail for both selected engineering values and selected fleet commercial values. For engineering values it shows measured/imputed/simulated lineage; for fleet commercial values it shows commercial display basis.
- Keep fixture data local and static for this slice; use no backend API, no Redpanda/Timescale/Iceberg calls, no credentials, and no live telemetry claims.

## Recommendation

Build the presentational-only digital twin as a React + SVG workbench slice using static fixture data. That choice matches the repo's current dependencies, the presentational-only constraint, and the need for inspectable topology, labels, lineage, and Value Basis separation. Do not add Phaser, PixiJS, or Three.js for this slice; revisit PixiJS or Canvas only after a real need appears for dense animated fields or high-frequency updates.

Use gas-cooled reactor topology as the model:

1. `Kaleidos Unit`: TRISO/prismatic graphite core, control drum, helium primary loop, helium circulators, turbomachinery/cooling, primary heat exchangers, reactor/shielding, vessel/container boundary, measured helium/core/process values, imputed core/loop heat state.
2. `Kaleidos Fleet`: lightweight first-slice commercial output strip plus optional later ensemble of identically produced, separately operated Kaleidos Units, summarizing unit-level availability phase, breaker-to-breaker run, electric/thermal output, commercial mode, accrued display value, freshness, and lineage without merging the units into one reactor.

Keep every displayed value tagged as `measured`, `imputed`, or `simulated`. Show measured values as direct resident-source observations, imputed values as twin read-model estimates with lineage/confidence, and simulated values as run-scoped scenario outputs. This is the boundary that keeps the frontend from degenerating into fake SCADA mush.

## Later Backend Horizontal Slices Informed By This Research

When backend work resumes, slice it horizontally by consumer contract rather than by frontend flourish:

- measured telemetry fixture/contract: tags, units, source timestamp, observed timestamp, status, expected interval, quality, source id, asset id;
- digital twin projection contract: entity id, `asOf`, mixed values, `valueBasis`, confidence, lineage, stale input summary;
- simulated result contract: run id, model id, worker id, artifact id, input window, result value, `valueBasis=simulated`;
- read-only Workbench API: current state, measured state, twin state, simulation results, selected-value lineage.

No backend slice should make the frontend a telemetry source. The frontend remains a consumer/projection surface.
