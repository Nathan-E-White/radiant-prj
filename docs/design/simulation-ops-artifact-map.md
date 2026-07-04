# Simulation Ops Artifact Map

| Field | Value |
| --- | --- |
| Document ID | SIMOPS-MAP-001 |
| Revision | 0.1 |
| Status | Mockup only |
| Owner | Software |
| Baseline | v3.0 planning input |

## Purpose

This map identifies what the integrated Simulation Ops mockup consumes, displays, and proposes to produce. It is docs-only and does not add fixture records, generated evidence, API clients, UI code, live transport, or worker orchestration.

## Consumed Existing Artifacts

| Artifact | Role in mockup | Boundary |
| --- | --- | --- |
| Current app IA | Keeps Simulation Ops inside the existing Compute Workbench and Evidence Matrix flow | No new top-level product lane |
| `JOB-HPC-404` | Primary scenario anchor for scheduler/module drift and failure triage | Synthetic scheduler and module logs only |
| `EP-HPC-404` | Existing evidence destination for HPC failure triage | Not evidence of actual Radiant infrastructure |
| `SLURM-GATEWAY-001` | Backend seam for submit/status semantics and gateway verification | Mock-first; no browser-held private keys |
| Deployment checks | Source for storage, service, config, and network-storage warning language | Dry-run/local-safe infrastructure intent |
| Existing ADR ASCII sketch | Raw spatial input for layout discussion | Must be reframed away from control-room claims |
| Simulation Ops telemetry contract | Source for scenario frames, payload fields, and run-summary examples | Contract and examples only; no live stream or containers |

## Displayed Mockup Outputs

| Display output | Source | Evidence path |
| --- | --- | --- |
| Workload scenario label | Selected mock scenario and selected job | Scenario review note |
| Multiphysics co-scheduler | Slurm allocation concept, MPI rank distribution, barrier wait timeline, queue Gantt | `SLURM-GATEWAY-001`, `EP-HPC-404` |
| IO/checkpoint burst buffer | Parallel file system IOPS, burst throughput, NVMe-oF saturation, heat map, countdown | Deployment finding candidate |
| Elastic cloud bursting | Synthetic hot-spot workload cue, ParallelCluster scaling, EFA drops, spot cost, topology graph | Scenario review note |
| Fabric/MPI profiler | InfiniBand counters, message-size distribution, non-blocking overhead, node-link map | Deployment finding candidate and triage note |
| Diagnostic log | Existing job logs and diagnosis model | `triage-note.md` |
| Evidence handoff | Evidence packs and controlled evidence records | Evidence Matrix review |

## Proposed Design-Only Artifacts

These are named outputs in the mockup. They are not generated or implemented by this task.

| Proposed artifact | Purpose | Review disposition |
| --- | --- | --- |
| `degraded-state-note.md` | Summarize storage, cloud-burst, or fabric warning during a synthetic scenario | Decide whether to add as future fixture/evidence record |
| `scenario-review-note.md` | Capture design review of scenario boundary, data quality, and evidence trail | Decide whether to rewrite ADRs from reviewed mockup |
| `simulation-ops-run-summary.json` | Candidate future machine-readable scenario summary | Shape now captured by `simops-run-summary.v1` |
| `mpi-barrier-window.csv` | Candidate future MPI barrier wait and synchronization-latency artifact | Represented in the scheduler payload; CSV remains optional later |

## Data Flow Map

```text
Compute Workbench selection
  -> JOB-HPC-404 fixture context
  -> Simulation Ops stress mode mockup
  -> scenario panels display scheduler, storage, cloud-burst, and fabric stress frames
  -> contract examples define NDJSON telemetry and run-summary artifacts
  -> Slurm gateway seam is referenced for submit/status behavior
  -> diagnostic and degraded-state notes are named
  -> Evidence Matrix shows where reviewable artifacts would land
```

## Interface Boundaries

| Boundary | Allowed in mockup | Not allowed in mockup |
| --- | --- | --- |
| Frontend | Layout, panel names, state labels, data-quality concepts | React implementation or browser credential handling |
| Backend | Reference gateway endpoints and modes | New API handlers or direct browser-to-Slurm design |
| Fixtures | Reference existing jobs/evidence/deployment checks | Readiness fixture schema edits or generated records |
| Contract | Define schemas, examples, and validation for future telemetry | Live transport, Docker/Kubernetes workers, or UI state |
| Evidence | Name candidate future artifacts | Claim production infrastructure evidence |
| Physics/safety | Public-safe context and explicit non-safety boundary | Control-room, SCRAM, safety-path, or validated reactor behavior |

## Traceability Review Matrix

| Mockup concern | Existing anchor | Future decision before implementation |
| --- | --- | --- |
| Scheduler drift | `JOB-HPC-404` and `EP-HPC-404` | Whether to add replayable scenario records |
| Gateway seam | `SLURM-GATEWAY-001` | Whether UI stays fixture-backed or uses a trusted proxy |
| Multiphysics co-scheduling | `JOB-HPC-404`, `EP-HPC-404`, gateway contract, and Simulation Ops schemas | Whether MPI rank and barrier-wait traces become live worker output |
| Checkpoint pressure | Deployment checks | Whether degraded evidence needs a controlled record type |
| Elastic cloud bursting | Job-description infrastructure context | Whether cloud-burst topology and cost metrics stay visual-only |
| Fabric/MPI profiling | Deployment checks and job-description context | Whether fabric metrics stay synthetic or become fixture-backed |
| Evidence handoff | Evidence Matrix | Whether mockup artifacts become generated evidence inputs |

## Acceptance Checklist

- The mockup integrates with the current Compute Workbench and Evidence Matrix.
- Every displayed stress signal has a consumed artifact or an explicitly proposed future artifact.
- No direct frontend Slurm credentials are implied.
- No real infrastructure, safety, or validated-physics claim is made.
- ADR rewrite remains a later task after design review.
