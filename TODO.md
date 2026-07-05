# Radiant SimOps Development TODO / Wishlist

Legend
- `P0` = must-have, `P1` = important, `P2` = optional enhancement
- Quarter format: `Q3 2026`, `Q4 2026`, `Q1 2027`, etc.
- Status: `in progress`, `blocked`, `done`

| ID | Priority | Quarter | Owner | ETA | Title | Scope | Status |
| --- | --- | --- | --- | --- | --- | --- | --- |
| SIMOPS-001 | P0 | Q3 2026 | backend | 2026-07 | Backend worker lifecycle orchestration | Implement full container lifecycle orchestration for Rust workers in the Go backend (create/list/stop/delete), keeping existing SimOps control-plane semantics and frontend-facing intent behavior. | in progress |
| SIMOPS-002 | P0 | Q3 2026 | frontend | 2026-07 | Minimal create/monitor/stop controls | Add frontend create/monitor/stop controls in React with worker run state visibility and lifecycle action paths. | in progress |
| SIMOPS-003 | P1 | Q4 2026 | platform/infra | 2026-09 | Kubernetes orchestration support | Add Kubernetes orchestration support with `radiant-runtime` `k8s-runtime` path (Kind + OpenTofu flow), preserving Docker as default for compatibility. | in progress |
| SIMOPS-004 | P1 | Q4 2026 | backend | 2026-10 | Non-long-lived worker modes | Add worker modes beyond long-lived execution: one-shot jobs, bounded run-step limits, profile presets (`worker_profile`), graceful exit/escalation handling, and heartbeat behavior. | in progress |
| SIMOPS-005 | P1 | Q4 2026 | frontend | 2026-10 | Richer controls and operator controls | Add per-run/per-worker detail views, logs stream, restart/retry/destroy controls, status polling/backoff, and list filtering/sorting/badges. | in progress |
| SIMOPS-006 | P0 | Q1 2027 | data-platform | 2027-01 | Integrate iceberg-rust + Redpanda + Postgres telemetry/artifact path | Integrate Postgres-backed SimOps control/store, Redpanda ingress for lifecycle/telemetry events, Iceberg schema/artifact writer path, and run-artifact queryability with lineage and UI-facing event stream hooks. | in progress |
| SIMOPS-007 | P2 | Q1 2027 | backend | 2027-01 | Hardening/security/rate-limits/audit trail | Add AuthN/Z enforcement, quotas/guardrails, immutable audit trail + structured events, and orchestrator-failure test harness for start/stop/delete failure modes. | in progress |
