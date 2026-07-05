package containerruntime

import (
	"context"
	"io"
	"strings"
	"time"
)

type ContainerSpec struct {
	Image             string
	Cmd               []string
	Env               []string
	Labels            map[string]string
	Name              string
	NetworkMode       string
	Capabilities      []string
	Privileged        bool
	User              string
	WorkingDir        string
	Entrypoint        []string
	Binds             []string
	RemoveOnExit      bool
	HealthProbeTimeout time.Duration
	MemoryLimit       int64
	CPULimit          int64
}

type ContainerHandle struct {
	ID string
}

type ContainerExitState struct {
	ExitCode int64
	Error    string
	OOMKilled bool
}

type ContainerManager interface {
	Start(ctx context.Context, spec ContainerSpec) (ContainerHandle, error)
	StreamLogs(ctx context.Context, id string, destination io.Writer) error
	Wait(ctx context.Context, id string) (ContainerExitState, error)
	Stop(ctx context.Context, id string, timeout time.Duration) error
	Remove(ctx context.Context, id string, force bool) error
}

func NewToyEchoSpec(message string) ContainerSpec {
	return ContainerSpec{
		Image:        "alpine:latest",
		Cmd:          []string{"sh", "-c", "echo " + shellQuoted(message) + " && sleep 2"},
		Labels:       map[string]string{"project": "orchestrator", "type": "toy"},
		RemoveOnExit: true,
		HealthProbeTimeout: 10 * time.Second,
	}
}

func RunToyEchoContainer(ctx context.Context, manager ContainerManager, message string) (ContainerHandle, error) {
	return manager.Start(ctx, NewToyEchoSpec(message))
}

func shellQuoted(value string) string {
	escaped := "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
	return escaped
}
