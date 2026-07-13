# SimOps Kubernetes Job Adapter Research

| Field | Value |
| --- | --- |
| Document ID | SIMOPS-KUBERNETES-JOB-RESEARCH-001 |
| Revision | 0.1 |
| Status | Research note |
| Owner | Software |
| Scope | Issue #24 research only; no production code |

## Purpose

This note records primary-source research for Radiant issue #24: adding a Kubernetes runtime adapter that uses `client-go` and native `batchv1.Job` resources while keeping SimOps semantics owned by the existing control plane. Sources: [issue #24](https://github.com/Nathan-E-White/radiant-prj/issues/24), [parent PRD #19](https://github.com/Nathan-E-White/radiant-prj/issues/19), [closed dependency #20](https://github.com/Nathan-E-White/radiant-prj/issues/20), and [open dependency #22](https://github.com/Nathan-E-White/radiant-prj/issues/22).

This note does not implement the adapter, add dependencies, or change gateway runtime code. Source boundary: [issue #24](https://github.com/Nathan-E-White/radiant-prj/issues/24).

## Source Set

Primary sources used:

- Radiant issue text verified with GitHub CLI on 2026-07-10: [issue #19](https://github.com/Nathan-E-White/radiant-prj/issues/19), [issue #20](https://github.com/Nathan-E-White/radiant-prj/issues/20), [issue #22](https://github.com/Nathan-E-White/radiant-prj/issues/22), [issue #24](https://github.com/Nathan-E-White/radiant-prj/issues/24).
- Kubernetes Job concept docs: <https://kubernetes.io/docs/concepts/workloads/controllers/job/>.
- Kubernetes `batch/v1` Job API reference: <https://kubernetes.io/docs/reference/kubernetes-api/batch/job-v1/>.
- Kubernetes `core/v1` Pod API reference: <https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/>.
- Kubernetes labels and selectors docs: <https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/>.
- Kubernetes cascading deletion docs: <https://kubernetes.io/docs/tasks/administer-cluster/use-cascading-deletion/>.
- Upstream `client-go` source for in-cluster config, kubeconfig loading, clientset construction, typed Job client, and fake clientset: <https://github.com/kubernetes/client-go/blob/master/rest/config.go>, <https://github.com/kubernetes/client-go/blob/master/tools/clientcmd/client_config.go>, <https://github.com/kubernetes/client-go/blob/master/kubernetes/clientset.go>, <https://github.com/kubernetes/client-go/blob/master/kubernetes/typed/batch/v1/job.go>, <https://github.com/kubernetes/client-go/blob/master/kubernetes/fake/clientset_generated.go>.
- Upstream `apimachinery` source for `metav1.DeleteOptions` and `DeletionPropagation`: <https://github.com/kubernetes/apimachinery/blob/master/pkg/apis/meta/v1/types.go>.
- Upstream `client-go` README for versioning and compatibility: <https://github.com/kubernetes/client-go/blob/master/README.md>.
- Local code: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go`, `backend/slurm-gateway/internal/gateway/simops_adapters.go`, `backend/slurm-gateway/internal/gateway/simops_controller.go`, `backend/slurm-gateway/internal/gateway/config.go`, `backend/slurm-gateway/internal/gateway/simops_types.go`, `workers/simops-generator/src/cli.rs`.

## Repo Baseline

Issue #24 is open and asks for a Kubernetes Job adapter behind the SimOps runtime interface, using `k8s.io/client-go`, `k8s.io/api`, and `k8s.io/apimachinery`; it explicitly excludes CRDs/operators, Argo, Tekton, OpenTofu per-run provisioning, multi-step DAGs, and direct data-plane credentials for ordinary workers. Source: [issue #24](https://github.com/Nathan-E-White/radiant-prj/issues/24).

Parent PRD #19 keeps v3 inside the existing SimOps control-plane module, says the runtime adapter interface should grow to launch, stop, and sync, keeps ordinary workers gateway-ingest-only, and reserves direct Redpanda/Postgres/Iceberg refs for trusted data-plane roles. Source: [issue #19](https://github.com/Nathan-E-White/radiant-prj/issues/19).

Issue #20 is closed and established `RunConnectionProfile` as the typed launch contract. It also states that heavyweight concrete dependencies such as Docker SDK, Kubernetes `client-go`, Kafka/Redpanda, pgx, and Iceberg/Arrow must stay out of `internal/gateway` core. Source: [issue #20](https://github.com/Nathan-E-White/radiant-prj/issues/20).

Issue #22 is still open and owns the final runtime-neutral observed lifecycle model: pending, active, succeeded, failed, missing, image-pull-failed, stopped. Issue #24 should map Kubernetes evidence into that model but should not finalize or rename it independently. Source: [issue #22](https://github.com/Nathan-E-White/radiant-prj/issues/22).

The current `RunConnectionProfile` already carries run identity, scenario, launch mode, worker identity, role, gateway ingest URLs/token, worker image, manifest path, labels, runtime connection data, cleanup policy, and optional trusted data-plane refs. Source: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:20-33`.

The runtime sub-struct already includes Kubernetes namespace and job name, while cleanup carries `TTLSecondsAfterFinished` and `AutoRemove`. Source: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:42-60`.

Ordinary worker profiles are built through `BuildRunWorkerConnectionProfile`, while trusted roles get data-plane refs only through `BuildTrustedRunConnectionProfile`; the profile attaches `DataPlane` only when `includeDataPlane` is true. Source: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:101-115`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:168-170`.

The profile label map currently includes `simops.run_id`, `simops.worker_id`, `simops.role`, `simops.launch_mode`, `simops.scenario_id`, `simops.worker_image`, and `simops.worker_kind` when present. Source: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:202-214`.

Kubernetes job names are already normalized to lowercase DNS-ish names and capped at 63 characters, but the label values are not Kubernetes-validated in the current profile code. Source: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:243-259`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:202-214`.

The current runtime interface exposes only `StartRun` and `StopRun`; `SyncRun` is absent and belongs to #22 before #24 can complete lifecycle sync cleanly. Source: `backend/slurm-gateway/internal/gateway/simops_adapters.go:25-28`, [issue #22](https://github.com/Nathan-E-White/radiant-prj/issues/22).

The worker CLI accepts `--manifest`, `--worker`, `--frames`, `--output`, `--run-id`, `--ingest-url`, `--ingest-token`, `--result-ingest-url`, and `--result-ingest-token`; it posts telemetry and results only when the paired URL/token arguments are supplied. Source: `workers/simops-generator/src/cli.rs:48-63`, `workers/simops-generator/src/cli.rs:98-120`.

Current lifecycle names in code are `created`, `starting`, `streaming`, `degraded`, `complete`, `failed`, and `stopped`, which are product/control-plane lifecycle values rather than the #22 runtime-observed states. Source: `backend/slurm-gateway/internal/gateway/simops_types.go:8-18`.

The current stop path delegates to `spooler.StopRun`, marks the run `stopped`, and marks stored workers `stopped`. Source: `backend/slurm-gateway/internal/gateway/simops_controller.go:236-258`.

Configuration currently has `WorkerRuntime`, `WorkerImage`, `WorkerManifestRoot`, `WorkerIngestBaseURL`, `WorkerFrameOverride`, `WorkerNetwork`, `WorkerKubernetesNamespace`, `WorkerCleanupTTL`, and `WorkerAutoRemove`, but validation only accepts `contract` and `docker` runtime values. Source: `backend/slurm-gateway/internal/gateway/config.go:37-56`, `backend/slurm-gateway/internal/gateway/config.go:440-459`.

## Implementation Implications

### Mapping

Map one ordinary worker profile to one `batchv1.Job` in `profile.Runtime.Kubernetes.Namespace`, with `ObjectMeta.Name` from `profile.Runtime.Kubernetes.JobName`. Kubernetes Jobs are one-off tasks that run to completion, create Pods, retry according to Job policy, and retain Job/Pod evidence until deleted or TTL cleanup runs. Sources: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:157-164`, <https://kubernetes.io/docs/concepts/workloads/controllers/job/>, <https://kubernetes.io/docs/reference/kubernetes-api/batch/job-v1/>.

Use `batchv1.JobSpec.Template` as the required Pod template and put the actual worker container in `corev1.PodSpec.Containers`. The Job API says `template` is required and only `Never` or `OnFailure` are allowed for `template.spec.restartPolicy`. Sources: <https://kubernetes.io/docs/reference/kubernetes-api/batch/job-v1/>, <https://kubernetes.io/docs/concepts/workloads/controllers/job/>.

Recommended literal shape for the first adapter slice:

```go
batchv1.Job{
	ObjectMeta: metav1.ObjectMeta{
		Name:      profile.Runtime.Kubernetes.JobName,
		Namespace: profile.Runtime.Kubernetes.Namespace,
		Labels:    kubernetesWorkerLabels(profile),
	},
	Spec: batchv1.JobSpec{
		BackoffLimit:            ptr.To[int32](0),
		TTLSecondsAfterFinished: ttlPointer(profile.Cleanup.TTLSecondsAfterFinished),
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: kubernetesWorkerLabels(profile),
			},
			Spec: corev1.PodSpec{
				RestartPolicy:                 corev1.RestartPolicyNever,
				ServiceAccountName:             adapterConfig.ServiceAccountName,
				AutomountServiceAccountToken:  ptr.To(false),
				EnableServiceLinks:            ptr.To(false),
				Containers: []corev1.Container{{
					Name:  profile.WorkerID,
					Image: profile.WorkerImage,
					Args:  workerArgs(profile, adapterConfig.FrameOverride),
					Env:   workerEnv(profile),
				}},
			},
		},
	},
}
```

Use `RestartPolicyNever` first unless #22 or a later retry policy explicitly wants in-container restarts. `OnFailure` is allowed by Kubernetes, but ordinary workers emit token-gated telemetry/results; automatic re-execution can duplicate output unless the control plane has explicit idempotency semantics for the worker payload stream. Sources: `workers/simops-generator/src/cli.rs:48-63`, <https://kubernetes.io/docs/concepts/workloads/controllers/job/>, <https://kubernetes.io/docs/reference/kubernetes-api/batch/job-v1/>.

Set `BackoffLimit` explicitly rather than accepting Kubernetes' default of 6. A default retry count is platform behavior, not SimOps semantics. If v3 wants retries, make that adapter config and test it; do not let default cluster behavior decide how many times a run-scoped worker re-emits gateway ingest. Source: <https://kubernetes.io/docs/reference/kubernetes-api/batch/job-v1/>.

Map `profile.Cleanup.TTLSecondsAfterFinished` to `JobSpec.TTLSecondsAfterFinished`, but preserve pointer semantics. Kubernetes treats nil as no automatic TTL cleanup and zero as eligible for immediate cleanup after finish. The current profile stores an `int32`, so if Radiant needs "unset" versus "zero seconds" it must add an explicit boolean or pointer before implementation. Sources: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:58-60`, <https://kubernetes.io/docs/concepts/workloads/controllers/job/>, <https://kubernetes.io/docs/reference/kubernetes-api/batch/job-v1/>.

