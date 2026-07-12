package simopskubernetes

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"

	"radiant/slurm-gateway/internal/gateway"
)

func TestStartRunProfilesCreatesGatewayOnlyJob(t *testing.T) {
	client := fake.NewClientset()
	spooler := Spooler{Config: testConfig(), Client: client, Now: fixedNow}
	run, profile := testRunAndProfile(t)

	workers, commands, err := spooler.StartRunProfiles(context.Background(), run, []gateway.RunConnectionProfile{profile})
	if err != nil {
		t.Fatalf("start profiles: %v", err)
	}
	job, err := client.BatchV1().Jobs("radiant-simops").Get(context.Background(), profile.Runtime.Kubernetes.JobName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get created job: %v", err)
	}
	if job.Labels["simops.run_id"] != run.RunID || job.Labels["simops.worker_id"] != profile.WorkerID || job.Labels["simops.worker_kind"] != string(profile.WorkerKind) {
		t.Fatalf("unexpected labels %#v", job.Labels)
	}
	if job.Spec.Template.Spec.ServiceAccountName != "simops-worker" || job.Spec.Template.Spec.RestartPolicy != corev1.RestartPolicyNever {
		t.Fatalf("unexpected pod policy %#v", job.Spec.Template.Spec)
	}
	if job.Spec.TTLSecondsAfterFinished == nil || *job.Spec.TTLSecondsAfterFinished != 600 {
		t.Fatalf("unexpected cleanup TTL %#v", job.Spec.TTLSecondsAfterFinished)
	}
	container := job.Spec.Template.Spec.Containers[0]
	joinedArgs := strings.Join(container.Args, " ")
	for _, expected := range []string{"--manifest /examples/simulation-ops/run-manifest.scheduler-drift.json", "--worker scheduler", "--run-id RUN-K8S-001", "--ingest-token ingest-token"} {
		if !strings.Contains(joinedArgs, expected) {
			t.Fatalf("job args missing %q: %#v", expected, container.Args)
		}
	}
	for _, env := range container.Env {
		if strings.Contains(strings.ToUpper(env.Name), "POSTGRES") || strings.Contains(strings.ToUpper(env.Name), "REDPANDA") || strings.Contains(strings.ToUpper(env.Name), "ICEBERG") {
			t.Fatalf("ordinary worker leaked data-plane env %#v", env)
		}
	}
	if len(workers) != 1 || workers[0].Runtime != "kubernetes" || workers[0].RuntimeID != "radiant-simops/"+job.Name {
		t.Fatalf("unexpected worker records %#v", workers)
	}
	if len(commands) != 1 || commands[0].Metadata["kubernetes_job"] != job.Name {
		t.Fatalf("unexpected commands %#v", commands)
	}
}

func TestStopRunProfilesDeletesJobsAndReturnsErrors(t *testing.T) {
	run, profile := testRunAndProfile(t)
	client := fake.NewClientset(&batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: profile.Runtime.Kubernetes.JobName, Namespace: profile.Runtime.Kubernetes.Namespace}})
	spooler := Spooler{Config: testConfig(), Client: client}
	if err := spooler.StopRunProfiles(context.Background(), run.RunID, []gateway.RunConnectionProfile{profile}); err != nil {
		t.Fatalf("stop profiles: %v", err)
	}
	if _, err := client.BatchV1().Jobs(profile.Runtime.Kubernetes.Namespace).Get(context.Background(), profile.Runtime.Kubernetes.JobName, metav1.GetOptions{}); err == nil {
		t.Fatal("expected job deletion")
	}

	client = fake.NewClientset()
	client.PrependReactor("delete", "jobs", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("rbac denied")
	})
	spooler.Client = client
	if err := spooler.StopRunProfiles(context.Background(), run.RunID, []gateway.RunConnectionProfile{profile}); err == nil || !strings.Contains(err.Error(), "rbac denied") {
		t.Fatalf("expected delete error, got %v", err)
	}
}

