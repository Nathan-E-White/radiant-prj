# Presentational Digital Twin Slice Implementation Plan

| Field | Value |
| --- | --- |
| Document ID | SWB-DT-SLICE-PLAN-001 |
| Revision | 0.1 |
| Status | Implementation planning document |
| Owner | Software |
| Scope | Complete presentational Simulator Workbench digital-twin slice; no backend wiring |

## Purpose

This plan defines the implementation path for a complete presentational-only Simulator Workbench digital twin slice. The slice is fixture-backed, read-only, and frontend-only. It shall not wire to backend APIs, Redpanda, Timescale/Postgres, Iceberg, WebTransport, Slurm, Docker, SCADA systems, billing systems, control systems, or live telemetry.

The slice proves that the frontend can present a coherent Kaleidos Unit digital twin and a lightweight commercial Kaleidos Fleet strip while preserving Value Basis: Measured State, Imputed State, and Simulated Result State.

## Source Inputs

- [CONTEXT.md](../../CONTEXT.md)
- [Presentational Digital Twin Research](./presentational-digital-twin-research.md)
- [Commercial Fleet Strip Domain Research](./commercial-fleet-strip-domain-research.md)
- [Simulator Workbench Stub Ledger](./simulator-workbench-stub-ledger.md)
- [Simulator Workbench Backend Dataflow Slice](./simulator-workbench-backend-dataflow-slice.md)
- [Simulator Workbench Visual Draft](./simulator-workbench-visual-draft.md)
- [ADR 0005](../adr/adr-0005.md)

## Scope

### In Scope

- One integrated Simulator Workbench screen.
- A horizontal commercial Kaleidos Fleet strip.
- A selected Kaleidos Unit context that drives the whole workbench.
- Shared high-quality beauty plate for all units.
- React SVG overlay for interactive topology regions and highlights.
- Fixture-backed Measured State, Imputed State, Simulated Result State, commercial display basis, and lineage.
- Compact simulation result monitoring.
- Bottom explanation rail for selected engineering values and selected commercial values.
- Render-level tests proving Value Basis, fleet selection, and explanation behavior.

### Out Of Scope

- Backend API wiring.
- Live data source wiring.
- Direct browser access to data-plane infrastructure.
- Reactor controls, safety paths, alarm management, SCRAM language, incident response, or remote operation.
- SCADA maintenance dashboard, calibration workflows, PLC/RTU diagnostics, or device health console.
- Billing, settlement, tariff modeling, trading, invoices, revenue recognition, capacity-market modeling, or outage economics.
- NTP, propulsion analogues, pebble-bed topology, VHTR comparator modes, or alternative reactor-family views.
- Phaser, PixiJS, Three.js, or new rendering-engine dependency for this slice.

## Settled Decisions

| Decision | Direction |
| --- | --- |
| Rendering | Hybrid beauty plate plus React SVG overlay. |
| Base visual | One shared beauty plate for all Kaleidos Units. |
| Interactivity | React state over SVG semantic regions and generous hit areas. |
| Topology | Single Kaleidos-style prismatic HTGR unit only. |
| Fleet | Horizontal top commercial fleet output strip. |
| Unit selection | Selecting a fleet unit changes the full workbench context. |
| Fleet cards | Simplified notation; details move to the bottom explanation rail. |
| Freshness | Show on cards only when late or stale; always include in explanation basis. |
| Simulation monitoring | Compact Simulated Result State summary only. |
| SimOps Control | Remains separate. |
| Explanation rail | Reused for engineering lineage and commercial display basis. |

## UX Layout

The first screen should be a dense workbench, not a landing page and not a set of internal tabs.

```text
+--------------------------------------------------------------------------------+
| Commercial Kaleidos Fleet Strip                                                 |
| KAL-01 | KAL-02 | KAL-03 | KAL-04 | KAL-05                                      |
+--------------------------------------------------------------------------------+
| Measured State       | Selected Kaleidos Unit Twin Viewport     | Imputed State  |
| multi-zone flux      | beauty plate + SVG overlays              | distribution   |
| helium loop          | selected region callout                  | margins        |
| heat exchanger       | layer chips                              | cooldown heat  |
| output values        |                                         | simulated refs |
+--------------------------------------------------------------------------------+
| Bottom Explanation Rail: lineage or commercial display basis                     |
+--------------------------------------------------------------------------------+
```

