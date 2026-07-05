# Version History

| Field | Value |
| --- | --- |
| Document ID | REL-005 |
| Revision | 3.0 |
| Status | Draft for v3.0 review |
| Owner | Quality |
| Target Version | v3.0.0 |

| Version | Date | Summary | Baseline Evidence |
| --- | --- | --- | --- |
| v1.0.0 | 2026-06-30 | Initial controlled readiness console baseline with requirements, evidence, deterministic tests, and infrastructure checks. | `scripts/checkpoint-v1.sh` |
| v2.0.0 | 2026-07-02 | Completed controlled documentation, release procedures, quality checks, WIP checkpointing, fold-back script, and v2 checkpoint script at commit `481490ecf58c5dc44227c9fbe008f73c329efe20`. | Existing `v2.0.0` tag |
| v2.1.0 | TBD | Fixture-backed process traceability and version-aware release tooling with v2 compatibility wrappers. | `scripts/checkpoint-version.sh` |
| v3.0.0 | TBD | Mock-first Go Slurm gateway handlers, mTLS identity checks, job status lookup, metrics, deploy artifacts, and secret hygiene. | `bun run backend:test`, `bun run ci` |
