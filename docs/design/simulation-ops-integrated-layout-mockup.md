# Simulation Ops Integrated Layout Mockup

| Field | Value |
| --- | --- |
| Document ID | SIMOPS-LAYOUT-001 |
| Revision | 0.1 |
| Status | Mockup only |
| Owner | Software |
| Baseline | v3.0 planning input |

## Purpose

This document mocks up how a Simulation Ops stress view would be absorbed into the existing Kaleidos Compute Readiness Console. It is a design artifact only. It does not implement React changes, backend calls, fixture schema, Phaser, WebGL, or any control-room behavior.

The mockup treats Simulation Ops as an enhanced Compute Workbench mode. It uses synthetic simulation-operations stress data to show multiphysics co-scheduling pressure, checkpoint storage backpressure, elastic cloud-burst orchestration, fabric topology, MPI profiling, diagnostic logs, and evidence handoff.

## Claim Boundary

- This is a non-safety simulation-operations workbench mockup.
- It is not a reactor control room, plant monitor, safety-path display, physics trainer, or validated reactor simulation.
- All scenario values are synthetic infrastructure stress signals.
- The Slurm gateway is represented as a trusted backend control-plane seam. Browser-to-Slurm direct credential handling is out of scope.

## Integrated App Flow

The existing top-level app flow stays intact:

```text
+--------------------------------------------------------------------------------+
| Kaleidos Compute Readiness Console                                             |
| public-safe demo | synthetic jobs | controlled evidence                         |
+--------------------------------------------------------------------------------+
| [Kaleidos Brief] [Compute Workbench] [Evidence Matrix]                         |
+--------------------------------------------------------------------------------+
```

Simulation Ops appears inside `Compute Workbench`, not as a fourth top-level app. The current queue, job detail, diagnosis, and bundle-state panels remain the base. Selecting `JOB-HPC-404` can reveal the stress layout because that fixture already represents synthetic infrastructure failure triage.

```text
+--------------------------------------------------------------------------------+
| Compute Workbench                                                              |
| Mode: [Job Queue] [Simulation Ops Stress]                                      |
+--------------------------------------------------------------------------------+
| Left rail: existing synthetic job queue                                        |
| - JOB-TRN-001                                                                 |
| - JOB-THM-001                                                                 |
| - JOB-FLT-001                                                                 |
| - JOB-HPC-404  selected                                                       |
+-----------------------------+-----------------------------+--------------------+
| Panel 1: Co-scheduler       | Panel 2: IO checkpoint      | Panel 3: Bursting |
| Slurm allocations           | IOPS / burst throughput     | thermal workload  |
| MPI rank distribution       | NVMe-oF cache saturation    | EFA drops / cost  |
| barrier wait timeline       | storage target heat map     | cloud topology    |
+-----------------------------+-----------------------------+--------------------+
| Panel 4: Fabric topology and MPI profiler                                      |
| InfiniBand counters | msg-size distribution | non-blocking comm overhead   |
| node-link map color-coded by utilization and synthetic fabric temperature      |
+--------------------------------------------------------------------------------+
| Diagnostic Log                                                                 |
| slurmstepd: launching 32 ranks across worker-a,worker-b                         |
| worker-b: modulecmd: module 'radlab_transport/0.4' not found                    |
| triage: worker image is stale relative to scheduler module index                |
+--------------------------------------------------------------------------------+
| Evidence Handoff                                                               |
| Generated or updated design/evidence trail                                     |
| - slurm-404.out                                                                |
| - module-inventory.diff                                                        |
| - triage-note.md                                                               |
| - degraded-state-note.md                                                       |
| Destination: Evidence Matrix -> EP-HPC-404 and SLURM-GATEWAY-001               |
+--------------------------------------------------------------------------------+
```

## Panel Responsibilities

| Panel | Consumes | Displays | Produces |
| --- | --- | --- | --- |
| Multiphysics Job Co-scheduler | Selected compute jobs, Slurm status concept, MPI rank distribution | Allocation state, rank split, barrier wait timeline, active queue Gantt | Scheduler stress review note |
| IO and Checkpoint Burst Buffer Monitor | Deployment checks, storage stress scenario | Parallel file system IOPS, burst throughput, NVMe-oF saturation, storage heat map, checkpoint countdown | Degraded-storage evidence candidate |
| Core Thermal Mesh Elastic Bursting Visualizer | Synthetic hot-spot workload scenario, cloud-burst concept | ParallelCluster scaling, EFA packet drops, spot cost tracking, cloud topology | Elastic-burst review note |
| Fabric Topology and MPI Profiler | Synthetic fabric metrics and MPI profiling concepts | InfiniBand counters, message-size distribution, communication overhead, node-link map | Fabric triage note |
| Diagnostic Log | Existing job logs and diagnosis | Runtime-style trace, root cause, next action | `triage-note.md` and review note |
| Evidence Handoff | Evidence packs and controlled evidence records | Destination records and limitations | Artifact map entry for review |

## Evidence Matrix Continuation

The Evidence Matrix remains the controlled destination. The mockup shows how the scenario would be traced without claiming production infrastructure.

```text
--------------------------------------------------------------------------------+
| Evidence Matrix                                                                |
+--------------------------------------------------------------------------------+
| Requirement links                                                              |
| SW-001: fixture-backed workbench display                                       |
| SW-002: diagnostic root cause and next action                                  |
| SW-004: dry-run deployment controls                                            |
| SW-008: gateway handler surface                                                |
+--------------------------------------------------------------------------------+
| Objective evidence                                                             |
| EP-HPC-404: HPC failure triage objective evidence                              |
| SLURM-GATEWAY-001: mock-first Slurm gateway backend baseline                   |
+--------------------------------------------------------------------------------+
| Scenario additions proposed by mockup                                          |
| degraded-state-note.md: data quality and storage/fabric warning summary        |
| scenario-review-note.md: design review disposition before implementation       |
+--------------------------------------------------------------------------------+
```

## Non-Goals

- Do not create a new top-level product surface.
- Do not add live browser calls to the Slurm gateway.
- Do not store client certificates in the frontend.
- Do not add reactor control, SCRAM, safety actuator, or validated physics language.
- Do not add Phaser/WebGL until the integrated layout has been reviewed and a measured rendering need is established.

## Review Questions

1. Does the layout feel like the current Compute Workbench evolving?
2. Can a reviewer trace `JOB-HPC-404` from scenario trigger to gateway seam to evidence output?
3. Are synthetic-data and non-safety boundaries visible without oral explanation?
