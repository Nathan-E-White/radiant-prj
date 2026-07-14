package simopsdocker

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/network"
	dockerclient "github.com/moby/moby/client"
	"github.com/opencontainers/image-spec/specs-go/v1"

	"radiant/slurm-gateway/internal/gateway"
)

func TestSpoolerCreatesAndStartsWorkerFromRunConnectionProfile(t *testing.T) {
	client := &fakeDockerClient{
		image:  image.InspectResponse{ID: "image-123"},
		create: container.CreateResponse{ID: "container-123"},
	}
	spooler := Spooler{
		Config: testDockerConfig(),
		Client: client,
		Now:    fixedNow,
	}
	run := testDockerRun("RUN-DOCKER-SDK-001")
	profiles := testDockerProfiles(t, run, gateway.SimopsWorkerStorage)

	workers, commands, err := spooler.StartRunProfiles(context.Background(), run, profiles)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	if client.inspectedImage != "radiant-simops-generator:test" {
		t.Fatalf("unexpected inspected image %q", client.inspectedImage)
	}
	if client.createdName != "simops-RUN-DOCKER-SDK-001-storage-01" {
		t.Fatalf("unexpected container name %q", client.createdName)
	}
	if client.createdConfig.Image != "radiant-simops-generator:test" {
		t.Fatalf("unexpected image %q", client.createdConfig.Image)
	}
	for _, label := range []string{"simops.run_id", "simops.worker_id", "simops.worker_kind", "simops.role", "simops.launch_mode", "simops.scenario_id", "simops.worker_image"} {
		if client.createdConfig.Labels[label] == "" {
			t.Fatalf("missing label %q in %#v", label, client.createdConfig.Labels)
		}
	}
	for _, env := range []string{
		"SIMOPS_RUN_ID=RUN-DOCKER-SDK-001",
		"SIMOPS_WORKER_ID=storage-01",
		"SIMOPS_WORKER_KIND=storage",
		"SIMOPS_INGEST_URL=http://host.docker.internal:8080/internal/simops/runs/RUN-DOCKER-SDK-001/ingest",
		"SIMOPS_INGEST_TOKEN=ingest-token",
		"SIMOPS_RESULT_INGEST_URL=http://host.docker.internal:8080/internal/simops/runs/RUN-DOCKER-SDK-001/results",
		"SIMOPS_RESULT_INGEST_TOKEN=ingest-token",
	} {
		if !slices.Contains(client.createdConfig.Env, env) {
			t.Fatalf("missing env %q in %#v", env, client.createdConfig.Env)
		}
	}
	joinedCmd := strings.Join(client.createdConfig.Cmd, " ")
	for _, arg := range []string{
		"--manifest /examples/simulation-ops/run-manifest.scheduler-drift.json",
		"--worker storage",
		"--run-id RUN-DOCKER-SDK-001",
		"--ingest-url http://host.docker.internal:8080/internal/simops/runs/RUN-DOCKER-SDK-001/ingest",
		"--result-ingest-url http://host.docker.internal:8080/internal/simops/runs/RUN-DOCKER-SDK-001/results",
		"--frames 2",
	} {
		if !strings.Contains(joinedCmd, arg) {
			t.Fatalf("missing command arg %q in %#v", arg, client.createdConfig.Cmd)
		}
	}
	if !client.createdHostConfig.AutoRemove {
		t.Fatalf("expected host config AutoRemove")
	}
	if client.createdHostConfig.NetworkMode != "radiant-simops-local" {
		t.Fatalf("unexpected host network mode %q", client.createdHostConfig.NetworkMode)
	}
	if client.createdNetworking.EndpointsConfig["radiant-simops-local"] == nil {
		t.Fatalf("expected radiant-simops-local endpoint, got %#v", client.createdNetworking.EndpointsConfig)
	}
	if client.startedContainer != "container-123" {
		t.Fatalf("expected container start, got %q", client.startedContainer)
	}
	if len(workers) != 1 || workers[0].Labels["simops.container_id"] != "container-123" {
		t.Fatalf("unexpected workers %#v", workers)
	}
	if len(commands) != 1 || !strings.Contains(commands[0].Message, "Worker container launched") {
		t.Fatalf("unexpected commands %#v", commands)
	}
	if commands[0].Metadata["container_id"] != "container-123" || commands[0].Metadata["container_name"] != "simops-RUN-DOCKER-SDK-001-storage-01" {
		t.Fatalf("missing structured launch metadata %#v", commands[0].Metadata)
	}
	if commands[0].Metadata["runtime_adapter"] != "docker-sdk" || commands[0].Metadata["worker_kind"] != "storage" {
		t.Fatalf("unexpected launch metadata %#v", commands[0].Metadata)
	}
}

