package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const incompleteStartStaleAfter = time.Minute

type SimopsRunLifecycleStage string

const (
	SimopsRunStagePersistence         SimopsRunLifecycleStage = "persistence"
	SimopsRunStageRuntimeLaunch       SimopsRunLifecycleStage = "runtime_launch"
	SimopsRunStageLaunchPersistence   SimopsRunLifecycleStage = "launch_persistence"
	SimopsRunStageStreamingTransition SimopsRunLifecycleStage = "streaming_transition"
	SimopsRunStageArtifactPlanning    SimopsRunLifecycleStage = "artifact_planning"
	SimopsRunStageArtifactPersistence SimopsRunLifecycleStage = "artifact_persistence"
	SimopsRunStageEventPublication    SimopsRunLifecycleStage = "event_publication"
	SimopsRunStageIncompleteRecovery  SimopsRunLifecycleStage = "incomplete_launch_recovery"
)

type SimopsRunLifecycleError struct {
	Stage             SimopsRunLifecycleStage
	Cause             error
	CompensationError error
	RecoveryError     error
}

func (e *SimopsRunLifecycleError) Error() string {
	message := fmt.Sprintf("SimOps Run lifecycle failed during %s: %v", e.Stage, e.Cause)
	if e.CompensationError != nil {
		message += fmt.Sprintf("; compensation failed: %v", e.CompensationError)
	}
	if e.RecoveryError != nil {
		message += fmt.Sprintf("; durable recovery failed: %v", e.RecoveryError)
	}
	return message
}

func (e *SimopsRunLifecycleError) Unwrap() error {
	return e.Cause
}

type SimopsRunLifecycleOutcome struct {
	Run     SimopsRunRecord
	Created bool
}

type SimopsRunLifecycle interface {
	Start(context.Context, SimopsRunRecord, []SimopsWorkerKind) (SimopsRunLifecycleOutcome, error)
}

type SimopsRunLifecyclePolicy struct {
	cfg      SimopsConfig
	store    SimopsStore
	spooler  SimopsSpooler
	eventLog SimopsEventLog
	artifact SimopsArtifactSink
	now      func() time.Time
}

func NewSimopsRunLifecyclePolicy(cfg SimopsConfig, store SimopsStore, spooler SimopsSpooler, eventLog SimopsEventLog, artifact SimopsArtifactSink) *SimopsRunLifecyclePolicy {
	return &SimopsRunLifecyclePolicy{
		cfg:      cfg,
		store:    store,
		spooler:  spooler,
		eventLog: eventLog,
		artifact: artifact,
		now:      time.Now,
	}
}

func (p *SimopsRunLifecyclePolicy) SetNow(now func() time.Time) {
	if now == nil {
		p.now = time.Now
		return
	}
	p.now = now
}

func (p *SimopsRunLifecyclePolicy) Start(ctx context.Context, record SimopsRunRecord, workerKinds []SimopsWorkerKind) (SimopsRunLifecycleOutcome, error) {
	planned := plannedWorkerRecords(record, workerKinds, p.now().UTC())
	stored, created, err := p.store.CreateRun(record, planned, nil)
	if err != nil {
		return SimopsRunLifecycleOutcome{}, &SimopsRunLifecycleError{Stage: SimopsRunStagePersistence, Cause: err}
	}
	outcome := SimopsRunLifecycleOutcome{Run: stored, Created: created}
	if !created {
		if stored.Lifecycle == SimopsStarting && p.incompleteStartIsStale(stored) {
			return p.recoverIncompleteStart(ctx, outcome)
		}
		return outcome, nil
	}

	launched, commands, err := p.startWorkers(ctx, stored, workerKinds)
	if err != nil {
		return p.fail(ctx, outcome, planned, launched, commands, SimopsRunStageRuntimeLaunch, err)
	}
	if err := validateLaunchedWorkers(planned, launched); err != nil {
		return p.fail(ctx, outcome, planned, launched, commands, SimopsRunStageRuntimeLaunch, err)
	}
	if err := p.store.SaveLaunch(stored.RunID, launched, commands); err != nil {
		return p.fail(ctx, outcome, planned, launched, commands, SimopsRunStageLaunchPersistence, err)
	}

	streaming, err := p.store.UpdateRunLifecycle(stored.RunID, SimopsStreaming)
	if err != nil {
		return p.fail(ctx, outcome, planned, launched, commands, SimopsRunStageStreamingTransition, err)
	}
	outcome.Run = streaming

	if p.artifact != nil {
		artifact := p.artifact.PlanArtifact(streaming)
		if err := validatePlannedArtifact(streaming.RunID, artifact); err != nil {
			return p.fail(ctx, outcome, planned, launched, commands, SimopsRunStageArtifactPlanning, err)
		}
		if err := p.store.SaveArtifact(artifact); err != nil {
			return p.fail(ctx, outcome, planned, launched, commands, SimopsRunStageArtifactPersistence, err)
		}
	}

	if err := p.eventLog.Publish(ctx, SimopsEvent{
		RunID:      streaming.RunID,
		EventType:  "run.lifecycle",
		Lifecycle:  streaming.Lifecycle,
		OccurredAt: p.now().UTC(),
	}); err != nil {
		return p.fail(ctx, outcome, planned, launched, commands, SimopsRunStageEventPublication, err)
	}

	return outcome, nil
}

