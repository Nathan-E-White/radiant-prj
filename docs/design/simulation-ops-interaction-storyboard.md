# Simulation Ops Interaction Storyboard

| Field | Value |
| --- | --- |
| Document ID | SIMOPS-STORY-001 |
| Revision | 0.1 |
| Status | Mockup only |
| Owner | Software |
| Baseline | v3.0 planning input |

## Purpose

This storyboard describes key-triggered synthetic Simulation Ops scenarios inside the Status Workbench lower HPC status bay. It is a design artifact only. It does not define keyboard handlers, React state, backend calls, or fixture schema.

## Shared Interaction Model

```text
User selects Status Workbench
  -> selects JOB-HPC-404
  -> reviews the lower HPC status bay
  -> triggers a synthetic scenario key
  -> workbench panels update in the mockup
  -> evidence handoff names the records that would be reviewed
  -> Evidence remains the destination for traceability
```

Each scenario is synthetic simulation-operations stress data. It is not a plant event, operator command, safety function, or physical reactor state.

## Scenario Keys

| Key | Scenario | Primary purpose | Evidence destination |
| --- | --- | --- | --- |
| N | Nominal replay | Show healthy baseline for comparison across all four panels | Existing job/evidence records |
| S | Scheduler module drift | Exercise the Multiphysics Job Co-scheduler with `JOB-HPC-404` | `EP-HPC-404` |
| C | Checkpoint pressure | Exercise IO and Checkpoint Burst Buffer Monitor storage degradation | Deployment checks and degraded-state note |
| B | Cloud burst | Exercise Core Thermal Mesh Elastic Bursting Visualizer orchestration | Scenario review note |
| F | Fabric warning | Exercise Fabric Topology and MPI Profiler diagnostics | Deployment checks and triage note |

## Storyboard: Nominal Replay

| Step | Workbench state | Notes |
| --- | --- | --- |
| 1 | User selects `N` while `JOB-HPC-404` is selected | Mockup resets the stress panels to healthy comparison values. |
| 2 | Multiphysics Job Co-scheduler shows nominal allocation and barrier wait timeline | The gateway remains a backend seam, not direct frontend Slurm control. |
| 3 | IO/checkpoint and fabric panels show no active warning | Display samples are not treated as audit records. |
| 4 | Elastic bursting panel shows local-only mode | Existing deployment checks remain visible for context. |
| 5 | Evidence Handoff shows no new degraded-state note | Reviewer can compare healthy versus degraded panels. |

## Storyboard: Scheduler Module Drift

| Step | Workbench state | Notes |
| --- | --- | --- |
| 1 | User selects `S` | Scenario expands the existing `JOB-HPC-404` failure. |
| 2 | Multiphysics Job Co-scheduler shows `scheduler drift`, `module-rerun`, allocation state, MPI rank split, and active queue Gantt | The trigger is an infrastructure workload condition, not reactor behavior. |
| 3 | Co-scheduler status shows held or failed synthetic job state | The view references the `SLURM-GATEWAY-001` contract for submit/status semantics. |
| 4 | Diagnostic Log shows the existing worker module mismatch lines | Root cause and next action are pulled from the current fixture concept. |
| 5 | Evidence Handoff points to `slurm-404.out`, `module-inventory.diff`, and `triage-note.md` | Evidence remains the controlled destination. |

## Storyboard: Checkpoint Pressure

| Step | Workbench state | Notes |
| --- | --- | --- |
| 1 | User selects `C` | Scenario emphasizes storage backpressure rather than job code failure. |
| 2 | IO and Checkpoint Burst Buffer Monitor shows IOPS, throughput, NVMe-oF cache saturation, storage heat map, and checkpoint countdown | This aligns with `DEP-001` style deployment findings. |
| 3 | Fabric and co-scheduler panels remain independent unless storage pressure delays job evidence capture | Display freshness and evidence durability are separate signals. |
| 4 | Diagnostic Log recommends isolating the artifact mount and retrying from verified input | No real Radiant infrastructure is implied. |
| 5 | Evidence Handoff proposes `degraded-state-note.md` | Note records data-quality limits for review before implementation. |

## Storyboard: Cloud Burst

| Step | Workbench state | Notes |
| --- | --- | --- |
| 1 | User selects `B` | Scenario stresses elastic orchestration and cost visibility. |
| 2 | Core Thermal Mesh Elastic Bursting Visualizer shows a synthetic hot-spot workload trigger and cloud topology graph | The hot spot is a compute workload cue, not a physics claim. |
| 3 | Panel shows ParallelCluster autoscaling state, EFA packet-drop rate, and spot cost tracker | Separate cloud orchestration from safety or plant control. |
| 4 | Diagnostic Log records whether cloud burst activation changed scheduler or fabric state | The log is synthetic and review-oriented. |
| 5 | Evidence Handoff proposes a scenario review note | Review should decide whether cloud-burst fields become fixture/schema work later. |

## Storyboard: Fabric Warning

| Step | Workbench state | Notes |
| --- | --- | --- |
| 1 | User selects `F` | Scenario stresses distributed simulation networking. |
| 2 | Fabric Topology and MPI Profiler shows InfiniBand counters, message-size distribution, non-blocking communication overhead, and topology heat | This is HPC infrastructure telemetry, not plant instrumentation. |
| 3 | Co-scheduler barrier waits may rise if fabric delay affects synchronization | The synchronization warning is visible in the same workbench. |
| 4 | Diagnostic Log recommends checking fabric counters, job placement, and artifact transfer path | This reflects job-description-aligned Linux/HPC triage. |
| 5 | Evidence Handoff links deployment checks and triage note | Evidence records limitations and synthetic provenance. |

## Review Notes

- Scenario keys are mockup shorthand only; no keyboard interface is approved by this document.
- Scenario names should remain infrastructure-focused.
- The workbench can use transient-like workload language only when it is explicitly labeled as synthetic simulation-operations stress data.
- Any later implementation must decide whether these scenarios stay fixture-backed, call a trusted backend proxy, or remain static demo states.