Map `profile.WorkerImage` to `corev1.Container.Image`. Leave `Command` unset unless the image entrypoint is not the worker binary; map the current worker CLI flags to `Args` because Kubernetes separates image entrypoint from args and does not run either through a shell by default. Sources: `workers/simops-generator/src/cli.rs:98-120`, <https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/>.

Map ordinary worker env to identity and gateway ingest only:

- `SIMOPS_RUN_ID`
- `SIMOPS_WORKER_ID`
- `SIMOPS_WORKER_KIND`
- `SIMOPS_ROLE`
- `SIMOPS_LAUNCH_MODE`
- `SIMOPS_SCENARIO_ID`
- `SIMOPS_INGEST_URL`
- `SIMOPS_INGEST_TOKEN`
- `SIMOPS_RESULT_INGEST_URL`
- `SIMOPS_RESULT_INGEST_TOKEN`
- optional `SIMOPS_CLEANUP_TTL_SECONDS`

Do not include Redpanda, Postgres, Iceberg, Docker, or Kubernetes credentials in ordinary worker env. Sources: [issue #19](https://github.com/Nathan-E-White/radiant-prj/issues/19), [issue #20](https://github.com/Nathan-E-White/radiant-prj/issues/20), `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:101-115`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:168-236`, <https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/>.

Set `ServiceAccountName` from adapter config, not from the current profile, because the profile does not carry it today. For ordinary worker Pods, set `AutomountServiceAccountToken=false` unless a specific worker role needs Kubernetes API access; the Pod API exposes both `serviceAccountName` and `automountServiceAccountToken`. Source: <https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/>.

Set `EnableServiceLinks=false` for ordinary workers so Kubernetes does not inject cluster Service environment variables that are outside the gateway-ingest-only contract. Source: <https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/>.

Do not assume `profile.ManifestPath` will exist in a Kubernetes image. The current worker reads a local path. The Kubernetes slice should either require the worker image to contain the manifest root, or extend the profile/config later with ConfigMap/volume mounting. That is a real runtime packaging decision, not a label-mapping detail. Source: `workers/simops-generator/src/cli.rs:35-38`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:144-145`.

### Create

Use the typed Job client: `clientset.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})`. The generated `JobInterface` includes `Create`, `Delete`, `Get`, `List`, and `Watch`, and `kubernetes.NewForConfig` builds the clientset from a REST config. Sources: <https://github.com/kubernetes/client-go/blob/master/kubernetes/typed/batch/v1/job.go>, <https://github.com/kubernetes/client-go/blob/master/kubernetes/clientset.go>.

Treat `AlreadyExists` as a launch conflict unless the existing Job carries the exact expected SimOps labels for run ID, worker ID, and worker kind. That mirrors the Docker conflict concern without pretending Kubernetes object names are globally reusable. Source: [issue #24](https://github.com/Nathan-E-White/radiant-prj/issues/24), <https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/>.

Do not set `manualSelector=true`. Kubernetes says leaving it unset lets the system pick unique labels/selectors; when users set it, they are responsible for uniqueness and wrong selectors can break Jobs. Put SimOps labels on the Job and Pod template for discovery, while allowing Kubernetes to own the Job controller selector. Source: <https://kubernetes.io/docs/reference/kubernetes-api/batch/job-v1/>.

### Labels And Selectors

Apply the run, worker, and kind labels to both `Job.ObjectMeta.Labels` and `Job.Spec.Template.ObjectMeta.Labels`. Jobs are findable by Job labels, while Pods are findable by Pod labels. Kubernetes labels are explicitly intended for organizing and selecting object subsets. Sources: `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:202-214`, <https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/>.

Use at least these labels for issue #24 acceptance:

- `simops.run_id`
- `simops.worker_id`
- `simops.worker_kind`
- `simops.role`
- `simops.launch_mode`
- `simops.scenario_id`
- `simops.worker_image`
- `simops.runtime=kubernetes`
- `simops.runtime_adapter=client-go-job`

Validate Kubernetes label syntax before create. Kubernetes label keys and values have length/character constraints; current Radiant run IDs allow up to 128 characters and `:`, while Kubernetes label values do not allow every current run ID character. If the control plane cannot constrain run IDs for Kubernetes runtime, store raw IDs in annotations and use a validated selector label for runtime operations, but do not silently mutate selector values without recording the raw identity. Sources: `backend/slurm-gateway/internal/gateway/simops_controller.go:20`, `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:202-214`, <https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/>.

Use `metav1.ListOptions{LabelSelector: ...}` for Pod listing during sync, built from `simops.run_id`, `simops.worker_id`, and `simops.worker_kind`. Kubernetes label selectors AND multiple requirements together, and Jobs support selector-style resources. Source: <https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/>.

### Delete And Stop

Default stop behavior should delete the Job by name in its namespace:

```go
policy := metav1.DeletePropagationBackground
err := clientset.BatchV1().
	Jobs(namespace).
	Delete(ctx, jobName, metav1.DeleteOptions{PropagationPolicy: &policy})