func (p *SimopsRunLifecyclePolicy) incompleteStartIsStale(run SimopsRunRecord) bool {
	return run.UpdatedAt.IsZero() || !p.now().UTC().Before(run.UpdatedAt.Add(incompleteStartStaleAfter))
}

func (p *SimopsRunLifecyclePolicy) recoverIncompleteStart(ctx context.Context, outcome SimopsRunLifecycleOutcome) (SimopsRunLifecycleOutcome, error) {
	workers, workersErr := p.store.ListWorkers(outcome.Run.RunID)
	commands, commandsErr := p.store.ListCommands(outcome.Run.RunID)
	launched := []SimopsWorkerRecord{}
	reconcileErrors := []error{errors.New("idempotent retry found an incomplete starting Run")}
	if workersErr != nil {
		reconcileErrors = append(reconcileErrors, fmt.Errorf("list workers for recovery: %w", workersErr))
	} else {
		recoveryCtx, cancel := p.recoveryContext(ctx)
		observations, err := p.spooler.SyncRun(recoveryCtx, outcome.Run, workers)
		cancel()
		if err != nil {
			reconcileErrors = append(reconcileErrors, fmt.Errorf("observe workers for recovery: %w", err))
		} else {
			workersByID := make(map[string]SimopsWorkerRecord, len(workers))
			for _, worker := range workers {
				workersByID[worker.WorkerID] = worker
			}
			for _, observation := range observations {
				worker, ok := workersByID[observation.WorkerID]
				if !ok {
					continue
				}
				if err := p.store.UpdateWorkerObservedLifecycle(observation); err != nil {
					reconcileErrors = append(reconcileErrors, fmt.Errorf("persist worker %s recovery observation: %w", observation.WorkerID, err))
				}
				applyObservedWorkerLifecycle(&worker, observation, p.now)
				if observedRuntimeResourceExists(observation) {
					launched = append(launched, worker)
				}
			}
		}
	}
	if commandsErr != nil {
		reconcileErrors = append(reconcileErrors, fmt.Errorf("list commands for recovery: %w", commandsErr))
		commands = nil
	}
	return p.fail(ctx, outcome, workers, launched, commands, SimopsRunStageIncompleteRecovery, errors.Join(reconcileErrors...))
}

func (p *SimopsRunLifecyclePolicy) startWorkers(ctx context.Context, record SimopsRunRecord, workers []SimopsWorkerKind) ([]SimopsWorkerRecord, []SimopsSpoolCommand, error) {
	if profileSpooler, ok := p.spooler.(RunConnectionProfileSpooler); ok {
		profiles, err := BuildRunWorkerConnectionProfiles(p.cfg, record, workers)
		if err != nil {
			return nil, nil, err
		}
		return profileSpooler.StartRunProfiles(ctx, record, profiles)
	}
	return p.spooler.StartRun(ctx, record, workers)
}

