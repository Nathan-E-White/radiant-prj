package gateway

import (
	"testing"
	"time"
)

func TestWorkerStateDerivesExecutionFailureOnlyAfterActiveExecution(t *testing.T) {
	state := initialSimopsWorkerState()
	now := time.Date(2026, 8, 8, 18, 0, 0, 0, time.UTC)
	state, contradiction := applyWorkerRuntimeObservation(state, ObservedWorkerLifecycle{State: ObservedWorkerActive, Runtime: "docker", ObservedAt: now})
	if contradiction != nil || state.ActiveExecution != SimopsExecutionActive || state.LaunchDisposition != SimopsLaunchLaunched {
		t.Fatalf("expected active execution fact, got state=%#v contradiction=%#v", state, contradiction)
	}
	state, contradiction = applyWorkerRuntimeObservation(state, ObservedWorkerLifecycle{State: ObservedWorkerFailed, Runtime: "docker", ObservedAt: now.Add(time.Minute)})
	if contradiction != nil || state.FailureStage != SimopsFailureExecution || state.Phase != SimopsWorkerFailed {
		t.Fatalf("expected execution-stage failure, got state=%#v contradiction=%#v", state, contradiction)
	}
}

func TestWorkerStateStopDuringUncertainLaunchDoesNotInventExecution(t *testing.T) {
	state := initialSimopsWorkerState()
	state.LaunchDisposition = SimopsLaunchAttemptedAbsent
	state.ActiveExecution = SimopsExecutionUncertain
	state.Phase = SimopsWorkerLaunching
	state, contradiction := applyWorkerRuntimeObservation(state, ObservedWorkerLifecycle{State: ObservedWorkerStopped, ObservedAt: time.Now().UTC()})
	if contradiction != nil || state.ActiveExecution != SimopsExecutionTerminal || state.FailureStage != SimopsFailureNone {
		t.Fatalf("stop during an uncertain launch must not imply execution or failure: %#v", state)
	}
}

func TestWorkerStateRetainsTerminalStateAndProducesEvidenceForContradiction(t *testing.T) {
	now := time.Now().UTC()
	state, _ := applyWorkerRuntimeObservation(initialSimopsWorkerState(), ObservedWorkerLifecycle{State: ObservedWorkerSucceeded, ObservedAt: now})
	state, contradiction := applyWorkerRuntimeObservation(state, ObservedWorkerLifecycle{State: ObservedWorkerActive, Runtime: "docker", Message: "late poll", ObservedAt: now.Add(time.Minute)})
	if state.Phase != SimopsWorkerCompleted || contradiction == nil || contradiction.Code != "terminal-contradiction" {
		t.Fatalf("expected terminal preservation with evidence, got state=%#v contradiction=%#v", state, contradiction)
	}
}

func TestWorkerStateKeepsUncertainLaunchSeparateFromCleanupFailure(t *testing.T) {
	now := time.Now().UTC()
	state, evidence := applyWorkerLaunchOutcome(initialSimopsWorkerState(), SimopsLaunchAttemptedAbsent, now, "spooler", "no resource observed")
	if evidence == nil || state.ActiveExecution != SimopsExecutionUncertain || state.Condition.Status != SimopsConditionDegraded {
		t.Fatalf("expected explicit uncertain launch evidence: %#v", state)
	}
	state, _ = applyWorkerCleanupOutcome(state, SimopsCleanupFailed, now.Add(time.Minute), "spooler", "delete failed")
	if state.FailureStage != SimopsFailureCleanup || state.Condition.Status != SimopsConditionFailed {
		t.Fatalf("cleanup failure must take derived-stage precedence: %#v", state)
	}
}

func TestRunStateAggregatesFailureAndUncertainLaunch(t *testing.T) {
	failed := SimopsWorkerRecord{State: initialSimopsWorkerState()}
	failed.State.Phase = SimopsWorkerFailed
	failed.State.Condition.Status = SimopsConditionFailed
	uncertain := SimopsWorkerRecord{State: initialSimopsWorkerState()}
	uncertain.State.Phase = SimopsWorkerLaunching
	uncertain.State.LaunchDisposition = SimopsLaunchAttemptedAbsent
	state := deriveSimopsRunState(initialSimopsRunState(time.Time{}), []SimopsWorkerRecord{uncertain, failed})
	if state.Phase != SimopsRunFailed || state.Condition.Status != SimopsConditionFailed {
		t.Fatalf("required worker failure must dominate aggregate condition: %#v", state)
	}
}
