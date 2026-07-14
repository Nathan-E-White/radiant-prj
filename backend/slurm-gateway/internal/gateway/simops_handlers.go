package gateway

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (g *Gateway) handleSimopsRuns(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/simops/runs" {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "SimOps run not found", Code: "simops_run_not_found"})
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

	var req SimopsRunRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxSimopsBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid JSON payload", Code: "bad_json"})
		return
	}
	defer r.Body.Close()
	if req.IdempotencyKey == "" {
		req.IdempotencyKey = strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	}

	response, status, err := g.simops.CreateRun(r.Context(), req, identity)
	if err != nil {
		writeJSON(w, status, ErrorResponse{Error: err.Error(), Code: simopsErrorCode(status)})
		return
	}
	writeJSON(w, status, response)
}

func (g *Gateway) handleSimopsRun(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/simops/runs/")
	if path == "" {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "SimOps run not found", Code: "simops_run_not_found"})
		return
	}
	identityOK := false
	if _, ok := g.authorizedIdentity(w, r); ok {
		identityOK = true
	}
	if !identityOK {
		return
	}

	if strings.HasSuffix(path, "/events") {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Only GET requests allowed", Code: "method_not_allowed"})
			return
		}
		runID := strings.TrimSuffix(path, "/events")
		runID = strings.TrimSuffix(runID, "/")
		events, status, err := g.simops.ListEvents(runID)
		if err != nil {
			writeJSON(w, status, ErrorResponse{Error: err.Error(), Code: simopsErrorCode(status)})
			return
		}
		writeJSON(w, status, map[string]any{
			"run_id": runID,
			"events": events,
		})
		return
	}

	if strings.HasSuffix(path, "/stop") {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Only POST requests allowed", Code: "method_not_allowed"})
			return
		}
		runID := strings.TrimSuffix(path, "/stop")
		runID = strings.TrimSuffix(runID, "/")
		response, status, err := g.simops.StopRun(r.Context(), runID)
		if err != nil {
			writeJSON(w, status, ErrorResponse{Error: err.Error(), Code: simopsErrorCode(status)})
			return
		}
		writeJSON(w, status, response)
		return
	}

	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Only GET requests allowed", Code: "method_not_allowed"})
		return
	}
	if strings.Contains(path, "/") {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "SimOps run not found", Code: "simops_run_not_found"})
		return
	}
	response, status, err := g.simops.GetRun(path)
	if err != nil {
		writeJSON(w, status, ErrorResponse{Error: err.Error(), Code: simopsErrorCode(status)})
		return
	}
	writeJSON(w, status, response)
}

func (g *Gateway) handleInternalSimopsRun(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/internal/simops/runs/")
	if strings.HasSuffix(path, "/results") {
		g.handleInternalSimopsResults(w, r, strings.TrimSuffix(path, "/results"))
		return
	}
	if !strings.HasSuffix(path, "/ingest") {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "Internal SimOps route not found", Code: "simops_route_not_found"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Only POST requests allowed", Code: "method_not_allowed"})
		return
	}

	runID := strings.TrimSuffix(path, "/ingest")
	runID = strings.TrimSuffix(runID, "/")
	token := strings.TrimSpace(r.Header.Get("X-Simops-Ingest-Token"))
	defer r.Body.Close()

	count, status, err := g.simops.Ingest(r.Context(), runID, token, r.Body)
	if err != nil {
		writeJSON(w, status, ErrorResponse{Error: err.Error(), Code: simopsErrorCode(status)})
		return
	}
	writeJSON(w, status, map[string]any{
		"accepted_frames": count,
		"run_id":          runID,
	})
}

func (g *Gateway) handleInternalSimopsResults(w http.ResponseWriter, r *http.Request, runPath string) {
	if g.workbench == nil {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "Workbench dataflow disabled", Code: "workbench_disabled"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Only POST requests allowed", Code: "method_not_allowed"})
		return
	}
	runID := strings.TrimSuffix(runPath, "/")
	token := strings.TrimSpace(r.Header.Get("X-Simops-Ingest-Token"))
	defer r.Body.Close()

	record, status, err := g.simops.getRecordForIngest(runID)
	if err != nil {
		writeJSON(w, status, ErrorResponse{Error: err.Error(), Code: simopsErrorCode(status)})
		return
	}
	if token == "" || token != record.IngestToken {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "invalid SimOps ingest token", Code: "unauthorized"})
		return
	}
	count, status, err := g.workbench.IngestResults(r.Context(), record, r.Body)
	if err != nil {
		writeJSON(w, status, ErrorResponse{Error: err.Error(), Code: workbenchErrorCode(status)})
		return
	}
	writeJSON(w, status, map[string]any{
		"accepted_results": count,
		"run_id":           runID,
	})
}

func simopsErrorCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_json"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusNotFound:
		return "simops_run_not_found"
	case http.StatusConflict:
		return "run_not_writable"
	case http.StatusTooManyRequests:
		return "simops_capacity"
	case http.StatusUnprocessableEntity:
		return "validation_failed"
	case http.StatusBadGateway:
		return "simops_backend_failed"
	default:
		return "simops_failed"
	}
}
