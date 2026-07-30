# Tekton Delivery-Layer Migration Research

| Field | Value |
| --- | --- |
| Document ID | TEKTON-DELIVERY-MIGRATION-RESEARCH-001 |
| Status | Research note; no implementation change |
| Updated | 2026-07-30 |
| Scope | Add upstream Tekton as a CI/CD layer to the repository's Docker/Kind to Podman/Buildah, CRC-OKD, and OKD transition. |

## Conclusion

Tekton can supply the missing *delivery* layer: it can clone, test, build,
scan, publish, attest, and promote an OCI image digest. It must not become the
application's scheduler. The gateway's existing Kubernetes Job path remains
responsible for user-requested SimOps work; Tekton runs only finite delivery
work triggered by source-control or a deliberate release action.

```text
commit / release request
          |
  Tekton PipelineRun (CI namespace)
  clone -> test -> Buildah build/push -> digest -> attest -> deploy/test
          |                                      |
          +-- Results: durable run evidence      +-- immutable registry identity
                                                     |
                                          CRC-OKD gate / target OKD
                                                     |
                                        application Deployment + Jobs
```

Use **upstream Tekton Pipelines** only after the target cluster operator has
approved its installation and version set. Pipelines is a Kubernetes extension
whose Tasks and PipelineRuns are CRDs; it is not built into Kubernetes or OKD.
[Tekton Pipelines](https://tekton.dev/docs/pipelines/) Red Hat OpenShift
Pipelines is a separately packaged product based on Tekton and has its own
compatibility and support matrix. Its documentation is not evidence that a
given upstream Tekton release is supported on OKD. [Red Hat OpenShift
Pipelines overview](https://docs.redhat.com/en/documentation/red_hat_openshift_pipelines/1.14/html/about_openshift_pipelines/index)

The recommended initial design is intentionally small: Pipelines plus one
repository-owned Pipeline, explicit Task images pinned by digest, a Buildah
build/push Task, registry-digest hand-off, and a short retention policy.
Add Triggers or Pipelines-as-Code (PAC), Chains, and Results only as each has a
named operational owner and an acceptance test. A collection of CRDs is not,
by itself, a release process.

## How Tekton fits the platform layers

| Layer | Responsibility | Boundary |
| --- | --- | --- |
| Podman + Buildah on developer/CI hosts | Fast local image parity, direct build and inspection. | Does not prove OKD admission or operate cluster workloads. |
| Upstream Tekton Pipelines | Cluster-side, finite delivery workflow: source, tests, build, publish, deployment verification. A Task executes its ordered Steps in a Pod. [Tasks](https://tekton.dev/docs/pipelines/tasks/) | Does not own long-running services or customer/SIMOPS job scheduling. |
| Registry | Stores an OCI image and returns the content digest used for promotion. | A mutable tag is a discovery convenience, not a promotion identity. |
| CRC with the `okd` preset | Local, single-node OpenShift-compatible delivery/admission experiment. | Not a target-cluster support or production-topology claim. [CRC use](https://crc.dev/docs/using/) |
| Target OKD | Applies the approved digest under its own RBAC, SCC, storage, network, and runtime policy. | Does not imply support for upstream Tekton, Kata, or any particular builder until independently established. |
| CRI-O with runc/crun; optional Kata RuntimeClass | Cluster-owned application execution. | A Tekton Task does not select or install a node runtime. A `runtimeClassName: kata` requires an existing, supported handler. [Kubernetes RuntimeClass](https://kubernetes.io/docs/concepts/containers/runtime-class/) |

## Pipeline model and repository seam

Tekton's useful primitives map cleanly to this repository's existing Docker
packaging and Kind smoke work:

| Tekton primitive | Recommended use | Evidence / constraint |
| --- | --- | --- |
| `Task` / `TaskRun` | A reusable, bounded step such as Go/Rust test, manifest validation, Buildah build-push, or digest deployment check. | Tasks are namespaced; steps run sequentially in a Pod. [Tasks](https://tekton.dev/docs/pipelines/tasks/) |
| `Pipeline` / `PipelineRun` | The ordered release unit: clone, test, build, scan, push, deploy digest, verify, notify. | A PipelineRun instantiates the Pipeline with concrete inputs/outputs. [Pipelines](https://tekton.dev/docs/pipelines/) |
| Params and Results | Pass immutable inputs (commit, target, registry repository) and output the image digest from build to deploy. | Task results may be consumed by later Tasks. [Tasks](https://tekton.dev/docs/pipelines/tasks/) |
| Workspaces | Source checkout and, if measured necessary, a build cache or shared artifact area. | Bind a specific volume in the PipelineRun; prefer one writable Workspace per Task. [Workspaces](https://tekton.dev/docs/pipelines/workspaces/) |
| `finally` tasks | Publish run status and clean deliberately scoped temporary state. | `finally` runs before Pipeline exit whether earlier work passes or fails. [Pipeline API](https://tekton.dev/docs/pipelines/pipeline-api/) |

Repository evidence shows Docker Buildx/Bake packaging in
`docker-bake.hcl` and GitHub Actions, alongside `simops:smoke:kind` and the
Kubernetes worker adapter. The first Tekton Pipeline should reproduce one
small image target and its existing test evidence, rather than translate every
Bake target, Compose service, and runtime adapter at once. Build target maps
must continue to capture context, stage, build arguments, architecture, image
configuration, digest, size, and behavioral probes.

### Workspace policy

Use `emptyDir` for disposable source checkout where a single Task can complete
the work. Use a per-PipelineRun `volumeClaimTemplate` only when multiple Tasks
must share files or a measured cache makes the storage complexity worthwhile.
PVC-backed Workspaces introduce access-mode and scheduling constraints; Tekton
documents an Affinity Assistant for shared PVC scheduling. [Workspaces](https://tekton.dev/docs/pipelines/workspaces/)

Do not put registry credentials, signing keys, or general-purpose shared
service-account tokens into a source Workspace. Prefer dedicated Secrets bound
to the least-privilege ServiceAccount; isolate any sensitive Workspace to the
Steps that need it where the selected Tekton version supports that feature.
[Tekton authentication](https://tekton.dev/docs/pipelines/auth/)

## Event entry: explicit run, Triggers, or PAC

| Choice | Use when | Cost / decision |
| --- | --- | --- |
| Explicit `PipelineRun` from GitHub Actions or a release operator | Start with one controlled integration and clear audit ownership. | Fewer components and a narrower webhook surface. Recommended first phase. |
| Tekton Triggers | The platform team needs generic event handling on-cluster. An EventListener, bindings, templates, and interceptors turn events into TaskRuns/PipelineRuns. [Triggers](https://tekton.dev/docs/triggers/) | Operates an inbound endpoint and webhook verification path. Add only with ingress, secret rotation, replay protection, and ownership. |
| Pipelines-as-Code | The team wants pipeline definitions in `.tekton/` beside reviewed application changes, with Git-provider integration and PR status. [PAC getting started](https://tekton.dev/docs/getting-started/pipelines-as-code/) | More convenient Git-native workflow, but adds a controller/provider integration and policy model. Evaluate after a stable Pipeline contract exists. |

Triggers and PAC solve different problems. Triggers is the lower-level generic
event framework. PAC supplies source-controlled pipeline discovery and
provider-facing workflow. Do not install both merely to make a diagram look
well supplied with arrows.

## Build and digest-promotion design

Build images with a Buildah Task only if the target cluster operator explicitly
approves the builder image, SCC/security context, storage, registry access,
resource budget, and cache strategy. Buildah is a builder; putting it in a
PipelineRun does not exempt it from cluster policy. Its official documentation
defines `buildah build`/`bud` as a Dockerfile/Containerfile build interface.
[Buildah build](https://github.com/containers/buildah/blob/main/docs/buildah-build.1.md)

The safer phased default is to build with Buildah on the existing external CI
runner, publish the digest, and use Tekton first for digest-based deployment
verification on CRC-OKD/OKD. This separates migration risk: a failed
in-cluster builder does not block the proof that target admission and delivery
work.

When in-cluster building is approved, use this contract:

1. Resolve the source revision and builder/Task images by immutable reference.
2. Build one OCI image target and push it to the approved registry.
3. Capture the registry-returned digest in a Task Result; do not reconstruct it
   from a tag.
4. Deploy `repository@sha256:...` to the CRC-OKD gate or target OKD project.
5. Run bounded health, Route/TLS, SCC/RBAC, and representative Job lifecycle
   checks against that same digest.
6. Promote the recorded digest between environments; tags may point to it for
   human convenience but must not select it for deployment.

OKD describes image IDs as digest-based immutable identifiers. [OKD image
management](https://docs.okd.io/latest/openshift_images/index.html) Buildah's
own rootless/container-storage behavior and any requirement for additional
privileges must be tested on the exact selected builder image and SCC. A
privileged build pod, Docker socket, Podman socket, or hostPath is not an
acceptable shortcut: it recreates the control-plane boundary this migration is
meant to remove.

## Security, identity, and runtime boundary

| Subject | Required posture | Validation |
| --- | --- | --- |
| Tekton controller installation | Platform-owned namespace, pinned versions, reviewed CRDs/controller RBAC, update and removal plan. | Cluster operator documents approval; controller health and namespace scope are recorded. |
| Pipeline service account | One delivery SA per environment; only required Git, registry, and deployment permissions. Separate build from deploy identity if possible. | `oc auth can-i` evidence; no broad cluster-admin binding. |
| Credentials | Git and registry Secrets attached only to the relevant SA; image-pull credentials separate from runtime credentials. | Tekton treats supported annotated Secrets as credentials for Run Pods; private step images require normal image-pull configuration. [Authentication](https://tekton.dev/docs/pipelines/auth/) |
| SCC / pod policy | Restricted-compatible builder/test/deploy Tasks: arbitrary UID-safe images, no escalation, dropped capabilities, RuntimeDefault seccomp, no host networking/PID/IPC or hostPath. | Target OKD admission test. SCCs govern pod security and should not be altered by editing defaults. [OKD SCCs](https://docs.okd.io/latest/authentication/managing-security-context-constraints.html) |
| Kata | Never a default for delivery Pods. | Use only if an approved target handler and threat model require it; this note makes no OKD availability claim. |
| Application workers | Gateway creates labelled Kubernetes Jobs through its narrow runtime SA. | Tekton cannot create or manage arbitrary customer work as a side effect of CI. |

Tekton PodTemplates expose settings such as `securityContext`, node placement,
`runtimeClassName`, image-pull secrets, and service-account-token automounting.
That power is a reason to keep Task definitions reviewed and repository-owned,
not a reason to grant every pull request a general cluster API identity.
[PodTemplates](https://tekton.dev/docs/pipelines/podtemplates/)

## Provenance, results, and retention

Tekton Chains observes completed TaskRuns and PipelineRuns, snapshots them,
formats/signs the payload, and stores the configured result. It can produce
signed OCI-image material and SLSA provenance; it needs Pipelines first.
[Tekton Chains](https://tekton.dev/docs/chains/), [Chains provenance](https://tekton.dev/docs/chains/slsa-provenance/)

Adopt Chains only after choosing the signing model (keyless/KMS/key-backed),
storage backend, verification consumer, key custody owner, and retention
period. A signature that no admission or release process verifies is evidence,
not enforcement. Avoid simultaneously publishing two competing task- and
pipeline-level attestations for one image unless their distinct purposes are
documented; Chains warns that this is confusing. [Chains provenance](https://tekton.dev/docs/chains/slsa-provenance/)

Tekton Results is a separate long-term record store: its watcher reports Run
updates to a persistent Results API, allowing completed Run CRDs to be removed
after storage. It also provides a retention-policy agent and can retain logs.
[Tekton Results](https://tekton.dev/docs/results/) It is optional in the first
pilot. Define owner, storage durability, access controls, export requirements,
log redaction, and the exact CRD/log retention rules before enabling cleanup.

## Local CRC-OKD, target OKD, and external CI

| Gate | Recommended scope | It does not establish |
| --- | --- | --- |
| External CI runner | Existing unit, contract, Buildah build/push, digest capture, and repository checks. GitHub Actions remains a valid entry point while Tekton is piloted. | OKD SCC, Route, target storage/network policy, or production support. |
| CRC `okd` preset | Manual or narrowly invoked PipelineRun with a deployment-verification Task. Validate `oc` access, restricted-compatible Pods, pull by digest, Service/Route behavior, and cleanup. | High availability, multi-node capacity, target identity/network/storage policy, or upstream Tekton support on target OKD. |
| Target OKD | A designated non-production project first. Use the exact promoted digest and constrained service accounts. | Approval to install a component in production without platform-owner change control. |

CRC is a desktop local environment. It is useful for catching an OpenShift API
and admission difference after the Kind smoke, but it is not a credible home
for a persistent shared delivery platform. Keep Kind as the quick Kubernetes
Job lifecycle test until its role is superseded by real target-OKD evidence.

## Staged adoption

| Stage | Change | Exit evidence |
| --- | --- | --- |
| 0. Decision record | Choose upstream Tekton versus the separately packaged OpenShift Pipelines product; obtain operator approval for the selected environment/version. | Recorded product/version, namespace, operators, support boundary, upgrade and rollback owner. |
| 1. Pipeline pilot | Install only Pipelines in a non-production environment; run a manually created PipelineRun that performs repository tests without registry push. | Restricted admission, resource/timeout limits, logs, cancellation, cleanup, and SA/RBAC evidence. |
| 2. Digest proof | Build one image externally with Buildah, push it, pass its digest to Tekton, and deploy/verify that digest in CRC-OKD then a target-OKD project. | Digest matches registry and deployment; Route/TLS, SCC/RBAC, logs, rollback, and representative application Job checks pass. |
| 3. In-cluster builder decision | Pilot one Buildah Task only if policy permits; otherwise retain external builder. | No privileged/socket/hostPath exception; reproducible digest and cache/resource evidence. |
| 4. Event decision | Add either explicit CI invocation, Triggers, or PAC based on the table above. | Authenticated/replay-resistant event path; review and cancellation audit trail. |
| 5. Supply-chain evidence | Add Chains, then Results if durable history and CRD cleanup justify it. | Attestation verification is exercised; records/log retention and deletion are proven. |
| 6. Expansion | Migrate remaining image targets after each target has parity evidence. | Existing Bake/Buildx comparator can be retired only after target-map, digest, size, behavior, and rollback evidence are complete. |

## Decision and risk register

| Decision / risk | Recommendation | Required proof |
| --- | --- | --- |
| Upstream Tekton availability on OKD | Treat as unverified until the target operator approves a release and installation approach. | Primary project release/install documentation plus target-cluster owner approval; do not infer it from OCP documentation. |
| Product choice | Decide separately between upstream Tekton and Red Hat OpenShift Pipelines. | Version/support matrix and upgrade ownership. |
| Build placement | External Buildah first; in-cluster Buildah only after a security and cost gate. | SCC, RBAC, storage, registry, resource, cache, and repeatable digest evidence. |
| Promotion identity | Registry digest only. | Deployment manifest/status resolves to the recorded digest. |
| Webhook attack surface | No Triggers/PAC until authentication, ingress, secret rotation, replay, and ownership are designed. | Negative tests for forged/replayed/untrusted events. |
| Secret leakage | Separate SAs and credential scopes; no credential-bearing source Workspace. | Namespace review and logs/artifact redaction test. |
| PVC/cache coupling | Default to `emptyDir`; introduce PVC only with measured benefit. | Concurrent/PVC scheduling, cleanup, and storage-cost evidence. |
| Tekton confused with application runtime | Keep SimOps/reaction worker Jobs under the gateway runtime adapter. | CI cannot start unbounded customer work; application cancellation and retention remain intact. |
| Kata scope creep | Exclude it from the delivery default. | Separate, target-supported RuntimeClass and threat-model decision. |

The migration is complete when delivery produces a recorded, verifiable OCI
digest and OKD deploys that exact digest through its normal policy boundary.
It is not complete when a pipeline happens to run a shell script inside a
cluster.
