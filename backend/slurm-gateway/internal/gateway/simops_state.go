package gateway

import "time"

type SimopsActiveExecution string

const (
	SimopsExecutionUnobserved SimopsActiveExecution = "unobserved"
	SimopsExecutionUncertain  SimopsActiveExecution = "uncertain"
	SimopsExecutionActive     SimopsActiveExecution = "active_observed"
	SimopsExecutionAbsent     SimopsActiveExecution = "absent_observed"
	SimopsExecutionTerminal   SimopsActiveExecution = "terminal_observed"
)

// SimopsRunPhase and SimopsWorkerPhase are control-plane lifecycle facts.
// They deliberately do not reuse SimopsLifecycle: Lifecycle remains the
// compatibility projection exposed by the v1 API.
type SimopsRunPhase string

const (
	SimopsRunAdmitted  SimopsRunPhase = "admitted"
	SimopsRunLaunching SimopsRunPhase = "launching"
	SimopsRunRunning   SimopsRunPhase = "running"
	SimopsRunStopping  SimopsRunPhase = "stopping"
	SimopsRunCompleted SimopsRunPhase = "completed"
	SimopsRunFailed    SimopsRunPhase = "failed"
	SimopsRunStopped   SimopsRunPhase = "stopped"
)

type SimopsWorkerPhase string

const (
	SimopsWorkerPlanned   SimopsWorkerPhase = "planned"
	SimopsWorkerLaunching SimopsWorkerPhase = "launching"
	SimopsWorkerExecuting SimopsWorkerPhase = "executing"
	SimopsWorkerCompleted SimopsWorkerPhase = "completed"
	SimopsWorkerFailed    SimopsWorkerPhase = "failed"
	SimopsWorkerStopped   SimopsWorkerPhase = "stopped"
)

type SimopsLaunchDisposition string

const (
	SimopsLaunchUnattempted     SimopsLaunchDisposition = "unattempted"
	SimopsLaunchAttemptedAbsent SimopsLaunchDisposition = "attempted-absent"
	SimopsLaunchLaunched        SimopsLaunchDisposition = "launched"
	SimopsLaunchRejected        SimopsLaunchDisposition = "rejected"
)

type SimopsConditionStatus string

const (
	SimopsConditionNominal  SimopsConditionStatus = "nominal"
	SimopsConditionDegraded SimopsConditionStatus = "degraded"
	SimopsConditionFailed   SimopsConditionStatus = "failed"
)

type SimopsDegradationEvidence struct {
	Code       string    `json:"code"`
	Source     string    `json:"source,omitempty"`
	Message    string    `json:"message,omitempty"`
	ObservedAt time.Time `json:"observed_at"`
}

type SimopsOperationalCondition struct {
	Status       SimopsConditionStatus       `json:"status"`
	Degradations []SimopsDegradationEvidence `json:"degradations,omitempty"`
}

type SimopsCleanupDisposition string

const (
	SimopsCleanupNotRequested SimopsCleanupDisposition = "not_requested"
	SimopsCleanupRequested    SimopsCleanupDisposition = "requested"
	SimopsCleanupInProgress   SimopsCleanupDisposition = "in_progress"
	SimopsCleanupCompleted    SimopsCleanupDisposition = "completed"
	SimopsCleanupIncomplete   SimopsCleanupDisposition = "incomplete"
	SimopsCleanupFailed       SimopsCleanupDisposition = "failed"
)

type SimopsFailureStage string

const (
	SimopsFailureNone      SimopsFailureStage = "none"
	SimopsFailureLaunch    SimopsFailureStage = "launch"
	SimopsFailureExecution SimopsFailureStage = "execution"
	SimopsFailureStop      SimopsFailureStage = "stop"
	SimopsFailureCleanup   SimopsFailureStage = "cleanup"
)

type SimopsWorkerState struct {
	Phase             SimopsWorkerPhase          `json:"phase"`
	LaunchDisposition SimopsLaunchDisposition    `json:"launch_disposition"`
	ActiveExecution   SimopsActiveExecution      `json:"active_execution"`
	Condition         SimopsOperationalCondition `json:"condition"`
	FailureStage      SimopsFailureStage         `json:"failure_stage"`
	Cleanup           SimopsCleanupDisposition   `json:"cleanup"`
}

type SimopsRunState struct {
	Phase                     SimopsRunPhase             `json:"phase"`
	Condition                 SimopsOperationalCondition `json:"condition"`
	Cleanup                   SimopsCleanupDisposition   `json:"cleanup"`
	LaunchObservationDeadline time.Time                  `json:"launch_observation_deadline,omitempty"`
}

func initialSimopsWorkerState() SimopsWorkerState {
	return SimopsWorkerState{Phase: SimopsWorkerPlanned, LaunchDisposition: SimopsLaunchUnattempted, ActiveExecution: SimopsExecutionUnobserved, Condition: SimopsOperationalCondition{Status: SimopsConditionNominal}, FailureStage: SimopsFailureNone, Cleanup: SimopsCleanupNotRequested}
}