func TestSpoolerMapsLaunchErrors(t *testing.T) {
	run := testDockerRun("RUN-DOCKER-FAIL")
	spooler := Spooler{
		Config: testDockerConfig(),
		Client: &fakeDockerClient{
			image:     image.InspectResponse{ID: "image-123"},
			createErr: errors.New("daemon said no"),
		},
		Now: fixedNow,
	}

	_, _, err := spooler.StartRunProfiles(context.Background(), run, testDockerProfiles(t, run, gateway.SimopsWorkerScheduler))
	if err == nil || !strings.Contains(err.Error(), "container_create") {
		t.Fatalf("expected mapped create error, got %v", err)
	}
	var launchErr LaunchError
	if !errors.As(err, &launchErr) {
		t.Fatalf("expected LaunchError, got %T", err)
	}
	if launchErr.Operation != "container_create" || launchErr.RunID != "RUN-DOCKER-FAIL" || launchErr.WorkerID != "scheduler-01" {
		t.Fatalf("unexpected launch error %#v", launchErr)
	}
}

func TestRunLifecycleOwnsDockerPartialLaunchCompensation(t *testing.T) {
	cfg := testDockerConfig()
	run := testDockerRun("RUN-DOCKER-LIFECYCLE-PARTIAL")
	run.Lifecycle = gateway.SimopsStarting
	run.SubmittedBy = "test"
	run.CreatedAt = fixedNow()
	run.UpdatedAt = fixedNow()
	client := &fakeDockerClient{
		image: image.InspectResponse{ID: "image-123"},
		createSequence: []container.CreateResponse{
			{ID: "container-scheduler"},
			{},
		},
		createErrorSequence: []error{nil, errors.New("storage create failed")},
		listed:              []container.Summary{dockerSummary("container-scheduler", run.RunID, "scheduler-01", "scheduler")},
	}
	spooler := &Spooler{Config: cfg, Client: client, Now: fixedNow}
	store := gateway.NewInMemorySimopsStore()
	lifecycle := gateway.NewSimopsRunLifecyclePolicy(cfg, store, spooler, gateway.MemorySimopsEventLog{Store: store}, gateway.IcebergArtifactPlanner{})
	lifecycle.SetNow(fixedNow)

	outcome, err := lifecycle.Start(context.Background(), run, []gateway.SimopsWorkerKind{gateway.SimopsWorkerScheduler, gateway.SimopsWorkerStorage})
	var lifecycleErr *gateway.SimopsRunLifecycleError
	if !errors.As(err, &lifecycleErr) || lifecycleErr.Stage != gateway.SimopsRunStageRuntimeLaunch {
		t.Fatalf("expected runtime launch failure, got outcome=%#v err=%v", outcome, err)
	}
	if !slices.Equal(client.stopped, []string{"container-scheduler"}) || !slices.Equal(client.removed, []string{"container-scheduler"}) {
		t.Fatalf("expected one policy-owned compensation, stopped=%#v removed=%#v", client.stopped, client.removed)
	}
	workers, listErr := store.ListWorkers(run.RunID)
	if listErr != nil {
		t.Fatalf("list workers: %v", listErr)
	}
	byID := map[string]gateway.SimopsWorkerRecord{}
	for _, worker := range workers {
		byID[worker.WorkerID] = worker
	}
	if byID["scheduler-01"].Lifecycle != gateway.SimopsStopped || byID["scheduler-01"].RuntimeID != "container-scheduler" {
		t.Fatalf("unexpected launched worker outcome %#v", byID["scheduler-01"])
	}
	if byID["storage-01"].Lifecycle != gateway.SimopsFailed {
		t.Fatalf("unexpected unlaunched worker outcome %#v", byID["storage-01"])
	}
}

