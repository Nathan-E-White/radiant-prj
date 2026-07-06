# Simulator Workbench Stub Ledger

| Field | Value |
| --- | --- |
| Document ID | SIMWB-STUB-001 |
| Revision | 0.1 |
| Status | Scaffold ledger |
| Owner | Software |
| Baseline | Post-v3.0 planning input |

## Purpose

This ledger records the first inert scaffold for the Simulator Workbench. It creates contract, example, validation, and compile-safe source anchors without wiring runtime behavior.

## Scope Guardrails

- This scaffold does not register backend routes, launch workers, add Compose services, create database migrations, or mount a new frontend tab.
- Measured, imputed, and simulated values remain separate through `valueBasis`; they must not be flattened into generic metrics.
- SCADA stand-ins are public-safe resident source abstractions. They are not real plant data, sensor diagnostics, calibration workflows, alarm management, or control-room behavior.
- Simulation Ops remains the working run-scoped simulation slice and can later become a Simulator Workbench subview.

## Scaffolded Seams

| Seam | Stub artifacts | Acceptance criteria before implementation claim |
| --- | --- | --- |
| Value-basis contract | `docs/schemas/simulator-workbench/value-basis.v1.schema.json` | All measured, imputed, and simulated examples validate and remain visually/API-distinguishable. |
| Resident measured-source contract | `docs/schemas/scada/`, `examples/scada/` | Mixed flux, temperature, pressure, actuator, electrical, and comms tags validate as `valueBasis=measured`. |
| Digital twin contract | `docs/schemas/digital-twin/`, `examples/digital-twin/` | Twin state includes measured, imputed, and simulated values with lineage. |
| Workbench state contract | `docs/schemas/simulator-workbench/workbench-state.v1.schema.json`, `examples/simulator-workbench/` | Workbench summary counts match twin values and references existing examples. |
| Contract validation | `scripts/check-simulator-workbench-contract.mjs` | `bun run simulator-workbench:contract:check` passes and fails on basis flattening or missing source coverage. |
| Frontend anchors | `src/api/simulatorWorkbench.ts`, `src/domain/simulator-workbench/`, `src/components/simulator-workbench/` | TypeScript compiles and tests pass without mounting the workbench. |
| Backend anchors | `backend/slurm-gateway/internal/gateway/scada_*.go`, `twin_*.go`, `simulator_workbench_*.go` | Go tests compile without registering routes or changing existing handlers. |
| Resident source crate | `workers/scada-standins/` | Cargo tests prove the mixed source set exists and remains measured-only. |
| Visual draft | `docs/design/simulator-workbench-visual-draft.md` | Concept is marked non-implementation and avoids safety, control, and validated-physics claims. |

## Next Implementation Threads

1. Promote contract examples into a real resident-source ingest path only after the gateway API and storage shape are reviewed.
2. Add HTTP-polled twin read APIs before any live WebTransport workbench stream.
3. Fold the existing SimOps control surface into a Simulator Workbench subview after the shell can explain measured, imputed, and simulated values clearly.