func initialSimopsRunState(deadline time.Time) SimopsRunState {
	return SimopsRunState{Phase: SimopsRunAdmitted, Condition: SimopsOperationalCondition{Status: SimopsConditionNominal}, Cleanup: SimopsCleanupNotRequested, LaunchObservationDeadline: deadline}
}

func ensureSimopsWorkerState(worker SimopsWorkerRecord) SimopsWorkerState {
	if worker.State.Phase != "" {
		return worker.State
	}
	state := initialSimopsWorkerState()
	switch worker.Lifecycle {
	case SimopsStarting:
		state.Phase = SimopsWorkerLaunching
	case SimopsStreaming:
		state.Phase = SimopsWorkerExecuting
	case SimopsComplete:
		state.Phase, state.ActiveExecution = SimopsWorkerCompleted, SimopsExecutionTerminal
	case SimopsFailed:
		state.Phase, state.ActiveExecution, state.Condition.Status = SimopsWorkerFailed, SimopsExecutionTerminal, SimopsConditionFailed
	case SimopsStopped:
		state.Phase, state.ActiveExecution = SimopsWorkerStopped, SimopsExecutionTerminal
	}
	return state
}

func ensureSimopsRunState(run SimopsRunRecord) SimopsRunState {
	if run.State.Phase != "" {
		return run.State
	}
	state := initialSimopsRunState(time.Time{})
	switch run.Lifecycle {
	case SimopsStarting:
		state.Phase = SimopsRunLaunching
	case SimopsStreaming:
		state.Phase = SimopsRunRunning
	case SimopsComplete:
		state.Phase = SimopsRunCompleted
	case SimopsFailed:
		state.Phase, state.Condition.Status = SimopsRunFailed, SimopsConditionFailed
	case SimopsStopped:
		state.Phase = SimopsRunStopped
	}
	return state
}

func applyDegradation(state *SimopsWorkerState, evidence SimopsDegradationEvidence) {
	state.Condition.Degradations = append(state.Condition.Degradations, evidence)
	if state.Condition.Status == SimopsConditionNominal {
		state.Condition.Status = SimopsConditionDegraded
	}
}

func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

func applyWorkerRuntimeObservation(state SimopsWorkerState, observation ObservedWorkerLifecycle) (SimopsWorkerState, *SimopsDegradationEvidence) {
	if state.ActiveExecution == SimopsExecutionTerminal && !observedWorkerStateIsTerminal(observation.State) {
		evidence := SimopsDegradationEvidence{Code: "terminal-contradiction", Source: observation.Runtime, Message: observation.Message, ObservedAt: observation.ObservedAt}
		return state, &evidence
	}
	wasActive := state.ActiveExecution == SimopsExecutionActive
	switch observation.State {
	case ObservedWorkerActive:
		state.ActiveExecution = SimopsExecutionActive
		state.LaunchDisposition = SimopsLaunchLaunched
		state.Phase = SimopsWorkerExecuting
	case ObservedWorkerSucceeded:
		state.ActiveExecution, state.Phase = SimopsExecutionTerminal, SimopsWorkerCompleted
	case ObservedWorkerFailed, ObservedWorkerImagePullFailed:
		state.ActiveExecution, state.Phase = SimopsExecutionTerminal, SimopsWorkerFailed
		state.Condition.Status = SimopsConditionFailed
		if wasActive {
			state.FailureStage = SimopsFailureExecution
		} else {
			state.FailureStage = SimopsFailureLaunch
		}
	case ObservedWorkerStopped:
		state.ActiveExecution, state.Phase = SimopsExecutionTerminal, SimopsWorkerStopped
	case ObservedWorkerMissing:
		if state.ActiveExecution == SimopsExecutionUnobserved || state.ActiveExecution == SimopsExecutionUncertain {
			state.ActiveExecution = SimopsExecutionAbsent
		}
	}
	return state, nil
}

func applyWorkerLaunchOutcome(state SimopsWorkerState, disposition SimopsLaunchDisposition, observedAt time.Time, source, message string) (SimopsWorkerState, *SimopsDegradationEvidence) {
	if state.LaunchDisposition == SimopsLaunchLaunched || state.LaunchDisposition == SimopsLaunchRejected {
		return state, nil
	}
	switch disposition {
	case SimopsLaunchLaunched:
		state.LaunchDisposition, state.Phase = SimopsLaunchLaunched, SimopsWorkerLaunching
	case SimopsLaunchRejected:
		state.LaunchDisposition, state.Phase, state.Condition.Status, state.FailureStage = SimopsLaunchRejected, SimopsWorkerFailed, SimopsConditionFailed, SimopsFailureLaunch
	case SimopsLaunchAttemptedAbsent:
		state.LaunchDisposition, state.ActiveExecution, state.Phase = SimopsLaunchAttemptedAbsent, SimopsExecutionUncertain, SimopsWorkerLaunching
		evidence := SimopsDegradationEvidence{Code: "launch-uncertain", Source: source, Message: message, ObservedAt: observedAt}
		applyDegradation(&state, evidence)
		return state, &evidence
	}
	return state, nil
}

