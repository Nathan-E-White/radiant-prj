# Runtime Launchpad (Randiant-derived)

This workspace is now organized as:

- `apps/orchestrator` - CLI application with:
  - `orchestrator docker run`
  - `orchestrator k8s run`
  - `orchestrator monitor run` (docker/k8s observer modes)
- `libs/container-runtime` - Library A (Docker container lifecycle)
- `libs/k8s-runtime` - Library B (OpenTofu + Kind + helper pod logs)
- `libs/surveillance-runtime` - Kali-based network telemetry runtime
- `infra/tofu/k8s` - Kubernetes Pod Terraform manifest
- `infra/docker/kali-observer/Dockerfile` - minimal headless Kali image recipe
- `tools/preflight.sh` - optional binary availability check
- `tools/build-kali-observer.sh` - build local `kali-observer:local` image

## Quick Run

```bash
cd /Users/nathanwhite/Software-Development/radiant/radiant-prj/sandbox/randiant-runtime
./tools/preflight.sh


go run ./apps/orchestrator docker run --message "hello from container"
go run ./apps/orchestrator k8s run --message "hello from k8s"
./tools/build-kali-observer.sh
go run ./apps/orchestrator monitor run --backend docker --mode dual --duration 30s
```

The K8s flow uses Kind for a local cluster and destroys that cluster on shutdown
if it was created by the orchestrator.
