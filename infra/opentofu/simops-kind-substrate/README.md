# SimOps Kind substrate

This OpenTofu root provisions only the mostly static Kubernetes substrate used
by the SimOps runtime adapter:

- namespace;
- gateway and ordinary-worker service accounts;
- namespace-scoped Job and Pod observation RBAC for the gateway;
- a ConfigMap whose keys match the Go adapter environment contract.

The Go control plane remains responsible for creating, observing, and deleting
run-scoped Jobs. This module contains no `kubernetes_job` resource and must not
be applied once per run.

Run the no-mutation repository preflight from the repository root:

```bash
bun run simops:tofu:preflight
```

To provision a local Kind substrate intentionally, provide the same kubeconfig
and context used by Kind and run `tofu apply` from this directory. Application
is deliberately absent from the repository preflight and CI path.

The `runtime_adapter_env` output can be mapped into a gateway Deployment or the
`simops-runtime-adapter` ConfigMap can be referenced with `envFrom`. The worker
service account disables automatic token mounting; ordinary workers still
receive gateway ingest inputs from their run connection profile, not cluster or
data-plane credentials.