func TestSpoolerMapsImageInspectAndStartErrors(t *testing.T) {
	t.Run("image inspect", func(t *testing.T) {
		run := testDockerRun("RUN-IMAGE-FAIL")
		spooler := Spooler{
			Config: testDockerConfig(),
			Client: &fakeDockerClient{imageErr: errors.New("missing image")},
			Now:    fixedNow,
		}

		_, _, err := spooler.StartRunProfiles(context.Background(), run, testDockerProfiles(t, run, gateway.SimopsWorkerScheduler))
		assertLaunchError(t, err, "image_inspect", "scheduler-01")
	})

	t.Run("empty container id", func(t *testing.T) {
		run := testDockerRun("RUN-EMPTY-ID")
		spooler := Spooler{
			Config: testDockerConfig(),
			Client: &fakeDockerClient{image: image.InspectResponse{ID: "image-123"}},
			Now:    fixedNow,
		}

		_, _, err := spooler.StartRunProfiles(context.Background(), run, testDockerProfiles(t, run, gateway.SimopsWorkerStorage))
		assertLaunchError(t, err, "container_create", "storage-01")
	})

	t.Run("start cleanup", func(t *testing.T) {
		run := testDockerRun("RUN-START-FAIL")
		client := &fakeDockerClient{
			image:    image.InspectResponse{ID: "image-123"},
			create:   container.CreateResponse{ID: "container-start-fail"},
			startErr: errors.New("cannot start"),
		}
		spooler := Spooler{Config: testDockerConfig(), Client: client, Now: fixedNow}

		_, _, err := spooler.StartRunProfiles(context.Background(), run, testDockerProfiles(t, run, gateway.SimopsWorkerFabric))
		assertLaunchError(t, err, "container_start", "fabric-01")
		if !slices.Equal(client.removed, []string{"container-start-fail"}) {
			t.Fatalf("expected failed start container cleanup, got %#v", client.removed)
		}
	})
}

func TestSpoolerStopsRunWorkersByRunWorkerAndRuntimeLabels(t *testing.T) {
	run := testDockerRun("RUN-STOP-001")
	client := &fakeDockerClient{
		listed: []container.Summary{
			{ID: "container-a", Labels: map[string]string{
				"simops.run_id":          "RUN-STOP-001",
				"simops.runtime_adapter": "docker-sdk",
				"simops.role":            "ordinary-worker",
				"simops.worker_id":       "scheduler-01",
				"simops.worker_kind":     "scheduler",
			}},
			{ID: "container-b", Labels: map[string]string{
				"simops.run_id":          "RUN-STOP-001",
				"simops.runtime_adapter": "docker-sdk",
				"simops.role":            "ordinary-worker",
				"simops.worker_id":       "storage-01",
				"simops.worker_kind":     "storage",
			}},
			{ID: "container-wrong-kind", Labels: map[string]string{
				"simops.run_id":          "RUN-STOP-001",
				"simops.runtime_adapter": "docker-sdk",
				"simops.role":            "ordinary-worker",
				"simops.worker_id":       "scheduler-01",
				"simops.worker_kind":     "storage",
			}},
			{ID: "container-other-runtime", Labels: map[string]string{
				"simops.run_id":          "RUN-STOP-001",
				"simops.runtime_adapter": "manual",
				"simops.role":            "ordinary-worker",
				"simops.worker_id":       "fabric-01",
				"simops.worker_kind":     "fabric",
			}},
			{ID: "container-unlabelled"},
		},
	}
	spooler := Spooler{Config: testDockerConfig(), Client: client, Now: fixedNow}

	if err := spooler.StopRunProfiles(context.Background(), "RUN-STOP-001", testDockerProfiles(t, run, gateway.SimopsWorkerScheduler, gateway.SimopsWorkerStorage)); err != nil {
		t.Fatalf("stop run: %v", err)
	}
	if !client.listOptions.All {
		t.Fatalf("expected all containers to be listed")
	}
	if got := client.listOptions.Filters["label"]; !got["simops.run_id=RUN-STOP-001"] {
		t.Fatalf("missing run label filter in %#v", got)
	}
	if got := client.listOptions.Filters["label"]; !got["simops.runtime_adapter=docker-sdk"] || !got["simops.role=ordinary-worker"] {
		t.Fatalf("missing runtime/role label filters in %#v", got)
	}
	if !slices.Equal(client.stopped, []string{"container-a", "container-b"}) {
		t.Fatalf("unexpected stop calls %#v", client.stopped)
	}
	if !slices.Equal(client.removed, []string{"container-a", "container-b"}) {
		t.Fatalf("unexpected remove calls %#v", client.removed)
	}
}