func (p *SimopsRunLifecyclePolicy) fail(ctx context.Context, outcome SimopsRunLifecycleOutcome, planned, launched []SimopsWorkerRecord, commands []SimopsSpoolCommand, stage SimopsRunLifecycleStage, cause error) (SimopsRunLifecycleOutcome, error) {
	recoveryCtx, cancel := p.recoveryContext(ctx)
	defer cancel()
	compensationErr := p.compensate(recoveryCtx, outcome.Run, planned)
	workerOutcomes := failedWorkerOutcomes(planned, launched, compensationErr == nil, p.now().UTC())
	commandOutcomes := failedCommandOutcomes(commands, launched, compensationErr == nil, p.now().UTC())

	recoveryErrors := []error{}
	if err := p.store.SaveLaunch(outcome.Run.RunID, workerOutcomes, commandOutcomes); err != nil {
		recoveryErrors = append(recoveryErrors, fmt.Errorf("persist worker outcomes: %w", err))
	}
	for _, worker := range workerOutcomes {
		if err := p.store.UpdateWorkerFrames(outcome.Run.RunID, worker.WorkerID, worker.Lifecycle, 0); err != nil {
			recoveryErrors = append(recoveryErrors, fmt.Errorf("persist worker %s lifecycle: %w", worker.WorkerID, err))
		}
	}

	failedRun, err := p.store.UpdateRunLifecycle(outcome.Run.RunID, SimopsFailed)
	if err != nil {
		recoveryErrors = append(recoveryErrors, fmt.Errorf("persist failed Run: %w", err))
	} else {
		outcome.Run = failedRun
	}
	artifacts, err := p.store.ListArtifacts(outcome.Run.RunID)
	if err != nil {
		recoveryErrors = append(recoveryErrors, fmt.Errorf("list artifacts for failure disposition: %w", err))
	} else {
		for _, artifact := range artifacts {
			if err := p.store.UpdateArtifactStatus(outcome.Run.RunID, artifact.ArtifactID, SimopsArtifactStatusFailed); err != nil {
				recoveryErrors = append(recoveryErrors, fmt.Errorf("mark artifact %s failed: %w", artifact.ArtifactID, err))
			}
		}
	}

	failureEvent := lifecycleFailureEvent(outcome.Run.RunID, stage, cause, compensationErr, p.now().UTC())
	if err := p.eventLog.Publish(recoveryCtx, failureEvent); err != nil {
		if storeErr := p.store.SaveEvent(failureEvent); storeErr != nil {
			recoveryErrors = append(recoveryErrors, fmt.Errorf("persist lifecycle failure event: %w", storeErr))
		}
	}
	if err := recoveryCtx.Err(); err != nil {
		recoveryErrors = append(recoveryErrors, fmt.Errorf("bounded lifecycle recovery: %w", err))
	}

	return outcome, &SimopsRunLifecycleError{
		Stage:             stage,
		Cause:             cause,
		CompensationError: compensationErr,
		RecoveryError:     errors.Join(recoveryErrors...),
	}
}

func (p *SimopsRunLifecyclePolicy) recoveryContext(ctx context.Context) (context.Context, context.CancelFunc) {
	timeout := p.cfg.LifecycleRecoveryTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return context.WithTimeout(context.WithoutCancel(ctx), timeout)
}

func (p *SimopsRunLifecyclePolicy) compensate(ctx context.Context, run SimopsRunRecord, workers []SimopsWorkerRecord) error {
	if profileStopper, ok := p.spooler.(RunConnectionProfileStopper); ok {
		profiles, err := BuildRunWorkerConnectionProfilesForRecords(p.cfg, run, workers)
		if err != nil {
			return err
		}
		return profileStopper.StopRunProfiles(ctx, run.RunID, profiles)
	}
	return p.spooler.StopRun(ctx, run.RunID)
}

func validatePlannedArtifact(runID string, artifact SimopsArtifactRecord) error {
	if strings.TrimSpace(artifact.ArtifactID) == "" {
		return fmt.Errorf("planned artifact_id is required")
	}
	if strings.TrimSpace(artifact.RunID) != strings.TrimSpace(runID) {
		return fmt.Errorf("planned artifact run_id does not match Run")
	}
	if strings.TrimSpace(artifact.Kind) == "" || strings.TrimSpace(artifact.Status) == "" {
		return fmt.Errorf("planned artifact kind and status are required")
	}
	return nil
}

