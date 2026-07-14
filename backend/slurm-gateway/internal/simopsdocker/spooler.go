package simopsdocker

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"radiant/slurm-gateway/internal/gateway"
)

type DockerClient interface {
	ImageInspect(ctx context.Context, imageID string) (image.InspectResponse, error)
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error)
	ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
}

type Spooler struct {
	Config gateway.SimopsConfig
	Client DockerClient
	Now    func() time.Time
}

type LaunchError struct {
	Operation     string
	RunID         string
	WorkerID      string
	ContainerName string
	Image         string
	Err           error
}

func (e LaunchError) Error() string {
	target := strings.TrimSpace(e.ContainerName)
	if target == "" {
		target = strings.TrimSpace(e.WorkerID)
	}
	if target == "" {
		target = strings.TrimSpace(e.Image)
	}
	message := "simops docker launch"
	if e.Operation != "" {
		message += " " + e.Operation
	}
	if target != "" {
		message += " for " + target
	}
	if e.Err != nil {
		message += ": " + e.Err.Error()
	}
	return message
}

func (e LaunchError) Unwrap() error {
	return e.Err
}

func NewSpooler(cfg gateway.SimopsConfig) (Spooler, error) {
	client, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return Spooler{}, fmt.Errorf("create docker client: %w", err)
	}
	return Spooler{
		Config: cfg,
		Client: engineClient{client: client},
		Now:    time.Now,
	}, nil
}

func (s Spooler) StartRun(ctx context.Context, run gateway.SimopsRunRecord, workers []gateway.SimopsWorkerKind) ([]gateway.SimopsWorkerRecord, []gateway.SimopsSpoolCommand, error) {
	profiles, err := gateway.BuildRunWorkerConnectionProfiles(s.Config, run, workers)
	if err != nil {
		return nil, nil, err
	}
	return s.StartRunProfiles(ctx, run, profiles)
}

func (s Spooler) StartRunProfiles(ctx context.Context, run gateway.SimopsRunRecord, profiles []gateway.RunConnectionProfile) ([]gateway.SimopsWorkerRecord, []gateway.SimopsSpoolCommand, error) {
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	default:
	}
	if s.Client == nil {
		return nil, nil, fmt.Errorf("docker client is required")
	}

	records := make([]gateway.SimopsWorkerRecord, 0, len(profiles))
	commands := make([]gateway.SimopsSpoolCommand, 0, len(profiles))
	for _, profile := range profiles {
		if err := validateLaunchProfile(run, profile); err != nil {
			return records, commands, err
		}
		containerID, err := s.startWorker(ctx, profile)
		if err != nil {
			return records, commands, err
		}

		now := s.now()
		records = append(records, gateway.SimopsWorkerRecord{
			RunID:      run.RunID,
			WorkerID:   profile.WorkerID,
			WorkerKind: profile.WorkerKind,
			Lifecycle:  gateway.SimopsStarting,
			LaunchMode: profile.LaunchMode,
			Endpoint:   profile.Gateway.IngestURL,
			Runtime:    "docker",
			RuntimeID:  containerID,
			UpdatedAt:  now,
			Labels:     launchLabels(profile, containerID),
		})
		commands = append(commands, gateway.SimopsSpoolCommand{
			CommandID: fmt.Sprintf("%s-%s-start", run.RunID, profile.WorkerID),
			RunID:     run.RunID,
			WorkerID:  profile.WorkerID,
			Mode:      profile.LaunchMode,
			State:     gateway.SimopsStarting,
			Message:   fmt.Sprintf("Worker container launched as %s (%s)", profile.Runtime.Docker.ContainerName, containerID),
			Metadata:  launchMetadata(profile, containerID),
			CreatedAt: now,
			UpdatedAt: now,
		})
	}

	return records, commands, nil
}

func (s Spooler) StopRun(ctx context.Context, runID string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if s.Client == nil {
		return fmt.Errorf("docker client is required")
	}
	return s.tryStopRunWorkers(ctx, strings.TrimSpace(runID))
}