func TestSyncRunProfilesMapsJobAndPodLifecycle(t *testing.T) {
	run, profile := testRunAndProfile(t)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: profile.Runtime.Kubernetes.JobName, Namespace: profile.Runtime.Kubernetes.Namespace, Labels: profile.Labels},
		Status:     batchv1.JobStatus{Active: 1},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: profile.Runtime.Kubernetes.JobName + "-abc", Namespace: profile.Runtime.Kubernetes.Namespace, Labels: profile.Labels},
		Status:     corev1.PodStatus{Phase: corev1.PodPending, ContainerStatuses: []corev1.ContainerStatus{{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff", Message: "image unavailable"}}}}},
	}
	spooler := Spooler{Config: testConfig(), Client: fake.NewClientset(job, pod), Now: fixedNow}
	observations, err := spooler.SyncRunProfiles(context.Background(), run, []gateway.RunConnectionProfile{profile})
	if err != nil {
		t.Fatalf("sync profiles: %v", err)
	}
	if len(observations) != 1 || observations[0].State != gateway.ObservedWorkerImagePullFailed || observations[0].Reason != "ImagePullBackOff" || observations[0].Runtime != "kubernetes" {
		t.Fatalf("unexpected observations %#v", observations)
	}

	job.Status = batchv1.JobStatus{Succeeded: 1, Conditions: []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue, Reason: "Completed"}}}
	spooler.Client = fake.NewClientset(job)
	observations, err = spooler.SyncRunProfiles(context.Background(), run, []gateway.RunConnectionProfile{profile})
	if err != nil || observations[0].State != gateway.ObservedWorkerSucceeded {
		t.Fatalf("expected succeeded observation, got %#v err=%v", observations, err)
	}
}

func TestSyncRunProfilesMapsPodPhases(t *testing.T) {
	run, profile := testRunAndProfile(t)
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: profile.Runtime.Kubernetes.JobName, Namespace: profile.Runtime.Kubernetes.Namespace, Labels: profile.Labels}, Status: batchv1.JobStatus{Active: 1}}
	for _, test := range []struct {
		name  string
		phase corev1.PodPhase
		want  gateway.ObservedWorkerState
	}{
		{name: "pending", phase: corev1.PodPending, want: gateway.ObservedWorkerPending},
		{name: "running", phase: corev1.PodRunning, want: gateway.ObservedWorkerActive},
		{name: "succeeded", phase: corev1.PodSucceeded, want: gateway.ObservedWorkerSucceeded},
		{name: "failed", phase: corev1.PodFailed, want: gateway.ObservedWorkerFailed},
	} {
		t.Run(test.name, func(t *testing.T) {
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: profile.Runtime.Kubernetes.JobName + "-pod", Namespace: profile.Runtime.Kubernetes.Namespace, Labels: profile.Labels}, Status: corev1.PodStatus{Phase: test.phase}}
			spooler := Spooler{Config: testConfig(), Client: fake.NewClientset(job, pod), Now: fixedNow}
			observations, err := spooler.SyncRunProfiles(context.Background(), run, []gateway.RunConnectionProfile{profile})
			if err != nil || len(observations) != 1 || observations[0].State != test.want {
				t.Fatalf("phase %s: observations=%#v err=%v", test.phase, observations, err)
			}
		})
	}
}

func TestStartRunProfilesReturnsCreateError(t *testing.T) {
	run, profile := testRunAndProfile(t)
	client := fake.NewClientset()
	client.PrependReactor("create", "jobs", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("namespace missing")
	})
	spooler := Spooler{Config: testConfig(), Client: client}
	if _, _, err := spooler.StartRunProfiles(context.Background(), run, []gateway.RunConnectionProfile{profile}); err == nil || !strings.Contains(err.Error(), "namespace missing") {
		t.Fatalf("expected create error, got %v", err)
	}
}

func testRunAndProfile(t *testing.T) (gateway.SimopsRunRecord, gateway.RunConnectionProfile) {
	t.Helper()
	run := gateway.SimopsRunRecord{RunID: "RUN-K8S-001", ScenarioID: "scheduler-drift", LaunchMode: "auto", IngestToken: "ingest-token"}
	profile, err := gateway.BuildRunWorkerConnectionProfile(testConfig(), run, gateway.SimopsWorkerScheduler)
	if err != nil {
		t.Fatalf("build profile: %v", err)
	}
	return run, profile
}

func testConfig() gateway.SimopsConfig {
	cfg := gateway.DefaultConfig().Simops
	cfg.WorkerRuntime = "kubernetes"
	cfg.WorkerImage = "simops-generator:test"
	cfg.WorkerKubernetesNamespace = "radiant-simops"
	cfg.WorkerKubernetesServiceAccount = "simops-worker"
	cfg.WorkerCleanupTTL = 10 * time.Minute
	return cfg
}

func fixedNow() time.Time { return time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC) }