func validateLaunchedWorkers(planned, launched []SimopsWorkerRecord) error {
	expected := make(map[string]SimopsWorkerKind, len(planned))
	for _, worker := range planned {
		expected[worker.WorkerID] = worker.WorkerKind
	}
	seen := make(map[string]struct{}, len(launched))
	for _, worker := range launched {
		kind, ok := expected[worker.WorkerID]
		if !ok {
			return fmt.Errorf("runtime returned unexpected worker %q", worker.WorkerID)
		}
		if kind != worker.WorkerKind {
			return fmt.Errorf("runtime returned worker %q with kind %q instead of %q", worker.WorkerID, worker.WorkerKind, kind)
		}
		if _, duplicate := seen[worker.WorkerID]; duplicate {
			return fmt.Errorf("runtime returned duplicate worker %q", worker.WorkerID)
		}
		seen[worker.WorkerID] = struct{}{}
	}
	if len(seen) != len(expected) {
		return fmt.Errorf("runtime returned %d of %d planned workers", len(seen), len(expected))
	}
	return nil
}

func observedRuntimeResourceExists(observation ObservedWorkerLifecycle) bool {
	if strings.TrimSpace(observation.RuntimeID) != "" {
		return true
	}
	switch observation.State {
	case ObservedWorkerPending, ObservedWorkerActive, ObservedWorkerSucceeded, ObservedWorkerFailed, ObservedWorkerStopped:
		return true
	default:
		return false
	}
}

func failedWorkerOutcomes(planned, launched []SimopsWorkerRecord, compensated bool, now time.Time) []SimopsWorkerRecord {
	launchedByID := make(map[string]SimopsWorkerRecord, len(launched))
	for _, worker := range launched {
		launchedByID[worker.WorkerID] = worker
	}
	outcomes := make([]SimopsWorkerRecord, 0, len(planned))
	for _, worker := range planned {
		if launchedWorker, ok := launchedByID[worker.WorkerID]; ok {
			worker = launchedWorker
			if compensated {
				worker.Lifecycle = SimopsStopped
			} else {
				worker.Lifecycle = SimopsFailed
			}
		} else {
			worker.Lifecycle = SimopsFailed
		}
		worker.UpdatedAt = now
		outcomes = append(outcomes, worker)
	}
	return outcomes
}

func failedCommandOutcomes(commands []SimopsSpoolCommand, launched []SimopsWorkerRecord, compensated bool, now time.Time) []SimopsSpoolCommand {
	launchedWorkerIDs := make(map[string]struct{}, len(launched))
	for _, worker := range launched {
		launchedWorkerIDs[worker.WorkerID] = struct{}{}
	}
	outcomes := make([]SimopsSpoolCommand, 0, len(commands))
	for _, command := range commands {
		_, workerLaunched := launchedWorkerIDs[command.WorkerID]
		if compensated && workerLaunched {
			command.State = SimopsStopped
			command.Message = strings.TrimSpace(command.Message + "; stopped by Run launch compensation")
		} else {
			command.State = SimopsFailed
			command.Message = strings.TrimSpace(command.Message + "; Run launch compensation failed")
		}
		command.UpdatedAt = now
		outcomes = append(outcomes, command)
	}
	return outcomes
}

func lifecycleFailureEvent(runID string, stage SimopsRunLifecycleStage, cause, compensationErr error, now time.Time) SimopsEvent {
	details := map[string]string{
		"stage":        string(stage),
		"error":        cause.Error(),
		"compensation": "succeeded",
	}
	if compensationErr != nil {
		details["compensation"] = "failed"
		details["compensation_error"] = compensationErr.Error()
	}
	frame, _ := json.Marshal(details)
	return SimopsEvent{
		RunID:      runID,
		EventType:  "run.lifecycle.failure",
		Lifecycle:  SimopsFailed,
		Frame:      frame,
		OccurredAt: now,
	}
}
