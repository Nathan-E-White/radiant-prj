# SimOps runtime adapters

Parent PRD: #19  
Implementation issues: #20–#27

## Control-plane boundary

The existing Simulation Ops run API owns run creation, identity, persistence,
and lifecycle presentation. Before launch, the gateway builds one
`RunConnectionProfile` per worker. The profile carries run and worker identity,
worker image and manifest, labels, gateway ingest URLs and token, cleanup
policy, and runtime-specific naming. Runtime adapters consume this profile; API
handlers do not construct Docker containers or Kubernetes Jobs.

`SyncRun` returns Observed Worker Lifecycle only. It does not infer telemetry
health, artifact disposition, Redpanda/Postgres/Iceberg health, simulated result
quality, or imputed twin state.

## Runtime implementations

The Docker adapter uses the Docker SDK through a narrow fakeable client. It
creates labeled run-scoped containers, records structured runtime metadata,
observes container state, and stops only containers matching the run and adapter
labels. It does not assemble or execute Docker CLI command strings.

The Kubernetes adapter uses `client-go` and native `batch/v1` Jobs. It maps the
profile into Job and Pod labels, worker arguments, namespace, service account,
restart policy, and `ttlSecondsAfterFinished`. Job conditions, Pod phases, and
image-pull failures map into the same runtime-neutral state set used by Docker.
It supports in-cluster configuration, an explicit kubeconfig through
`SIMOPS_WORKER_KUBECONFIG`, or the default kubeconfig loading rules.

## Credential boundary

Ordinary workers follow Gateway-Only Worker Ingest. They receive identity,
manifest selection, gateway ingest URLs, and run tokens. They do not receive
direct Redpanda/Kafka, Postgres, Iceberg, S3, Docker, Kubernetes, or Workbench
credentials. Trusted stream gateways, projectors, and writers use separate
trusted profiles with only the data-plane fields their roles require.

The OpenTofu worker service account disables automatic token mounting. The
gateway service account receives namespace-scoped Job create/get/list/watch/
delete and Pod get/list/watch permissions.

## Cleanup policy

| Outcome and mode | Policy |
| --- | --- |
| Successful Docker smoke worker | Zero-TTL proof removes the container after success evidence |
| Failed Docker local worker | Retained for logs by default; smoke force cleanup removes labeled containers |
| Successful Kubernetes Job | `ttlSecondsAfterFinished` is rendered from the profile; smoke verifies the value and deletes the proved Job |
| Failed Kubernetes local Job | Retained when `SIMOPS_KIND_FORCE_CLEANUP=0`; automated smoke defaults force cleanup on |
| CI or smoke exit | Scoped run resources and temporary Kind clusters are force-cleaned |

Cleanup remains runtime-resource cleanup. It does not delete telemetry,
simulated results, artifacts, projections, or lineage evidence.

## OpenTofu boundary

OpenTofu provisions static substrate only: namespace, gateway and worker service
accounts, scoped RBAC, and the runtime adapter ConfigMap. Its outputs match
`SIMOPS_WORKER_RUNTIME`, `SIMOPS_WORKER_KUBERNETES_NAMESPACE`,
`SIMOPS_WORKER_KUBERNETES_SERVICE_ACCOUNT`, and
`SIMOPS_WORKER_CLEANUP_TTL`. The Go adapter creates every run-scoped Job; there
is no per-run OpenTofu apply.

## Verification commands

```bash
bun run backend:test
bun run backend:deps:check
bun run simops:smoke:json:test
bun run simops:smoke:docker-orbstack
bun run simops:smoke:kind:check
bun run simops:smoke:kind -- --timeout 300 --build auto
bun run simops:tofu:check
bun run simops:tofu:preflight
bun run ci
bun run build
```

Docker/OrbStack and Kind smoke commands require access to the local container
engine and should run elevated in the Codex sandbox. The static checks and unit
tests do not create containers or clusters.

## Final closeout evidence — 2026-07-12

The first Docker `--build auto` attempt reused an obsolete gateway image whose
worker records lacked the Docker SDK adapter labels. The bounded smoke stopped
its run. A targeted `--build always` rebuilt only the gateway and worker images;
the subsequent proof passed:

```text
RUN-340A19182FB1: succeeded, runtime=docker, frames=2, zero-TTL container cleanup
RUN-31086CA4E4C5: failed, worker logs retained, then force-cleaned
Docker/OrbStack SimOps runtime smoke passed
```

The final Kind proof passed and deleted its cluster on exit:

```text
RUN-73D34421D3CE: Job succeeded, runtime=kubernetes, frames=2
RUN-5A8A944EC9FA: Job image-pull-failed, retained through evidence capture, then force-cleaned
Kind/client-go SimOps smoke passed
```

The final no-mutation OpenTofu preflight reported:

```text
Plan: 6 to add, 0 to change, 0 to destroy.
namespace=radiant-simops service_account=simops-worker runtime_config_map=simops-runtime-adapter mutation=false
```

The final ordinary quality suites reported:

```text
bun run ci: 13 Vitest files passed, 43 tests passed
backend:test: internal/gateway, internal/simopsdocker, and internal/simopskubernetes passed; command packages had no test files
simops:generator:test: 14 library tests and 2 contract tests passed
scada:standins:test: 4 contract tests passed
bun run build: TypeScript compilation and Vite production build passed
```

Backend dependency isolation, documentation, storage-policy, size, and
guarded-cleanup checks also passed as part of `bun run ci`.

## Deferred work

The v3 runtime slice does not include a SimopsRun CRD/operator, Argo Workflows,
Tekton Pipelines, host-facing Redpanda listeners, production hardening, or
production cloud provisioning. Kind and OrbStack are local verification
substrates, not the production runtime architecture.