The Simulator Workbench opens directly into this usable screen. It shall not ask the user to choose a mode before showing the workbench.

## Fleet Strip Plan

### Fixture Units

The first slice shall include multiple fixture-backed Kaleidos Units, not placeholder cards.

| Unit | Availability Phase | Commercial Mode | Card emphasis |
| --- | --- | --- | --- |
| `KAL-01` | online generation | PPA electric | electric output and `$ est` |
| `KAL-02` | ramping | facility heat | partial electric/thermal output and `$ est` |
| `KAL-03` | online generation | desalination heat | thermal service and `$ est` |
| `KAL-04` | cooldown | resilience backup | residual heat; no commercial output |
| `KAL-05` | planned maintenance outage or refueling outage | direct unit sale | outage/refueling context; breaker-to-breaker reset |

At most one first-slice unit may use unplanned maintenance outage. If included, keep it quiet and non-alarmist.

### Card Fields

Fleet cards should use compact notation:

```text
KAL-03
online generation | B2B 184d
0.94 MWe | 2.6 MWt
desalination heat
$18.4k (est)
```

For cooldown:

```text
KAL-04
cooldown | B2B reset
0.18 MWth residual heat
no commercial output
```

Freshness appears only when degraded, e.g. `late 4m` or `stale 18m`.

### Commercial Display Basis

Selecting a commercial fleet value shows the display basis in the bottom explanation rail:

- commercial mode;
- output window;
- delivered energy;
- delivered heat;
- rate-assumption label;
- freshness timestamp;
- exclusions: not billing, not settlement, not tariff, not market-cleared, not dispatch.

Do not display real revenue language. Use `Accrued Display Value` as the domain term and compact UI notation like `$18.4k (est)`.

## Reactor-Wise Measured State

Measured State values are direct public-safe stand-ins from Resident Sources. These are intentionally capped for this slice.

| Measured stand-in | Example unit | Primary use |
| --- | --- | --- |
| flux axial low | relative | Core Power Distribution Estimate |
| flux axial mid | relative | Core Power Distribution Estimate |
| flux axial high | relative | Core Power Distribution Estimate |
| flux radial north | relative | Core Power Distribution Estimate |
| flux radial south | relative | Core Power Distribution Estimate |
| core inlet helium temp | degC | helium heat pickup |
| core outlet helium temp | degC | hot-region estimate |
| primary helium pressure | MPa | loop state |
| primary helium flow or circulator speed | kg/s or percent | heat-removal confidence |
| control drum position | degrees or percent | configuration context |
| heat exchanger hot-side temp | degC | heat-exchanger duty |
| heat exchanger cold-side temp | degC | heat-exchanger duty |
| vessel/container boundary temp | degC | cooldown/passive context |
| electric output | MWe | delivered energy/commercial display basis |
| useful thermal output | MWt | delivered heat/commercial display basis |
| source freshness / quality | status/time | confidence degradation |

Do not add vibration, bearing temperature, valve diagnostics, PLC health, comms health, alarm queues, calibration, or maintenance SCADA in this slice.

## Imputed State

Imputed State values are model-derived twin read-model values. They are not raw measurements and shall be visually distinct from Measured State.

| Imputed value | Required inputs | Display rule |
| --- | --- | --- |
| Core Power Distribution Estimate | multiple flux stand-ins, control drum position, helium temperatures, core-zone fixture geometry | Coarse axial/radial overlay only; not validated neutronics |
| Unmeasured Fuel/Block Temperature Estimate | relative power, helium flow, boundary temp, prismatic-block fixture model | Estimate label required |
| Local Thermal Margin | hot-region estimate, pressure/flow state, configured envelope | Confidence visible |
| Helium-Loop Heat Balance | inlet/outlet helium temps, pressure, flow/circulator state | Lineage visible |
| Heat-Exchanger Duty Proxy | hot/cold side temps, useful thermal output | Public-safe duty proxy only |
| Circulator Operating Margin | pressure, speed/flow, temperature delta | Not a safety limit |
| Cooldown Heat | boundary temp, prior power state, cooldown fixture model | Reactor-state context, not delivered heat |
| Imputed Confidence | freshness, missing tags, stale measured inputs | Data qualification, not sensor health |