func TestSpoolerSyncRunProfilesMapsDockerContainerStates(t *testing.T) {
	tests := []struct {
		name      string
		run       gateway.SimopsRunRecord
		summary   container.Summary
		inspect   container.InspectResponse
		want      gateway.ObservedWorkerState
		wantCause string
	}{
		{
			name:    "created is pending",
			run:     testDockerRun("RUN-SYNC-CREATED"),
			summary: dockerSummary("container-created", "RUN-SYNC-CREATED", "scheduler-01", "scheduler"),
			inspect: dockerInspect("container-created", container.State{
				Status: container.StateCreated,
			}),
			want: gateway.ObservedWorkerPending,
		},
		{
			name:    "running is active",
			run:     testDockerRun("RUN-SYNC-RUNNING"),
			summary: dockerSummary("container-running", "RUN-SYNC-RUNNING", "scheduler-01", "scheduler"),
			inspect: dockerInspect("container-running", container.State{
				Status:  container.StateRunning,
				Running: true,
			}),
			want: gateway.ObservedWorkerActive,
		},
		{
			name:    "zero exit is succeeded",
			run:     testDockerRun("RUN-SYNC-SUCCESS"),
			summary: dockerSummary("container-success", "RUN-SYNC-SUCCESS", "scheduler-01", "scheduler"),
			inspect: dockerInspect("container-success", container.State{
				Status:   container.StateExited,
				ExitCode: 0,
			}),
			want: gateway.ObservedWorkerSucceeded,
		},
		{
			name:    "nonzero exit is failed",
			run:     testDockerRun("RUN-SYNC-FAIL"),
			summary: dockerSummary("container-fail", "RUN-SYNC-FAIL", "scheduler-01", "scheduler"),
			inspect: dockerInspect("container-fail", container.State{
				Status:   container.StateExited,
				ExitCode: 17,
				Error:    "worker failed",
			}),
			want:      gateway.ObservedWorkerFailed,
			wantCause: "exit-code",
		},
		{
			name:    "dead is failed",
			run:     testDockerRun("RUN-SYNC-DEAD"),
			summary: dockerSummary("container-dead", "RUN-SYNC-DEAD", "scheduler-01", "scheduler"),
			inspect: dockerInspect("container-dead", container.State{
				Status: container.StateDead,
				Dead:   true,
				Error:  "container dead",
			}),
			want:      gateway.ObservedWorkerFailed,
			wantCause: "dead",
		},
		{
			name:    "image pull error is image-pull-failed",
			run:     testDockerRun("RUN-SYNC-IMAGE-PULL"),
			summary: dockerSummary("container-image-pull", "RUN-SYNC-IMAGE-PULL", "scheduler-01", "scheduler"),
			inspect: dockerInspect("container-image-pull", container.State{
				Status:   container.StateExited,
				ExitCode: 125,
				Error:    "pull access denied for radiant-simops-generator:test",
			}),
			want:      gateway.ObservedWorkerImagePullFailed,
			wantCause: "image-pull",
		},
		{
			name: "present container after stopped run with stop exit is stopped",
			run: func() gateway.SimopsRunRecord {
				run := testDockerRun("RUN-SYNC-STOPPED")
				run.Lifecycle = gateway.SimopsStopped
				return run
			}(),
			summary: dockerSummary("container-stopped", "RUN-SYNC-STOPPED", "scheduler-01", "scheduler"),
			inspect: dockerInspect("container-stopped", container.State{
				Status:   container.StateExited,
				ExitCode: 143,
			}),
			want:      gateway.ObservedWorkerStopped,
			wantCause: "stopped",
		},
		{
			name: "present failed container after stopped run remains failed",
			run: func() gateway.SimopsRunRecord {
				run := testDockerRun("RUN-SYNC-STOPPED-FAILED")
				run.Lifecycle = gateway.SimopsStopped
				return run
			}(),
			summary: dockerSummary("container-stopped-failed", "RUN-SYNC-STOPPED-FAILED", "scheduler-01", "scheduler"),
			inspect: dockerInspect("container-stopped-failed", container.State{
				Status:   container.StateExited,
				ExitCode: 17,
				Error:    "worker failed before stop",
			}),
			want:      gateway.ObservedWorkerFailed,
			wantCause: "exit-code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeDockerClient{
				image:   image.InspectResponse{ID: "image-123"},
				listed:  []container.Summary{tt.summary},
				inspect: map[string]container.InspectResponse{tt.summary.ID: tt.inspect},
			}
			spooler := Spooler{Config: testDockerConfig(), Client: client, Now: fixedNow}
			observations, err := spooler.SyncRunProfiles(context.Background(), tt.run, testDockerProfiles(t, tt.run, gateway.SimopsWorkerScheduler))
			if err != nil {
				t.Fatalf("sync run: %v", err)
			}
			if len(observations) != 1 {
				t.Fatalf("expected one observation, got %#v", observations)
			}
			got := observations[0]
			if got.State != tt.want {
				t.Fatalf("expected %q, got %#v", tt.want, got)
			}
			if tt.wantCause != "" && got.Reason != tt.wantCause {
				t.Fatalf("expected reason %q, got %#v", tt.wantCause, got)
			}
		})
	}
}

