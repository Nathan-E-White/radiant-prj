# SCADA Stand-Ins

`scada-standins` emits public-safe resident measured-source stand-ins through the gateway-only Workbench ingest path.

The crate currently declares the mixed public-safe source set only:

- flux
- temperature
- pressure
- actuator state
- electrical state
- comms

All tags remain `valueBasis=measured`. Static fixture mode preserves the original mixed source identity. Dynamic mode requires a stable `--source-id`, `--reactor-id`, and worker index from `0` through `2`; it scopes tags and live timestamps to that reactor and stops after the configured `--max-frames` bound.

The trusted gateway launches and removes containers. Each worker receives only its gateway URL and source-scoped, reactor-bound ingest token. It receives no broker, database, lake, Docker, or cluster credential. The crate does not emulate field devices, implement alarms, expose maintenance diagnostics, or represent production SCADA.
