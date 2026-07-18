package gateway

import (
	"encoding/json"
	"net/http"
)

func (g *Gateway) handleFleetBoardIntent(w http.ResponseWriter, r *http.Request) {
	if g.reactorTelemetry == nil && g.artifactForge == nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "Fleet Board backend disabled", Code: "fleet_board_disabled"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Only POST requests allowed", Code: "method_not_allowed"})
		return
	}
	identity, ok := g.authorizedIdentity(w, r)
	if !ok {
		return
	}
	defer r.Body.Close()
	var request FleetBoardIntentRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxSubmitBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid JSON payload", Code: "bad_json"})
		return
	}
	if result, handled := NewFleetBoardIntentModule(g.reactorTelemetry).ExecuteDynamicReactor(r.Context(), request); handled {
		writeJSON(w, result.Status, result.Body)
		return
	}
	switch request.Intent {
	case "requestArtifactForge":
		if g.artifactForge == nil {
			writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "Artifact Forge backend disabled", Code: "artifact_forge_disabled"})
			return
		}
		outcome, created, err := g.artifactForge.Request(r.Context(), ArtifactForgeRequest{
			GameSessionID: request.GameSessionID, ReactorID: request.ReactorID, SimulationJobID: request.SimulationJobID,
			SimulationJobState: request.SimulationJobState, SimulationRecipe: request.SimulationRecipe, IdempotencyKey: request.IdempotencyKey,
		}, identity)
		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, ErrorResponse{Error: err.Error(), Code: "artifact_forge_rejected"})
			return
		}
		status := http.StatusOK
		if created {
			status = http.StatusAccepted
		}
		if outcome.Decision == ArtifactForgeIntentRejected {
			status = http.StatusUnprocessableEntity
		}
		writeJSON(w, status, outcome)
	default:
		writeJSON(w, http.StatusUnprocessableEntity, ErrorResponse{Error: "Unsupported Fleet Board intent", Code: "intent_not_supported"})
	}
}