```

The generated Job client exposes `Delete(ctx, name, metav1.DeleteOptions)`, and Kubernetes `DeleteOptions.PropagationPolicy` accepts `Orphan`, `Background`, and `Foreground`. Sources: <https://github.com/kubernetes/client-go/blob/master/kubernetes/typed/batch/v1/job.go>, <https://github.com/kubernetes/apimachinery/blob/master/pkg/apis/meta/v1/types.go>.

Use background propagation for ordinary stop/cleanup unless the caller needs to wait for dependent Pod deletion. Kubernetes uses background cascading deletion by default; foreground adds a `foregroundDeletion` finalizer and holds the owner until dependents are deleted. Orphaning leaves Pods behind and should be a deliberate debug-retention mode, not the normal stop path. Source: <https://kubernetes.io/docs/tasks/administer-cluster/use-cascading-deletion/>.

Do not rely on `ttlSecondsAfterFinished` for explicit stop. TTL cleanup only applies after a Job finishes as Complete or Failed. A user stop while Pods are active should delete the Job or, if #22 chooses a retained stopped state, update `spec.suspend=true`; the Job API says suspending a Job stops creating Pods and deletes active Pods until resumed. Sources: <https://kubernetes.io/docs/concepts/workloads/controllers/job/>, <https://kubernetes.io/docs/reference/kubernetes-api/batch/job-v1/>.

Treat NotFound during stop/delete as idempotent success for runtime cleanup, but preserve Forbidden, invalid namespace, and other API errors as adapter errors. Source: [issue #24](https://github.com/Nathan-E-White/radiant-prj/issues/24), <https://github.com/kubernetes/client-go/blob/master/kubernetes/typed/batch/v1/job.go>.

### Sync

Sync must read both Job and Pod status. Job status gives terminal conditions, active count, succeeded count, and failed count; Pod status gives phase plus container waiting/terminated reasons, which are needed to distinguish image pull failures from generic pending or failed states. Sources: <https://kubernetes.io/docs/reference/kubernetes-api/batch/job-v1/>, <https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/>.

Recommended mapping into #22's runtime-neutral state model, subject to #22 final naming:

| Kubernetes evidence | Runtime-neutral state |
| --- | --- |
| Job not found | `missing`, or `stopped` only if control-plane stop intent is known |
| Job condition `Complete=True` or all selected Pods `Succeeded` | `succeeded` |
| Job condition `Failed=True` or selected Pods `Failed` | `failed` |
| Pod waiting reason `ErrImagePull` or `ImagePullBackOff` | `image-pull-failed` |
| Job `status.active > 0` or selected Pod `Running` | `active` |
| Job exists with no terminal condition and Pods are `Pending` or not created yet | `pending` |
| Job/Pods have deletion timestamp after stop/delete request | `stopped` or transitional stopped per #22 |

Do not fold telemetry health, Redpanda health, Postgres projection health, Iceberg completion, or WebTransport/MoQ quality into this runtime lifecycle mapping. Issue #22 explicitly keeps runtime lifecycle separate from telemetry and data-plane health. Source: [issue #22](https://github.com/Nathan-E-White/radiant-prj/issues/22).

### Config

Represent Kubernetes client config separately from runtime-neutral profile construction. The adapter should accept a small config such as:

```go
type KubernetesAdapterConfig struct {
	Namespace                   string
	ServiceAccountName          string
	KubeconfigPath              string
	MasterURL                   string
	InCluster                   bool
	DeletePropagation           metav1.DeletionPropagation
	BackoffLimit                int32
	RestartPolicy               corev1.RestartPolicy
	AutomountServiceAccountToken bool
}
```

The exact names can follow Radiant's config style, but the important boundary is that Kubernetes API types and `client-go` imports stay in the concrete adapter package, not in `internal/gateway`. Source: [issue #20](https://github.com/Nathan-E-White/radiant-prj/issues/20).

For in-cluster execution, use `rest.InClusterConfig()`. Upstream source says it uses the Pod's service account token and CA file and returns `ErrNotInCluster` outside a Kubernetes environment. Source: <https://github.com/kubernetes/client-go/blob/master/rest/config.go>.

For kubeconfig execution, use `clientcmd.BuildConfigFromFlags(masterURL, kubeconfigPath)` or the underlying deferred loading client config. Upstream source says it falls back to in-cluster config when neither master URL nor kubeconfig is passed, then to default config. Source: <https://github.com/kubernetes/client-go/blob/master/tools/clientcmd/client_config.go>.

Add `kubernetes` as a valid `SIMOPS_WORKER_RUNTIME` only when implementing production code. Current validation accepts only `contract` and `docker`, so #24 will need a config validation change after #22 lands. Source: `backend/slurm-gateway/internal/gateway/config.go:440-459`.

### Testing

Use `kubernetes/fake.NewSimpleClientset` for unit tests that exercise create/delete/list/watch calls without a real cluster. Upstream source says the simple clientset processes create/update/delete as-is without applying server validation, defaults, or field management, so tests must assert adapter-built literals and explicit error reactors rather than assuming the fake behaves like the API server. Source: <https://github.com/kubernetes/client-go/blob/master/kubernetes/fake/clientset_generated.go>.

Use known-good `batchv1.Job` literals for spec construction tests. Assert object name, namespace, labels, template labels, restart policy, container image, args, env, service account, token automount, `EnableServiceLinks`, backoff limit, and TTL pointer semantics. Sources: [issue #24](https://github.com/Nathan-E-White/radiant-prj/issues/24), <https://kubernetes.io/docs/reference/kubernetes-api/batch/job-v1/>, <https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/>.

Use fake client reactors for create/delete errors. The fake clientset is suitable for adapter behavior tests, but because it does not validate Kubernetes object rules, add pure unit tests for label-value validation and restart-policy selection. Sources: <https://github.com/kubernetes/client-go/blob/master/kubernetes/fake/clientset_generated.go>, <https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/>.

For lifecycle sync tests, construct fake `batchv1.Job` and `corev1.Pod` objects with status conditions, pod phases, and container waiting reasons. Do not require a real cluster. Sources: [issue #24](https://github.com/Nathan-E-White/radiant-prj/issues/24), <https://kubernetes.io/docs/reference/kubernetes-api/batch/job-v1/>, <https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/>.

### Build Weight

Keep `k8s.io/client-go`, `k8s.io/api`, and `k8s.io/apimachinery` imports out of `backend/slurm-gateway/internal/gateway` if possible. A concrete adapter package such as `backend/slurm-gateway/internal/simopskubernetes` can import Kubernetes packages and implement the gateway's runtime interface. The server command can wire it in, while gateway handlers/profile/controller tests keep compiling the lean core. Source: [issue #20](https://github.com/Nathan-E-White/radiant-prj/issues/20).

The `client-go` README states that `v0.x.y` tags can change Go APIs across versions, recommends `v0.x.y` tags for Kubernetes releases >= 1.17, and publishes a compatibility matrix by Kubernetes minor version. Pin the Kubernetes module versions deliberately in implementation rather than adding `@latest` casually. Source: <https://github.com/kubernetes/client-go/blob/master/README.md>.

A useful implementation-time guard is:

```sh
go list -deps ./internal/gateway | rg 'k8s.io/(client-go|api|apimachinery)'
```

The expected result for the core package should be empty. Adapter package tests will compile Kubernetes dependencies; core gateway/profile tests should not.

## Recommended Test Matrix For Issue #24

| Test | Behavior Proven | Sources |
| --- | --- | --- |
| Job literal construction from profile | Namespace, job name, template, restart policy, image, args, env, TTL, labels, service account, token automount, and service links are rendered as intended | `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:20-60`, <https://kubernetes.io/docs/reference/kubernetes-api/batch/job-v1/>, <https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/> |
| Required labels on Job and Pod template | `simops.run_id`, `simops.worker_id`, and `simops.worker_kind` are present on both resources and usable in label selectors | `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:202-214`, <https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/> |
| Kubernetes label validation | Unsupported run/worker label values fail before create or are represented through an explicit raw-annotation plus safe-selector-label strategy | `backend/slurm-gateway/internal/gateway/simops_controller.go:20`, <https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/> |
| Gateway-ingest-only worker env | Ordinary worker env contains run/worker identity and ingest URLs/tokens, but no Redpanda/Postgres/Iceberg/Kubernetes credentials | [issue #20](https://github.com/Nathan-E-White/radiant-prj/issues/20), `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:101-115`, `workers/simops-generator/src/cli.rs:48-63` |
| Service account mapping | Configured service account maps to `PodSpec.ServiceAccountName`, and ordinary worker token automount defaults false | <https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/> |
| TTL pointer semantics | Nonzero TTL maps to pointer value; zero behavior is tested as immediate cleanup or no cleanup according to the final Radiant config decision | `backend/slurm-gateway/internal/gateway/simops_run_connection_profile.go:58-60`, <https://kubernetes.io/docs/concepts/workloads/controllers/job/> |
| Create error handling | Fake clientset or reactor covers already exists, forbidden, invalid namespace, and API failure paths | <https://github.com/kubernetes/client-go/blob/master/kubernetes/fake/clientset_generated.go>, <https://github.com/kubernetes/client-go/blob/master/kubernetes/typed/batch/v1/job.go> |
| Stop/delete behavior | Adapter deletes Job by name using configured propagation policy and treats NotFound as idempotent cleanup | <https://github.com/kubernetes/apimachinery/blob/master/pkg/apis/meta/v1/types.go>, <https://kubernetes.io/docs/tasks/administer-cluster/use-cascading-deletion/> |
| Pod list by selector | Sync finds Pods using run ID, worker ID, and worker kind labels rather than owner-name guessing | <https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/> |
| Job status mapping | Complete/Failed/Active/Succeeded/Failed counters map into #22 runtime-neutral states without changing final #22 vocabulary | [issue #22](https://github.com/Nathan-E-White/radiant-prj/issues/22), <https://kubernetes.io/docs/reference/kubernetes-api/batch/job-v1/> |
| Pod status mapping | Pending/Running/Succeeded/Failed phases and container waiting reasons map into pending/active/succeeded/failed/image-pull-failed | <https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/> |
| Fake clientset boundary | Tests assert known-good objects and custom reactors because fake clientset does not apply server validation/defaulting | <https://github.com/kubernetes/client-go/blob/master/kubernetes/fake/clientset_generated.go> |
| Build-weight guard | `internal/gateway` does not import Kubernetes modules; only the concrete adapter package does | [issue #20](https://github.com/Nathan-E-White/radiant-prj/issues/20), <https://github.com/kubernetes/client-go/blob/master/README.md> |

## Implementation-Relevant Conclusion

Issue #24 should add a concrete Kubernetes Job runtime adapter behind the existing SimOps runtime seam, not a CRD, operator, workflow engine, or OpenTofu per-run launcher. The adapter should render a single `batchv1.Job` per ordinary worker from `RunConnectionProfile`, label both Jobs and Pods with run/worker identity, keep ordinary env gateway-ingest-only, and use typed `client-go` create/delete/list calls.

The strongest implementation sequence is:

1. Wait for #22 or depend on its final runtime-neutral `SyncRun` contract.
2. Add Kubernetes config fields and `SIMOPS_WORKER_RUNTIME=kubernetes` validation.
3. Put the concrete adapter in a package outside `internal/gateway`, for example `internal/simopskubernetes`.
4. Build and unit-test Job literals from `RunConnectionProfile`.
5. Add fake-client create/delete tests and pure status-mapping tests.
6. Add a build-weight guard proving Kubernetes dependencies do not leak into gateway core.

The main gotchas are not mysterious: Kubernetes label values are stricter than Radiant run IDs, `ttlSecondsAfterFinished` has nil-versus-zero semantics that the current profile does not express, fake clientsets do not validate/default like the API server, and lifecycle sync cannot be finalized until #22 owns the vocabulary. Handle those explicitly and the adapter stays a deep runtime module instead of turning the control plane into Kubernetes soup.
