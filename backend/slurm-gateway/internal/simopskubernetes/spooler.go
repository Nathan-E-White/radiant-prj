package simopskubernetes

import (
	"context"
	"fmt"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"radiant/slurm-gateway/internal/gateway"
)

type Spooler struct {
	Config gateway.SimopsConfig
	Client kubernetes.Interface
	Now    func() time.Time
}

func NewSpooler(cfg gateway.SimopsConfig) (*Spooler, error) {
	restConfig, err := kubernetesConfig(cfg.WorkerKubeconfig)
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create Kubernetes client: %w", err)
	}
	return &Spooler{Config: cfg, Client: client, Now: time.Now}, nil
}

func kubernetesConfig(explicitPath string) (*rest.Config, error) {
	if path := strings.TrimSpace(explicitPath); path != "" {
		cfg, err := clientcmd.BuildConfigFromFlags("", path)
		if err != nil {
			return nil, fmt.Errorf("load SIMOPS_WORKER_KUBECONFIG %q: %w", path, err)
		}
		return cfg, nil
	}
	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load in-cluster or default kubeconfig: %w", err)
	}
	return cfg, nil
}

func (s Spooler) StartRun(ctx context.Context, run gateway.SimopsRunRecord, workers []gateway.SimopsWorkerKind) ([]gateway.SimopsWorkerRecord, []gateway.SimopsSpoolCommand, error) {
	profiles, err := gateway.BuildRunWorkerConnectionProfiles(s.Config, run, workers)
	if err != nil {
		return nil, nil, err
	}
	return s.StartRunProfiles(ctx, run, profiles)
}

func (s Spooler) StartRunProfiles(ctx context.Context, run gateway.SimopsRunRecord, profiles []gateway.RunConnectionProfile) ([]gateway.SimopsWorkerRecord, []gateway.SimopsSpoolCommand, error) {
	if s.Client == nil {
		return nil, nil, fmt.Errorf("Kubernetes client is required")
	}
	now := s.now()
	workers := make([]gateway.SimopsWorkerRecord, 0, len(profiles))
	commands := make([]gateway.SimopsSpoolCommand, 0, len(profiles))
	created := make([]gateway.RunConnectionProfile, 0, len(profiles))
	for _, profile := range profiles {
		job := jobForProfile(profile, s.Config.WorkerFrameOverride)
		if _, err := s.Client.BatchV1().Jobs(profile.Runtime.Kubernetes.Namespace).Create(ctx, job, metav1.CreateOptions{}); err != nil {
			_ = s.StopRunProfiles(context.WithoutCancel(ctx), run.RunID, created)
			return nil, nil, fmt.Errorf("create Kubernetes Job %s/%s: %w", job.Namespace, job.Name, err)
		}
		created = append(created, profile)
		runtimeID := profile.Runtime.Kubernetes.Namespace + "/" + profile.Runtime.Kubernetes.JobName
		workers = append(workers, gateway.SimopsWorkerRecord{
			RunID: run.RunID, WorkerID: profile.WorkerID, WorkerKind: profile.WorkerKind,
			Lifecycle: gateway.SimopsStarting, LaunchMode: profile.LaunchMode,
			Endpoint: profile.Gateway.IngestURL, Runtime: "kubernetes", RuntimeID: runtimeID,
			UpdatedAt: now, Labels: cloneLabels(profile.Labels),
		})
		commands = append(commands, gateway.SimopsSpoolCommand{
			CommandID: run.RunID + "-" + profile.WorkerID + "-start", RunID: run.RunID,
			WorkerID: profile.WorkerID, Mode: profile.LaunchMode, State: gateway.SimopsStarting,
			Message:   "Kubernetes Job created as " + runtimeID,
			Metadata:  map[string]string{"kubernetes_namespace": profile.Runtime.Kubernetes.Namespace, "kubernetes_job": profile.Runtime.Kubernetes.JobName},
			CreatedAt: now, UpdatedAt: now,
		})
	}
	return workers, commands, nil
}

