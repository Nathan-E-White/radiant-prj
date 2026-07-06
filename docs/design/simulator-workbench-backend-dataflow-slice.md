# Simulator Workbench Backend Dataflow Slice

| Field | Value |
| --- | --- |
| Document ID | SWB-DATAFLOW-001 |
| Revision | 0.1 |
| Status | Implemented backend slice |
| Owner | Software |
| Baseline | v3.0 candidate |

## Purpose

This design record controls the backend-only Workbench dataflow slice. The slice proves that one resident measured SCADA unit, one SimOps operational telemetry unit, one synthetic simulated result unit, and one imputed twin value travel through Redpanda, Postgres projections, Iceberg tables, and read-only Workbench APIs.

No frontend controls, visualization wiring, real physics computation, SCADA health panel, or detailed Simulation Health panel is included in this cut.

## Domain Boundary

| Producer | Stream | Value basis rule |
| --- | --- | --- |
| `workers/scada-standins` | `scada.telemetry.v1` | Always `measured` |
| `workers/simops-generator` | `simops.telemetry.v1` | Operational telemetry only; no value basis |
| `workers/simops-generator` | `simops.results.v1` | Always `simulated` |
| `backend/slurm-gateway/cmd/twin-projector` | `digital-twin.state.v1` | May emit `measured`, `simulated`, and `imputed`; only this process emits `imputed` |

## End-To-End Flow

```mermaid
flowchart LR
  SCADA["Resident SCADA Stand-In"] --> SCADA_INGEST["SCADA Ingest"]
  SCADA_INGEST --> SCADA_TOPIC["Redpanda: scada.telemetry.v1"]
  SCADA_TOPIC --> SCADA_TS["Timescale: scada_measured_frames"]
  SCADA_TOPIC --> SCADA_LAKE["Iceberg: scada.measured_frames"]

  SIM["Run-Scoped SimOps Worker"] --> SIM_TEL["SimOps Telemetry Ingest"]
  SIM_TEL --> SIM_TOPIC["Redpanda: simops.telemetry.v1"]
  SIM_TOPIC --> SIM_TS["Timescale: simops_telemetry_frames"]
  SIM_TOPIC --> SIM_LAKE["Iceberg: simops.telemetry_frames"]

  SIM --> RESULT_INGEST["Simulated Result Ingest"]
  RESULT_INGEST --> RESULT_TOPIC["Redpanda: simops.results.v1"]
  RESULT_TOPIC --> RESULT_TS["Timescale: simops_result_values"]
  RESULT_TOPIC --> RESULT_LAKE["Iceberg: simops.simulated_results"]

  SCADA_TOPIC --> TWIN["Twin Projector"]
  RESULT_TOPIC --> TWIN
  TWIN --> TWIN_STATE["Twin State + Lineage Tables"]
  TWIN --> TWIN_LAKE["Iceberg: digital_twin.state_values"]
  TWIN_STATE --> API["Read-Only Workbench APIs"]
```

## Service Architecture

```mermaid
flowchart TB
  subgraph Compose["Local Compose Platform"]
    GW["slurm-gateway"]
    RP["Redpanda"]
    PG["Timescale/Postgres"]
    MINIO["MinIO Warehouse"]
    SCADAW["scada-standins"]
    SIMW["simops-generator"]
    TSWR["Projection Writers"]
    ICEWR["Iceberg Writers"]
    TWINP["twin-projector"]
  end

  SCADAW --> GW
  SIMW --> GW
  GW --> RP
  RP --> TSWR
  TSWR --> PG
  RP --> ICEWR
  ICEWR --> MINIO
  RP --> TWINP
  TWINP --> PG
  TWINP --> RP
```

## Controlled Interfaces

| Interface | Method | Purpose |
| --- | --- | --- |
| `/internal/scada/sources` | POST | Register public-safe resident source declarations |
| `/internal/scada/telemetry` | POST | Ingest measured SCADA stand-in frames |
| `/internal/simops/runs/{run_id}/ingest` | POST | Ingest operational SimOps telemetry |
| `/internal/simops/runs/{run_id}/results` | POST | Ingest synthetic simulated result frames |
| `/api/simulator-workbench/state` | GET | Read compact Workbench state summary |
| `/api/simulator-workbench/measured` | GET | Read latest measured frames |
| `/api/simulator-workbench/twin` | GET | Read current twin state |
| `/api/simulator-workbench/lineage/{value_id}` | GET | Read selected value lineage |

## Verification

`bun run simulator-workbench:dataflow:smoke` is the objective evidence command for this slice. It starts the local compose platform, sends one bounded SCADA frame batch, launches a `scheduler-drift` run with the `burst-01` worker, and verifies:

- Redpanda topics exist for SCADA telemetry, SimOps telemetry, SimOps results, and twin state.
- Postgres projections contain measured, telemetry, simulated result, imputed twin, and lineage rows.
- Iceberg catalog tables exist for `simops.telemetry_frames`, `scada.measured_frames`, `simops.simulated_results`, and `digital_twin.state_values`.
- MinIO contains Parquet-backed Iceberg data files.
- Read-only Workbench APIs return measured, simulated, imputed, and lineage data.
