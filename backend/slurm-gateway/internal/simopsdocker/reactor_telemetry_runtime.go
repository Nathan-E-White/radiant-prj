package simopsdocker

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"

	"radiant/slurm-gateway/internal/gateway"
)

type ReactorTelemetryRuntime struct {
	Client  DockerClient
	Image   string
	Network string
}

func NewReactorTelemetryRuntime(imageName, networkName string) (ReactorTelemetryRuntime, error) {
	client, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return ReactorTelemetryRuntime{}, fmt.Errorf("create Docker client for reactor telemetry: %w", err)
	}
	return ReactorTelemetryRuntime{Client: engineClient{client: client}, Image: imageName, Network: networkName}, nil
}

func (r ReactorTelemetryRuntime) StartWorkerSet(ctx context.Context, launch gateway.ReactorTelemetryLaunch) error {
	if r.Client == nil {
		return fmt.Errorf("reactor telemetry Docker client is required")
	}
	if strings.TrimSpace(r.Image) == "" {
		return fmt.Errorf("reactor telemetry worker image is required")
	}
	if len(launch.Workers) == 0 || len(launch.Workers) > 3 {
		return fmt.Errorf("reactor telemetry launch must contain between one and three workers")
	}
	if strings.TrimSpace(launch.SetID) == "" {
		return fmt.Errorf("reactor telemetry set ID is required")
	}
	if _, err := r.Client.ImageInspect(ctx, r.Image); err != nil {
		return fmt.Errorf("inspect reactor telemetry image %s: %w", r.Image, err)
	}
	if err := r.removeWorkerSetContainers(ctx, launch.SetID); err != nil {
		return fmt.Errorf("reconcile existing reactor telemetry worker set %s: %w", launch.SetID, err)
	}
	created := make([]string, 0, len(launch.Workers))
	rollback := func() {
		for _, containerID := range created {
			_ = r.Client.ContainerRemove(context.Background(), containerID, container.RemoveOptions{Force: true})
		}
	}
	for _, worker := range launch.Workers {
		if err := validateReactorTelemetryWorker(worker); err != nil {
			rollback()
			return err
		}
		config := &container.Config{
			Image: r.Image,
			Cmd: []string{
				"--source-id", worker.SourceID,
				"--reactor-id", worker.ReactorID,
				"--worker-index", strconv.Itoa(worker.WorkerIndex),
				"--ingest-base-url", worker.Gateway.IngestBaseURL,
				"--ingest-token", worker.Gateway.IngestToken,
				"--interval-ms", "1000",
				"--max-frames", strconv.FormatUint(worker.MaxFrames, 10),
			},
			Labels: map[string]string{
				"radiant.worker.role":              "resident-source",
				"radiant.reactor-telemetry.set-id": launch.SetID,
				"radiant.game-session-id":          worker.GameSessionID,
				"radiant.reactor-id":               worker.ReactorID,
				"radiant.resident-source-id":       worker.SourceID,
			},
		}
		hostConfig := &container.HostConfig{
			NetworkMode:    container.NetworkMode(strings.TrimSpace(r.Network)),
			ExtraHosts:     []string{"host.docker.internal:host-gateway"},
			AutoRemove:     false,
			ReadonlyRootfs: true,
			SecurityOpt:    []string{"no-new-privileges:true"},
			CapDrop:        []string{"ALL"},
			RestartPolicy:  container.RestartPolicy{Name: container.RestartPolicyOnFailure, MaximumRetryCount: 10},
		}
		name := "radiant-" + worker.WorkerID
		createdContainer, err := r.Client.ContainerCreate(ctx, config, hostConfig, &network.NetworkingConfig{}, nil, name)
		if err != nil {
			rollback()
			return fmt.Errorf("create reactor telemetry worker %s: %w", worker.WorkerID, err)
		}
		created = append(created, createdContainer.ID)
		if err := r.Client.ContainerStart(ctx, createdContainer.ID, container.StartOptions{}); err != nil {
			rollback()
			return fmt.Errorf("start reactor telemetry worker %s: %w", worker.WorkerID, err)
		}
	}
	return nil
}

func (r ReactorTelemetryRuntime) StopWorkerSet(ctx context.Context, setID string) error {
	if r.Client == nil {
		return fmt.Errorf("reactor telemetry Docker client is required")
	}
	if strings.TrimSpace(setID) == "" {
		return fmt.Errorf("reactor telemetry set ID is required")
	}
	return r.removeWorkerSetContainers(ctx, setID)
}

func (r ReactorTelemetryRuntime) removeWorkerSetContainers(ctx context.Context, setID string) error {
	listed, err := r.Client.ContainerList(ctx, container.ListOptions{All: true, Filters: filters.NewArgs(
		filters.Arg("label", "radiant.worker.role=resident-source"),
		filters.Arg("label", "radiant.reactor-telemetry.set-id="+setID),
	)})
	if err != nil {
		return fmt.Errorf("list reactor telemetry workers for %s: %w", setID, err)
	}
	timeout := 10
	for _, item := range listed {
		if err := r.Client.ContainerStop(ctx, item.ID, container.StopOptions{Timeout: &timeout}); err != nil {
			return fmt.Errorf("stop reactor telemetry container %s: %w", item.ID, err)
		}
		if err := r.Client.ContainerRemove(ctx, item.ID, container.RemoveOptions{Force: true}); err != nil {
			return fmt.Errorf("remove reactor telemetry container %s: %w", item.ID, err)
		}
	}
	return nil
}

func validateReactorTelemetryWorker(worker gateway.ReactorTelemetryWorker) error {
	if strings.TrimSpace(worker.WorkerID) == "" || strings.TrimSpace(worker.SourceID) == "" || strings.TrimSpace(worker.GameSessionID) == "" || strings.TrimSpace(worker.ReactorID) == "" {
		return fmt.Errorf("reactor telemetry worker identity is incomplete")
	}
	if worker.WorkerIndex < 0 || worker.WorkerIndex >= 3 {
		return fmt.Errorf("reactor telemetry worker index must be 0, 1, or 2")
	}
	if worker.MaxFrames == 0 || worker.MaxFrames > 86400 {
		return fmt.Errorf("reactor telemetry worker max frames must be between 1 and 86400")
	}
	if strings.TrimSpace(worker.Gateway.IngestBaseURL) == "" || strings.TrimSpace(worker.Gateway.IngestToken) == "" {
		return fmt.Errorf("reactor telemetry worker gateway profile is incomplete")
	}
	if worker.BrokerURL != "" || worker.DatabaseURL != "" || worker.LakeURL != "" || worker.ContainerSocket != "" || worker.ClusterCredential != "" {
		return fmt.Errorf("reactor telemetry worker profile contains forbidden credentials")
	}
	return nil
}
