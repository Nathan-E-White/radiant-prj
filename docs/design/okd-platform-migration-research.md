# Docker, Kubernetes, and Kind to OKD, Podman, Buildah, runc, Kata Containers, and CRC/OpenShift Local

| Field | Value |
| --- | --- |
| Document ID | CONTAINER-PLATFORM-TRANSITION-RESEARCH-001 |
| Status | Research note; no implementation change |
| Updated | 2026-07-30 |
| Scope | Replace the repository's Docker-centred local/CI workflow and Kind acceptance environment with a layered Podman/Buildah, CRC local-OKD, and OKD workflow; retain a separately approved Kata isolation option. |

## Executive conclusion

The sensible target is not a different spelling of Docker. It is an explicit
separation of image construction, laptop feedback, OpenShift admission
validation, and production operation:

```text
Containerfile/Dockerfile --Buildah/Podman--> OCI image digest --> registry
       |                         |                                 |
       |                    local process                       OKD pull
       |                         |                                 |
       +-------------------------+-------------------------------+
                                 |
       Podman local         CRC OKD preset            multi-node OKD
       fast inner loop   single-node OpenShift proof   production-like target
                                                        |
                                                     CRI-O
                                                   /       \
                                      standard: runc/crun   exception: Kata RuntimeClass
```