func TestSpoolerSyncRunProfilesCleansSucceededContainersAfterTTL(t *testing.T) {
	run := testDockerRun("RUN-SYNC-CLEANUP")
	cfg := testDockerConfig()
	cfg.WorkerAutoRemove = false
	cfg.WorkerCleanupTTL = 0
	client := &fakeDockerClient{
		image: image.InspectResponse{ID: "image-123"},
		listed: []container.Summary{
			dockerSummary("container-success", "RUN-SYNC-CLEANUP", "scheduler-01", "scheduler"),
		},
		inspect: map[string]container.InspectResponse{
			"container-success": dockerInspect("container-success", container.State{
				Status:     container.StateExited,
				ExitCode:   0,
				FinishedAt: fixedNow().Add(-time.Second).Format(time.RFC3339Nano),
			}),
		},
	}
	spooler := Spooler{Config: cfg, Client: client, Now: fixedNow}

	observations, err := spooler.SyncRunProfiles(context.Background(), run, testDockerProfilesWithConfig(t, cfg, run, gateway.SimopsWorkerScheduler))
	if err != nil {
		t.Fatalf("sync run: %v", err)
	}
	if len(observations) != 1 || observations[0].State != gateway.ObservedWorkerSucceeded {
		t.Fatalf("expected succeeded observation before cleanup, got %#v", observations)
	}
	if !slices.Equal(client.removed, []string{"container-success"}) {
		t.Fatalf("expected succeeded container cleanup, got %#v", client.removed)
	}
}

