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
	intentModule := g.fleetBoardIntentModule()
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

func (g *Gateway) fleetBoardIntentModule() *FleetBoardIntentModule {
	if g.fleetBoard == nil {
		g.fleetBoard = g.newFleetBoardIntentModule()
	}
	return g.fleetBoard
}

func (g *Gateway) newFleetBoardIntentModule() *FleetBoardIntentModule {
	var reactorTelemetry DynamicReactorIntentManager
	lifecycle := FleetBoardSessionLifecycle{}
	activity := make([]FleetBoardSessionActivityRecorder, 0, 2)
	if g.reactorTelemetry != nil {
		reactorTelemetry = g.reactorTelemetry
		activity = append(activity, g.reactorTelemetry)
	}
	var artifactForge ArtifactForgeIntentManager
	if g.artifactForge != nil {
		artifactForge = g.artifactForge
		lifecycle.ArtifactForge = g.artifactForge
		activity = append(activity, g.artifactForge)
	}
	if g.workbench != nil {
		lifecycle.MeasuredRetention = g.workbench
	}
	lifecycle.Activity = activity
	return NewFleetBoardIntentModuleWithSessionLifecycle(reactorTelemetry, artifactForge, lifecycle)
}
