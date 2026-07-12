# Kind/client-go SimOps runtime proof

Issue: #25  
Parent PRD: #19  
Runtime adapter: #24

## Scope

This proof exercises the existing SimOps API against a local Kind cluster on
OrbStack. It proves Kubernetes Job launch, gateway ingest, runtime lifecycle
sync, success TTL configuration, failed-Job retention, and forced smoke cleanup.
It does not claim production cluster hardening, CRD/operator behavior,
Argo/Tekton orchestration, host-facing Redpanda listeners, or lakehouse
performance.

Ordinary workers receive only run identity and gateway ingest inputs. The smoke
rejects Redpanda, Postgres, Iceberg, and AWS credential material in worker Pod
arguments or environment.

## Command

Install or expose `kind`, `kubectl`, and Docker, then run:

```bash
SIMOPS_DOCKER_CONTEXT=orbstack bun run simops:smoke:kind -- --timeout 300 --build auto
```

Use `--build always` after gateway or worker source changes. Use `--build never`
to prove already-built images. The smoke uses an isolated kubeconfig under
`/tmp`, creates `radiant-simops-smoke`, and deletes the cluster by default.
Set `SIMOPS_KIND_FORCE_CLEANUP=0` only when retaining a failed cluster for local
diagnosis; delete it afterward with:

```bash
DOCKER_CONTEXT=orbstack kind delete cluster --name radiant-simops-smoke
```

CI can verify the smoke contract without creating a cluster:

```bash
bun run simops:smoke:kind:check
bun run simops:smoke:json:test
```

## Evidence captured 2026-07-12

The elevated OrbStack run used Kind `v0.32.0` and node image
`kindest/node:v1.36.1`.

Successful path:

```text
cluster_context=kind-radiant-simops-smoke namespace=radiant-simops job_name=simops-run-d5c9f0342c87-scheduler-01 run_id=RUN-D5C9F0342C87 final_lifecycle=succeeded
Runtime worker proof: worker=scheduler-01 state=succeeded runtime=kubernetes frames=2
```

Failure-retention path:

```text
cluster_context=kind-radiant-simops-smoke namespace=radiant-simops job_name=simops-run-8fde1a677a53-scheduler-01 run_id=RUN-8FDE1A677A53 final_lifecycle=image-pull-failed retained=true
```

The successful Job reported `ttlSecondsAfterFinished=60`. The smoke deleted the
successful Job after proof, changed the gateway worker image to a deliberately
invalid reference, observed and retained the failed Job, then force-deleted that
Job. The exit trap deleted the Kind cluster and temporary kubeconfig.

## Local image requirements

The script builds and loads:

- `radiant-slurm-gateway:kind` from `deploy/slurm-gateway.Dockerfile`, target
  `gateway-runtime`;
- `radiant-simops-generator:kind` from
  `deploy/simops-generator.Dockerfile`.

Kind loads both local images into its control-plane node. The Kubernetes
manifests set `imagePullPolicy: Never`, so a missing load fails visibly rather
than pulling a similarly named registry image.
