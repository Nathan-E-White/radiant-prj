package containerruntime

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
)

type fakeManager struct {
	startedSpec ContainerSpec
}

func (f *fakeManager) Start(_ context.Context, spec ContainerSpec) (ContainerHandle, error) {
	f.startedSpec = spec
	return ContainerHandle{ID: "container-id"}, nil
}

func (f *fakeManager) StreamLogs(_ context.Context, _ string, _ io.Writer) error {
	return nil
}

func (f *fakeManager) Wait(_ context.Context, _ string) (ContainerExitState, error) {
	return ContainerExitState{}, nil
}

func (f *fakeManager) Stop(_ context.Context, _ string, _ time.Duration) error {
	return nil
}

func (f *fakeManager) Remove(_ context.Context, _ string, _ bool) error {
	return nil
}

func TestNewToyEchoSpec(t *testing.T) {
	spec := NewToyEchoSpec("hello")
	if spec.Image != "alpine:latest" {
		t.Fatalf("unexpected image: %s", spec.Image)
	}
	if len(spec.Cmd) == 0 || spec.Cmd[0] != "sh" {
		t.Fatalf("expected shell command, got %#v", spec.Cmd)
	}
}

func TestRunToyEchoContainerUsesManager(t *testing.T) {
	mgr := &fakeManager{}
	_, err := RunToyEchoContainer(context.Background(), mgr, "from test")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if mgr.startedSpec.Image != "alpine:latest" {
		t.Fatalf("expected toy image, got %s", mgr.startedSpec.Image)
	}
}

func TestBuildContainerCreateInputsMapsObserverFields(t *testing.T) {
	cfg, hostCfg := buildContainerCreateInputs(ContainerSpec{
		Image:        "alpine:latest",
		Cmd:          []string{"echo", "hi"},
		User:         "nobody",
		WorkingDir:   "/tmp",
		Entrypoint:   []string{"/bin/sh", "-c"},
		NetworkMode:  "bridge",
		Capabilities: []string{"NET_ADMIN"},
		Privileged:   true,
		Binds:        []string{"/host:/container:rw"},
	})

	if cfg.User != "nobody" {
		t.Fatalf("expected user nobody, got %q", cfg.User)
	}
	if cfg.WorkingDir != "/tmp" {
		t.Fatalf("expected working dir /tmp, got %q", cfg.WorkingDir)
	}
	if hostCfg.NetworkMode != container.NetworkMode("bridge") {
		t.Fatalf("expected network mode bridge, got %q", hostCfg.NetworkMode)
	}
	if len(hostCfg.CapAdd) != 1 || hostCfg.CapAdd[0] != "NET_ADMIN" {
		t.Fatalf("expected cap add NET_ADMIN, got %#v", hostCfg.CapAdd)
	}
	if !hostCfg.Privileged {
		t.Fatalf("expected privileged true")
	}
	if len(hostCfg.Binds) != 1 || hostCfg.Binds[0] != "/host:/container:rw" {
		t.Fatalf("expected bind mounted path, got %#v", hostCfg.Binds)
	}
	if len(cfg.Entrypoint) != 2 || cfg.Entrypoint[0] != "/bin/sh" || cfg.Entrypoint[1] != "-c" {
		t.Fatalf("unexpected entrypoint %#v", cfg.Entrypoint)
	}
}
