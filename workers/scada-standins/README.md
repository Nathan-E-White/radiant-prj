# SCADA Stand-Ins

`scada-standins` is a compile-safe scaffold for resident measured-source stand-ins used by the future Simulator Workbench.

The crate currently declares the mixed public-safe source set only:

- flux
- temperature
- pressure
- actuator state
- electrical state
- comms

All tags remain `valueBasis=measured`. The crate does not ingest telemetry, launch containers, emulate field devices, implement alarms, or expose maintenance diagnostics.