func (s Spooler) SyncRun(ctx context.Context, run gateway.SimopsRunRecord, workers []gateway.SimopsWorkerRecord) ([]gateway.ObservedWorkerLifecycle, error) {
	profiles, err := gateway.BuildRunWorkerConnectionProfilesForRecords(s.Config, run, workers)
	if err != nil {
		return nil, err
	}
	return s.syncRunProfiles(ctx, run, profiles, workerRecordsByID(workers))
}

func (s Spooler) StopRunProfiles(ctx context.Context, runID string, profiles []gateway.RunConnectionProfile) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if s.Client == nil {
		return fmt.Errorf("docker client is required")
	}
	return s.tryStopRunProfiles(ctx, strings.TrimSpace(runID), profiles)
}

func (s Spooler) SyncRunProfiles(ctx context.Context, run gateway.SimopsRunRecord, profiles []gateway.RunConnectionProfile) ([]gateway.ObservedWorkerLifecycle, error) {
	return s.syncRunProfiles(ctx, run, profiles, nil)
}

func (s Spooler) syncRunProfiles(ctx context.Context, run gateway.SimopsRunRecord, profiles []gateway.RunConnectionProfile, priorWorkers map[string]gateway.SimopsWorkerRecord) ([]gateway.ObservedWorkerLifecycle, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if s.Client == nil {
		return nil, fmt.Errorf("docker client is required")
	}

	runID := strings.TrimSpace(run.RunID)
	containers, err := s.Client.ContainerList(ctx, container.ListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", "simops.run_id="+runID),
			filters.Arg("label", "simops.runtime_adapter=docker-sdk"),
			filters.Arg("label", "simops.role="+string(gateway.RunConnectionRoleOrdinaryWorker)),
		),
	})
	if err != nil {
		return nil, fmt.Errorf("list simops worker containers for run %s: %w", runID, err)
	}

	byWorker := make(map[string]container.Summary, len(containers))
	expectedWorkers := profileWorkerSet(profiles)
	for _, item := range containers {
		if !matchesRunWorker(runID, item.Labels, expectedWorkers) {
			continue
		}
		workerID := strings.TrimSpace(item.Labels["simops.worker_id"])
		if workerID != "" {
			byWorker[workerID] = item
		}
	}

	observations := make([]gateway.ObservedWorkerLifecycle, 0, len(profiles))
	for _, profile := range profiles {
		item, ok := byWorker[profile.WorkerID]
		if !ok {
			if observation, ok := cleanedUpSucceededObservation(run, profile, priorWorkers[profile.WorkerID], s.now()); ok {
				observations = append(observations, observation)
				continue
			}
			observations = append(observations, s.missingWorkerObservation(ctx, run, profile))
			continue
		}
		inspect, err := s.Client.ContainerInspect(ctx, item.ID)
		if err != nil {
			observations = append(observations, observedFromProfile(run, profile, gateway.ObservedWorkerMissing, item.ID, "container_inspect", err.Error(), nil, item.Labels, s.now()))
			continue
		}
		observation := observedFromDockerState(run, profile, item, inspect, s.now())
		if succeededCleanupDue(profile, inspect, observation, s.now()) {
			if err := s.Client.ContainerRemove(ctx, observation.RuntimeID, container.RemoveOptions{Force: true}); err != nil {
				return nil, fmt.Errorf("cleanup succeeded simops worker container %s for run %s: %w", observation.RuntimeID, runID, err)
			}
		}
		observations = append(observations, observation)
	}
	return observations, nil
}

