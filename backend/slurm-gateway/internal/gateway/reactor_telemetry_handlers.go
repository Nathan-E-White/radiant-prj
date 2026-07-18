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
	var reactorTelemetry DynamicReactorIntentManager
	if g.reactorTelemetry != nil {
		reactorTelemetry = g.reactorTelemetry
	}
	var artifactForge ArtifactForgeIntentManager
	if g.artifactForge != nil {
		artifactForge = g.artifactForge
	}
	intentModule := NewFleetBoardIntentModule(reactorTelemetry, artifactForge)
	if result := intentModule.ReconcileDynamicReactors(r.Context()); result != nil {
		writeJSON(w, result.Status, result.Body)
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
	if result, handled := intentModule.ExecuteDynamicReactor(r.Context(), request); handled {
		writeJSON(w, result.Status, result.Body)
		return
	}
	if result, handled := intentModule.ExecuteArtifactForge(r.Context(), request, identity); handled {
		writeJSON(w, result.Status, result.Body)
		return
	}
	writeJSON(w, http.StatusUnprocessableEntity, ErrorResponse{Error: "Unsupported Fleet Board intent", Code: "intent_not_supported"})
}
