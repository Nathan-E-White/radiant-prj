# SCADA Stand-Ins

`scada-standins` emits public-safe resident measured-source stand-ins through the gateway-only Workbench ingest path.

One `ResidentSource` module owns the declaration, tag catalog, controlled clock,
sequence, declaration-before-publication order, and bounded reactor emission.
The CLI only assembles a configured source with an HTTP publisher and production
clock/cadence. Tests use the same publisher interface with an in-memory adapter.

The mixed public-safe source set contains:

- flux
- temperature
- pressure
- actuator state
- electrical state
- comms

All tags remain `valueBasis=measured`; the worker cannot configure any other
value basis. Static mode preserves the mixed source identity and emits until its
host stops it. Dynamic mode requires a stable `--source-id`, `--reactor-id`, and
worker index from `0` through `2`; it scopes tags and timestamps to that reactor
and stops after the configured `--max-frames` bound.

The trusted gateway launches and removes containers. Each worker receives only its gateway URL and source-scoped, reactor-bound ingest token. It receives no broker, database, lake, Docker, or cluster credential. The crate does not emulate field devices, implement alarms, expose maintenance diagnostics, or represent production SCADA.
