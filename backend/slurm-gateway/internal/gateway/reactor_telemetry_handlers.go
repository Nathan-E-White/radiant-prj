package gateway

import (
	"encoding/json"
	"errors"
	"net/http"
)

type fleetBoardIntentRequest struct {
	Intent         string `json:"intent"`
	GameSessionID  string `json:"gameSessionId"`
	ReactorID      string `json:"reactorId"`
	IdempotencyKey string `json:"idempotencyKey"`
}

func (g *Gateway) handleFleetBoardIntent(w http.ResponseWriter, r *http.Request) {
	if g.reactorTelemetry == nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "Fleet Board backend disabled", Code: "fleet_board_disabled"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Only POST requests allowed", Code: "method_not_allowed"})
		return
	}
	if _, ok := g.authorizedIdentity(w, r); !ok {
		return
	}
	if err := g.reactorTelemetry.ReconcileExpired(r.Context()); err != nil {
		writeJSON(w, http.StatusBadGateway, ErrorResponse{Error: err.Error(), Code: "reactor_telemetry_expiry_cleanup_failed"})
		return
	}
	defer r.Body.Close()
	var request fleetBoardIntentRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxSubmitBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid JSON payload", Code: "bad_json"})
		return
	}
	switch request.Intent {
	case "registerDynamicReactor":
		set, created, err := g.reactorTelemetry.RegisterDynamicReactor(r.Context(), RegisterDynamicReactorRequest{
			GameSessionID: request.GameSessionID, ReactorID: request.ReactorID, IdempotencyKey: request.IdempotencyKey,
		})
		if err != nil {
			status := http.StatusBadGateway
			code := "reactor_telemetry_failed"
			if errors.Is(err, ErrReactorTelemetrySessionCap) {
				status = http.StatusConflict
				code = "reactor_telemetry_cap"
			}
			writeJSON(w, status, ErrorResponse{Error: err.Error(), Code: code})
			return
		}
		status := http.StatusOK
		if created {
			status = http.StatusAccepted
		}
		writeJSON(w, status, map[string]any{"created": created, "workerSet": set})
	case "removeDynamicReactor":
		set, err := g.reactorTelemetry.RemoveDynamicReactor(r.Context(), RemoveDynamicReactorRequest{
			GameSessionID: request.GameSessionID, ReactorID: request.ReactorID, IdempotencyKey: request.IdempotencyKey,
		})
		if err != nil {
			status := http.StatusBadGateway
			if errors.Is(err, ErrReactorTelemetryNotFound) {
				status = http.StatusNotFound
			}
			writeJSON(w, status, map[string]any{"error": err.Error(), "code": "reactor_telemetry_cleanup_failed", "workerSet": set})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"workerSet": set})
	default:
		writeJSON(w, http.StatusUnprocessableEntity, ErrorResponse{Error: "Unsupported Fleet Board intent", Code: "intent_not_supported"})
	}
}
