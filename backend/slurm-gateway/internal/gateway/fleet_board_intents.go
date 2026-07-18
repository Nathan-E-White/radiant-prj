package gateway

import (
	"context"
	"errors"
	"net/http"
)

type FleetBoardIntentRequest struct {
	Intent             string `json:"intent"`
	GameSessionID      string `json:"gameSessionId"`
	ReactorID          string `json:"reactorId"`
	SimulationJobID    string `json:"simulationJobId,omitempty"`
	SimulationJobState string `json:"simulationJobState,omitempty"`
	SimulationRecipe   string `json:"simulationRecipe,omitempty"`
	IdempotencyKey     string `json:"idempotencyKey"`
}

type RegisterDynamicReactorOutcome struct {
	Created   bool                      `json:"created"`
	WorkerSet ReactorTelemetryWorkerSet `json:"workerSet"`
}

type RemoveDynamicReactorOutcome struct {
	WorkerSet ReactorTelemetryWorkerSet `json:"workerSet"`
}

type FleetBoardIntentError struct {
	Error     string                     `json:"error"`
	Code      string                     `json:"code"`
	WorkerSet *ReactorTelemetryWorkerSet `json:"workerSet,omitempty"`
}

type FleetBoardIntentResult struct {
	Status int
	Body   any
}

type DynamicReactorIntentManager interface {
	RegisterDynamicReactor(context.Context, RegisterDynamicReactorRequest) (ReactorTelemetryWorkerSet, bool, error)
	RemoveDynamicReactor(context.Context, RemoveDynamicReactorRequest) (ReactorTelemetryWorkerSet, error)
	ReconcileExpired(context.Context) error
}

type ArtifactForgeIntentManager interface {
	Request(context.Context, ArtifactForgeRequest, string) (ArtifactForgeRecord, bool, error)
}

type ArtifactForgeSessionReconciler interface {
	ReconcileExpired() (int64, error)
}

type DynamicMeasuredRetentionReconciler interface {
	ReconcileDynamicMeasuredRetention() error
}

type ConfiguredDataFlushParticipant interface {
	Plan(context.Context) (ConfiguredDataFlushPlan, error)
	Apply(context.Context, string) (ConfiguredDataFlushResult, error)
}

type FleetBoardSessionActivityRecorder interface {
	TouchSession(string) error
}

type FleetBoardSessionLifecycle struct {
	ArtifactForge     ArtifactForgeSessionReconciler
	MeasuredRetention DynamicMeasuredRetentionReconciler
	ConfiguredFlush   ConfiguredDataFlushParticipant
	Activity          []FleetBoardSessionActivityRecorder
}

type FleetBoardIntentModule struct {
	reactorTelemetry DynamicReactorIntentManager
	artifactForge    ArtifactForgeIntentManager
	sessionLifecycle FleetBoardSessionLifecycle
}

func NewFleetBoardIntentModule(reactorTelemetry DynamicReactorIntentManager, artifactForge ArtifactForgeIntentManager) *FleetBoardIntentModule {
	return &FleetBoardIntentModule{reactorTelemetry: reactorTelemetry, artifactForge: artifactForge}
}

func NewFleetBoardIntentModuleWithSessionLifecycle(reactorTelemetry DynamicReactorIntentManager, artifactForge ArtifactForgeIntentManager, lifecycle FleetBoardSessionLifecycle) *FleetBoardIntentModule {
	module := NewFleetBoardIntentModule(reactorTelemetry, artifactForge)
	module.sessionLifecycle = lifecycle
	return module
}

func (m *FleetBoardIntentModule) ReconcileSessions(ctx context.Context) error {
	if m == nil {
		return nil
	}
	var reconcileErr error
	if m.reactorTelemetry != nil {
		reconcileErr = errors.Join(reconcileErr, m.reactorTelemetry.ReconcileExpired(ctx))
	}
	if m.sessionLifecycle.ArtifactForge != nil {
		_, err := m.sessionLifecycle.ArtifactForge.ReconcileExpired()
		reconcileErr = errors.Join(reconcileErr, err)
	}
	if m.sessionLifecycle.MeasuredRetention != nil {
		reconcileErr = errors.Join(reconcileErr, m.sessionLifecycle.MeasuredRetention.ReconcileDynamicMeasuredRetention())
	}
	return reconcileErr
}

