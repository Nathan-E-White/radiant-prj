# Simulation Ops Generator

`simops-generator` emits synthetic Simulation Ops telemetry from the controlled
`simops.run-manifest.v1` contract. Runs remain deterministic by default, and an
optional `randomization.distributionProfile` can add seeded correlated sampling
for frame emission, stream delay, burst loss, and per-worker sensitivity.

This worker is intentionally local and transport-agnostic. It reads a manifest,
selects one or more worker profiles, and writes `simops.telemetry.v1` NDJSON.
The Go ingestion/control-plane boundary remains responsible for future
authentication, encrypted transport, and orchestration.

Example:

```sh
cargo run -- \
  --manifest ../../examples/simulation-ops/run-manifest.scheduler-drift.json \
  --worker all \
  --frames 2 \
  --output -
```
