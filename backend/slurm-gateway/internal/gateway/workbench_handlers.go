package gateway

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

func (g *Gateway) handleInternalScada(w http.ResponseWriter, r *http.Request) {
	if g.workbench == nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "Workbench dataflow disabled", Code: "workbench_disabled"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Only POST requests allowed", Code: "method_not_allowed"})
		return
	}
	token := strings.TrimSpace(r.Header.Get("X-Workbench-Ingest-Token"))
	if token == "" || token != g.cfg.Workbench.InternalIngestToken {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "Invalid Workbench ingest token", Code: "unauthorized"})
		return
	}
	defer r.Body.Close()

	switch r.URL.Path {
	case "/internal/scada/sources":
		var source ScadaResidentSourceDeclaration
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxWorkbenchBodyBytes))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&source); err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid JSON payload", Code: "bad_json"})
			return
		}
		status, err := g.workbench.RegisterSource(source)
		if err != nil {
			writeJSON(w, status, ErrorResponse{Error: err.Error(), Code: workbenchErrorCode(status)})
			return
		}
		writeJSON(w, status, map[string]any{
			"source_id": source.SourceID,
			"accepted":  true,
		})
	case "/internal/scada/telemetry":
		count, status, err := g.workbench.IngestScada(r.Context(), r.Body)
		if err != nil {
			writeJSON(w, status, ErrorResponse{Error: err.Error(), Code: workbenchErrorCode(status)})
			return
		}
		writeJSON(w, status, map[string]any{
			"accepted_frames": count,
		})
	default:
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "Internal SCADA route not found", Code: "scada_route_not_found"})
	}
}

func (g *Gateway) handleSimulatorWorkbench(w http.ResponseWriter, r *http.Request) {
	if g.workbench == nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "Workbench dataflow disabled", Code: "workbench_disabled"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Only GET requests allowed", Code: "method_not_allowed"})
		return
	}
	if _, ok := g.authorizedIdentity(w, r); !ok {
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/simulator-workbench/")
	switch {
	case path == "state":
		snapshot, err := g.workbench.Snapshot()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to read workbench state", Code: "workbench_failed"})
			return
		}
		writeJSON(w, http.StatusOK, snapshot.State)
	case path == "measured":
		frames, err := g.workbench.Measured()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to read measured state", Code: "workbench_failed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"frames": frames})
	case path == "twin":
		state, err := g.workbench.Twin()
		if errors.Is(err, ErrWorkbenchNotFound) {
			writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "Twin state not found", Code: "twin_not_found"})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to read twin state", Code: "workbench_failed"})
			return
		}
		writeJSON(w, http.StatusOK, state)
	case strings.HasPrefix(path, "lineage/"):
		valueID := strings.TrimSpace(strings.TrimPrefix(path, "lineage/"))
		if valueID == "" || strings.Contains(valueID, "/") {
			writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "Lineage not found", Code: "lineage_not_found"})
			return
		}
		lineage, err := g.workbench.Lineage(valueID)
		if errors.Is(err, ErrWorkbenchNotFound) {
			writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "Lineage not found", Code: "lineage_not_found"})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to read lineage", Code: "workbench_failed"})
			return
		}
		writeJSON(w, http.StatusOK, lineage)
	default:
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "Simulator Workbench route not found", Code: "workbench_route_not_found"})
	}
}

func workbenchErrorCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_json"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusUnprocessableEntity:
		return "validation_failed"
	case http.StatusBadGateway:
		return "workbench_backend_failed"
	default:
		return "workbench_failed"
	}
}
