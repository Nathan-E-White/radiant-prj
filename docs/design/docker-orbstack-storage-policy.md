# Docker/OrbStack Storage Policy

| Field | Value |
| --- | --- |
| Document ID | DOP-001 |
| Revision | 1.0 |
| Status | Draft for v3.0 review |
| Owner | Software |
| Baseline | DEV-HYGIENE-001 |

## Purpose

This policy defines how Radiant reports and eventually manages local Docker/OrbStack storage. Reporting may inspect the local context, while cleanup must identify Radiant-owned resources before it can remove anything.

## Storage classes

| Storage class | What the report means | Default policy |
| --- | --- | --- |
| Images | Docker's image inventory, including total, active, size, and reclaimable values | Report only; do not prune by default |
| Build cache | Docker's builder cache inventory and reclaimable value | Report only; do not prune by default |
| Containers | Active and inactive container inventory | Protect active containers; do not remove containers by default |
| Volumes | Local persistent data, including volumes attached to services | Protected by default; never include in an unscoped cleanup |

`bun run hygiene:size` is the reporting path. When Docker is available, it queries `docker --context orbstack system df` and includes Docker's own `TOTAL`, `ACTIVE`, `SIZE`, and `RECLAIMABLE` fields. When Docker or the OrbStack context is unavailable, the report marks this optional section as skipped and continues without failing the audit.

## Cleanup boundary

The hygiene lane rejects generic `docker system prune` and `docker volume prune` behavior. Future cleanup must:

1. use the `orbstack` context explicitly;
2. select a storage class explicitly;
3. scope removal to Radiant-owned labels or named Compose resources;
4. print a dry-run plan before execution; and
5. require an explicit execution flag, with a separate confirmation for volumes.

The existing `scripts/docker-prune-hygiene.sh` is a guarded, category-based helper for planned future cleanup. Execution requires `--scope-label KEY=VALUE`, applies that label filter to every selected storage class, and separately requires volume confirmation. Its dry-run mode and volume confirmation do not override this policy: unscoped cleanup remains prohibited.

## Local preconditions

Before running a Docker storage report locally:

- Docker Desktop or OrbStack must be installed and running.
- The `orbstack` Docker context must exist and be reachable.
- Docker-reported reclaimable space is not proof that an object is disposable to Radiant.
- Volume contents must be treated as persistent runtime state unless a named-resource review proves otherwise.

The report is read-only. It must not create, stop, remove, or prune Docker resources.
