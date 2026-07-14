# Simulator Workbench Stub Ledger

| Field | Value |
| --- | --- |
| Document ID | SIMWB-STUB-001 |
| Revision | 0.2 |
| Status | Presentational slice ledger |
| Owner | Software |
| Baseline | Post-v3.0 planning input |

## Purpose

This ledger records the Status Workbench scaffold, backend dataflow promotion, and presentational digital-twin frontend slice. The contract, example, validation, and source anchors remain the controlled starting point. Backend runtime dataflow has now been promoted in `docs/design/simulator-workbench-backend-dataflow-slice.md`; the backend/API path names remain `simulator-workbench` for controlled compatibility, while the user-facing route is Status Workbench.

## Scope Guardrails

- Scaffold-only items still must not claim real plant telemetry, safety behavior, actuation, alarm management, or validated physics.
- Backend routes, resident SCADA service wiring, Compose services, and database migrations are controlled by `SWB-DATAFLOW-001` rather than this scaffold ledger.
- The frontend projection now prefers one coherent live Workbench Snapshot. Local examples remain an explicit whole-Snapshot development fallback only when the initial read is unavailable or empty.
- The presentational digital-twin slice uses a horizontal Kaleidos Fleet strip, selected-unit context, a shared beauty plate, React SVG semantic overlays, and a bottom explanation rail.
- Measured, imputed, and simulated values remain separate through `valueBasis`; they must not be flattened into generic metrics.
- SCADA stand-ins are public-safe resident source abstractions. They are not real plant data, sensor diagnostics, calibration workflows, alarm management, or control-room behavior.
- Simulation Ops is absorbed into Status Workbench as the Container Orchestration region rather than remaining a standalone top-level surface.
- The queue-driven HPC status bay is synthetic operational telemetry; detailed live worker, stream, Redpanda, Timescale, Iceberg, and WebTransport health presentations remain later work.
- Commercial display values are display-only estimates. The UI term is `Accrued Display Value`; billing, settlement, tariff, market-clearing, dispatch, and real revenue systems remain out of scope.

## Scaffolded Seams

| Seam | Stub artifacts | Acceptance criteria before implementation claim |
| --- | --- | --- |
| Value-basis contract | `docs/schemas/simulator-workbench/value-basis.v1.schema.json` | All measured, imputed, and simulated examples validate and remain visually/API-distinguishable. |
| Resident measured-source contract | `docs/schemas/scada/`, `examples/scada/` | Mixed multi-zone flux, temperature, pressure, flow, actuator, and electrical tags validate as `valueBasis=measured`; SCADA maintenance diagnostics remain absent. |
| Digital twin contract | `docs/schemas/digital-twin/`, `examples/digital-twin/` | Twin state includes per-unit measured, imputed, and simulated values with lineage, unit IDs, and stable viewport entity IDs. |
| Workbench state contract | `docs/schemas/simulator-workbench/workbench-state.v1.schema.json`, `examples/simulator-workbench/` | Selected-unit summary counts match twin values and references fleet, commercial basis, measured, twin, and lineage examples. |
| Fleet strip fixtures | `examples/simulator-workbench/fleet-units.mixed.json`, `examples/simulator-workbench/commercial-display-basis.mixed.json` | Five Kaleidos Units cover source-backed phases and commercial modes; cooldown shows residual heat with no commercial output. |
| Contract validation | `scripts/check-simulator-workbench-contract.mjs` | `bun run simulator-workbench:contract:check` passes and fails on basis flattening, missing source coverage, unsupported fleet modes/phases, banned commercial terms, or single-flux core distribution estimates. |
| Frontend projection | `src/api/simulatorWorkbench.ts`, `src/domain/simulator-workbench/`, `src/components/simulator-workbench/` | TypeScript compiles, projection/render tests pass, selected fleet unit drives panels/viewport/rail, and Status Workbench keeps value bases visually distinguishable. |
| Backend dataflow | `backend/slurm-gateway/internal/gateway/*workbench*.go`, `backend/slurm-gateway/cmd/workbench-projection-writer/`, `backend/slurm-gateway/cmd/twin-projector/`, `backend/slurm-gateway/cmd/workbench-iceberg-writer/` | Backend tests and `bun run simulator-workbench:dataflow:smoke` prove measured, simulated, and imputed dataflow. |
| Resident source crate | `workers/scada-standins/` | Cargo tests prove the mixed source set exists and remains measured-only; Compose runs it as a resident source. |
| Visual draft | `docs/design/simulator-workbench-visual-draft.md` | Concept is marked non-implementation and avoids safety, control, and validated-physics claims. |

## Next Implementation Threads

1. Keep the live read path on the generation-bound read-only Snapshot; do not rebuild a UI generation from component endpoints.
2. Add live-backed HPC status presentations after the static status bay has a backend source.
3. Refine the embedded Container Orchestration region without resurrecting a SimOps Control top-level tab.
4. Consider a live Workbench stream only if the generation-bound Snapshot refresh no longer meets presentation needs.
