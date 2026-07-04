package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

const maxSubmitBodyBytes = 64 * 1024

var scriptNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

type Gateway struct {
	cfg     Config
	spooler SlurmSpooler
	store   JobStore
	metrics *Metrics
	simops  *SimopsController
	now     func() time.Time
}

func NewDefaultGateway(cfg Config) (*Gateway, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	var spooler SlurmSpooler = MockSpooler{}
	if cfg.Mode == ModeSbatch {
		spooler = SbatchSpooler{
			Command:    cfg.SbatchBin,
			ScriptRoot: cfg.ScriptRoot,
			Runner:     RealCommandRunner{},
		}
	}

	app := NewGateway(cfg, spooler, NewInMemoryJobStore(), NewMetrics())
	simops, err := NewDefaultSimopsController(cfg.Simops)
	if err != nil {
		return nil, err
	}
	app.simops = simops
	return app, nil
}

func NewGateway(cfg Config, spooler SlurmSpooler, store JobStore, metrics *Metrics) *Gateway {
	if metrics == nil {
		metrics = NewMetrics()
	}
	if store == nil {
		store = NewInMemoryJobStore()
	}
	if spooler == nil {
		spooler = MockSpooler{}
	}
	return &Gateway{
		cfg:     cfg,
		spooler: spooler,
		store:   store,
		metrics: metrics,
		now:     time.Now,
	}
}

func (g *Gateway) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", g.handleHealth)
	mux.HandleFunc("/readyz", g.handleReady)
	mux.HandleFunc("/metrics", g.handleMetrics)
	if g.simops != nil {
		mux.HandleFunc("/api/simops/runs", g.handleSimopsRuns)
		mux.HandleFunc("/api/simops/runs/", g.handleSimopsRun)
		mux.HandleFunc("/internal/simops/runs/", g.handleInternalSimopsRun)
	}
	mux.HandleFunc("/api/jobs/submit", g.handleSubmitJob)
	mux.HandleFunc("/api/jobs/", g.handleJobStatus)
	return mux
}

func (g *Gateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Only GET requests allowed", Code: "method_not_allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (g *Gateway) handleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Only GET requests allowed", Code: "method_not_allowed"})
		return
	}
	if err := g.readyError(); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: "Gateway is not ready", Code: "not_ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ready",
		"mode":   string(g.cfg.Mode),
		"simops": map[string]any{
			"enabled":              g.cfg.Simops.Enabled,
			"control_store":        g.cfg.Simops.ControlStore,
			"telemetry_log":        g.cfg.Simops.TelemetryLog,
			"stream_protocol":      "moq-webtransport",
			"stream_gateway":       g.cfg.Simops.MoQWebTransportURL,
			"iceberg_catalog":      g.cfg.Simops.IcebergCatalog,
			"iceberg_writer_mode":  g.cfg.Simops.IcebergWriterMode,
			"adapter_contracts_v1": true,
		},
	})
}

func (g *Gateway) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Only GET requests allowed", Code: "method_not_allowed"})
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(g.metrics.Render(g.readyError() == nil)))
}

func (g *Gateway) handleSubmitJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Only POST requests allowed", Code: "method_not_allowed"})
		return
	}

	identity, ok := g.authorizedIdentity(w, r)
	if !ok {
		g.metrics.IncSubmit("unauthorized", string(g.cfg.Mode))
		return
	}

	req, ok := g.decodeSubmitRequest(w, r)
	if !ok {
		g.metrics.IncSubmit("rejected", string(g.cfg.Mode))
		return
	}

	if err := g.validateSubmitRequest(&req); err != nil {
		g.metrics.IncSubmit("rejected", string(g.cfg.Mode))
		writeJSON(w, http.StatusUnprocessableEntity, ErrorResponse{Error: err.Error(), Code: "validation_failed"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), g.cfg.RequestTimeout)
	defer cancel()

	result, err := g.spooler.Submit(ctx, req, identity)
	if err != nil {
		g.metrics.IncSubmit("failed", string(g.cfg.Mode))
		log.Printf("audit job_submit failed client=%q mode=%q error=%q", identity, g.cfg.Mode, err.Error())
		writeJSON(w, http.StatusBadGateway, ErrorResponse{Error: "Failed to spool Slurm request", Code: "spool_failed"})
		return
	}

	record := JobRecord{
		JobID:       result.JobID,
		State:       result.State,
		Mode:        result.Mode,
		ScriptName:  req.ScriptName,
		Partition:   req.Partition,
		NodeCount:   req.NodeCount,
		RankCount:   req.RankCount,
		SubmittedBy: identity,
		SubmittedAt: g.now().UTC(),
	}
	g.store.Save(record)
	g.metrics.IncSubmit("success", result.Mode)
	log.Printf("audit job_submit accepted client=%q job_id=%q mode=%q", identity, result.JobID, result.Mode)

	writeJSON(w, http.StatusAccepted, SubmitResponse{
		Message: result.Message,
		JobID:   result.JobID,
		State:   result.State,
		Mode:    result.Mode,
	})
}