func (m *FleetBoardIntentModule) PlanConfiguredDataFlush(ctx context.Context) (ConfiguredDataFlushPlan, error) {
	if m == nil || m.sessionLifecycle.ConfiguredFlush == nil {
		return ConfiguredDataFlushPlan{}, errors.New("configured data flush participant is required")
	}
	return m.sessionLifecycle.ConfiguredFlush.Plan(ctx)
}

func (m *FleetBoardIntentModule) ApplyConfiguredDataFlush(ctx context.Context, reviewedPlanID string) (ConfiguredDataFlushResult, error) {
	if m == nil || m.sessionLifecycle.ConfiguredFlush == nil {
		return ConfiguredDataFlushResult{}, errors.New("configured data flush participant is required")
	}
	return m.sessionLifecycle.ConfiguredFlush.Apply(ctx, reviewedPlanID)
}

func (m *FleetBoardIntentModule) Plan(ctx context.Context) (ConfiguredDataFlushPlan, error) {
	return m.PlanConfiguredDataFlush(ctx)
}

func (m *FleetBoardIntentModule) Apply(ctx context.Context, reviewedPlanID string) (ConfiguredDataFlushResult, error) {
	return m.ApplyConfiguredDataFlush(ctx, reviewedPlanID)
}

func (m *FleetBoardIntentModule) ReconcileDynamicReactors(ctx context.Context) *FleetBoardIntentResult {
	if m == nil || (m.reactorTelemetry == nil && m.sessionLifecycle.ArtifactForge == nil && m.sessionLifecycle.MeasuredRetention == nil) {
		return nil
	}
	if err := m.ReconcileSessions(ctx); err != nil {
		result := fleetBoardIntentFailure(http.StatusBadGateway, err.Error(), "fleet_board_session_reconciliation_failed", nil)
		return &result
	}
	return nil
}

func (m *FleetBoardIntentModule) ExecuteDynamicReactor(ctx context.Context, request FleetBoardIntentRequest) (FleetBoardIntentResult, bool) {
	switch request.Intent {
	case "registerDynamicReactor", "removeDynamicReactor":
	default:
		return FleetBoardIntentResult{}, false
	}

	if m.reactorTelemetry == nil {
		return fleetBoardIntentFailure(http.StatusNotFound, "Reactor Telemetry backend disabled", "reactor_telemetry_disabled", nil), true
	}

	if request.Intent == "registerDynamicReactor" {
		return m.registerDynamicReactor(ctx, request), true
	}
	return m.removeDynamicReactor(ctx, request), true
}

func (m *FleetBoardIntentModule) ExecuteArtifactForge(ctx context.Context, request FleetBoardIntentRequest, identity string) (FleetBoardIntentResult, bool) {
	if request.Intent != "requestArtifactForge" {
		return FleetBoardIntentResult{}, false
	}
	if m.artifactForge == nil {
		return fleetBoardIntentFailure(http.StatusNotFound, "Artifact Forge backend disabled", "artifact_forge_disabled", nil), true
	}

	translated := ArtifactForgeRequest{
		GameSessionID:      request.GameSessionID,
		ReactorID:          request.ReactorID,
		SimulationJobID:    request.SimulationJobID,
		SimulationJobState: request.SimulationJobState,
		SimulationRecipe:   request.SimulationRecipe,
		IdempotencyKey:     request.IdempotencyKey,
	}
	normalizeArtifactForgeRequest(&translated)
	if err := validateArtifactForgeIdentity(translated); err != nil {
		return fleetBoardIntentFailure(http.StatusUnprocessableEntity, err.Error(), "artifact_forge_rejected", nil), true
	}
	if err := m.recordSessionActivity(translated.GameSessionID); err != nil {
		return fleetBoardIntentFailure(http.StatusBadGateway, err.Error(), "fleet_board_session_activity_failed", nil), true
	}
	outcome, created, err := m.artifactForge.Request(ctx, translated, identity)
	if err != nil {
		return fleetBoardIntentFailure(http.StatusUnprocessableEntity, err.Error(), "artifact_forge_rejected", nil), true
	}
	status := http.StatusOK
	if created {
		status = http.StatusAccepted
	}
	if outcome.Decision == ArtifactForgeIntentRejected {
		status = http.StatusUnprocessableEntity
	}
	return FleetBoardIntentResult{Status: status, Body: outcome}, true
}

