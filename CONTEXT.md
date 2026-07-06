# Radiant

Radiant is a public-safe engineering workbench for compute readiness, Simulation Ops, and Simulator Workbench demonstrations. Its language keeps data provenance explicit so measured observations, digital-twin estimates, and simulation results are not collapsed into one generic metric stream.

## Language

**Simulator Workbench**:
A top-level workbench surface that presents measured state, imputed state, simulated result state, and lineage together for public-safe review.
_Avoid_: SimOps tab, SCADA dashboard, control room

**Value Basis**:
The provenance class of a displayed value: measured, imputed, or simulated. It is part of the value's meaning, not a visual label that can be dropped.
_Avoid_: Metric type, category, display group

**Measured State**:
Direct sensor or SCADA stand-in observations from resident sources. In this repo it is always public-safe stand-in data, not real plant telemetry.
_Avoid_: Simulated reading, model output

**Imputed State**:
Digital-twin state inferred from measured inputs, model state, and lineage. It is model-derived state, not a raw observation.
Only the twin projector emits imputed state; Slurm/SimOps workers must never label their output as imputed.
_Avoid_: Measurement, sensor value

**Simulated Result State**:
Run-scoped scientific or compute result state produced by simulation workers and tied to a run, model, or artifact.
SimOps workers produce simulated result state separately from operational telemetry; the twin may consume it to create imputed state.
_Avoid_: Measurement, SCADA value

**Resident Source**:
A public-safe measured-source stand-in that exists independently of any single simulation run.
_Avoid_: Simulation worker, run worker

**Lineage**:
The source tags, model steps, simulation runs, and artifacts that explain why a displayed value exists.
_Avoid_: Log, trace, breadcrumb

**Simulation Health Summary**:
A compact trust summary for simulation result state, covering run completion and artifact disposition at a glance.
_Avoid_: Detailed health panel, SCADA health panel, infrastructure diagnostics
