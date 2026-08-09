package gateway

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// SimopsStopOrchestrator owns the ordered control-plane transition for a
// stop request. Runtime adapters execute work; this module records its facts.
type SimopsStopOrchestrator struct {
	cfg     SimopsConfig
	store   SimopsStore
	spooler SimopsSpooler
	now     func() time.Time
}

func NewSimopsStopOrchestrator(cfg SimopsConfig, store SimopsStore, spooler SimopsSpooler, now func() time.Time) *SimopsStopOrchestrator {
	if now == nil {
		now = time.Now
	}
	return &SimopsStopOrchestrator{cfg: cfg, store: store, spooler: spooler, now: now}
}

func (o *SimopsStopOrchestrator) Stop(ctx context.Context, runID string) (SimopsStopOutcome, error) {
	run, err := o.store.GetRun(runID)
	if err != nil {
		return SimopsStopOutcome{}, err
	}
	workers, err := o.store.ListWorkers(runID)
	if err != nil {
		return SimopsStopOutcome{}, err
	}
	if _, err := o.store.ApplyStopTransition(SimopsStopTransition{RunID: runID, Cleanup: SimopsCleanupRequested, OccurredAt: o.now()}); err != nil {
		return SimopsStopOutcome{}, err
	}
	profiles, err := BuildRunWorkerConnectionProfilesForRecords(o.cfg, run, workers)
	if err != nil {
		return SimopsStopOutcome{}, err
	}
	stopErr := o.spooler.StopRunProfiles(ctx, runID, profiles)
	if _, err := o.store.ApplyStopTransition(SimopsStopTransition{RunID: runID, Cleanup: SimopsCleanupInProgress, StopFailure: errorText(stopErr), StopConfirmed: isConfirmedRuntimeFailure(stopErr), OccurredAt: o.now()}); err != nil {
		return SimopsStopOutcome{}, err
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), o.cfg.WorkerCleanupTTL)
	defer cancel()
	cleanupErr := o.spooler.CleanupRunProfiles(cleanupCtx, runID, profiles)
	cleanup := SimopsCleanupCompleted
	if cleanupErr != nil {
		if isConfirmedRuntimeFailure(cleanupErr) {
			cleanup = SimopsCleanupFailed
		} else {
			cleanup = SimopsCleanupIncomplete
		}
	}
	outcome, persistErr := o.store.ApplyStopTransition(SimopsStopTransition{RunID: runID, Cleanup: cleanup, StopFailure: errorText(stopErr), StopConfirmed: isConfirmedRuntimeFailure(stopErr), CleanupFailure: errorText(cleanupErr), CleanupConfirmed: isConfirmedRuntimeFailure(cleanupErr), OccurredAt: o.now()})
	if persistErr != nil {
		return SimopsStopOutcome{}, persistErr
	}
	if stopErr != nil || cleanupErr != nil {
		return outcome, fmt.Errorf("stop SimOps Run %s: %w", runID, errors.Join(stopErr, cleanupErr))
	}
	return outcome, nil
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// SimopsConfirmedRuntimeFailure distinguishes an adapter-confirmed operation
// failure from a transport outcome that leaves the operation uncertain.
type SimopsConfirmedRuntimeFailure struct{ Cause error }

func (e SimopsConfirmedRuntimeFailure) Error() string { return e.Cause.Error() }
func (e SimopsConfirmedRuntimeFailure) Unwrap() error { return e.Cause }
func isConfirmedRuntimeFailure(err error) bool {
	var confirmed SimopsConfirmedRuntimeFailure
	return errors.As(err, &confirmed)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