func jobForProfile(profile gateway.RunConnectionProfile, frameOverride int) *batchv1.Job {
	backoffLimit := int32(0)
	args := gateway.BuildRunWorkerCommand(profile, frameOverride)
	jobLabels := cloneLabels(profile.Labels)
	jobLabels["simops.runtime_adapter"] = "kubernetes-job"
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: profile.Runtime.Kubernetes.JobName, Namespace: profile.Runtime.Kubernetes.Namespace, Labels: jobLabels},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: ttlPointer(profile.Cleanup.TTLSecondsAfterFinished),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: cloneLabels(jobLabels)},
				Spec: corev1.PodSpec{
					ServiceAccountName: profile.Runtime.Kubernetes.ServiceAccount,
					RestartPolicy:      corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name: "simops-worker", Image: profile.WorkerImage, Args: args,
						Env: []corev1.EnvVar{{Name: "SIMOPS_RUN_ID", Value: profile.RunID}, {Name: "SIMOPS_WORKER_ID", Value: profile.WorkerID}, {Name: "SIMOPS_WORKER_KIND", Value: string(profile.WorkerKind)}},
					}},
				},
			},
		},
	}
}

func (s Spooler) StopRun(ctx context.Context, runID string) error {
	if s.Client == nil {
		return fmt.Errorf("Kubernetes client is required")
	}
	selector := labels.Set{"simops.run_id": strings.TrimSpace(runID), "simops.runtime_adapter": "kubernetes-job"}.AsSelector().String()
	jobs, err := s.Client.BatchV1().Jobs(s.Config.WorkerKubernetesNamespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return fmt.Errorf("list Kubernetes Jobs for run %s: %w", runID, err)
	}
	profiles := make([]gateway.RunConnectionProfile, 0, len(jobs.Items))
	for _, job := range jobs.Items {
		profiles = append(profiles, gateway.RunConnectionProfile{Runtime: gateway.RunRuntimeConnection{Kubernetes: gateway.KubernetesRunConnection{Namespace: job.Namespace, JobName: job.Name}}})
	}
	return s.StopRunProfiles(ctx, runID, profiles)
}

func (s Spooler) StopRunProfiles(ctx context.Context, runID string, profiles []gateway.RunConnectionProfile) error {
	if s.Client == nil {
		return fmt.Errorf("Kubernetes client is required")
	}
	policy := metav1.DeletePropagationForeground
	var firstErr error
	for _, profile := range profiles {
		ref := profile.Runtime.Kubernetes
		err := s.Client.BatchV1().Jobs(ref.Namespace).Delete(ctx, ref.JobName, metav1.DeleteOptions{PropagationPolicy: &policy})
		if err != nil && !apierrors.IsNotFound(err) && firstErr == nil {
			firstErr = fmt.Errorf("delete Kubernetes Job %s/%s for run %s: %w", ref.Namespace, ref.JobName, runID, err)
		}
	}
	return firstErr
}

func (s Spooler) SyncRun(ctx context.Context, run gateway.SimopsRunRecord, workers []gateway.SimopsWorkerRecord) ([]gateway.ObservedWorkerLifecycle, error) {
	profiles, err := gateway.BuildRunWorkerConnectionProfilesForRecords(s.Config, run, workers)
	if err != nil {
		return nil, err
	}
	return s.SyncRunProfiles(ctx, run, profiles)
}

func (s Spooler) SyncRunProfiles(ctx context.Context, run gateway.SimopsRunRecord, profiles []gateway.RunConnectionProfile) ([]gateway.ObservedWorkerLifecycle, error) {
	if s.Client == nil {
		return nil, fmt.Errorf("Kubernetes client is required")
	}
	observations := make([]gateway.ObservedWorkerLifecycle, 0, len(profiles))
	for _, profile := range profiles {
		ref := profile.Runtime.Kubernetes
		job, err := s.Client.BatchV1().Jobs(ref.Namespace).Get(ctx, ref.JobName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			observations = append(observations, observation(profile, gateway.ObservedWorkerMissing, "JobNotFound", "Kubernetes Job was not found", s.now()))
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("get Kubernetes Job %s/%s: %w", ref.Namespace, ref.JobName, err)
		}
		obs := observation(profile, jobState(job), jobReason(job), jobMessage(job), s.now())
		pods, err := s.Client.CoreV1().Pods(ref.Namespace).List(ctx, metav1.ListOptions{LabelSelector: labels.Set{"simops.run_id": profile.RunID, "simops.worker_id": profile.WorkerID}.AsSelector().String()})
		if err != nil {
			return nil, fmt.Errorf("list Pods for Kubernetes Job %s/%s: %w", ref.Namespace, ref.JobName, err)
		}
		for i := range pods.Items {
			if state, reason, message, ok := podLifecycle(&pods.Items[i]); ok {
				obs.State, obs.Reason, obs.Message = state, reason, message
				if state == gateway.ObservedWorkerImagePullFailed || state == gateway.ObservedWorkerFailed {
					break
				}
			}
		}
		observations = append(observations, obs)
	}
	return observations, nil
}

