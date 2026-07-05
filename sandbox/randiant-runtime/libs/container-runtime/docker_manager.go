package containerruntime

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
)

type DockerManager struct {
	client *client.Client
}

func NewDockerManagerFromEnv() (*DockerManager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &DockerManager{client: cli}, nil
}

func NewDockerManager(clientRef *client.Client) *DockerManager {
	return &DockerManager{client: clientRef}
}

func (m *DockerManager) Close() error {
	if m == nil || m.client == nil {
		return nil
	}
	return m.client.Close()
}

func (m *DockerManager) Start(ctx context.Context, spec ContainerSpec) (ContainerHandle, error) {
	if m == nil || m.client == nil {
		return ContainerHandle{}, fmt.Errorf("docker manager is not initialized")
	}
	if strings.TrimSpace(spec.Image) == "" {
		return ContainerHandle{}, fmt.Errorf("image is required")
	}

	if err := m.ensureImage(ctx, spec.Image); err != nil {
		return ContainerHandle{}, err
	}

	cfg, hostConfig := buildContainerCreateInputs(spec)

	containerName := spec.Name
	resp, err := m.client.ContainerCreate(ctx, cfg, hostConfig, nil, nil, containerName)
	if err != nil {
		return ContainerHandle{}, fmt.Errorf("create container: %w", err)
	}

	if err := m.client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		_ = m.client.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
		return ContainerHandle{}, fmt.Errorf("start container: %w", err)
	}

	handle := ContainerHandle{ID: resp.ID}
	if spec.HealthProbeTimeout > 0 {
		if err := m.waitUntilRunning(ctx, handle.ID, spec.HealthProbeTimeout); err != nil {
			_ = m.Stop(ctx, handle.ID, 5*time.Second)
			_ = m.Remove(ctx, handle.ID, true)
			return ContainerHandle{}, err
		}
	}

	return handle, nil
}

func buildContainerCreateInputs(spec ContainerSpec) (*container.Config, *container.HostConfig) {
	cfg := &container.Config{
		Image:      spec.Image,
		Cmd:        spec.Cmd,
		Env:        spec.Env,
		Labels:     spec.Labels,
		User:       spec.User,
		WorkingDir: spec.WorkingDir,
	}
	if len(spec.Entrypoint) > 0 {
		cfg.Entrypoint = append([]string(nil), spec.Entrypoint...)
	}

	hostConfig := &container.HostConfig{
		AutoRemove: spec.RemoveOnExit,
		Resources: container.Resources{
			Memory:   spec.MemoryLimit,
			NanoCPUs: spec.CPULimit,
		},
		Privileged: spec.Privileged,
		Binds:      append([]string(nil), spec.Binds...),
	}
	if strings.TrimSpace(spec.NetworkMode) != "" {
		hostConfig.NetworkMode = container.NetworkMode(spec.NetworkMode)
	}
	if len(spec.Capabilities) > 0 {
		hostConfig.CapAdd = strslice.StrSlice(append([]string(nil), spec.Capabilities...))
	}
	return cfg, hostConfig
}

func (m *DockerManager) StreamLogs(ctx context.Context, id string, destination io.Writer) error {
	if m == nil || m.client == nil {
		return fmt.Errorf("docker manager is not initialized")
	}
	logs, err := m.client.ContainerLogs(ctx, id, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	if err != nil {
		if client.IsErrNotFound(err) {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	defer logs.Close()

	_, err = io.Copy(destination, logs)
	return err
}

func (m *DockerManager) Wait(ctx context.Context, id string) (ContainerExitState, error) {
	state := ContainerExitState{}
	if m == nil || m.client == nil {
		return state, fmt.Errorf("docker manager is not initialized")
	}

	statusCh, errCh := m.client.ContainerWait(ctx, id, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return state, err
		}
		return state, nil
	case status := <-statusCh:
		state.ExitCode = int64(status.StatusCode)
		s, inspectErr := m.client.ContainerInspect(ctx, id)
		if inspectErr != nil {
			return state, inspectErr
		}
		if s.State != nil {
			if s.State.ExitCode != 0 {
				state.ExitCode = int64(s.State.ExitCode)
			}
			state.OOMKilled = s.State.OOMKilled
			state.Error = s.State.Error
		}
		if state.ExitCode != 0 {
			if state.Error == "" {
				state.Error = fmt.Sprintf("container exited with code %d", state.ExitCode)
			}
			return state, fmt.Errorf("container exited with code %d", state.ExitCode)
		}
		return state, nil
	case <-ctx.Done():
		return state, ctx.Err()
	}
}

func (m *DockerManager) Stop(ctx context.Context, id string, timeout time.Duration) error {
	if m == nil || m.client == nil {
		return fmt.Errorf("docker manager is not initialized")
	}
	timeoutSec := timeout
	if timeoutSec <= 0 {
		timeoutSec = 10 * time.Second
	}

	err := m.client.ContainerStop(ctx, id, &timeoutSec)
	if err == nil {
		return nil
	}
	if client.IsErrNotFound(err) {
		return nil
	}
	return err
}

func (m *DockerManager) Remove(ctx context.Context, id string, force bool) error {
	if m == nil || m.client == nil {
		return fmt.Errorf("docker manager is not initialized")
	}
	err := m.client.ContainerRemove(ctx, id, types.ContainerRemoveOptions{
		Force:         force,
		RemoveVolumes: true,
	})
	if err == nil {
		return nil
	}
	if client.IsErrNotFound(err) {
		return nil
	}
	if strings.Contains(err.Error(), "No such container") {
		return nil
	}
	return err
}

func (m *DockerManager) ensureImage(ctx context.Context, image string) error {
	_, _, err := m.client.ImageInspectWithRaw(ctx, image)
	if err == nil {
		return nil
	}

	reader, err := m.client.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("ensure image %s: %w", image, err)
	}
	defer reader.Close()
	_, _ = io.Copy(io.Discard, reader)
	return nil
}

func (m *DockerManager) waitUntilRunning(ctx context.Context, id string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		state, err := m.client.ContainerInspect(ctx, id)
		if err != nil {
			return err
		}
		if state.State != nil && state.State.Running {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("container %s failed to become running within %s", id, timeout)
		}
		select {
		case <-time.After(250 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
