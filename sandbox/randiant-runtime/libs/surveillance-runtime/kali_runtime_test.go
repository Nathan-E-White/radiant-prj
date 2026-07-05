package surveilanceruntime

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	containerruntime "github.com/radiant/container-runtime"
)

type fakeManager struct {
	startedSpec containerruntime.ContainerSpec
}

func (f *fakeManager) Start(_ context.Context, spec containerruntime.ContainerSpec) (containerruntime.ContainerHandle, error) {
	f.startedSpec = spec
	return containerruntime.ContainerHandle{ID: "container-id"}, nil
}

func (f *fakeManager) StreamLogs(_ context.Context, _ string, _ io.Writer) error {
	return nil
}

func (f *fakeManager) Wait(_ context.Context, _ string) (containerruntime.ContainerExitState, error) {
	return containerruntime.ContainerExitState{}, nil
}

func (f *fakeManager) Stop(_ context.Context, _ string, _ time.Duration) error {
	return nil
}

func (f *fakeManager) Remove(_ context.Context, _ string, _ bool) error {
	return nil
}

func TestGetDefaultKaliImage(t *testing.T) {
	t.Setenv("RADIANT_KALI_OBSERVER_IMAGE", "custom:test")
	if image := GetDefaultKaliImage(); image != "custom:test" {
		t.Fatalf("expected override image, got %q", image)
	}
}

func TestNormalizeKaliSpecDefaults(t *testing.T) {
	_ = os.Unsetenv("RADIANT_KALI_OBSERVER_IMAGE")
	spec, err := normalizeKaliSpec(KaliObserverSpec{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if spec.Mode != SurveillanceModeDual {
		t.Fatalf("expected default dual mode, got %q", spec.Mode)
	}
	if spec.Duration != 30*time.Second {
		t.Fatalf("expected default duration, got %v", spec.Duration)
	}
}

func TestBuildPassiveAndActiveCommands(t *testing.T) {
	passive, err := buildObservationCommand(KaliObserverSpec{
		Mode:       SurveillanceModePassive,
		Interface:  "eth0",
		Duration:   5 * time.Second,
		PCAPFilter: "icmp",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(passive, "tcpdump") {
		t.Fatalf("expected tcpdump command, got %q", passive)
	}
	if !strings.Contains(passive, "icmp") {
		t.Fatalf("expected filter in command, got %q", passive)
	}

	active, err := buildObservationCommand(KaliObserverSpec{
		Mode:     SurveillanceModeActive,
		Duration: 5 * time.Second,
		Targets:  []string{"127.0.0.1"},
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(active, "nmap") {
		t.Fatalf("expected nmap command, got %q", active)
	}
}

func TestBuildDualCommandContainsBoth(t *testing.T) {
	dual, err := buildObservationCommand(KaliObserverSpec{
		Mode:       SurveillanceModeDual,
		Interface:  "any",
		Duration:   2 * time.Second,
		Targets:    []string{"127.0.0.1"},
		PCAPFilter: "icmp",
		NMapArgs:   []string{"-T4"},
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(dual, "tcpdump") {
		t.Fatalf("expected tcpdump in dual command, got %q", dual)
	}
	if !strings.Contains(dual, "nmap") {
		t.Fatalf("expected nmap in dual command, got %q", dual)
	}
	if !strings.Contains(dual, "wait") {
		t.Fatalf("expected wait in dual command, got %q", dual)
	}
}

func TestStartRejectsExternalTargetByDefault(t *testing.T) {
	fake := &fakeManager{}
	manager := NewKaliObserver(fake)
	_, err := manager.Start(context.Background(), KaliObserverSpec{
		Mode:    SurveillanceModeActive,
		Targets: []string{"8.8.8.8"},
	})
	if err == nil {
		t.Fatal("expected validation error for external target")
	}
}

func TestStartMapsObserverDefaults(t *testing.T) {
	fake := &fakeManager{}
	manager := NewKaliObserver(fake)
	handle, err := manager.Start(context.Background(), KaliObserverSpec{
		Mode:     SurveillanceModePassive,
		Duration: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if handle.ContainerID != "container-id" {
		t.Fatalf("unexpected container id: %s", handle.ContainerID)
	}
	if fake.startedSpec.Image == "" {
		t.Fatalf("expected image in container spec")
	}
	if fake.startedSpec.User != "root" {
		t.Fatalf("expected root user for passive mode")
	}
}

func TestStartHonorsRunAsRootIfNeeded(t *testing.T) {
	fake := &fakeManager{}
	manager := NewKaliObserver(fake)
	_, err := manager.Start(context.Background(), KaliObserverSpec{
		Mode:              SurveillanceModeActive,
		Targets:           []string{"127.0.0.1"},
		RunAsRootIfNeeded: true,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if fake.startedSpec.User != "root" {
		t.Fatalf("expected root user when runAsRootIfNeeded=true")
	}
}
