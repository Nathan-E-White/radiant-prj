package gateway

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSimopsStopOrchestratorRecordsCompletedCleanupAndPublication(t *testing.T) {
	store, run, workers := stopOrchestratorFixture(t)
	orchestrator := NewSimopsStopOrchestrator(DefaultConfig().Simops, store, ContractSimopsSpooler{Mode: "resident"}, time.Now)

	outcome, err := orchestrator.Stop(context.Background(), run.RunID)
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if outcome.Run.State.Phase != SimopsRunStopped || outcome.Run.State.Cleanup != SimopsCleanupCompleted || outcome.Intent == nil {
		t.Fatalf("expected stopped run, completed cleanup, and publication intent: %#v", outcome)
	}
	for _, worker := range outcome.Workers {
		if worker.State.Cleanup != SimopsCleanupCompleted || worker.State.Phase != SimopsWorkerStopped {
			t.Fatalf("worker cleanup state: %#v", worker)
		}
	}
	if len(workers) != len(outcome.Workers) {
		t.Fatalf("worker count changed")
	}
}

func TestSimopsStopOrchestratorRecordsIncompleteCleanupAfterRuntimeFailure(t *testing.T) {
	store, run, _ := stopOrchestratorFixture(t)
	spooler := failingStopSpooler{ContractSimopsSpooler: ContractSimopsSpooler{Mode: "resident"}, stopErr: errors.New("stop transport unavailable")}
	orchestrator := NewSimopsStopOrchestrator(DefaultConfig().Simops, store, spooler, time.Now)

	_, err := orchestrator.Stop(context.Background(), run.RunID)
	if err == nil {
		t.Fatal("expected runtime failure")
	}
	workers, err := store.ListWorkers(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	for _, worker := range workers {
		if worker.State.Cleanup != SimopsCleanupCompleted {
			t.Fatalf("cleanup should still be attempted with fresh context: %#v", worker.State)
		}
		if worker.State.Condition.Status != SimopsConditionDegraded {
			t.Fatalf("stop uncertainty must be persisted as degradation: %#v", worker.State)
		}
	}
}

func TestSimopsStopOrchestratorRetainsIncompleteCleanupEvidence(t *testing.T) {
	store, run, _ := stopOrchestratorFixture(t)
	spooler := failingCleanupSpooler{ContractSimopsSpooler: ContractSimopsSpooler{Mode: "resident"}, cleanupErr: SimopsConfirmedRuntimeFailure{Cause: errors.New("runtime delete rejected")}}
	orchestrator := NewSimopsStopOrchestrator(DefaultConfig().Simops, store, spooler, time.Now)

	_, err := orchestrator.Stop(context.Background(), run.RunID)
	if err == nil {
		t.Fatal("expected cleanup failure")
	}
	workers, err := store.ListWorkers(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if workers[0].State.Cleanup != SimopsCleanupFailed || workers[0].State.FailureStage != SimopsFailureCleanup {
		t.Fatalf("expected confirmed cleanup failure stage: %#v", workers[0].State)
	}
	evidence, err := store.ListWorkerDegradationEvidence(run.RunID, workers[0].WorkerID)
	if err != nil || len(evidence) != 1 || evidence[0].Code != "cleanup-failed" {
		t.Fatalf("expected append-only cleanup evidence: %#v, %v", evidence, err)
	}
}

func TestInMemoryStopLedgerKeepsStoppingThroughRequestedAndInProgress(t *testing.T) {
	store, run, _ := stopOrchestratorFixture(t)
	for _, cleanup := range []SimopsCleanupDisposition{SimopsCleanupRequested, SimopsCleanupInProgress} {
		outcome, err := store.ApplyStopTransition(SimopsStopTransition{RunID: run.RunID, Cleanup: cleanup, OccurredAt: time.Now()})
		if err != nil || outcome.Run.State.Phase != SimopsRunStopping || outcome.Run.Lifecycle != SimopsDegraded {
			t.Fatalf("cleanup=%s must persist stopping before runtime work: %#v err=%v", cleanup, outcome.Run, err)
		}
	}
}

func TestStopCleanupDoesNotReplaceExistingRunFailureWithStoppedPublication(t *testing.T) {
	store, run, _ := stopOrchestratorFixture(t)
	workers, err := store.ListWorkers(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	workers[0].State.Condition.Status = SimopsConditionFailed
	workers[0].State.Phase = SimopsWorkerFailed
	workers[0].State.FailureStage = SimopsFailureExecution
	if err := store.SaveLaunch(run.RunID, workers, nil); err != nil {
		t.Fatal(err)
	}
	orchestrator := NewSimopsStopOrchestrator(DefaultConfig().Simops, store, ContractSimopsSpooler{Mode: "resident"}, time.Now)
	outcome, err := orchestrator.Stop(context.Background(), run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Run.State.Phase != SimopsRunFailed || outcome.Intent != nil {
		t.Fatalf("cleanup cannot overwrite existing failure: %#v", outcome)
	}
}

func stopOrchestratorFixture(t *testing.T) (*InMemorySimopsStore, SimopsRunRecord, []SimopsWorkerRecord) {
	t.Helper()
	store := NewInMemorySimopsStore()
	now := time.Now().UTC()
	run := SimopsRunRecord{RunID: "RUN-STOP-ORCHESTRATOR", ScenarioID: "scheduler-drift", Lifecycle: SimopsStreaming, LaunchMode: "resident", IngestToken: "test-token", State: SimopsRunState{Phase: SimopsRunRunning, Condition: SimopsOperationalCondition{Status: SimopsConditionNominal}, Cleanup: SimopsCleanupNotRequested}, CreatedAt: now, UpdatedAt: now}
	workers := []SimopsWorkerRecord{{RunID: run.RunID, WorkerID: "scheduler-01", WorkerKind: SimopsWorkerScheduler, Lifecycle: SimopsStreaming, State: SimopsWorkerState{Phase: SimopsWorkerExecuting, LaunchDisposition: SimopsLaunchLaunched, ActiveExecution: SimopsExecutionActive, Condition: SimopsOperationalCondition{Status: SimopsConditionNominal}, FailureStage: SimopsFailureNone, Cleanup: SimopsCleanupNotRequested}, UpdatedAt: now}}
	if _, _, err := store.CreateRun(run, workers, nil); err != nil {
		t.Fatal(err)
	}
	return store, run, workers
}

type failingStopSpooler struct {
	ContractSimopsSpooler
	stopErr error
}

func (s failingStopSpooler) StopRunProfiles(context.Context, string, []RunConnectionProfile) error {
	return s.stopErr
}

type failingCleanupSpooler struct {
	ContractSimopsSpooler
	cleanupErr error
}

func (s failingCleanupSpooler) CleanupRunProfiles(context.Context, string, []RunConnectionProfile) error {
	return s.cleanupErr
}