func (s Spooler) startWorker(ctx context.Context, profile gateway.RunConnectionProfile) (string, error) {
	if strings.TrimSpace(profile.WorkerImage) == "" {
		return "", launchError(profile, "image_required", errors.New("worker image is required"))
	}
	if _, err := s.Client.ImageInspect(ctx, profile.WorkerImage); err != nil {
		return "", launchError(profile, "image_inspect", err)
	}

	response, err := s.Client.ContainerCreate(
		ctx,
		&container.Config{
			Image:  profile.WorkerImage,
			Labels: containerLabels(profile),
			Env:    dockerEnv(profile),
			Cmd:    gateway.BuildRunWorkerCommand(profile, s.Config.WorkerFrameOverride),
		},
		dockerHostConfig(profile),
		dockerNetworking(profile),
		nil,
		profile.Runtime.Docker.ContainerName,
	)
	if err != nil {
		return "", launchError(profile, "container_create", err)
	}

	containerID := strings.TrimSpace(response.ID)
	if containerID == "" {
		return "", launchError(profile, "container_create", errors.New("docker returned empty container id"))
	}
	if err := s.Client.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		_ = s.Client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
		return "", launchError(profile, "container_start", err)
	}
	return containerID, nil
}

func (s Spooler) missingWorkerObservation(ctx context.Context, run gateway.SimopsRunRecord, profile gateway.RunConnectionProfile) gateway.ObservedWorkerLifecycle {
	if run.Lifecycle == gateway.SimopsStopped {
		return observedFromProfile(run, profile, gateway.ObservedWorkerStopped, "", "stopped", "worker runtime resource is absent after stop", nil, profile.Labels, s.now())
	}
	if run.Lifecycle == gateway.SimopsFailed && strings.TrimSpace(profile.WorkerImage) != "" {
		if _, err := s.Client.ImageInspect(ctx, profile.WorkerImage); err != nil {
			return observedFromProfile(run, profile, gateway.ObservedWorkerImagePullFailed, "", "image_inspect", err.Error(), nil, profile.Labels, s.now())
		}
	}
	return observedFromProfile(run, profile, gateway.ObservedWorkerMissing, "", "missing", "expected worker runtime resource was not found", nil, profile.Labels, s.now())
}

func observedFromDockerState(run gateway.SimopsRunRecord, profile gateway.RunConnectionProfile, summary container.Summary, inspect container.InspectResponse, now time.Time) gateway.ObservedWorkerLifecycle {
	state := inspect.State
	if state == nil {
		return observedFromProfile(run, profile, gateway.ObservedWorkerMissing, summary.ID, "container_inspect", "container inspect response did not include state", nil, summary.Labels, now)
	}
	runtimeID := strings.TrimSpace(inspect.ID)
	if runtimeID == "" {
		runtimeID = strings.TrimSpace(summary.ID)
	}
	status := strings.TrimSpace(state.Status)
	exitCode := dockerIntPtr(state.ExitCode)
	switch {
	case isImagePullFailure(state):
		return observedFromProfile(run, profile, gateway.ObservedWorkerImagePullFailed, runtimeID, "image-pull", state.Error, exitCode, summary.Labels, now)
	case state.OOMKilled:
		return observedFromProfile(run, profile, gateway.ObservedWorkerFailed, runtimeID, "oom-killed", state.Error, exitCode, summary.Labels, now)
	case state.Dead || status == container.StateDead:
		return observedFromProfile(run, profile, gateway.ObservedWorkerFailed, runtimeID, "dead", state.Error, exitCode, summary.Labels, now)
	case status == container.StateCreated:
		return observedFromProfile(run, profile, gateway.ObservedWorkerPending, runtimeID, "created", "container exists but is not running", nil, summary.Labels, now)
	case status == container.StateRunning:
		return observedFromProfile(run, profile, gateway.ObservedWorkerActive, runtimeID, "running", "container is running", nil, summary.Labels, now)
	case status == container.StateRestarting:
		return observedFromProfile(run, profile, gateway.ObservedWorkerActive, runtimeID, "restarting", "container is restarting", nil, summary.Labels, now)
	case status == container.StatePaused:
		return observedFromProfile(run, profile, gateway.ObservedWorkerActive, runtimeID, "paused", "container is paused", nil, summary.Labels, now)
	case status == container.StateExited && state.ExitCode == 0:
		return observedFromProfile(run, profile, gateway.ObservedWorkerSucceeded, runtimeID, "exited", "container exited successfully", exitCode, summary.Labels, now)
	case run.Lifecycle == gateway.SimopsStopped && status == container.StateExited && isStopExitCode(state.ExitCode):
		return observedFromProfile(run, profile, gateway.ObservedWorkerStopped, runtimeID, "stopped", "container exited after run stop", exitCode, summary.Labels, now)
	case status == container.StateExited:
		return observedFromProfile(run, profile, gateway.ObservedWorkerFailed, runtimeID, "exit-code", state.Error, exitCode, summary.Labels, now)
	case status == container.StateRemoving:
		return observedFromProfile(run, profile, gateway.ObservedWorkerPending, runtimeID, "removing", "container is being removed", nil, summary.Labels, now)
	default:
		return observedFromProfile(run, profile, gateway.ObservedWorkerPending, runtimeID, status, "container state is pending classification", nil, summary.Labels, now)
	}
}