func (m *FleetBoardIntentModule) registerDynamicReactor(ctx context.Context, request FleetBoardIntentRequest) FleetBoardIntentResult {
	translated := RegisterDynamicReactorRequest{
		GameSessionID:  request.GameSessionID,
		ReactorID:      request.ReactorID,
		IdempotencyKey: request.IdempotencyKey,
	}
	if err := validateDynamicReactorIdentity(translated.GameSessionID, translated.ReactorID, translated.IdempotencyKey); err != nil {
		return fleetBoardIntentFailure(http.StatusBadGateway, err.Error(), "reactor_telemetry_failed", nil)
	}
	if err := m.recordSessionActivity(translated.GameSessionID); err != nil {
		return fleetBoardIntentFailure(http.StatusBadGateway, err.Error(), "fleet_board_session_activity_failed", nil)
	}
	set, created, err := m.reactorTelemetry.RegisterDynamicReactor(ctx, translated)
	if err != nil {
		if errors.Is(err, ErrReactorTelemetrySessionCap) {
			return fleetBoardIntentFailure(http.StatusConflict, err.Error(), "reactor_telemetry_cap", nil)
		}
		return fleetBoardIntentFailure(http.StatusBadGateway, err.Error(), "reactor_telemetry_failed", nil)
	}
	status := http.StatusOK
	if created {
		status = http.StatusAccepted
	}
	return FleetBoardIntentResult{Status: status, Body: RegisterDynamicReactorOutcome{Created: created, WorkerSet: set}}
}

func (m *FleetBoardIntentModule) recordSessionActivity(gameSessionID string) error {
	for _, recorder := range m.sessionLifecycle.Activity {
		if recorder == nil {
			continue
		}
		if err := recorder.TouchSession(gameSessionID); err != nil {
			return err
		}
	}
	return nil
}

func (m *FleetBoardIntentModule) removeDynamicReactor(ctx context.Context, request FleetBoardIntentRequest) FleetBoardIntentResult {
	translated := RemoveDynamicReactorRequest{
		GameSessionID:  request.GameSessionID,
		ReactorID:      request.ReactorID,
		IdempotencyKey: request.IdempotencyKey,
	}
	if err := validateDynamicReactorIdentity(translated.GameSessionID, translated.ReactorID, translated.IdempotencyKey); err != nil {
		return fleetBoardIntentFailure(http.StatusBadGateway, err.Error(), "reactor_telemetry_cleanup_failed", nil)
	}
	set, err := m.reactorTelemetry.RemoveDynamicReactor(ctx, translated)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, ErrReactorTelemetryNotFound) {
			status = http.StatusNotFound
		}
		return fleetBoardIntentFailure(status, err.Error(), "reactor_telemetry_cleanup_failed", &set)
	}
	return FleetBoardIntentResult{Status: http.StatusOK, Body: RemoveDynamicReactorOutcome{WorkerSet: set}}
}

func fleetBoardIntentFailure(status int, message, code string, set *ReactorTelemetryWorkerSet) FleetBoardIntentResult {
	return FleetBoardIntentResult{Status: status, Body: FleetBoardIntentError{Error: message, Code: code, WorkerSet: set}}
}
