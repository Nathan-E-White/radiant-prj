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

type FleetBoardIntentModule struct {
	reactorTelemetry DynamicReactorIntentManager
}

func NewFleetBoardIntentModule(reactorTelemetry DynamicReactorIntentManager) *FleetBoardIntentModule {
	if concrete, ok := reactorTelemetry.(*ReactorTelemetryManager); ok && concrete == nil {
		reactorTelemetry = nil
	}
	return &FleetBoardIntentModule{reactorTelemetry: reactorTelemetry}
}

func (m *FleetBoardIntentModule) ReconcileDynamicReactors(ctx context.Context) *FleetBoardIntentResult {
	if m.reactorTelemetry == nil {
		return nil
	}
	if err := m.reactorTelemetry.ReconcileExpired(ctx); err != nil {
		result := fleetBoardIntentFailure(http.StatusBadGateway, err.Error(), "reactor_telemetry_expiry_cleanup_failed", nil)
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

func (m *FleetBoardIntentModule) registerDynamicReactor(ctx context.Context, request FleetBoardIntentRequest) FleetBoardIntentResult {
	translated := RegisterDynamicReactorRequest{
		GameSessionID:  request.GameSessionID,
		ReactorID:      request.ReactorID,
		IdempotencyKey: request.IdempotencyKey,
	}
	if err := validateDynamicReactorIdentity(translated.GameSessionID, translated.ReactorID, translated.IdempotencyKey); err != nil {
		return fleetBoardIntentFailure(http.StatusBadGateway, err.Error(), "reactor_telemetry_failed", nil)
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