func isStopExitCode(exitCode int) bool {
	return exitCode == 130 || exitCode == 137 || exitCode == 143
}

func isImagePullFailure(state *container.State) bool {
	if state == nil {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(state.Error + " " + state.Status))
	for _, marker := range []string{
		"errimagepull",
		"imagepullbackoff",
		"image pull",
		"pull access denied",
		"manifest unknown",
		"no such image",
		"repository does not exist",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func observedFromProfile(run gateway.SimopsRunRecord, profile gateway.RunConnectionProfile, state gateway.ObservedWorkerState, runtimeID string, reason string, message string, exitCode *int, labels map[string]string, now time.Time) gateway.ObservedWorkerLifecycle {
	return gateway.ObservedWorkerLifecycle{
		RunID:      run.RunID,
		WorkerID:   profile.WorkerID,
		WorkerKind: profile.WorkerKind,
		State:      state,
		Runtime:    "docker",
		RuntimeID:  runtimeID,
		Reason:     reason,
		Message:    message,
		ExitCode:   exitCode,
		ObservedAt: now.UTC(),
		Labels:     copyLabels(labels),
	}
}

func workerRecordsByID(workers []gateway.SimopsWorkerRecord) map[string]gateway.SimopsWorkerRecord {
	byID := make(map[string]gateway.SimopsWorkerRecord, len(workers))
	for _, worker := range workers {
		workerID := strings.TrimSpace(worker.WorkerID)
		if workerID != "" {
			byID[workerID] = worker
		}
	}
	return byID
}

func cleanedUpSucceededObservation(run gateway.SimopsRunRecord, profile gateway.RunConnectionProfile, worker gateway.SimopsWorkerRecord, now time.Time) (gateway.ObservedWorkerLifecycle, bool) {
	if worker.ObservedLifecycle != gateway.ObservedWorkerSucceeded {
		return gateway.ObservedWorkerLifecycle{}, false
	}
	return observedFromProfile(
		run,
		profile,
		gateway.ObservedWorkerSucceeded,
		worker.RuntimeID,
		"cleaned-up",
		"worker runtime resource was cleaned up after success",
		worker.ObservedExitCode,
		worker.Labels,
		now,
	), true
}

func succeededCleanupDue(profile gateway.RunConnectionProfile, inspect container.InspectResponse, observation gateway.ObservedWorkerLifecycle, now time.Time) bool {
	if observation.State != gateway.ObservedWorkerSucceeded || strings.TrimSpace(observation.RuntimeID) == "" {
		return false
	}
	if profile.Cleanup.TTLSecondsAfterFinished < 0 {
		return false
	}
	state := inspect.State
	if state == nil {
		return false
	}
	ttl := time.Duration(profile.Cleanup.TTLSecondsAfterFinished) * time.Second
	finishedAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(state.FinishedAt))
	if err != nil || finishedAt.IsZero() {
		return ttl == 0
	}
	return !now.Before(finishedAt.Add(ttl))
}

func dockerIntPtr(value int) *int {
	return &value
}

func (s Spooler) tryStopRunWorkers(ctx context.Context, runID string) error {
	return s.tryStopRunProfiles(ctx, runID, nil)
}

func (s Spooler) tryStopRunProfiles(ctx context.Context, runID string, profiles []gateway.RunConnectionProfile) error {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil
	}
	workers := profileWorkerSet(profiles)
	containers, err := s.Client.ContainerList(ctx, container.ListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", "simops.run_id="+runID),
			filters.Arg("label", "simops.runtime_adapter=docker-sdk"),
			filters.Arg("label", "simops.role="+string(gateway.RunConnectionRoleOrdinaryWorker)),
		),
	})
	if err != nil {
		return fmt.Errorf("list simops worker containers for run %s: %w", runID, err)
	}

	var firstErr error
	for _, item := range containers {
		if !matchesRunWorker(runID, item.Labels, workers) {
			continue
		}
		containerID := strings.TrimSpace(item.ID)
		if containerID == "" {
			continue
		}
		timeout := 10
		if err := s.Client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("stop simops worker container %s: %w", containerID, err)
		}
		if err := s.Client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("remove simops worker container %s: %w", containerID, err)
		}
	}
	return firstErr
}