Several Multi-Zone Flux Stand-Ins are required before the UI may show a Core Power Distribution Estimate. A single flux channel supports only power level/trend or a hot-region thermal estimate.

## Simulated Result State

The slice may show compact simulated result context, but it shall not become a run console.

Allowed:

- active/recent scenario name;
- simulated result count;
- latest result status;
- artifact/result reference label;
- one or two simulated outputs influencing the selected unit.

Not allowed:

- run launch/stop controls;
- worker logs;
- Redpanda offsets;
- Timescale/Iceberg details;
- WebTransport tracks;
- Slurm lifecycle panels;
- backend/control-plane health.

## Visual Module Plan

### Modules To Deepen Or Add

| Module | Interface | Responsibility |
| --- | --- | --- |
| `FleetStrip` | selected unit, unit summaries, select callback | Render compact commercial fleet selector. |
| `TwinViewport` | selected unit viewport model, selected value/entity, callbacks | Render beauty plate and SVG overlays. |
| `TwinOverlayModel` | projection input to viewport anchors/layers | Map values to SVG entity IDs and overlay states. |
| `MeasuredStatePanel` | measured group for selected unit | Replace scaffold with real measured cards. |
| `TwinStatePanel` | imputed group for selected unit | Replace scaffold with real imputed cards. |
| `SimulationResultsPanel` | compact simulated result summary | Replace scaffold with result summary. |
| `LineagePanel` | explanation rail model | Render engineering lineage or commercial display basis. |
| `SimulatorWorkbenchSurface` | full projection, selected unit/value state | Coordinate modules; no private panel sprawl. |

The external seam should stay fixture-backed for this slice. The future HTTP adapter remains parked and unused by the presentational route.

### Rendering Rules

- Use the existing/generated beauty plate as the base visual.
- Place a responsive SVG overlay above it.
- SVG groups use stable entity IDs: `core`, `controlDrums`, `primaryLoop`, `heatExchangers`, `circulators`, `vessel`, `shielding`, `containerBoundary`, `secondaryHeatUse`, `powerConversion`.
- Add invisible or generous hit areas for pipes and small regions.
- Use basis-specific overlay styles for measured, imputed, and simulated values.
- Keep text and tables in React/HTML, not inside the beauty plate.
- Do not recreate photorealism in SVG. SVG owns semantic interaction, not the glossy render.

## Fixture Plan

Add or expand example fixtures so the presentational route has all state locally:

| Fixture area | Proposed path | Purpose |
| --- | --- | --- |
| fleet units | `examples/simulator-workbench/fleet-units.mixed.json` | Unit cards, phases, commercial modes, display values. |
| commercial basis | `examples/simulator-workbench/commercial-display-basis.mixed.json` | Explanation rail for selected commercial values. |
| measured telemetry | `examples/scada/telemetry.mixed.ndjson` | Expand measured tags for multi-zone flux and loop values. |
| resident sources | `examples/scada/resident-sources.mixed.json` | Declare the measured stand-ins. |
| twin state | `examples/digital-twin/twin-state.mixed.json` | Per-unit measured/imputed/simulated values. |
| lineage | `examples/digital-twin/value-lineage.core-margin.json` plus new records | Explain selected imputed values and commercial basis. |
| workbench state | `examples/simulator-workbench/workbench-state.mixed.json` | Top-level selected unit and panel summaries. |

Do not extend fixtures with real plant data or source names that imply production SCADA.

## Domain And Projection Plan

Add presentational types and projection helpers under `src/domain/simulator-workbench/`:

- `KaleidosUnitSummary`
- `AvailabilityPhase`
- `CommercialMode`
- `CommercialDisplayBasis`
- `AccruedDisplayValue`
- `TwinViewportEntity`
- `TwinViewportLayer`
- `WorkbenchExplanation`

Projection responsibilities:

- select active unit;
- group values by Value Basis for selected unit;
- build fleet strip cards;
- build commercial display basis for selected commercial value;
- build engineering lineage for selected measured/imputed/simulated value;
- build SVG overlay model from selected unit values;
- mark degraded freshness without rendering normal freshness noise.

## Implementation Sequence

1. Update or add fixtures for fleet units, commercial display basis, multi-zone flux stand-ins, selected-unit twin state, and additional lineage records.
2. Extend `scripts/check-simulator-workbench-contract.mjs` only as much as needed to validate new fixture semantics: no lease, no settlement terms, no basis flattening, multiple flux stand-ins before distribution estimate.
3. Add domain projection tests for selected unit switching, fleet card shaping, commercial display basis, cooldown heat basis, and Core Power Distribution Estimate prerequisites.
4. Create/deepen presentational modules: `FleetStrip`, `TwinViewport`, real panel modules, and explanation rail.
5. Refactor `SimulatorWorkbenchSurface` into a coordinator using the new modules.
6. Add CSS for the fleet strip, SVG overlay, selected states, compact cards, degraded freshness chips, and responsive layout.
7. Add render-level tests proving the integrated screen preserves Value Basis, unit switching, explanation rail behavior, and no backend adapter usage.
8. Run visual QA in the browser at desktop and mobile widths.
9. Update docs/ledger references if the implementation creates new fixtures, schemas, or checks.

## Verification Plan

Run these checks for the implementation slice:

- `bun run typecheck`
- `bun run test`
- `bun run simulator-workbench:contract:check`
- `bun run quality:check`
- `git diff --check`
- Browser QA through the local Vite app.

Visual QA must confirm:

- fleet strip is visible and horizontally usable;
- selecting each unit updates the full workbench context;
- beauty plate renders and SVG overlay aligns on desktop and mobile;
- measured, imputed, and simulated values remain visually distinct;
- bottom rail switches between engineering lineage and commercial display basis;
- cooldown shows residual heat but no commercial output;
- late/stale freshness appears only when degraded;
- no backend calls are required for the Simulator Workbench tab.

## Acceptance Criteria

- Simulator Workbench is a complete presentational screen backed entirely by local fixtures.
- The selected Kaleidos Unit drives fleet strip state, measured panels, imputed panels, simulated summary, viewport overlays, and bottom explanation rail.
- The fleet strip uses source-backed commercial modes only: PPA electric, direct unit sale, facility heat, desalination heat, and resilience backup.
- The fleet strip uses source-backed availability phases only: online generation, ramping, cooldown, planned maintenance outage, unplanned maintenance outage, and refueling outage.
- The UI uses compact notation such as `$18.4k (est)` while the rail explains Commercial Display Basis.
- The UI does not show revenue, settlement, invoice, tariff, LMP, bid, offer, capacity payment, lost-generation cost, or outage cost.
- The reactor-wise twin uses the capped measured stand-ins and capped imputed values from this plan.
- Core Power Distribution Estimate appears only when multiple flux stand-ins are present.
- Cooldown Heat appears as reactor-state context and uses Value Basis in the rail.
- SimOps Control remains separate.
- No backend API, live stream, broker, database, lake, Docker, Slurm, SCADA, billing, or control-system wiring is introduced.
- Phaser/Pixi/Three are not added.
- The slice remains public-safe and avoids real plant telemetry, safety, validated physics, and control-room claims.

## Deferred Work

- Backend read-only Workbench API integration.
- Live stream or WebTransport workbench delivery.
- Detailed simulation health panel.
- SimOps Control migration or folding.
- Additional reactor units beyond fixture presentation.
- Billing, settlement, market, tariff, or outage-economics systems.
- Real SCADA, historian, calibration, alarm, or maintenance workflows.
- Advanced rendering engine evaluation for high-frequency animated fields.