func jobState(job *batchv1.Job) gateway.ObservedWorkerState {
	for _, condition := range job.Status.Conditions {
		if condition.Status != corev1.ConditionTrue {
			continue
		}
		if condition.Type == batchv1.JobComplete {
			return gateway.ObservedWorkerSucceeded
		}
		if condition.Type == batchv1.JobFailed {
			return gateway.ObservedWorkerFailed
		}
	}
	if job.Status.Succeeded > 0 {
		return gateway.ObservedWorkerSucceeded
	}
	if job.Status.Failed > 0 {
		return gateway.ObservedWorkerFailed
	}
	if job.Status.Active > 0 {
		return gateway.ObservedWorkerActive
	}
	return gateway.ObservedWorkerPending
}

func jobReason(job *batchv1.Job) string {
	for _, condition := range job.Status.Conditions {
		if condition.Status == corev1.ConditionTrue && condition.Reason != "" {
			return condition.Reason
		}
	}
	return "JobObserved"
}

func jobMessage(job *batchv1.Job) string {
	for _, condition := range job.Status.Conditions {
		if condition.Status == corev1.ConditionTrue && condition.Message != "" {
			return condition.Message
		}
	}
	return "Kubernetes Job lifecycle observed"
}

func podLifecycle(pod *corev1.Pod) (gateway.ObservedWorkerState, string, string, bool) {
	for _, status := range pod.Status.ContainerStatuses {
		if waiting := status.State.Waiting; waiting != nil {
			switch waiting.Reason {
			case "ErrImagePull", "ImagePullBackOff", "InvalidImageName":
				return gateway.ObservedWorkerImagePullFailed, waiting.Reason, waiting.Message, true
			}
		}
	}
	switch pod.Status.Phase {
	case corev1.PodPending:
		return gateway.ObservedWorkerPending, podReason(pod, "PodPending"), pod.Status.Message, true
	case corev1.PodRunning:
		return gateway.ObservedWorkerActive, podReason(pod, "PodRunning"), pod.Status.Message, true
	case corev1.PodSucceeded:
		return gateway.ObservedWorkerSucceeded, podReason(pod, "PodSucceeded"), pod.Status.Message, true
	case corev1.PodFailed:
		return gateway.ObservedWorkerFailed, podReason(pod, "PodFailed"), pod.Status.Message, true
	}
	return "", "", "", false
}

func podReason(pod *corev1.Pod, fallback string) string {
	if reason := strings.TrimSpace(pod.Status.Reason); reason != "" {
		return reason
	}
	return fallback
}

func observation(profile gateway.RunConnectionProfile, state gateway.ObservedWorkerState, reason, message string, now time.Time) gateway.ObservedWorkerLifecycle {
	return gateway.ObservedWorkerLifecycle{RunID: profile.RunID, WorkerID: profile.WorkerID, WorkerKind: profile.WorkerKind, State: state, Runtime: "kubernetes", RuntimeID: profile.Runtime.Kubernetes.Namespace + "/" + profile.Runtime.Kubernetes.JobName, Reason: reason, Message: message, ObservedAt: now, Labels: cloneLabels(profile.Labels)}
}

func (s Spooler) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}
func ttlPointer(ttl int32) *int32 {
	if ttl <= 0 {
		return nil
	}
	return &ttl
}
func cloneLabels(source map[string]string) map[string]string {
	result := make(map[string]string, len(source)+1)
	for key, value := range source {
		result[key] = value
	}
	return result
}
