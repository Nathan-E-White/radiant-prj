# Docker and Kubernetes to OKD, Podman, Buildah, and Kata Containers Migration Research

| Field | Value |
| --- | --- |
| Document ID | CONTAINER-PLATFORM-MIGRATION-RESEARCH-001 |
| Status | Research note; no implementation change |
| Updated | 2026-07-30 |
| Scope | Radiant's Docker/Compose development and Kubernetes worker path moving to OKD, with Podman and Buildah for development and CI, plus a policy-gated Kata Containers option for stronger workload isolation |

## Decision

Adopt this supported separation of concerns:

```text
developer and CI:  Dockerfiles/Containerfiles -> Buildah or podman build -> OCI image registry
standard workload: OKD control plane -> kubelet -> CRI-O -> runc or crun -> Pod
isolation option:  OKD runtime handler -> Kata Containers VM boundary -> Pod
```

Keep the application contract portable Kubernetes objects and apply an OKD
overlay for Routes, SCC-compatible security, storage selection, and identity.
Use Podman and Buildah to build, inspect, push, and run local containers; they
do **not** replace OKD's node engine. On OKD, CRI-O is the Kubernetes CRI
implementation and the current documented lower-level runtime choices are
`runc` and `crun`. [OKD container runtime](https://docs.okd.io/latest/nodes/containers/nodes-containers-using.html), [CRI-O upstream](https://github.com/cri-o/cri-o), [OKD runtime configuration](https://docs.okd.io/latest/machine_configuration/machine-configs-custom.html).

Kata Containers is an optional workload-isolation track, not a replacement for
Podman, Buildah, CRI-O, or the OKD control plane. Kata provides lightweight VMs
that retain a container-oriented experience while adding a VM isolation
boundary. Adopt it only for workloads whose threat model justifies its resource,
startup-latency, node-capability, and operational cost; the ordinary target
remains the OKD-managed `runc` or `crun` path. [Kata Containers](https://github.com/kata-containers/kata-containers), [OKD runtime configuration](https://docs.okd.io/latest/machine_configuration/machine-configs-custom.html).

## Source and terminology boundary

This note combines the repository inventory with official OKD, OCI, CRI-O,
Podman, Buildah, and Kata Containers project documentation. “OCI compatible” does not
mean that products occupy the same layer:

| Layer | Role | Target decision |
| --- | --- | --- |
| OKD | Kubernetes distribution, API, admission, networking, identity, and lifecycle platform. | Production orchestrator. |
| Kubernetes kubelet / CRI | The node agent asks a CRI implementation to create Pod sandboxes and containers. | Application code uses the Kubernetes API, never this node socket. |
| CRI-O | Kubernetes-native CRI engine that pulls OCI images and invokes a low-level OCI runtime. It follows Kubernetes minor releases. | OKD-managed node component; do not treat it as a developer Docker daemon. [CRI-O](https://github.com/cri-o/cri-o) |
| OCI Runtime Specification | Defines an OCI bundle and runtime lifecycle/configuration, including host-specific Linux settings. It is a specification, not a scheduler or image builder. | Image and workload portability boundary, subject to platform admission. [OCI runtime spec](https://specs.opencontainers.org/runtime-spec/) |
| runc / crun | Low-level OCI runtime that creates the process/container environment. `runc` is an OCI runtime implementation, deliberately not an end-user deployment tool. | Let OKD select and manage `runc` or `crun`; do not call either from Pods or CI. [runc](https://github.com/opencontainers/runc), [OKD runtime configuration](https://docs.okd.io/latest/machine_configuration/machine-configs-custom.html) |
| Podman | Daemonless container CLI; its build command uses Buildah internally. | Local development, inspection, and optionally CI execution. [Podman](https://docs.podman.io/en/latest/markdown/podman.1.html) |
| Buildah | Image-builder CLI that consumes Containerfile/Dockerfile syntax and can push image output. | Explicit reproducible CI image build/push mechanism. [Buildah build](https://github.com/containers/buildah/blob/main/docs/buildah-build.1.md) |
| Kata Containers | Virtual-machine-backed container runtime project providing workload isolation and security properties associated with VMs. | Optional, policy-gated isolation path; validate the target cluster's runtime-handler implementation before adopting. [Kata Containers](https://github.com/kata-containers/kata-containers) |

The useful consequence is pleasantly mundane: retain Dockerfile-compatible
image recipes, publish immutable image digests, and let the cluster own
container execution. Installing Buildah, Podman, or runc ad hoc on OKD nodes
would breach that boundary and create an upgrade and security burden. Kata, if
accepted, is different: it must be installed and managed through a supported
cluster runtime-handler integration, never copied into an application image or
treated as a developer-side runtime switch.

## Repository inventory and migration impact

| Current concern | Local evidence | Migration implication |
| --- | --- | --- |
| Image builds | `Dockerfile`, `worker.Dockerfile`, and the three `deploy/*.Dockerfile` files; `docker-bake.hcl` enumerates packaging targets. | Preserve multi-stage recipes initially. Replace Bake-specific target expansion and verification deliberately; do not rename files merely for aesthetics. |
| Local stacks | `docker-compose.yml` plus `deploy/slurm-gateway.compose.yml`. The larger stack has profiles, secrets, health checks, volumes, TCP/UDP mappings, and `depends_on`. | Compose is a local integration environment, not a deployment manifest. Test a selected Compose provider feature by feature before switching it. |
| Engine-coupled runtime | `backend/slurm-gateway/internal/simopsdocker/` uses Moby's client; `deploy/slurm-gateway.compose.yml` mounts `/var/run/docker.sock` and runs its gateway as `0:0`. Reactor telemetry also uses the Docker adapter. | This is the main security and portability blocker. Make Kubernetes/OKD the production worker execution path and remove the engine socket from the production design. |
| Kubernetes path | `backend/slurm-gateway/internal/simopskubernetes/spooler.go` creates Jobs; `cmd/server/main.go` selects Docker or Kubernetes from `SIMOPS_WORKER_RUNTIME`. `infra/opentofu/simops-kind-substrate/` is the present local-cluster substrate. | The Job adapter is the most direct route to OKD, but Kind acceptance is not OKD admission proof. |
| CI and checks | Packaging scripts and config reference Docker storage, Buildx/Bake, Compose, and Docker smoke flows. | Replace the checks in phases, keeping build, digest, image-pull, contract, and runtime evidence comparable. |

The listed paths are an inventory, not a claim that every Docker reference must
disappear. Docker-labelled data and compatibility tests may remain during the
transition; production deployment must not require the Docker Engine API.

## Supported target architecture

1. CI builds every repository image with Buildah (or `podman build` during a
   measured transition), pushes an OCI image to the chosen registry, records
   the immutable digest, and verifies a clean pull/run. Buildah accepts
   Containerfile/Dockerfile build instructions; Podman uses Buildah for image
   builds. [Buildah](https://github.com/containers/buildah/blob/main/docs/buildah-build.1.md), [Podman build](https://docs.podman.io/en/stable/markdown/podman-build.1.html).
2. Deploy a portable Kubernetes base: Namespace, ServiceAccounts/RBAC,
   ConfigMaps/Secrets, Deployments, Services, Jobs, PVCs, and NetworkPolicies.
   Add an OKD overlay for Routes, storage class, cluster OAuth groups, and only
   the SCC access that a restricted-compatible workload cannot avoid. [OKD SCCs](https://docs.okd.io/latest/authentication/managing-security-context-constraints.html), [OKD Routes](https://docs.okd.io/latest/networking/routes/route-configuration.html).
3. The gateway creates labelled `batch/v1` Jobs using its narrowly scoped
   service account. Ordinary worker Pods get no engine socket and normally no
   Kubernetes API credentials. Jobs are suitable for work that runs to
   completion. [Kubernetes Jobs](https://kubernetes.io/docs/concepts/workloads/controllers/job/), [Kubernetes service accounts](https://kubernetes.io/docs/concepts/security/service-accounts/).
4. Kubelet passes the workload to CRI-O; CRI-O invokes the selected node
   runtime. The application neither knows nor controls whether the cluster
   uses the documented `runc` or `crun` setting. If a hardened workload needs
   Kata, make it an explicitly scheduled runtime-handler class after proving
   the target cluster's supported installation, node prerequisites, and policy
   controls. Node runtime policy changes use the OKD Machine Config Operator
   path, not a Pod image or application setting. [OKD runtime configuration](https://docs.okd.io/latest/machine_configuration/machine-configs-custom.html), [Kata Containers](https://github.com/kata-containers/kata-containers).

## Portability and security gates

Use the Kubernetes Restricted baseline as the default: non-root execution,
`allowPrivilegeEscalation: false`, dropped capabilities, `RuntimeDefault`
seccomp, read-only root filesystems where the application supports them, and
explicit writable `emptyDir` or persistent volumes. Build images so executable
and write paths work with an arbitrary namespace-assigned UID. OKD SCCs govern
UID/GID, SELinux, capabilities, privilege, host access, volume types, and
seccomp; do not edit default SCCs, which upgrades can reset. [Kubernetes Pod
Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/), [OKD SCCs](https://docs.okd.io/latest/authentication/managing-security-context-constraints.html).

No workload may depend on `/var/run/docker.sock`, `/run/podman/podman.sock`,
privileged containers, `hostPath`, host network/PID namespaces, or a fixed UID
unless a reviewed exception proves it necessary. A Podman API socket is also a
high-privilege control plane: Podman's own documentation warns that access to
the service grants full Podman access. If temporary Docker-client compatibility
is necessary locally, Podman offers a Docker-compatible API layer, but it must
be isolated behind an adapter and must never be the OKD execution model.
[Podman service](https://docs.podman.io/en/latest/markdown/podman-system-service.1.html).

## Build, Compose, and developer workflow

Retain each current Dockerfile as a portable build recipe, then create a small
explicit target map from the Bake targets to `buildah bud`/`buildah push` (or
the equivalent `podman build` commands). Include build context, target stage,
build arguments, tag, platform, digest output, and SBOM/provenance policy in
that map. The claimed output is an image digest that the cluster pulls, not a
developer's local image name.

For the two Compose files, `podman compose` is a wrapper around an external
provider. Pin the provider/version and test profiles, secrets, health and
dependency ordering, named volumes, read-only/tmpfs mounts, TCP/UDP ports, and
teardown. On macOS, Podman operates through a managed Linux VM, so bind mounts,
network reachability, and image stores require a macOS acceptance pass rather
than a Linux-only assertion. [Podman compose](https://docs.podman.io/en/latest/markdown/podman-compose.1.html), [Podman machine](https://docs.podman.io/en/latest/markdown/podman-machine.1.html).

The `simopsdocker` and reactor-telemetry Docker adapter should remain available
only as a deliberately bounded local compatibility path while the Kubernetes
implementations reach feature parity. Do not point it blindly at a Podman
socket: test every used Moby API call and error case, and remove the mounted
socket from steady-state service definitions.

## CI, migration stages, and validation

| Stage | Change | Exit evidence |
| --- | --- | --- |
| 0. Baseline | Inventory image recipes, sockets, privileged settings, API calls, current digest and smoke evidence. | Reproducible current build; an owner for every engine-coupled path. |
| 1. Image parity | Add Buildah build/push by target without deleting Docker/Bake. Pull by digest and run unit/contract tests. | Recipe, labels, entrypoint, architecture, digest, and test results recorded for every target. |
| 2. Local workflow | Pilot a pinned Compose provider with Podman on Linux and macOS; keep Docker fallback. | Feature matrix passes, including the 15-service stack; clean-up and volume behaviour demonstrated. |
| 3. Runtime decoupling | Move production SimOps launches and reactor telemetry to the Kubernetes API path. Remove gateway socket/UID=0 requirements from target manifests. | No production deployment has an engine socket; labelled Job lifecycle, cancellation, logs, retry, and cleanup work. |
| 4. OKD pre-production | Apply overlays to a representative OKD project and use registry digests. | SCC admission, Route/TLS, DNS, storage, RBAC, network policy, and image pull verification pass. |
| 5. Kata decision gate | For any workload seeking VM isolation, prove the risk rationale, target-cluster runtime-handler support, node capacity, scheduling, observability, networking, storage, and rollback independently. | A documented accept/defer decision; no default use solely because Kata is available. |
| 6. Cutover | Route a bounded workload slice to OKD, compare contract outputs and observability, then promote. | Rollback exercised; SLO/error and data-integrity criteria meet the agreed threshold. |
| 7. Retirement | Delete obsolete Docker production path only after sustained evidence. | Repository search, deployment review, and CI prove no production socket/Engine dependence remains. |

## Decision table and residual risks

| Decision | Recommendation | Why / validation |
| --- | --- | --- |
| Image format | OCI image, pinned by digest. | OCI is the interoperable image/runtime boundary; verify registry pull on OKD. |
| Builder | Buildah first; `podman build` acceptable for developer ergonomics. | Both consume existing recipes; build and run every target. |
| Local orchestration | Pinned external Compose provider through Podman, after compatibility proof. | Podman does not itself implement Compose. |
| Production orchestration | OKD with portable Kubernetes base plus OKD overlays. | Fits existing Job adapter; validate SCC, Routes, storage and identity. |
| Node engine/runtime | OKD-managed CRI-O with `runc` or `crun`. | Only documented configurable choices; no application ownership. |
| Hardened workload isolation | Kata Containers only when the workload threat model justifies a VM boundary. | Requires explicit target-cluster runtime-handler support, node-capacity, performance, networking, storage, observability, and rollback validation. |

The greatest practical risks are false Docker API compatibility, Compose
provider drift, arbitrary-UID/SCC failures, registry manifest/platform
mismatches, and treating a node runtime experiment as an application change.
Each is controllable with the stage gates above. The migration succeeds when
the application runs from a signed or otherwise governed digest as a
restricted, socket-free OKD workload—not when a different command happens to
launch the same container on a laptop.