func validateLaunchProfile(run gateway.SimopsRunRecord, profile gateway.RunConnectionProfile) error {
	if strings.TrimSpace(profile.RunID) != strings.TrimSpace(run.RunID) {
		return launchError(profile, "profile_validate", fmt.Errorf("profile run %q does not match run %q", profile.RunID, run.RunID))
	}
	if strings.TrimSpace(profile.WorkerID) == "" {
		return launchError(profile, "profile_validate", fmt.Errorf("worker identity is required"))
	}
	if strings.TrimSpace(string(profile.WorkerKind)) == "" {
		return launchError(profile, "profile_validate", fmt.Errorf("worker kind is required"))
	}
	return nil
}

func (s Spooler) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func dockerEnv(profile gateway.RunConnectionProfile) []string {
	env := []string{
		"SIMOPS_RUN_ID=" + profile.RunID,
		"SIMOPS_WORKER_ID=" + profile.WorkerID,
		"SIMOPS_WORKER_KIND=" + string(profile.WorkerKind),
		"SIMOPS_ROLE=" + string(profile.Role),
		"SIMOPS_LAUNCH_MODE=" + profile.LaunchMode,
		"SIMOPS_SCENARIO_ID=" + profile.ScenarioID,
		"SIMOPS_INGEST_URL=" + profile.Gateway.IngestURL,
		"SIMOPS_INGEST_TOKEN=" + profile.Gateway.IngestToken,
		"SIMOPS_RESULT_INGEST_URL=" + profile.Gateway.ResultIngestURL,
		"SIMOPS_RESULT_INGEST_TOKEN=" + profile.Gateway.IngestToken,
	}
	if profile.Cleanup.TTLSecondsAfterFinished > 0 {
		env = append(env, "SIMOPS_CLEANUP_TTL_SECONDS="+strconv.Itoa(int(profile.Cleanup.TTLSecondsAfterFinished)))
	}
	return env
}

func dockerHostConfig(profile gateway.RunConnectionProfile) *container.HostConfig {
	hostConfig := &container.HostConfig{
		AutoRemove: profile.Runtime.Docker.AutoRemove,
	}
	if networkName := strings.TrimSpace(profile.Runtime.Docker.Network); networkName != "" {
		hostConfig.NetworkMode = container.NetworkMode(networkName)
	}
	return hostConfig
}

func dockerNetworking(profile gateway.RunConnectionProfile) *network.NetworkingConfig {
	networkName := strings.TrimSpace(profile.Runtime.Docker.Network)
	if networkName == "" {
		return nil
	}
	return &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			networkName: {},
		},
	}
}