func TestSpoolerSyncRunProfilesRetainsFailedContainers(t *testing.T) {
	run := testDockerRun("RUN-SYNC-RETAIN")
	cfg := testDockerConfig()
	cfg.WorkerAutoRemove = false
	cfg.WorkerCleanupTTL = 0
	client := &fakeDockerClient{
		image: image.InspectResponse{ID: "image-123"},
		listed: []container.Summary{
			dockerSummary("container-failed", "RUN-SYNC-RETAIN", "scheduler-01", "scheduler"),
		},
		inspect: map[string]container.InspectResponse{
			"container-failed": dockerInspect("container-failed", container.State{
				Status:     container.StateExited,
				ExitCode:   2,
				Error:      "manifest validation failed",
				FinishedAt: fixedNow().Add(-time.Second).Format(time.RFC3339Nano),
			}),
		},
	}
	spooler := Spooler{Config: cfg, Client: client, Now: fixedNow}

	observations, err := spooler.SyncRunProfiles(context.Background(), run, testDockerProfilesWithConfig(t, cfg, run, gateway.SimopsWorkerScheduler))
	if err != nil {
		t.Fatalf("sync run: %v", err)
	}
	if len(observations) != 1 || observations[0].State != gateway.ObservedWorkerFailed {
		t.Fatalf("expected failed observation, got %#v", observations)
	}
	if len(client.removed) != 0 {
		t.Fatalf("expected failed container retention, got removals %#v", client.removed)
	}
}

func TestSpoolerSyncRunPreservesSucceededStateAfterCleanup(t *testing.T) {
	run := testDockerRun("RUN-SYNC-CLEANED")
	cfg := testDockerConfig()
	client := &fakeDockerClient{image: image.InspectResponse{ID: "image-123"}}
	spooler := Spooler{Config: cfg, Client: client, Now: fixedNow}

	observations, err := spooler.SyncRun(context.Background(), run, []gateway.SimopsWorkerRecord{{
		RunID:             run.RunID,
		WorkerID:          "scheduler-01",
		WorkerKind:        gateway.SimopsWorkerScheduler,
		ObservedLifecycle: gateway.ObservedWorkerSucceeded,
		Runtime:           "docker",
		RuntimeID:         "container-success",
	}})
	if err != nil {
		t.Fatalf("sync run: %v", err)
	}
	if len(observations) != 1 || observations[0].State != gateway.ObservedWorkerSucceeded {
		t.Fatalf("expected cleaned-up succeeded observation, got %#v", observations)
	}
	if observations[0].Reason != "cleaned-up" || observations[0].RuntimeID != "container-success" {
		t.Fatalf("expected cleaned-up detail, got %#v", observations[0])
	}
}

