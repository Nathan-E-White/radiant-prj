# OpenTofu SimOps substrate lane

Issue: #26  
Parent PRD: #19  
Related runtime issues: #24 and #25

## Boundary

OpenTofu owns mostly static Kubernetes substrate: the namespace, gateway and
worker service accounts, namespace-scoped RBAC, and the runtime adapter
ConfigMap. The Go SimOps control plane owns every run-scoped Docker container or
Kubernetes Job. This lane contains no `kubernetes_job` resource and performs no
per-run apply.

Ordinary worker service accounts disable automatic token mounting. Gateway RBAC
is limited to Job create/get/list/watch/delete and Pod get/list/watch inside the
SimOps namespace.

## No-mutation preflight

Run from the repository root:

```bash
bun run simops:tofu:check
bun run simops:tofu:preflight
```

The preflight uses an isolated synthetic kubeconfig, `TF_DATA_DIR`, plugin
cache, and plan file under `/tmp`. It runs:

```text
tofu fmt -check -recursive
tofu init -backend=false -input=false
tofu validate
tofu plan -no-color -input=false -lock=false -refresh=false
```

It never runs `tofu apply`. The committed provider lockfile is used read-only.
The plan file and unique transient data directory are deleted on exit. The
shared provider cache remains under `/tmp` for reuse by later verification and
can be deleted without touching repository state. Evidence values are read from
the saved plan JSON and checked against the Go adapter environment contract.

## Evidence captured 2026-07-12

OpenTofu `v1.12.3` selected `hashicorp/kubernetes v2.38.0` and reported:

```text
plan_summary=Plan: 6 to add, 0 to change, 0 to destroy.
namespace=radiant-simops service_account=simops-worker runtime_config_map=simops-runtime-adapter mutation=false
```

The planned resources were one namespace, two service accounts, one Role, one
RoleBinding, and one ConfigMap. Outputs included:

```text
SIMOPS_WORKER_RUNTIME=kubernetes
SIMOPS_WORKER_KUBERNETES_NAMESPACE=radiant-simops
SIMOPS_WORKER_KUBERNETES_SERVICE_ACCOUNT=simops-worker
SIMOPS_WORKER_CLEANUP_TTL=60s
```

These keys are consumed directly by `LoadConfigFromEnv` and the Kubernetes
runtime adapter. The plan does not provision the Kind cluster, application
Deployment, run workers, Redpanda, Postgres, or Iceberg.