func (g *Gateway) handleJobStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "Only GET requests allowed", Code: "method_not_allowed"})
		return
	}
	if _, ok := g.authorizedIdentity(w, r); !ok {
		return
	}

	jobID := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
	if jobID == "" || strings.Contains(jobID, "/") {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "Job not found", Code: "job_not_found"})
		return
	}

	record, err := g.store.Get(jobID)
	if errors.Is(err, ErrJobNotFound) {
		writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "Job not found", Code: "job_not_found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "Failed to read job status", Code: "status_failed"})
		return
	}

	writeJSON(w, http.StatusOK, record.StatusResponse())
}

func (g *Gateway) decodeSubmitRequest(w http.ResponseWriter, r *http.Request) (SubmitRequest, bool) {
	defer r.Body.Close()

	var req SubmitRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxSubmitBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid JSON payload", Code: "bad_json"})
		return SubmitRequest{}, false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "Invalid JSON payload", Code: "bad_json"})
		return SubmitRequest{}, false
	}
	return req, true
}

func (g *Gateway) validateSubmitRequest(req *SubmitRequest) error {
	req.ScriptName = strings.TrimSpace(req.ScriptName)
	req.Partition = strings.TrimSpace(req.Partition)
	if req.RankCount == 0 {
		req.RankCount = req.NodeCount
	}

	if !scriptNamePattern.MatchString(req.ScriptName) {
		return errors.New("script_name must start with an alphanumeric character and contain only alphanumerics, underscores, or dashes")
	}
	if _, ok := g.cfg.AllowedScripts[req.ScriptName]; !ok {
		return errors.New("script_name is not in the configured allowlist")
	}
	if _, ok := g.cfg.AllowedPartitions[req.Partition]; !ok {
		return errors.New("partition is not in the configured allowlist")
	}
	if req.NodeCount < 1 || req.NodeCount > g.cfg.MaxNodes {
		return errors.New("node_count is outside the configured bounds")
	}
	if req.RankCount < 1 || req.RankCount > g.cfg.MaxRanks {
		return errors.New("rank_count is outside the configured bounds")
	}
	return nil
}

func (g *Gateway) authorizedIdentity(w http.ResponseWriter, r *http.Request) (string, bool) {
	if !g.cfg.RequireClientCert {
		return "local-dev", true
	}

	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "mTLS client certificate missing", Code: "client_cert_missing"})
		return "", false
	}

	clientCert := r.TLS.PeerCertificates[0]
	identity := strings.TrimSpace(clientCert.Subject.CommonName)
	if identity == "" {
		writeJSON(w, http.StatusForbidden, ErrorResponse{Error: "mTLS client certificate common name is empty", Code: "client_identity_empty"})
		return "", false
	}
	if _, ok := g.cfg.AllowedClients[identity]; !ok {
		writeJSON(w, http.StatusForbidden, ErrorResponse{Error: "mTLS client identity is not authorized", Code: "client_forbidden"})
		return "", false
	}

	return identity, true
}

func (g *Gateway) readyError() error {
	if err := g.cfg.Validate(); err != nil {
		return err
	}
	if g.cfg.Mode == ModeSbatch {
		info, err := os.Stat(g.cfg.ScriptRoot)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return errors.New("script root is not a directory")
		}
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}