func TestSpoolerSyncRunProfilesReportsMissingStoppedAndImagePullFailure(t *testing.T) {
	run := testDockerRun("RUN-SYNC-MISSING")
	profiles := testDockerProfiles(t, run, gateway.SimopsWorkerScheduler)

	t.Run("missing container with present image", func(t *testing.T) {
		client := &fakeDockerClient{image: image.InspectResponse{ID: "image-123"}}
		spooler := Spooler{Config: testDockerConfig(), Client: client, Now: fixedNow}

		observations, err := spooler.SyncRunProfiles(context.Background(), run, profiles)
		if err != nil {
			t.Fatalf("sync run: %v", err)
		}
		if len(observations) != 1 || observations[0].State != gateway.ObservedWorkerMissing {
			t.Fatalf("expected missing observation, got %#v", observations)
		}
	})

	t.Run("missing container after stopped run", func(t *testing.T) {
		stoppedRun := run
		stoppedRun.Lifecycle = gateway.SimopsStopped
		client := &fakeDockerClient{image: image.InspectResponse{ID: "image-123"}}
		spooler := Spooler{Config: testDockerConfig(), Client: client, Now: fixedNow}

		observations, err := spooler.SyncRunProfiles(context.Background(), stoppedRun, profiles)
		if err != nil {
			t.Fatalf("sync run: %v", err)
		}
		if len(observations) != 1 || observations[0].State != gateway.ObservedWorkerStopped {
			t.Fatalf("expected stopped observation, got %#v", observations)
		}
	})

	t.Run("missing container with missing image is still missing", func(t *testing.T) {
		client := &fakeDockerClient{imageErr: errors.New("missing image")}
		spooler := Spooler{Config: testDockerConfig(), Client: client, Now: fixedNow}

		observations, err := spooler.SyncRunProfiles(context.Background(), run, profiles)
		if err != nil {
			t.Fatalf("sync run: %v", err)
		}
		if len(observations) != 1 || observations[0].State != gateway.ObservedWorkerMissing {
			t.Fatalf("expected missing observation, got %#v", observations)
		}
		if client.inspectedImage != "" {
			t.Fatalf("expected sync not to reclassify missing container through image inspection, inspected %q", client.inspectedImage)
		}
	})

	t.Run("failed run with missing container and missing image is image pull failed", func(t *testing.T) {
		failedRun := run
		failedRun.Lifecycle = gateway.SimopsFailed
		client := &fakeDockerClient{imageErr: errors.New("pull access denied")}
		spooler := Spooler{Config: testDockerConfig(), Client: client, Now: fixedNow}

		observations, err := spooler.SyncRunProfiles(context.Background(), failedRun, profiles)
		if err != nil {
			t.Fatalf("sync run: %v", err)
		}
		if len(observations) != 1 || observations[0].State != gateway.ObservedWorkerImagePullFailed {
			t.Fatalf("expected image-pull-failed observation, got %#v", observations)
		}
		if observations[0].Reason != "image_inspect" || client.inspectedImage != "radiant-simops-generator:test" {
			t.Fatalf("expected image inspect failure detail, got observation=%#v inspected=%q", observations[0], client.inspectedImage)
		}
	})
}

func testDockerConfig() gateway.SimopsConfig {
	cfg := gateway.DefaultConfig().Simops
	cfg.LaunchMode = "auto"
	cfg.WorkerRuntime = "docker"
	cfg.WorkerImage = "radiant-simops-generator:test"
	cfg.WorkerManifestRoot = "/examples/simulation-ops"
	cfg.WorkerIngestBaseURL = "http://host.docker.internal:8080"
	cfg.WorkerFrameOverride = 2
	cfg.WorkerNetwork = "radiant-simops-local"
	cfg.WorkerKubernetesNamespace = "radiant-simops"
	cfg.WorkerCleanupTTL = 10 * time.Minute
	cfg.WorkerAutoRemove = true
	return cfg
}

func testDockerRun(runID string) gateway.SimopsRunRecord {
	return gateway.SimopsRunRecord{
		RunID:           runID,
		ScenarioID:      "scheduler-drift",
		LaunchMode:      "auto",
		RuntimeLimitSec: 120,
		IngestToken:     "ingest-token",
	}
}

func testDockerProfiles(t *testing.T, run gateway.SimopsRunRecord, workers ...gateway.SimopsWorkerKind) []gateway.RunConnectionProfile {
	t.Helper()
	return testDockerProfilesWithConfig(t, testDockerConfig(), run, workers...)
}

func testDockerProfilesWithConfig(t *testing.T, cfg gateway.SimopsConfig, run gateway.SimopsRunRecord, workers ...gateway.SimopsWorkerKind) []gateway.RunConnectionProfile {
	t.Helper()
	profiles, err := gateway.BuildRunWorkerConnectionProfiles(cfg, run, workers)
	if err != nil {
		t.Fatalf("build profiles: %v", err)
	}
	return profiles
}

