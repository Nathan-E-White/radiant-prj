# Simulator Workbench Stub Ledger

| Field | Value |
| --- | --- |
| Document ID | SIMWB-STUB-001 |
| Revision | 0.1 |
| Status | Scaffold ledger |
| Owner | Software |
| Baseline | Post-v3.0 planning input |

## Purpose

This ledger records the Simulator Workbench scaffold and the first promoted frontend projection slice. The contract, example, validation, and source anchors remain the controlled starting point. Backend runtime dataflow has now been promoted in `docs/design/simulator-workbench-backend-dataflow-slice.md`; frontend controls and visualization wiring remain deferred.

## Scope Guardrails

- Scaffold-only items still must not claim real plant telemetry, safety behavior, actuation, alarm management, or validated physics.
- Backend routes, resident SCADA service wiring, Compose services, and database migrations are controlled by `SWB-DATAFLOW-001` rather than this scaffold ledger.
- The frontend projection mounts a top-level Simulator Workbench tab backed by existing examples, not live backend read APIs.
- Measured, imputed, and simulated values remain separate through `valueBasis`; they must not be flattened into generic metrics.
- SCADA stand-ins are public-safe resident source abstractions. They are not real plant data, sensor diagnostics, calibration workflows, alarm management, or control-room behavior.
- Simulation Ops remains the working run-scoped simulation slice and can later become a Simulator Workbench subview.
- The simulation health surface in this slice is a compact summary only; detailed worker, stream, Redpanda, Timescale, Iceberg, and WebTransport health presentations remain later work.

## Scaffolded Seams

| Seam | Stub artifacts | Acceptance criteria before implementation claim |
| --- | --- | --- |
| Value-basis contract | `docs/schemas/simulator-workbench/value-basis.v1.schema.json` | All measured, imputed, and simulated examples validate and remain visually/API-distinguishable. |
| Resident measured-source contract | `docs/schemas/scada/`, `examples/scada/` | Mixed flux, temperature, pressure, actuator, electrical, and comms tags validate as `valueBasis=measured`. |
| Digital twin contract | `docs/schemas/digital-twin/`, `examples/digital-twin/` | Twin state includes measured, imputed, and simulated values with lineage. |
| Workbench state contract | `docs/schemas/simulator-workbench/workbench-state.v1.schema.json`, `examples/simulator-workbench/` | Workbench summary counts match twin values and references existing examples. |
| Contract validation | `scripts/check-simulator-workbench-contract.mjs` | `bun run simulator-workbench:contract:check` passes and fails on basis flattening or missing source coverage. |
| Frontend projection | `src/api/simulatorWorkbench.ts`, `src/domain/simulator-workbench/`, `src/components/simulator-workbench/` | TypeScript compiles, projection tests pass, and the mounted tab keeps value bases visually/API-distinguishable. |
| Backend dataflow | `backend/slurm-gateway/internal/gateway/*workbench*.go`, `backend/slurm-gateway/cmd/workbench-projection-writer/`, `backend/slurm-gateway/cmd/twin-projector/`, `backend/slurm-gateway/cmd/workbench-iceberg-writer/` | Backend tests and `bun run simulator-workbench:dataflow:smoke` prove measured, simulated, and imputed dataflow. |
| Resident source crate | `workers/scada-standins/` | Cargo tests prove the mixed source set exists and remains measured-only; Compose runs it as a resident source. |
| Visual draft | `docs/design/simulator-workbench-visual-draft.md` | Concept is marked non-implementation and avoids safety, control, and validated-physics claims. |

## Next Implementation Threads

1. Wire the existing frontend projection to the read-only backend APIs in a separate frontend-control thread.
2. Add detailed simulation health presentations after the compact summary has a live backend source.
3. Fold the existing SimOps control surface into a Simulator Workbench subview after the shell can explain measured, imputed, and simulated values clearly.
4. Consider a live Workbench stream only after the read-only API path remains stable.
