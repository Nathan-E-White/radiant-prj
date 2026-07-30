# Kubernetes-to-OKD Migration Research

| Field | Value |
| --- | --- |
| Document ID | K8S-OKD-MIGRATION-RESEARCH-001 |
| Status | Research note |
| Scope | Moving this repository's Kubernetes-backed SimOps runtime and supporting application workloads to OKD; no implementation change |
| Last updated | 2026-07-30 |

## Conclusion

The Kubernetes workload model is largely portable to OKD: Deployments, Jobs, Services, ConfigMaps, Secrets, PersistentVolumeClaims, ServiceAccounts, RBAC, and NetworkPolicies remain Kubernetes APIs. The migration is therefore chiefly an **admission, platform-integration, and operating-model** exercise, not a wholesale manifest rewrite. The material compatibility gates are Security Context Constraints (SCCs), Route/Ingress ownership and TLS, available StorageClasses and access modes, identity/RBAC mapping, and operator lifecycle policy. Sources: [Kubernetes API overview](https://kubernetes.io/docs/reference/using-api/), [OKD architecture](https://docs.okd.io/latest/architecture/architecture.html), [OKD SCCs](https://docs.okd.io/latest/authentication/managing-security-context-constraints.html).

For Radiant, the current Kubernetes adapter is a useful starting point rather than migration proof: it already creates namespace-scoped `batch/v1` Jobs using a service account, but the Kind-oriented substrate must be translated and tested against OKD admission, cluster DNS/Routes, persistent storage, image pull policy, and the production identity boundary. The repository's own adapter and substrate are the local baseline: `backend/slurm-gateway/internal/simopskubernetes/`, `infra/opentofu/simops-kind-substrate/`, and `deploy/slurm-gateway.compose.yml`.

This note uses only upstream Kubernetes, OKD/OpenShift, and Red Hat documentation, plus the local repository for the stated baseline. It is not a supportability statement: OKD is the community distribution, while Red Hat's supported product has separate lifecycle and support terms.

## Source set

Primary platform sources: [OKD architecture](https://docs.okd.io/latest/architecture/index.html), [OKD control plane](https://docs.okd.io/latest/architecture/control-plane.html), [OKD node operating system and CRI-O](https://docs.okd.io/latest/architecture/architecture-rhcos.html), [OKD SCCs](https://docs.okd.io/latest/authentication/managing-security-context-constraints.html), [OKD Routes and Ingress](https://docs.okd.io/latest/networking/routes/route-configuration.html), [OKD storage](https://docs.okd.io/latest/storage/understanding-persistent-storage.html), [OKD authentication](https://docs.okd.io/latest/authentication/understanding-authentication.html), [OKD service accounts](https://docs.okd.io/latest/authentication/understanding-and-creating-service-accounts.html), [OKD OLM](https://docs.okd.io/latest/operators/understanding/olm/olm-understanding-olm.html), and [Kubernetes documentation](https://kubernetes.io/docs/home/). Links in the body attach the relevant source to each material claim.

## Compatibility and target architecture

OKD is Kubernetes with additional platform APIs and operators. Keep portable Kubernetes resources as the application contract; isolate OKD-specific objects such as `route.openshift.io/v1` Routes and `security.openshift.io/v1` SCC policy in an environment overlay. This makes Kubernetes a tested portability baseline and OKD a first-class deployment target, rather than relying on the rather optimistic theory that any YAML accepted by one cluster will be welcomed by every other cluster. Sources: [Kubernetes API overview](https://kubernetes.io/docs/reference/using-api/), [OKD Operators](https://docs.okd.io/latest/operators/understanding/olm/olm-understanding-olm.html), [OKD Routes](https://docs.okd.io/latest/networking/routes/route-configuration.html).

Recommended boundary:

```text
portable base: Namespace, ServiceAccount, RBAC, ConfigMap/Secret,
               Deployment/StatefulSet/Job, Service, PVC, NetworkPolicy
OKD overlay:  Route, SCC entitlement (only if justified), Image/registry policy,
               StorageClass selection, Operator subscriptions, cluster OAuth groups
```

Do not migrate cluster infrastructure or a per-run Job into an Operator merely because OLM exists. A native `batch/v1` Job remains the appropriate primitive for a finite worker run; Kubernetes documents Jobs as workloads that run to completion and create Pods. Operators add an operational dependency and must be selected because they own a genuinely long-lived domain concern, not to make the architecture look more expensive. Sources: [Kubernetes Jobs](https://kubernetes.io/docs/concepts/workloads/controllers/job/), [OKD OLM concepts](https://docs.okd.io/latest/operators/understanding/olm/olm-understanding-olm.html).

## Workload manifest and image readiness

Start by inventorying every workload's API version, image, command, ports, service account, volume, security context, and external dependency. Kubernetes removes and deprecates APIs by release; validate the actual target version with the API resources available on that cluster rather than treating an old manifest as a family heirloom. Sources: [Kubernetes deprecated API migration guide](https://kubernetes.io/docs/reference/using-api/deprecation-guide/), [OKD API compatibility](https://docs.okd.io/latest/updating/updating_a_cluster/understanding-upgrade-channels-releases.html).

For each application container, make it runnable under an arbitrary non-root UID. Avoid a fixed `runAsUser` unless the application truly requires it; use writable mounted directories where needed, avoid `hostPath`, host networking, privileged containers, and a Docker socket mount. Kubernetes advises running Linux workloads as non-root and avoiding privileged mode; OKD's restricted SCC enforces a namespace-allocated UID and SELinux context and disallows privileged, host-directory, host-network, host-port, and host-PID use. Sources: [Kubernetes Linux security constraints](https://kubernetes.io/docs/concepts/security/linux-kernel-security-constraints/), [OKD SCCs](https://docs.okd.io/latest/authentication/managing-security-context-constraints.html).

Use a portable restricted security baseline and test it before the cutover:

```yaml
spec:
  securityContext:
    runAsNonRoot: true
    seccompProfile: {type: RuntimeDefault}
  containers:
  - name: app
    securityContext:
      allowPrivilegeEscalation: false
      capabilities: {drop: ["ALL"]}
```

This is a starting point, not a magical talisman: it must be reconciled with the image's filesystem ownership and its required volumes. The Kubernetes Restricted Pod Security Standard requires non-root execution, prohibits privilege escalation, and requires dropping `ALL` capabilities; it also defines allowed volume types. Sources: [Kubernetes Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/), [Kubernetes security context configuration](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/).

For Radiant specifically, remove the gateway's development Docker-socket dependency from the OKD deployment design. The existing compose file mounts `/var/run/docker.sock`, but the Kubernetes adapter calls the Kubernetes API to create Jobs; workers should retain the repository's gateway-ingest-only credential contract and should not receive cluster, Docker, or data-plane credentials. Local sources: `deploy/slurm-gateway.compose.yml:90`, `backend/slurm-gateway/internal/simopskubernetes/spooler.go`, `docs/adr/adr-0007.md`.

## Security, SCCs, identity, and RBAC

OKD applies SCC admission in addition to ordinary Kubernetes security controls. SCCs govern privileged execution, privilege escalation, capabilities, host directories, SELinux, UID/GID, host namespaces/networking, volume types, and seccomp. Do not edit default SCCs: OKD documents that upgrades can reset them; create a custom SCC only after a restricted-compatible redesign has been shown impossible. Sources: [OKD SCC management](https://docs.okd.io/latest/authentication/managing-security-context-constraints.html).

Grant a custom SCC narrowly through RBAC's `use` verb to the particular service account and namespace; do not bind broad users or a whole project to `anyuid` or `privileged`. The OKD guidance explicitly supports SCC access through RBAC and notes that direct SCC subject assignment has cluster-wide scope. Sources: [OKD SCC RBAC](https://docs.okd.io/latest/authentication/managing-security-context-constraints.html), [Kubernetes RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/).

Model OKD projects as application namespaces, with separate service accounts for gateway, worker, CI deployer, and operator controllers. Give the gateway only the verbs it needs to create/get/list/watch/delete its labeled Jobs and Pods in its target namespace; ordinary workers should normally have no Kubernetes API token. Kubernetes service-account credentials are intended for in-cluster API access, and the Pod API allows disabling service-account-token automounting. Sources: [Kubernetes service accounts](https://kubernetes.io/docs/concepts/security/service-accounts/), [Kubernetes Pod API](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/).

OKD authenticates users through its OAuth server and configured identity providers; map existing human and CI identities to groups, then bind those groups to project-scoped Roles/RoleBindings where possible. Treat migration of identity provider configuration as a platform change with an access rollback plan; a deployment that has been perfectly ported but cannot be administered is merely an exhibit. Sources: [OKD authentication](https://docs.okd.io/latest/authentication/understanding-authentication.html), [OKD identity providers](https://docs.okd.io/latest/authentication/understanding-identity-provider.html), [OKD RBAC](https://docs.okd.io/latest/authentication/using-rbac.html).

Audit CI and automation for durable service-account token Secrets. OKD no longer auto-creates long-lived service-account token Secrets in current releases; use bounded service-account tokens or the TokenRequest flow, and separately verify registry pull credentials and any external registry policy. Sources: [OKD service accounts](https://docs.okd.io/latest/authentication/understanding-and-creating-service-accounts.html), [Kubernetes TokenRequest](https://kubernetes.io/docs/reference/kubernetes-api/authentication-resources/token-request-v1/).

## Networking, Ingress, and Routes

Keep internal traffic on Kubernetes Services and DNS names. For public HTTP(S) endpoints, choose one exposure model per hostname: use an OKD Route for normal platform ingress, or retain Kubernetes `Ingress` only where the target cluster's Ingress implementation and ownership model have been explicitly verified. Routes are an OKD API managed through the Ingress Operator and support host, TLS, and service targeting; they are not a mechanical synonym for every Ingress-controller annotation. Sources: [OKD Route configuration](https://docs.okd.io/latest/networking/routes/route-configuration.html), [OKD Ingress Operator](https://docs.okd.io/latest/networking/networking_operators/ingress-operator.html), [Kubernetes Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/).

Inventory each current Ingress annotation and translate it deliberately: TLS termination mode, redirects, timeouts, session affinity, WebSocket/streaming behavior, client-IP expectations, and certificates. Verify the chosen route hostname is within the cluster's ingress domain and decide whether the platform default certificate or a managed custom certificate is required. Sources: [OKD Route TLS configuration](https://docs.okd.io/latest/networking/routes/route-configuration.html), [OKD default ingress certificates](https://docs.okd.io/latest/security/certificates/replacing-default-ingress-certificate.html).

Retain NetworkPolicies as Kubernetes resources, but test the CNI's actual enforcement and all required flows: router-to-service, gateway-to-workers, workers-to-gateway ingest, metrics, DNS, storage, and external data planes. A default-deny policy without explicit DNS and platform allowances is a reliable way to demonstrate the importance of pre-production testing. Sources: [Kubernetes NetworkPolicy](https://kubernetes.io/docs/concepts/services-networking/network-policies/), [OKD network policy APIs](https://docs.okd.io/latest/networking/network_security/network-policy.html).

## Storage and state migration

PersistentVolumeClaims are portable API objects, but provisioning behavior is supplied by the cluster's StorageClasses and CSI drivers. Catalogue every PVC's requested size, access mode, volume mode, reclaim expectation, backup method, topology requirement, and performance expectation; map each to an OKD StorageClass that actually exists in the target cluster. Dynamic provisioning selects a StorageClass and produces a PV when a PVC requests it, while access modes and expansion support are driver-dependent. Sources: [Kubernetes persistent volumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/), [OKD persistent storage](https://docs.okd.io/latest/storage/understanding-persistent-storage.html).

Do not assume that a development `hostPath`, local-path provisioner, or node-local volume has a production equivalent. Use a supported CSI-backed class and perform an application-consistent data migration: quiesce or replicate the source, restore or synchronize to target storage, verify checksums/record counts, and only then cut over writers. Kubernetes documents that PV lifecycle and reclaim policy are storage-class concerns; it does not promise that bytes quietly migrate themselves while everyone is looking elsewhere. Sources: [Kubernetes PV lifecycle](https://kubernetes.io/docs/concepts/storage/persistent-volumes/), [OKD storage overview](https://docs.okd.io/latest/storage/index.html).

For databases, object stores, and Kafka-compatible data planes currently represented in the compose environment, prefer their vendor-supported replication/backup procedures over a PVC-only copy. A PVC is a storage attachment abstraction, not a consistency protocol. Local source: `deploy/slurm-gateway.compose.yml`.

## Operators, delivery, and runtime differences

OLM manages Operators through catalogs, subscriptions, install plans, CSVs, and OperatorGroups. Before adopting an Operator in the target cluster, record its source catalog, channel, version policy, namespace scope, required CRDs, required SCC/RBAC, backing storage, upgrade behavior, and removal plan. Pin or approve upgrades according to change control rather than allowing production dependencies to drift because the subscription was feeling adventurous. Sources: [OKD OLM concepts and resources](https://docs.okd.io/latest/operators/understanding/olm/olm-understanding-olm.html), [OKD Operator installation](https://docs.okd.io/latest/operators/admin/olm-adding-operators-to-cluster.html).

Keep CI/CD capable of applying the portable base plus the OKD overlay with `oc` or `kubectl` compatible calls, but make promotion gates query the actual cluster: API availability, SCC admission, RBAC authorization, image pull, rollout/job completion, Route reachability, and observability. `oc` supports Kubernetes-compatible commands and OKD-specific operations; it is appropriate to use both where the resource dictates. Sources: [OKD CLI overview](https://docs.okd.io/latest/cli_reference/openshift_cli/getting-started-cli.html), [Kubernetes deployment rollout](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/).

Move image builds outside the production cluster unless an OKD-native build workflow is an explicit requirement. If OpenShift Pipelines is chosen, it is Tekton-based and is an operator-managed platform capability, so treat it as an intentional delivery-system migration with its own RBAC, workspace, registry, and supply-chain design—not a side effect of changing the scheduler. Sources: [OpenShift Pipelines overview](https://docs.redhat.com/en/documentation/red_hat_openshift_pipelines/latest/html/about_openshift_pipelines/about-pipelines), [Tekton Pipeline](https://tekton.dev/docs/pipelines/).

The target runtime is CRI-O rather than Docker Engine, on an OS managed as part of the platform rather than as a general-purpose Docker host. Application images remain OCI/container images, but workloads must not depend on a Docker daemon/socket or Docker-specific runtime control surface in a Pod, and node customization belongs in the platform's controlled machine-configuration process. For Radiant, Kubernetes Job creation is already via `client-go`, which fits this direction; the compose Docker path remains a separate local-development runtime. Sources: [OKD node operating system and CRI-O](https://docs.okd.io/latest/architecture/architecture-rhcos.html), [OKD CRI-O troubleshooting](https://docs.okd.io/latest/support/troubleshooting/troubleshooting-crio-issues.html), [Kubernetes container runtimes](https://kubernetes.io/docs/setup/production-environment/container-runtimes/), local `backend/slurm-gateway/internal/simopskubernetes/spooler.go`.

## Staged migration plan

1. **Discover and classify.** Produce an inventory of all manifests, Helm/Kustomize inputs, images, namespaces, permissions, ingress rules, PVCs, secrets, external endpoints, and operational dependencies. Classify each object as portable, OKD overlay, or requires redesign. Validate target OKD version/API availability. Sources: [Kubernetes API deprecation guide](https://kubernetes.io/docs/reference/using-api/deprecation-guide/), [OKD API compatibility](https://docs.okd.io/latest/updating/updating_a_cluster/understanding-upgrade-channels-releases.html).

2. **Make workloads restricted-compatible.** Rebuild images to support arbitrary non-root UIDs; remove Docker socket/host access; add explicit security context; disable service-account automounting for workloads that do not call the API. Test manifests in a non-production OKD project before asking for a custom SCC. Sources: [Kubernetes Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/), [OKD SCCs](https://docs.okd.io/latest/authentication/managing-security-context-constraints.html).

3. **Provision the substrate.** Create projects, service accounts, least-privilege Roles/RoleBindings, ResourceQuota/LimitRange as required, StorageClasses/PVCs, registry access, NetworkPolicies, and an initial Route. For Radiant, translate `infra/opentofu/simops-kind-substrate/` into an OKD-targeted substrate; keep runtime Job creation in the adapter rather than infrastructure provisioning. Sources: [OKD projects](https://docs.okd.io/latest/applications/projects/working-with-projects.html), [Kubernetes RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/), [Kubernetes Jobs](https://kubernetes.io/docs/concepts/workloads/controllers/job/).

4. **Deploy a shadow environment.** Apply the portable base and OKD overlay, deploy the gateway and a representative worker Job, and run read-only or duplicate traffic where business rules permit. Do not migrate persistent writers until storage and data-plane consistency checks succeed. Sources: [Kubernetes Jobs](https://kubernetes.io/docs/concepts/workloads/controllers/job/), [Kubernetes PVs](https://kubernetes.io/docs/concepts/storage/persistent-volumes/).

5. **Migrate state and cut over.** Take an application-consistent backup or use a product-supported replication method; restore/synchronize to target storage; validate data; shift Route/DNS traffic in a controlled window; observe error rate, latency, Jobs, and storage health; retain the source environment read-only until acceptance criteria and rollback window close. Sources: [Kubernetes PV lifecycle](https://kubernetes.io/docs/concepts/storage/persistent-volumes/), [OKD Routes](https://docs.okd.io/latest/networking/routes/route-configuration.html).

6. **Decommission deliberately.** Revoke old credentials and registry access, preserve needed audit/evidence and backups, remove old DNS/ingress only after rollback expiry, and retire old resources according to their reclaim policy. Sources: [Kubernetes PV reclaim policy](https://kubernetes.io/docs/concepts/storage/persistent-volumes/), [OKD RBAC](https://docs.okd.io/latest/authentication/using-rbac.html).

## Principal risks and validation evidence

| Risk | Failure mode | Required evidence before cutover |
| --- | --- | --- |
| SCC admission | Pod rejected or application fails on arbitrary UID/SELinux-labelled volume | `oc apply`/events show restricted admission; process UID, writable paths, and volume ownership verified; no unjustified privileged SCC. [OKD SCCs](https://docs.okd.io/latest/authentication/managing-security-context-constraints.html) |
| Runtime coupling | Docker socket or Engine API assumed in a CRI-O cluster | Search/deployment review shows no production socket mount; gateway creates a test Job via Kubernetes API; worker requires no cluster token. [OKD CRI-O](https://docs.okd.io/latest/support/troubleshooting/troubleshooting-crio-issues.html) |
| Route/TLS | Hostname, TLS termination, streaming, or proxy behavior differs | External and in-cluster probes, certificate validation, and representative protocol tests pass against the Route. [OKD Routes](https://docs.okd.io/latest/networking/routes/route-configuration.html) |
| NetworkPolicy | DNS, ingress-router, telemetry, or data-plane flow is blocked | Default-deny test plus explicitly allowed flow matrix and observed connectivity results. [Kubernetes NetworkPolicy](https://kubernetes.io/docs/concepts/services-networking/network-policies/) |
| Storage | Unsupported access mode/performance or inconsistent data copy | Target StorageClass/PVC bind and mount tests; restore/replication evidence; application-level counts/checksums and performance test. [OKD storage](https://docs.okd.io/latest/storage/understanding-persistent-storage.html) |
| Identity/RBAC | CI or operators lose access, or service account is overprivileged | Identity/group mapping, `oc auth can-i` matrix, and negative authorization tests for gateway, worker, deployer, and operator accounts. [OKD RBAC](https://docs.okd.io/latest/authentication/using-rbac.html) |
| Operators | Catalog/version upgrade changes CRDs, permissions, or behavior | Recorded Subscription/CSV/InstallPlan, pinned/approved version policy, upgrade and rollback rehearsal. [OKD OLM](https://docs.okd.io/latest/operators/understanding/olm/olm-understanding-olm.html) |
| Cutover | DNS/traffic shift masks a latent dependency or damages state | Timed rollback plan, observability dashboard, business acceptance checks, and source retained read-only until sign-off. [OKD Routes](https://docs.okd.io/latest/networking/routes/route-configuration.html) |

## Recommended first deliverable for this repository

Create an OKD deployment overlay and a non-production acceptance procedure before changing application logic. The overlay should include a project, least-privilege gateway/worker service accounts and RBAC, restricted-compatible Deployment/Job security contexts, storage-class parameters, NetworkPolicies, and a Route for each public service. The procedure should run one ordinary SimOps Job, prove gateway-only ingest, capture SCC/RBAC/Route/PVC evidence, and exercise stop and cleanup semantics. This is a bounded, falsifiable test of the migration's difficult parts; adding an Operator before it passes would be administrative theatre.