func containerLabels(profile gateway.RunConnectionProfile) map[string]string {
	labels := copyLabels(profile.Labels)
	labels["simops.runtime"] = "docker"
	labels["simops.runtime_adapter"] = "docker-sdk"
	labels["simops.worker_image"] = profile.WorkerImage
	labels["simops.worker_mode"] = profile.LaunchMode
	labels["simops.launch_script"] = "simops-generator"
	labels["simops.docker_network"] = profile.Runtime.Docker.Network
	labels["simops.kubernetes_namespace"] = profile.Runtime.Kubernetes.Namespace
	return labels
}

func launchLabels(profile gateway.RunConnectionProfile, containerID string) map[string]string {
	labels := containerLabels(profile)
	labels["simops.container_id"] = containerID
	return labels
}

func launchMetadata(profile gateway.RunConnectionProfile, containerID string) map[string]string {
	return map[string]string{
		"runtime_adapter": "docker-sdk",
		"container_id":    containerID,
		"container_name":  profile.Runtime.Docker.ContainerName,
		"worker_image":    profile.WorkerImage,
		"worker_id":       profile.WorkerID,
		"worker_kind":     string(profile.WorkerKind),
		"docker_network":  profile.Runtime.Docker.Network,
	}
}

func copyLabels(labels map[string]string) map[string]string {
	copied := make(map[string]string, len(labels))
	for key, value := range labels {
		copied[key] = value
	}
	return copied
}

func launchError(profile gateway.RunConnectionProfile, operation string, err error) LaunchError {
	return LaunchError{
		Operation:     operation,
		RunID:         profile.RunID,
		WorkerID:      profile.WorkerID,
		ContainerName: profile.Runtime.Docker.ContainerName,
		Image:         profile.WorkerImage,
		Err:           err,
	}
}

func profileWorkerSet(profiles []gateway.RunConnectionProfile) map[string]gateway.SimopsWorkerKind {
	if len(profiles) == 0 {
		return nil
	}
	workers := make(map[string]gateway.SimopsWorkerKind, len(profiles))
	for _, profile := range profiles {
		workerID := strings.TrimSpace(profile.WorkerID)
		if workerID == "" {
			continue
		}
		workers[workerID] = profile.WorkerKind
	}
	return workers
}

func matchesRunWorker(runID string, labels map[string]string, workers map[string]gateway.SimopsWorkerKind) bool {
	if labels["simops.run_id"] != runID {
		return false
	}
	if labels["simops.runtime_adapter"] != "docker-sdk" {
		return false
	}
	if labels["simops.role"] != string(gateway.RunConnectionRoleOrdinaryWorker) {
		return false
	}
	workerID := strings.TrimSpace(labels["simops.worker_id"])
	workerKind := strings.TrimSpace(labels["simops.worker_kind"])
	if workerID == "" || workerKind == "" {
		return false
	}
	if len(workers) == 0 {
		return true
	}
	expectedKind, ok := workers[workerID]
	return ok && string(expectedKind) == workerKind
}

type engineClient struct {
	client *dockerclient.Client
}

func (c engineClient) ImageInspect(ctx context.Context, imageID string) (image.InspectResponse, error) {
	return c.client.ImageInspect(ctx, imageID)
}

func (c engineClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error) {
	return c.client.ContainerCreate(ctx, config, hostConfig, networkingConfig, platform, containerName)
}

func (c engineClient) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	return c.client.ContainerStart(ctx, containerID, options)
}

func (c engineClient) ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
	return c.client.ContainerList(ctx, options)
}

func (c engineClient) ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error) {
	return c.client.ContainerInspect(ctx, containerID)
}

func (c engineClient) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	return c.client.ContainerStop(ctx, containerID, options)
}

func (c engineClient) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	return c.client.ContainerRemove(ctx, containerID, options)
}
