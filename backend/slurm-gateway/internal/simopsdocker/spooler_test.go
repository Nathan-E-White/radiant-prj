package simopsdocker

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
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
	if got := client.listOptions.Filters.Get("label"); !slices.Contains(got, "simops.run_id=RUN-STOP-001") {
		t.Fatalf("missing run label filter in %#v", got)
	}
	if got := client.listOptions.Filters.Get("label"); !slices.Contains(got, "simops.runtime_adapter=docker-sdk") || !slices.Contains(got, "simops.role=ordinary-worker") {
		t.Fatalf("missing runtime/role label filters in %#v", got)
	}
	if !slices.Equal(client.stopped, []string{"container-a", "container-b"}) {
		t.Fatalf("unexpected stop calls %#v", client.stopped)
	}
	if !slices.Equal(client.removed, []string{"container-a", "container-b"}) {
		t.Fatalf("unexpected remove calls %#v", client.removed)
	}
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
	profiles, err := gateway.BuildRunWorkerConnectionProfiles(testDockerConfig(), run, workers)
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
	image     image.InspectResponse
	imageErr  error
	create    container.CreateResponse
	createErr error
	startErr  error
	listed    []container.Summary
	listErr   error
	stopErr   error
	removeErr error

	inspectedImage    string
	createdConfig     container.Config
	createdHostConfig container.HostConfig
	createdNetworking network.NetworkingConfig
	createdPlatform   *v1.Platform
	createdName       string
	startedContainer  string
	listOptions       container.ListOptions
	stopped           []string
	removed           []string
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
	return c.create, c.createErr
}

func (c *fakeDockerClient) ContainerStart(_ context.Context, containerID string, _ container.StartOptions) error {
	c.startedContainer = containerID
	return c.startErr
}

func (c *fakeDockerClient) ContainerList(_ context.Context, options container.ListOptions) ([]container.Summary, error) {
	c.listOptions = options
	return c.listed, c.listErr
}

func (c *fakeDockerClient) ContainerStop(_ context.Context, containerID string, _ container.StopOptions) error {
	c.stopped = append(c.stopped, containerID)
	return c.stopErr
}

func (c *fakeDockerClient) ContainerRemove(_ context.Context, containerID string, _ container.RemoveOptions) error {
	c.removed = append(c.removed, containerID)
	return c.removeErr
}