Build with Buildah (or `podman build` where the developer experience warrants
it), publish an immutable OCI image digest, and deploy that same digest to the
cluster. Podman is a local container engine, not an OKD node runtime; Buildah
is an image builder, not a scheduler; and runc/crun are cluster-managed,
low-level OCI runtimes, not CI tools. OKD deploys CRI-O on its nodes and
documents runc or crun as its low-level runtime choices. [OKD nodes](https://docs.okd.io/latest/nodes/index.html), [OKD container runtimes](https://docs.okd.io/4.14/nodes/containers/nodes-containers-using.html), [OCI Runtime Specification](https://specs.opencontainers.org/runtime-spec/).

CRC is the community local-cluster runner. It offers distinct `openshift`
(OCP), `okd`, and `microshift` presets, of which one is active at a time.
**Red Hat OpenShift Local** is Red Hat's OCP-bundle distribution of the same
local-development idea, formerly called CodeReady Containers. For this
migration, use the **CRC OKD preset** when the intended local gate is OKD;
calling that combination “OKD-CRC” is understandable shorthand, but not a
separate product. It remains a single-node desktop development/testing
environment—not a multi-node production rehearsal and not proof that an image
will pass a particular target OKD cluster's policy, storage, networking, or
operator configuration. [CRC](https://crc.dev/), [CRC presets and use](https://crc.dev/docs/using/), [OpenShift Local overview](https://developers.redhat.com/products/openshift-local/overview).

Kata Containers should remain an exception path for a documented isolation
threat model. Kubernetes `RuntimeClass` selects a configured runtime handler;
the cluster administrator must first provide and support that handler on the
eligible nodes. A `runtimeClassName: kata` in an application manifest is not an
installation method, and CRC/OpenShift Local should not be assumed to supply one.
[Kubernetes RuntimeClass](https://kubernetes.io/docs/concepts/containers/runtime-class/), [Kata Containers](https://github.com/kata-containers/kata-containers), [OKD RuntimeClass API](https://docs.okd.io/4.8/rest_api/node_apis/runtimeclass-node-k8s-io-v1.html).

## Product and support boundary

| Environment / component | What it is | What it proves | What it must not be mistaken for |
| --- | --- | --- | --- |
| Podman + Buildah | Local container engine and image-building toolchain. Podman can build, run, inspect, and push; its build command uses Buildah. [Podman](https://docs.podman.io/en/latest/markdown/podman.1.html), [Podman build](https://docs.podman.io/en/stable/markdown/podman-build.1.html) | Recipe syntax, image contents, entrypoint, local process behaviour, and registry push/pull. | Kubernetes/OKD admission, cluster networking, SCCs, or service-account policy. |
| Kind | Kubernetes-in-Docker local cluster. [Kind quick start](https://kind.sigs.k8s.io/docs/user/quick-start/) | Kubernetes API contract and the existing Job adapter's basic lifecycle. | OpenShift APIs, SCC admission, Routes, CRI-O, cluster OAuth, or production storage/ingress. |
| CRC with the `okd` preset | Community local runner's single-node OKD environment for desktop development/testing. CRC separately supports an OCP preset, distributed by Red Hat as OpenShift Local; select the intended preset explicitly. [CRC](https://crc.dev/), [CRC use](https://crc.dev/docs/using/) | `oc`, OpenShift project/RBAC/Route/SCC-style checks against locally bundled OKD. | A supported production topology, HA/failure testing, or the actual target cluster's operator/storage/network policy. |
| OKD | The target Kubernetes platform: its nodes run CRI-O and platform-owned configuration. [OKD architecture](https://docs.okd.io/latest/architecture/architecture.html) | The real target's admission, Route/TLS, registry, storage, RBAC, NetworkPolicy and workload operation. | A developer's local engine or a generic Kubernetes distribution. |
| runc / crun | Low-level OCI runtimes selected and managed by OKD through CRI-O. [OKD container runtimes](https://docs.okd.io/4.14/nodes/containers/nodes-containers-using.html) | Standard Linux-container execution on target nodes. | An application dependency or a command that should be executed inside a Pod. |
| Kata Containers | VM-backed container isolation technology, selected through a supported runtime handler/`RuntimeClass`. [Kata Containers](https://github.com/kata-containers/kata-containers) | Only the approved workload's stronger isolation, after separate node and integration evidence. | A universal performance, compatibility, or security upgrade. |

CRC's current installation guide lists macOS 13 Ventura or newer, no nested
virtualization, and four physical CPU cores, 10.5 GB free memory, and 35 GB
disk for the OCP and OKD presets. Its OCP preset supports Apple silicon; its
OKD preset does not. That makes an Apple-silicon laptop unsuitable for the
proposed CRC-OKD gate even though it can run OpenShift Local/OCP. Pin the CRC
release and recheck this host matrix before enrolment; host requirements are
release-specific rather than a timeless law of thermodynamics. [CRC install
requirements](https://crc.dev/docs/installing/).

On macOS, Podman itself runs containers in a managed Linux VM; it therefore
needs a macOS acceptance pass for mounts, networking, port exposure, image
architecture, and credential forwarding. Podman documents that macOS and
Windows require a VM, and that `podman machine` operations are rootless.
[Podman machine](https://docs.podman.io/en/latest/markdown/podman-machine.1.html).

## Repository inventory

| Current concern | Evidence in this repository | Transition treatment |
| --- | --- | --- |
| Image recipes and Buildx/Bake packaging | `Dockerfile`, `worker.Dockerfile`, `deploy/*.Dockerfile`, `docker-bake.hcl`, and `config/docker-packaging-inputs.json` enumerate image targets. | Preserve the recipes initially. Create an explicit Buildah target map for context, stage, build arguments, platform, tag, and pushed digest. Do not pretend a file rename has changed a build system. |
| Docker Compose local integration | `docker-compose.yml` and `deploy/slurm-gateway.compose.yml`; the latter uses profiles, secrets, health checks, volumes, TCP/UDP ports, and dependency ordering. | Keep Compose as a local integration fixture, not a deployment manifest. `podman compose` delegates to an external provider, so pin and prove the provider feature matrix. [Podman compose](https://docs.podman.io/en/latest/markdown/podman-compose.1.html) |
| Docker Engine control plane | `backend/slurm-gateway/internal/simopsdocker/`; `deploy/slurm-gateway.compose.yml:85-90` runs the gateway as `0:0` and mounts `/var/run/docker.sock`. | This is the central portability and privilege debt. Move production worker launch to the Kubernetes adapter and delete the production socket requirement only after its contract is proven. |
| Existing Kubernetes path | `backend/slurm-gateway/internal/simopskubernetes/spooler.go`; `SIMOPS_WORKER_RUNTIME` selects the runtime in `backend/slurm-gateway/cmd/server/main.go`; `infra/opentofu/simops-kind-substrate/` supplies current Kind plumbing. | Use this Job path as the migration seam. It needs SCC/RBAC/Route/storage and real-OKD acceptance beyond Kind. Kubernetes Jobs are run-to-completion workloads. [Kubernetes Jobs](https://kubernetes.io/docs/concepts/workloads/controllers/job/) |
| Checks and CI | Docker Desktop scripts, Docker/OrbStack smoke scripts, Buildx packaging verification, Compose smoke, a Kind smoke, and Docker-labelled Go integration tests appear in `package.json`, `scripts/`, and `docs/verification/verification-plan.md`. | Add parallel Podman/Buildah and OpenShift acceptance evidence before deleting old gates; preserve comparable digest, lifecycle, logs, cancellation, and cleanup evidence. |

The inventory does not say every occurrence of the word “Docker” is a defect.
Compatibility fixtures and historical evidence can remain. The steady-state
production deployment, however, must not require the Docker Engine socket, a
Docker-specific runtime API, a fixed root UID, or host-level privileges.

## Target roles and security boundary

| Layer | Owner | Required role | Forbidden shortcut |
| --- | --- | --- | --- |
| Dockerfile/Containerfile | Repository | Portable build recipe; arbitrary-UID-safe image filesystem. | Making the recipe dependent on a local daemon, host socket, or fixed UID. |
| Buildah / Podman | Developer and CI | Build, inspect, tag, sign/govern as applicable, push and pull by digest. | Treating a local tag as a deployment identity. |
| Registry | Delivery platform | Stores the OCI image that clusters pull. | Mutable `latest` promotion without a recorded digest. |
| Kubernetes API | Gateway service account | Creates, observes, cancels, and removes only its labelled Jobs. | Giving ordinary workers cluster credentials or using an engine socket. |
| OKD policy | Cluster platform | SCCs, RBAC, Routes, storage, identity, image policy, and CRI-O runtime configuration. | Installing Podman/Buildah/runc manually into workload Pods or editing default SCCs. |
| runc / crun | Cluster operator | Standard OCI process isolation behind CRI-O. | Application-managed runtime selection. |
| Kata | Cluster operator plus workload owner | Optional runtime handler for a defined, eligible workload. | Setting `runtimeClassName` before handler, capacity, observability, storage, networking, and rollback have been tested. |

Use a portable Restricted-oriented baseline: non-root operation, no privilege
escalation, all Linux capabilities dropped, `RuntimeDefault` seccomp, no host
namespace/hostPath, and explicit writable volumes where needed. Kubernetes
defines the Restricted policy controls; OKD SCCs add admission controls and
should be approached with a restricted-compatible design before considering a
custom entitlement. [Kubernetes Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/), [OKD SCCs](https://docs.okd.io/latest/authentication/managing-security-context-constraints.html).

### Socket removal is a design change, not a mount change

The gateway must create Jobs through its narrowly scoped Kubernetes service
account. Normal workers should receive their gateway URL and scoped ingest
credential, but no Docker, Podman, or Kubernetes control credential. Disable
service-account token automounting where it is unnecessary. [Kubernetes service
accounts](https://kubernetes.io/docs/concepts/security/service-accounts/).

Do not substitute `/run/podman/podman.sock` for `/var/run/docker.sock` in an
OKD deployment. Podman's API service is powerful enough that Podman warns
access grants full Podman access; its Docker-compatible API is a bounded local
compatibility bridge, not a production execution model. [Podman system
service](https://docs.podman.io/en/latest/markdown/podman-system-service.1.html).

## OCI build and deployment flow

1. Keep the existing Dockerfile syntax as the initial portable recipe surface;
   Buildah's `bud` builds from a Containerfile/Dockerfile, and Podman uses
   Buildah for image builds. [Buildah build](https://github.com/containers/buildah/blob/main/docs/buildah-build.1.md), [Podman build](https://docs.podman.io/en/stable/markdown/podman-build.1.html)
2. CI builds every packaging target from a clean context, records the image
   digest and immutable metadata, and pushes it to the selected registry.
3. CI pulls the digest into a clean Podman environment and runs the existing
   unit/contract and bounded smoke checks. This detects a local-cache victory,
   which is a form of victory best enjoyed privately.
4. Deploy the digest—not merely its tag—through a portable Kubernetes base.
   Apply an OKD overlay for Routes, storage class selection, identity/groups,
   and any justified SCC entitlement. OKD documents image IDs as digest-based
   immutable identifiers. [OKD images](https://docs.okd.io/latest/openshift_images/index.html)
5. The OKD kubelet sends the Pod to CRI-O, which uses the configured low-level
   OCI runtime. The application does not select `runc` or `crun`.

## What each environment proves

| Gate | Required workload | Positive evidence | Deliberate non-claim |
| --- | --- | --- | --- |
| Podman local | Each image recipe plus selected Compose services. | Build succeeds; image runs as non-root where intended; local ports, mounted files, entrypoint, health endpoint, push/pull by digest. | Not a Kubernetes or OpenShift admission test. |
| Kind | Existing SimOps Job smoke. | The client-go adapter creates labelled Jobs, observes success/failure/cancellation, captures logs, honours retention and cleanup. | Not an OKD test: Kind's Docker-based nodes do not establish SCC, Route, CRI-O, or OpenShift API behaviour. |
| CRC with `okd` preset | One gateway deployment, one representative Job, a Service/Route, least-privilege identities. | Developer can use `oc`; project/RBAC, restricted-compatible image, Route/TLS behaviour and basic **OKD** admission are exercised locally. | Not HA or production support; not proof of the target cluster's configured storage, identity provider, network policy or Kata handler. |
| Target OKD | Same digest and portable base plus real overlay. | SCC admission, `oc auth can-i`, registry pulls, Route/TLS, DNS, storage, NetworkPolicy, Job lifecycle, metrics/logs and rollback all pass. | Not evidence that a Kata workload is ready unless the separate Kata gate passes. |
| Kata-only OKD pool | A specifically justified workload using the approved RuntimeClass. | Runtime handler resolves, nodes schedule, resources and startup cost are acceptable, storage/network/telemetry work, and fallback to standard runtime is rehearsed. | Not a reason to apply Kata to all workloads. |

## Kata decision gate

Kata adds a VM boundary and may therefore be appropriate for a workload whose
threat model needs it. It also changes resource consumption, startup behaviour,
hardware/virtualization assumptions, node pool configuration, observability,
and occasionally workload compatibility. The Kubernetes API only says that a
RuntimeClass maps a Pod to a handler and may carry scheduling/overhead
information; it does not certify a particular handler on any cluster. Red Hat's
OpenShift Sandboxed Containers documentation describes an OCP operator that
installs Kata as an optional secondary runtime, configures CRI-O handlers, and
creates the `kata` RuntimeClass. That is not a published OKD or CRC-OKD support
statement, so this note treats Kata there as unverified until the exact target
release and operator source establish support. [Kubernetes RuntimeClass](https://kubernetes.io/docs/concepts/containers/runtime-class/), [OpenShift Sandboxed Containers](https://docs.redhat.com/en/documentation/openshift_sandboxed_containers/1.5/html/openshift_sandboxed_containers_user_guide/deploying-sandboxed-containers-workloads).

| Decision question | Evidence required before approval |
| --- | --- |
| Why not standard runc/crun? | Written threat model and owner; a specific risk that the stronger VM boundary materially reduces. |
| Can this OKD cluster run Kata? | Cluster operator's supported installation/configuration evidence, an actual `RuntimeClass`, eligible labelled/tainted nodes, and version compatibility. |
| Does the workload still function? | Image pull, networking/DNS, volume mount, Route/service, non-root/SCC admission, logs/metrics, resource and cold-start tests. |
| Can it fail safely? | Scheduling-capacity test, handler-missing failure procedure, standard-runtime fallback, and documented rollback. |

Until that table is satisfied, standard CRI-O with the platform's runc/crun
selection is the baseline. This is not asceticism; it is simply declining to
make every container a small virtual-machine project for no stated reason.

## CI and developer operating model

| Area | Transitional state | End state / acceptance criterion |
| --- | --- | --- |
| Developer image loop | Docker remains available while Buildah/Podman parity is measured. | `podman build` or Buildah builds every target and a clean pull/run passes. |
| macOS | Podman machine and CRC each consume a Linux VM. | Document CPU/memory/disk allocation, mount/network behaviour, and a recovery/reset procedure; note that the CRC OKD preset does not support Apple silicon; do not run nested virtualization experiments as a daily workflow. |
| Compose | Existing Compose smoke remains authoritative until its selected external provider passes a feature matrix. | Profiles, secrets, health checks, ordering, named volumes, tmpfs/read-only mounts, TCP/UDP ports, teardown and cleanup have Linux and macOS evidence. |
| Kubernetes | Keep the Kind smoke as a fast API/lifecycle test. | Kind runs retain their limited purpose; CRC-OKD and target OKD add the OpenShift-specific gates. |
| CI delivery | Docker Buildx/Bake checks remain as comparator evidence first. | Buildah-based build/push/digest verification is mandatory; deployment tests consume the exact digest. |
| Production runtime | Docker adapter/socket path available only as explicitly bounded local compatibility. | Gateway uses Kubernetes API; no production manifest contains either Docker or Podman socket, root-for-socket identity, privileged host access, or generic worker API token. |

## Staged sequence

| Stage | Change | Exit evidence |
| --- | --- | --- |
| 0 — Baseline | Catalogue images, Bake targets, sockets, Docker API calls, Compose features, Kind substrate, security contexts, and current smoke evidence. | Every engine-coupled path has an owner and a removal/retention decision. |
| 1 — Image parity | Add Buildah build/push and clean Podman pull/run per target; retain Buildx comparator. | Context, stage, args, platform, image config, digest and test outcomes are recorded. |
| 2 — Local-stack proof | Pilot a pinned Compose provider through Podman on Linux and macOS. | Feature matrix and teardown/volume behaviour pass; Docker fallback remains until then. |
| 3 — Runtime decoupling | Make the Kubernetes Job adapter the production worker path; eliminate socket/root-for-socket requirements in target manifests. | A representative SimOps and reactor-telemetry flow completes, cancels, logs and cleans up through the Kubernetes API. |
| 4 — CRC-OKD development gate | Install the pinned CRC release and `okd` preset on eligible developer hosts; deploy a restricted-compatible gateway and Job. | `oc` project/RBAC, Route, image-pull and ordinary workload evidence is reproducible; single-node and host-platform limitations are recorded. |
| 5 — Real OKD pre-production | Apply portable base plus an OKD overlay to a non-production project. | Target SCC/RBAC, storage, registry, Route/TLS, NetworkPolicy, observability and rollback evidence passes. |
| 6 — Kata decision, if needed | Conduct the separate handler/node/workload gate. | Explicit accept or defer decision; no implied default. |
| 7 — Bounded cutover | Send one reversible workload slice to OKD and compare contracts, state and telemetry. | Error/data-integrity thresholds, rollback rehearsal and support ownership are accepted. |
| 8 — Retirement | Remove production Docker Engine coupling and obsolete gates only after sustained operation. | Repository search, manifests and CI demonstrate no production socket/Engine dependency. |

## Risk and validation table

| Risk | Failure mode | Required validation |
| --- | --- | --- |
| Local equivalence fantasy | Podman or Kind passes while OpenShift admission/Route/SCC fails. | Maintain distinct Podman, Kind, CRC-OKD and target-OKD gates; no gate substitutes for another. |
| CRC naming/platform confusion | Team uses OpenShift Local/OCP when it intended CRC-OKD, or treats either local result as production proof. | Record the exact CRC release/preset, target OKD release/configuration, and the unproven differences in every acceptance report. |
| Socket replacement | A Podman socket replaces the Docker socket, preserving the same privilege boundary failure. | Manifest/repository scan and negative test prove workers and production gateway have no engine socket. |
| Arbitrary UID / SCC | Image writes only as root or conflicts with SELinux/allocated UID. | Target OKD event/admission evidence plus runtime checks of UID and writable paths. |
| Compose drift | External provider silently differs on profiles, secrets or health ordering. | Version-pinned provider matrix on macOS and Linux; retain Docker fallback until green. |
| Digest drift | CI tests one local tag and deploys another image. | Registry digest recorded at build and asserted in Pod/Job status. |
| Kata optimism | RuntimeClass exists but handler/nodes/volumes/telemetry are absent or unsuitable. | Separate Kata gate, capacity measurements and standard-runtime rollback rehearsal. |
| Local VM resource contention | Two Linux VMs, laptop heat, memory pressure, or nested virtualization limits invalidate local tests. | Host prereq check; documented VM allocation; treat insufficient host capacity as a move to remote non-production OKD, not a networking puzzle. |

## Recommended first deliverable

Create a non-production OKD deployment overlay and one repeatable acceptance
procedure before changing every local command. It should deploy the gateway
and one representative worker image by digest, use least-privilege service
accounts, prove restricted-compatible SCC admission, expose a Route, verify
storage/network flows, and exercise Job cancellation and cleanup. In parallel,
add Buildah digest parity for the existing image targets and maintain Kind as a
fast Kubernetes adapter check. CRC with its OKD preset is then an accessible
developer gate—not a shadow production cluster—and Kata remains out of the critical path
unless a workload earns its additional complexity.