func applyWorkerCleanupOutcome(state SimopsWorkerState, disposition SimopsCleanupDisposition, observedAt time.Time, source, message string) (SimopsWorkerState, *SimopsDegradationEvidence) {
	state.Cleanup = disposition
	switch disposition {
	case SimopsCleanupIncomplete:
		evidence := SimopsDegradationEvidence{Code: "cleanup-incomplete", Source: source, Message: message, ObservedAt: observedAt}
		applyDegradation(&state, evidence)
		return state, &evidence
	case SimopsCleanupFailed:
		state.Condition.Status, state.FailureStage = SimopsConditionFailed, SimopsFailureCleanup
		return state, nil
	}
	return state, nil
}

func materializeWorkerLaunchState(worker *SimopsWorkerRecord) *SimopsDegradationEvidence {
	worker.State = ensureSimopsWorkerState(*worker)
	now := worker.UpdatedAt.UTC()
	if worker.RuntimeID != "" {
		worker.State, _ = applyWorkerLaunchOutcome(worker.State, SimopsLaunchLaunched, now, "launch", "runtime accepted launch")
		return nil
	}
	if worker.Lifecycle == SimopsFailed {
		worker.State, _ = applyWorkerLaunchOutcome(worker.State, SimopsLaunchRejected, now, "launch", "runtime rejected launch")
		return nil
	}
	if worker.State.LaunchDisposition == SimopsLaunchUnattempted {
		state, evidence := applyWorkerLaunchOutcome(worker.State, SimopsLaunchAttemptedAbsent, now, "launch", "launch accepted without an observed runtime resource")
		worker.State = state
		return evidence
	}
	return nil
}

func projectWorkerLifecycle(state SimopsWorkerState) SimopsLifecycle {
	switch state.Phase {
	case SimopsWorkerPlanned:
		return SimopsCreated
	case SimopsWorkerLaunching:
		return SimopsStarting
	case SimopsWorkerExecuting:
		return SimopsStreaming
	case SimopsWorkerCompleted:
		return SimopsComplete
	case SimopsWorkerFailed:
		return SimopsFailed
	case SimopsWorkerStopped:
		return SimopsStopped
	default:
		return SimopsCreated
	}
}

// deriveSimopsRunState is the only aggregate judgment over worker facts. It
// does not mutate workers and intentionally does not perform deadline work;
// #215 owns scheduled evaluation of persisted deadlines.
func deriveSimopsRunState(current SimopsRunState, workers []SimopsWorkerRecord) SimopsRunState {
	state := current
	state.Condition.Degradations = nil
	state.Condition.Status = SimopsConditionNominal
	failed, stopping, running, completed := false, false, false, true
	for _, worker := range workers {
		workerState := worker.State
		if workerState.Condition.Status == SimopsConditionFailed || workerState.Cleanup == SimopsCleanupFailed {
			failed = true
		}
		if workerState.Condition.Status == SimopsConditionDegraded || workerState.LaunchDisposition == SimopsLaunchAttemptedAbsent || workerState.Cleanup == SimopsCleanupIncomplete {
			state.Condition.Status = SimopsConditionDegraded
		}
		state.Condition.Degradations = append(state.Condition.Degradations, workerState.Condition.Degradations...)
		switch workerState.Phase {
		case SimopsWorkerPlanned, SimopsWorkerLaunching:
			completed = false
		case SimopsWorkerExecuting:
			running, completed = true, false
		case SimopsWorkerStopped:
			stopping = true
		}
	}
	if failed {
		state.Condition.Status, state.Phase = SimopsConditionFailed, SimopsRunFailed
		return state
	}
	if stopping {
		state.Phase = SimopsRunStopped
	} else if running {
		state.Phase = SimopsRunRunning
	} else if completed {
		state.Phase = SimopsRunCompleted
	} else {
		state.Phase = SimopsRunLaunching
	}
	return state
}

func projectRunLifecycle(state SimopsRunState) SimopsLifecycle {
	switch state.Phase {
	case SimopsRunAdmitted:
		return SimopsCreated
	case SimopsRunLaunching:
		return SimopsStarting
	case SimopsRunRunning:
		return SimopsStreaming
	case SimopsRunStopping:
		return SimopsDegraded
	case SimopsRunCompleted:
		return SimopsComplete
	case SimopsRunFailed:
		return SimopsFailed
	case SimopsRunStopped:
		return SimopsStopped
	default:
		return SimopsCreated
	}
}
