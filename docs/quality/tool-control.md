# Tool Control Procedure

| Field | Value |
| --- | --- |
| Document ID | QP-008 |
| Revision | 2.1 |
| Status | Draft for v2.1 review |
| Owner | Software |
| Parent Plan | QP-001 |

## Purpose

This procedure defines controls for development, verification, and infrastructure tools used by the project.

## Tool Categories

| Tool | Use | Control |
| --- | --- | --- |
| Bun | Package execution, tests, build | `package.json` scripts and local install record |
| TypeScript | Static type checking | `tsconfig.json` |
| Vitest | Unit tests | `src/domain/readiness.test.ts` |
| Playwright | Assembled browser acceptance and recovery tests | `playwright.config.ts` and `tests/e2e/` |
| Stryker | Mutation analysis of Workbench Snapshot policy | Pinned package versions, `stryker.config.mjs`, and `bun run test:mutation:workbench`; `bun run ci` invokes the gate and fails below 80% |
| React Chaos | Browser fault injection at the Simulation Health presentation boundary | Pinned package version and `tests/e2e/workbench-health-chaos.spec.ts` |
| Vite | Web build | `vite.config.ts` |
| Node.js | Utility scripts | Script review and deterministic output |
| Terraform | Optional infrastructure validation | Static check plus optional native validation |
| Ansible | Optional syntax validation | Static check plus optional syntax check |
| Git | Configuration and release baselines | Branch, commit, tag, and worktree controls |

Toxiproxy is used only through a repository-owned, bounded experiment definition. It is not a substitute for deterministic browser outcome tests and is not invoked ad hoc against shared services. Language-specific Go analyzers apply when a change includes Go source or when a controlled backend verification procedure names them.

## Tool Qualification Approach

Tools are controlled by versioned configuration, deterministic command wrappers, and independent review of generated or transformed outputs. Release readiness does not rely on unreviewed tool output alone.

## Tool Failure Handling

When an optional tool is unavailable, the static check shall report that native validation was skipped and confirm the static artifact check still passed. Required tool failures block release.
