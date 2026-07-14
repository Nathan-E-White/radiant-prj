# Fleet Board Mechanics

| Field | Value |
| --- | --- |
| Document ID | FLEET-BOARD-MECH-001 |
| Revision | 0.1 |
| Status | Implemented feature note |
| Owner | Software |
| Scope | Fleet Board v1 gameplay mechanics |

## Purpose

Fleet Board is a desktop-first logistics game inside Simulator Workbench. It is not a reactor control console, dispatch system, SCADA panel, billing engine, or operations simulator.

The player builds a small public-safe fleet support board during a 30-day contract sprint. Facilities cost cash, fuel is abstract TRISO supply, reactors convert fuel into electric and thermal service, customers turn output into display credits, and roaming pawns create pressure.

## Core Loop

1. Place facility cards on a grid-snapped board.
2. Let nearby compatible facilities auto-route.
3. Tick the day.
4. Watch fuel, output, credits, debt, Inspector pressure, and Trouble events move.
5. Refuel or recover when a reactor enters refueling/outage state.
6. Finish the 30-day sprint with a score, or get removed after sustained debt.

## Facilities

| Facility | Role | Notes |
| --- | --- | --- |
| TRISO Supply | Produces abstract fuel blocks. | No enrichment, fuel-cycle process, or operational details. |
| Reactor | Consumes fuel and produces electric/thermal output. | Represents the toy game source of service value. |
| Desal Plant | Converts nearby thermal service into water credits. | Display-only service context. |
| Base Load | Converts nearby electric service into resilience credits. | Public-safe customer/load context. |
| Battery Sink | Absorbs electric service for smaller resilience contribution. | Grid/storage toy target. |

## Pressure Pawns

`Inspector` is predictable review pressure. It moves around the board and can hold a reactor when Workbench freshness/confidence pressure is high enough.

`Trouble` is dynamic event-deck pressure made visible on the map. It moves independently and can force short outages or routing pressure.

Both pawns are deterministic under the seeded reducer so tests and browser acceptance remain repeatable.

## Workbench Coupling

Fleet Board uses existing Simulator Workbench projection state as light gameplay modifiers:

- measured freshness raises Inspector pressure;
- imputed confidence changes reactor yield;
- simulated result health can add scenario pressure;
- Value Basis counts remain visible and separate.

The game does not flatten measured, imputed, and simulated values into one metric stream.

## V2: Local Simulation Job Economy

Fleet Board v2 includes a deterministic, local-only Simulation Job loop. The reducer owns capacity purchase, queueing, job progression, blocked events, and rewards; React owns accessible intent and summaries; the scene model supplies render-ready states; and the mounted Phaser runtime renders those states without becoming a second game engine.

Implemented rules:

- a new game starts with 6 Simulation Budget;
- each reactor's Reactor Slot Rail holds at most two Simulation Container Tokens;
- each Simulation Container Token costs 2 Simulation Budget;
- buying capacity and queueing a Simulation Job are separate actions;
- a job requires an idle token, starts on the next `tickDay`, and completes after three advances;
- completion returns the token to idle and awards one reactor-scoped Insight Token;
- one Insight Token automatically absorbs one Inspector or Trouble non-refueling outage for its reactor and creates a readable event;
- refueling outages ignore Insight Tokens and retain the existing fuel behavior;
- blocked purchase and queue requests create readable events;
- slot badges expose idle, queued, and running states, while Insight Token badges remain reactor-scoped.

Simulation Budget, Simulation Container Tokens, Simulation Jobs, and Insight Tokens are local game state. They are not cloud spend, real infrastructure capacity, SimOps Runs, Slurm jobs, backend artifacts, or objective evidence. This V2 loop does not persist actions through SimOps. Backend-backed behavior remains a later gated horizon, and any backend simulation output must retain the project's `Simulated Result State` and Lineage semantics rather than becoming Measured or Imputed State by implication.

## Prototype Verdict

The logic prototype is available with:

```sh
bun run fleet-board:prototype
```

It uses the same reducer as the app. Its purpose is to tune the economy and pawn pressure before Phaser polish hides weak mechanics.
