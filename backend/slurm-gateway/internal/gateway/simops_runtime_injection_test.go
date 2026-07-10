package gateway

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestDefaultSimopsControllerRequiresInjectedDockerRuntime(t *testing.T) {
	cfg := testRunConnectionProfileConfig()

	_, err := NewDefaultSimopsController(cfg)
	if err == nil || !strings.Contains(err.Error(), "runtime adapter") {
		t.Fatalf("expected injected runtime adapter error, got %v", err)
	}
}

func TestDefaultSimopsControllerUsesInjectedRuntime(t *testing.T) {
	cfg := testRunConnectionProfileConfig()
	spooler := &recordingSimopsSpooler{}

	controller, err := NewDefaultSimopsControllerWithSpooler(cfg, spooler)
	if err != nil {
		t.Fatalf("new controller: %v", err)
	}
	controller.runID = func() string { return "RUN-INJECTED-001" }

	_, status, err := controller.CreateRun(context.Background(), SimopsRunRequest{
		ScenarioID:  "scheduler-drift",
		WorkerKinds: []string{"scheduler"},
		LaunchMode:  "auto",
	}, "react-backend-client")
	if err != nil {
		t.Fatalf("create run status=%d: %v", status, err)
	}
	if spooler.legacyStarts != 0 {
		t.Fatalf("expected legacy start path to stay unused, got %d", spooler.legacyStarts)
	}
	if spooler.profileStarts != 1 {
		t.Fatalf("expected injected profile spooler to start once, got %d", spooler.profileStarts)
	}
	if spooler.runID != "RUN-INJECTED-001" {
		t.Fatalf("unexpected run id %q", spooler.runID)
	}
	if len(spooler.profiles) != 1 || spooler.profiles[0].WorkerID != "scheduler-01" || spooler.profiles[0].WorkerKind != SimopsWorkerScheduler {
		t.Fatalf("unexpected profiles %#v", spooler.profiles)
	}

	_, status, err = controller.StopRun(context.Background(), "RUN-INJECTED-001")
	if err != nil {
		t.Fatalf("stop run status=%d: %v", status, err)
	}
	if spooler.profileStops != 1 {
		t.Fatalf("expected injected profile stopper to stop once, got %d", spooler.profileStops)
	}
	if len(spooler.stopProfiles) != 1 || spooler.stopProfiles[0].WorkerID != "scheduler-01" || spooler.stopProfiles[0].WorkerKind != SimopsWorkerScheduler {
		t.Fatalf("unexpected stop profiles %#v", spooler.stopProfiles)
	}
}

type recordingSimopsSpooler struct {
	legacyStarts  int
	profileStarts int
	profileStops  int
	runID         string
	profiles      []RunConnectionProfile
	stopProfiles  []RunConnectionProfile
}

func (s *recordingSimopsSpooler) StartRun(_ context.Context, _ SimopsRunRecord, _ []SimopsWorkerKind) ([]SimopsWorkerRecord, []SimopsSpoolCommand, error) {
	s.legacyStarts++
	return nil, nil, fmt.Errorf("legacy worker start should not be used")
}

func (s *recordingSimopsSpooler) StartRunProfiles(ctx context.Context, run SimopsRunRecord, profiles []RunConnectionProfile) ([]SimopsWorkerRecord, []SimopsSpoolCommand, error) {
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	default:
	}
	s.profileStarts++
	s.runID = run.RunID
	s.profiles = append([]RunConnectionProfile(nil), profiles...)
	now := fixedGatewayRuntimeNow()
	workers := make([]SimopsWorkerRecord, 0, len(profiles))
	commands := make([]SimopsSpoolCommand, 0, len(profiles))
	for _, profile := range profiles {
		workers = append(workers, SimopsWorkerRecord{
			RunID:      run.RunID,
			WorkerID:   profile.WorkerID,
			WorkerKind: profile.WorkerKind,
			Lifecycle:  SimopsStarting,
			LaunchMode: profile.LaunchMode,
			UpdatedAt:  now,
		})
		commands = append(commands, SimopsSpoolCommand{
			CommandID: run.RunID + "-" + profile.WorkerID + "-start",
			RunID:     run.RunID,
			WorkerID:  profile.WorkerID,
			Mode:      profile.LaunchMode,
			State:     SimopsStarting,
			Message:   "injected runtime accepted worker launch",
			CreatedAt: now,
			UpdatedAt: now,
		})
	}
	return workers, commands, nil
}

func (s *recordingSimopsSpooler) StopRun(ctx context.Context, _ string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (s *recordingSimopsSpooler) StopRunProfiles(ctx context.Context, runID string, profiles []RunConnectionProfile) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	s.profileStops++
	s.runID = runID
	s.stopProfiles = append([]RunConnectionProfile(nil), profiles...)
	return nil
}

func (s *recordingSimopsSpooler) SyncRun(ctx context.Context, _ SimopsRunRecord, workers []SimopsWorkerRecord) ([]ObservedWorkerLifecycle, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	observations := make([]ObservedWorkerLifecycle, 0, len(workers))
	for _, worker := range workers {
		observations = append(observations, ObservedWorkerLifecycle{
			RunID:      worker.RunID,
			WorkerID:   worker.WorkerID,
			WorkerKind: worker.WorkerKind,
			State:      ObservedWorkerPending,
			Runtime:    "test",
			Reason:     "recording-runtime",
			ObservedAt: fixedGatewayRuntimeNow(),
		})
	}
	return observations, nil
}

func fixedGatewayRuntimeNow() time.Time {
	return time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
}