func assertLaunchError(t *testing.T, err error, operation string, workerID string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected launch error")
	}
	var launchErr LaunchError
	if !errors.As(err, &launchErr) {
		t.Fatalf("expected LaunchError, got %T", err)
	}
	if launchErr.Operation != operation || launchErr.WorkerID != workerID {
		t.Fatalf("unexpected launch error %#v", launchErr)
	}
}

func fixedNow() time.Time {
	return time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
}

type fakeDockerClient struct {
	image               image.InspectResponse
	imageErr            error
	create              container.CreateResponse
	createErr           error
	createSequence      []container.CreateResponse
	createErrorSequence []error
	createCalls         int
	startErr            error
	listed              []container.Summary
	listErr             error
	inspect             map[string]container.InspectResponse
	inspectErr          error
	stopErr             error
	removeErr           error

	inspectedImage      string
	createdConfig       container.Config
	createdHostConfig   container.HostConfig
	createdNetworking   network.NetworkingConfig
	createdPlatform     *v1.Platform
	createdName         string
	startedContainer    string
	listOptions         dockerclient.ContainerListOptions
	inspectedContainers []string
	stopped             []string
	removed             []string
}

func (c *fakeDockerClient) ImageInspect(_ context.Context, imageID string) (image.InspectResponse, error) {
	c.inspectedImage = imageID
	return c.image, c.imageErr
}

func (c *fakeDockerClient) ContainerCreate(_ context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *v1.Platform, containerName string) (container.CreateResponse, error) {
	if config != nil {
		c.createdConfig = *config
	}
	if hostConfig != nil {
		c.createdHostConfig = *hostConfig
	}
	if networkingConfig != nil {
		c.createdNetworking = *networkingConfig
	}
	c.createdPlatform = platform
	c.createdName = containerName
	if c.createCalls < len(c.createSequence) {
		response := c.createSequence[c.createCalls]
		var err error
		if c.createCalls < len(c.createErrorSequence) {
			err = c.createErrorSequence[c.createCalls]
		}
		c.createCalls++
		return response, err
	}
	c.createCalls++
	return c.create, c.createErr
}

func (c *fakeDockerClient) ContainerStart(_ context.Context, containerID string, _ dockerclient.ContainerStartOptions) error {
	c.startedContainer = containerID
	return c.startErr
}

func (c *fakeDockerClient) ContainerList(_ context.Context, options dockerclient.ContainerListOptions) ([]container.Summary, error) {
	c.listOptions = options
	return c.listed, c.listErr
}

func (c *fakeDockerClient) ContainerInspect(_ context.Context, containerID string) (container.InspectResponse, error) {
	c.inspectedContainers = append(c.inspectedContainers, containerID)
	if c.inspectErr != nil {
		return container.InspectResponse{}, c.inspectErr
	}
	if c.inspect == nil {
		return container.InspectResponse{}, errors.New("container not found")
	}
	response, ok := c.inspect[containerID]
	if !ok {
		return container.InspectResponse{}, errors.New("container not found")
	}
	return response, nil
}

func (c *fakeDockerClient) ContainerStop(_ context.Context, containerID string, _ dockerclient.ContainerStopOptions) error {
	c.stopped = append(c.stopped, containerID)
	return c.stopErr
}

func (c *fakeDockerClient) ContainerRemove(_ context.Context, containerID string, _ dockerclient.ContainerRemoveOptions) error {
	c.removed = append(c.removed, containerID)
	return c.removeErr
}

func dockerSummary(containerID string, runID string, workerID string, workerKind string) container.Summary {
	return container.Summary{
		ID:    containerID,
		State: container.StateRunning,
		Labels: map[string]string{
			"simops.run_id":          runID,
			"simops.runtime_adapter": "docker-sdk",
			"simops.role":            string(gateway.RunConnectionRoleOrdinaryWorker),
			"simops.worker_id":       workerID,
			"simops.worker_kind":     workerKind,
		},
	}
}

func dockerInspect(containerID string, state container.State) container.InspectResponse {
	return container.InspectResponse{
		ID:    containerID,
		State: &state,
	}
}
