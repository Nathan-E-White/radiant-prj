# Tool Control Procedure

| Field | Value |
| --- | --- |
| Document ID | QP-008 |
| Revision | 2.0 |
| Status | Draft for v2 review |
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
| Vite | Web build | `vite.config.ts` |
| Node.js | Utility scripts | Script review and deterministic output |
| Terraform | Optional infrastructure validation | Static check plus optional native validation |
| Ansible | Optional syntax validation | Static check plus optional syntax check |
| Git | Configuration and release baselines | Branch, commit, tag, and worktree controls |

## Tool Qualification Approach

Tools are controlled by versioned configuration, deterministic command wrappers, and independent review of generated or transformed outputs. Release readiness does not rely on unreviewed tool output alone.

## Tool Failure Handling

When an optional tool is unavailable, the static check shall report that native validation was skipped and confirm the static artifact check still passed. Required tool failures block release.

