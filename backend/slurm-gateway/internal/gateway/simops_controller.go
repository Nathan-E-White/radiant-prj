package gateway

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

const maxSimopsBodyBytes = 256 * 1024

var runIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)

type SimopsController struct {
	cfg      SimopsConfig
	store    SimopsStore
	spooler  SimopsSpooler
	eventLog SimopsEventLog
	artifact SimopsArtifactSink
	writer   SimopsArtifactWriter
	intent   *SimopsArtifactIntentProcessor
	now      func() time.Time
	runID    func() string
}

func NewDefaultSimopsController(cfg SimopsConfig) (*SimopsController, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	spooler := SimopsSpooler(ContractSimopsSpooler{Mode: cfg.LaunchMode})
	if cfg.WorkerRuntime == "docker" {
		spooler = NewDockerSimopsSpooler(cfg)
	}

	var store SimopsStore = NewInMemorySimopsStore()
	if cfg.ControlStore == "postgres" {
		postgresStore, err := NewPostgresSimopsStore(cfg.PostgresDSN)
		if err != nil {
			return nil, err
		}
		store = postgresStore
	}

	eventLog := SimopsEventLog(MemorySimopsEventLog{Store: store})
	if cfg.TelemetryLog == "redpanda" {
		redpandaLog, err := NewRedpandaEventLog(cfg, store)
		if err != nil {
			return nil, err
		}
		eventLog = redpandaLog
	}

	writer, err := NewSimopsArtifactWriter(cfg, store, time.Now)
	if err != nil {
		return nil, err
	}
	intent := NewSimopsArtifactIntentProcessor(writer, eventLog, cfg.RedpandaTopic, 1, time.Now)

	return NewSimopsController(
		cfg,
		store,
		spooler,
		eventLog,
		IcebergArtifactPlanner{
			Warehouse: cfg.IcebergWarehouse,
			Bucket:    cfg.IcebergS3Bucket,
			Catalog:   cfg.IcebergCatalog,
		},
		writer,
		intent,
	), nil
}

func NewSimopsController(cfg SimopsConfig, store SimopsStore, spooler SimopsSpooler, eventLog SimopsEventLog, artifact SimopsArtifactSink, writer SimopsArtifactWriter, intent *SimopsArtifactIntentProcessor) *SimopsController {
	if store == nil {
		store = NewInMemorySimopsStore()
	}
	if spooler == nil {
		spooler = ContractSimopsSpooler{Mode: cfg.LaunchMode}
	}
	if eventLog == nil {
		eventLog = MemorySimopsEventLog{Store: store}
	}
	if artifact == nil {
		artifact = IcebergArtifactPlanner{Warehouse: cfg.IcebergWarehouse, Bucket: cfg.IcebergS3Bucket, Catalog: cfg.IcebergCatalog}
	}
	if writer == nil {
		writer = &DisabledSimopsArtifactWriter{base: &simopsArtifactWriterBase{store: store, topic: cfg.RedpandaTopic, manifestDir: cfg.IcebergManifestDir, now: time.Now}}
	}
	if intent == nil {
		intent = NewSimopsArtifactIntentProcessor(writer, eventLog, cfg.RedpandaTopic, 1, time.Now)
	}
	return &SimopsController{
		cfg:      cfg,
		store:    store,
		spooler:  spooler,
		eventLog: eventLog,
		artifact: artifact,
		writer:   writer,
		intent:   intent,
		now:      time.Now,
		runID:    defaultRunID,
	}
}

func (c *SimopsController) CreateRun(ctx context.Context, req SimopsRunRequest, identity string) (SimopsRunResponse, int, error) {
	workers, err := normalizeWorkerKinds(req.WorkerKinds)
	if err != nil {
		return SimopsRunResponse{}, http.StatusUnprocessableEntity, err
	}
	if err := normalizeRunRequest(&req, c.cfg.LaunchMode); err != nil {
		return SimopsRunResponse{}, http.StatusUnprocessableEntity, err
	}
	if req.IdempotencyKey != "" {
		if existing, err := c.store.GetRunByIdempotency(identity, req.IdempotencyKey); err == nil {
			resp, _, err := c.responseFor(existing, false)
			return resp, http.StatusOK, err
		} else if !errors.Is(err, ErrSimopsRunNotFound) {
			return SimopsRunResponse{}, http.StatusInternalServerError, err
		}
	}
	if c.store.ActiveRunCount() >= c.cfg.MaxActiveRuns {
		return SimopsRunResponse{}, http.StatusTooManyRequests, fmt.Errorf("maximum active SimOps runs reached")
	}

	now := c.now().UTC()
	record := SimopsRunRecord{
		RunID:           c.runID(),
		ScenarioID:      req.ScenarioID,
		Lifecycle:       SimopsStarting,
		Source:          req.Source,
		WorkScript:      req.WorkScript,
		LaunchMode:      req.LaunchMode,
		RuntimeLimitSec: req.RuntimeLimitSec,
		IdempotencyKey:  req.IdempotencyKey,
		SubmittedBy:     identity,
		IngestToken:     randomToken(),
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	workerRecords, commands, err := c.spooler.StartRun(ctx, record, workers)
	if err != nil {
		return SimopsRunResponse{}, http.StatusBadGateway, fmt.Errorf("failed to start SimOps workers: %w", err)
	}

	record, created, err := c.store.CreateRun(record, workerRecords, commands)
	if err != nil {
		return SimopsRunResponse{}, http.StatusInternalServerError, err
	}
	if !created {
		resp, _, err := c.responseFor(record, false)
		return resp, http.StatusOK, err
	}

	if _, err := c.store.UpdateRunLifecycle(record.RunID, SimopsStreaming); err != nil {
		return SimopsRunResponse{}, http.StatusInternalServerError, err
	}
	record, _ = c.store.GetRun(record.RunID)

	if c.artifact != nil {
		if err := c.store.SaveArtifact(c.artifact.PlanArtifact(record)); err != nil {
			return SimopsRunResponse{}, http.StatusInternalServerError, err
		}
	}

	if err := c.eventLog.Publish(ctx, SimopsEvent{
		RunID:      record.RunID,
		EventType:  "run.lifecycle",
		Lifecycle:  record.Lifecycle,
		OccurredAt: c.now().UTC(),
	}); err != nil {
		return SimopsRunResponse{}, http.StatusBadGateway, err
	}

	return c.responseFor(record, true)
}

func (c *SimopsController) GetRun(runID string) (SimopsRunResponse, int, error) {
	if !runIDPattern.MatchString(runID) {
		return SimopsRunResponse{}, http.StatusNotFound, ErrSimopsRunNotFound
	}
	record, err := c.store.GetRun(runID)
	if errors.Is(err, ErrSimopsRunNotFound) {
		return SimopsRunResponse{}, http.StatusNotFound, err
	}
	if err != nil {
		return SimopsRunResponse{}, http.StatusInternalServerError, err
	}
	resp, _, err := c.responseFor(record, false)
	return resp, http.StatusOK, err
}

func (c *SimopsController) ListEvents(runID string) ([]SimopsEvent, int, error) {
	if !runIDPattern.MatchString(runID) {
		return nil, http.StatusNotFound, ErrSimopsRunNotFound
	}
	events, err := c.store.ListEvents(runID)
	if errors.Is(err, ErrSimopsRunNotFound) {
		return nil, http.StatusNotFound, err
	}
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	return events, http.StatusOK, nil
}

func (c *SimopsController) StopRun(ctx context.Context, runID string) (SimopsRunResponse, int, error) {
	record, status, err := c.getRecordForWrite(runID)
	if err != nil {
		return SimopsRunResponse{}, status, err
	}
	if record.Lifecycle == SimopsStopped || record.Lifecycle == SimopsComplete {
		resp, _, err := c.responseFor(record, false)
		return resp, http.StatusOK, err
	}
	if err := c.spooler.StopRun(ctx, runID); err != nil {
		return SimopsRunResponse{}, http.StatusBadGateway, err
	}
	record, err = c.store.UpdateRunLifecycle(runID, SimopsStopped)
	if err != nil {
		return SimopsRunResponse{}, http.StatusInternalServerError, err
	}
	workers, err := c.store.ListWorkers(runID)
	if err != nil {
		return SimopsRunResponse{}, http.StatusInternalServerError, err
	}
	for _, worker := range workers {
		_ = c.store.UpdateWorkerFrames(runID, worker.WorkerID, SimopsStopped, 0)
	}
	if err := c.eventLog.Publish(ctx, SimopsEvent{
		RunID:      record.RunID,
		EventType:  "run.lifecycle",
		Lifecycle:  record.Lifecycle,
		OccurredAt: c.now().UTC(),
	}); err != nil {
		return SimopsRunResponse{}, http.StatusBadGateway, err
	}
	resp, _, err := c.responseFor(record, false)
	return resp, http.StatusOK, err
}

func (c *SimopsController) Ingest(ctx context.Context, runID string, token string, body io.Reader) (int, int, error) {
	record, status, err := c.getRecordForWrite(runID)
	if err != nil {
		return 0, status, err
	}
	if token == "" || token != record.IngestToken {
		return 0, http.StatusUnauthorized, fmt.Errorf("invalid SimOps ingest token")
	}

	frames, err := decodeTelemetryFrames(body)
	if err != nil {
		return 0, http.StatusBadRequest, err
	}
	if len(frames) == 0 {
		return 0, http.StatusBadRequest, fmt.Errorf("at least one telemetry frame is required")
	}

	for _, frame := range frames {
		if err := validateTelemetryFrame(record, frame); err != nil {
			return 0, http.StatusUnprocessableEntity, err
		}
		raw, _ := json.Marshal(frame)
		if err := c.eventLog.Publish(ctx, SimopsEvent{
			RunID:      runID,
			WorkerID:   frame.WorkerID,
			EventType:  "worker.telemetry",
			Frame:      raw,
			OccurredAt: c.now().UTC(),
		}); err != nil {
			return 0, http.StatusBadGateway, err
		}
		if err := c.store.UpdateWorkerFrames(runID, frame.WorkerID, SimopsStreaming, 1); err != nil {
			return 0, http.StatusUnprocessableEntity, err
		}
		if c.intent != nil {
			if _, err := c.intent.ProcessEvent(ctx, SimopsEvent{
				RunID:      runID,
				WorkerID:   frame.WorkerID,
				EventType:  "worker.telemetry",
				Frame:      raw,
				OccurredAt: c.now().UTC(),
			}); err != nil {
				return 0, http.StatusBadGateway, err
			}
		}
	}

	return len(frames), http.StatusAccepted, nil
}

func (c *SimopsController) getRecordForWrite(runID string) (SimopsRunRecord, int, error) {
	if !runIDPattern.MatchString(runID) {
		return SimopsRunRecord{}, http.StatusNotFound, ErrSimopsRunNotFound
	}
	record, err := c.store.GetRun(runID)
	if errors.Is(err, ErrSimopsRunNotFound) {
		return SimopsRunRecord{}, http.StatusNotFound, err
	}
	if err != nil {
		return SimopsRunRecord{}, http.StatusInternalServerError, err
	}
	return record, http.StatusOK, nil
}

func (c *SimopsController) responseFor(record SimopsRunRecord, created bool) (SimopsRunResponse, int, error) {
	workers, err := c.store.ListWorkers(record.RunID)
	if err != nil {
		return SimopsRunResponse{}, http.StatusInternalServerError, err
	}
	sort.Slice(workers, func(i, j int) bool {
		return workers[i].WorkerID < workers[j].WorkerID
	})
	commands, err := c.store.ListCommands(record.RunID)
	if err != nil {
		return SimopsRunResponse{}, http.StatusInternalServerError, err
	}
	artifacts, err := c.store.ListArtifacts(record.RunID)
	if err != nil {
		return SimopsRunResponse{}, http.StatusInternalServerError, err
	}

	return SimopsRunResponse{
		RunID:           record.RunID,
		ScenarioID:      record.ScenarioID,
		Lifecycle:       record.Lifecycle,
		Source:          record.Source,
		LaunchMode:      record.LaunchMode,
		RuntimeLimitSec: record.RuntimeLimitSec,
		Created:         created,
		SubmittedBy:     record.SubmittedBy,
		CreatedAt:       record.CreatedAt,
		UpdatedAt:       record.UpdatedAt,
		MoQSubscription: buildMoQSubscription(c.cfg, record, workers, c.now().UTC()),
		Workers:         workers,
		SpoolCommands:   commands,
		Artifacts:       artifacts,
	}, http.StatusAccepted, nil
}

func normalizeRunRequest(req *SimopsRunRequest, defaultLaunchMode string) error {
	req.ScenarioID = strings.TrimSpace(req.ScenarioID)
	req.Source = strings.TrimSpace(req.Source)
	req.WorkScript = strings.TrimSpace(req.WorkScript)
	req.LaunchMode = strings.TrimSpace(req.LaunchMode)
	req.IdempotencyKey = strings.TrimSpace(req.IdempotencyKey)

	if req.ScenarioID == "" {
		req.ScenarioID = "scheduler-drift"
	}
	if !allowedScenario(req.ScenarioID) {
		return fmt.Errorf("scenario_id is not supported")
	}
	if req.Source == "" {
		req.Source = "frontend"
	}
	if req.Source != "frontend" && req.Source != "work-script" {
		return fmt.Errorf("source must be frontend or work-script")
	}
	if req.WorkScript == "" {
		req.WorkScript = req.ScenarioID
	}
	if !allowedWorkScript(req.WorkScript) {
		return fmt.Errorf("work_script is not in the configured SimOps allowlist")
	}
	if req.LaunchMode == "" {
		req.LaunchMode = defaultLaunchMode
	}
	switch req.LaunchMode {
	case "resident", "spawn", "auto":
	default:
		return fmt.Errorf("launch_mode must be resident, spawn, or auto")
	}
	if req.RuntimeLimitSec == 0 {
		req.RuntimeLimitSec = 120
	}
	if req.RuntimeLimitSec < 1 || req.RuntimeLimitSec > 3600 {
		return fmt.Errorf("runtime_limit_sec is outside the configured bounds")
	}
	return nil
}

func normalizeWorkerKinds(raw []string) ([]SimopsWorkerKind, error) {
	if len(raw) == 0 {
		return append([]SimopsWorkerKind(nil), defaultSimopsWorkers...), nil
	}
	if len(raw) > 4 {
		return nil, fmt.Errorf("at most four SimOps worker kinds can be selected")
	}
	seen := make(map[SimopsWorkerKind]struct{})
	workers := make([]SimopsWorkerKind, 0, len(raw))
	for _, value := range raw {
		worker := SimopsWorkerKind(strings.TrimSpace(value))
		if !allowedWorker(worker) {
			return nil, fmt.Errorf("worker_kinds contains unsupported worker %q", value)
		}
		if _, ok := seen[worker]; ok {
			return nil, fmt.Errorf("worker_kinds contains duplicate worker %q", value)
		}
		seen[worker] = struct{}{}
		workers = append(workers, worker)
	}
	return workers, nil
}

func decodeTelemetryFrames(body io.Reader) ([]SimopsTelemetryFrame, error) {
	payload, err := io.ReadAll(io.LimitReader(body, maxSimopsBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(payload) > maxSimopsBodyBytes {
		return nil, fmt.Errorf("telemetry payload exceeds %d bytes", maxSimopsBodyBytes)
	}
	var batch SimopsTelemetryBatch
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&batch); err == nil && len(batch.Frames) > 0 {
		return batch.Frames, nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(payload))
	scanner.Buffer(make([]byte, 0, 64*1024), maxSimopsBodyBytes)
	frames := []SimopsTelemetryFrame{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var frame SimopsTelemetryFrame
		if err := json.Unmarshal([]byte(line), &frame); err != nil {
			return nil, fmt.Errorf("invalid NDJSON telemetry frame: %w", err)
		}
		frames = append(frames, frame)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return frames, nil
}

func validateTelemetryFrame(run SimopsRunRecord, frame SimopsTelemetryFrame) error {
	if frame.SchemaVersion != "simops.telemetry.v1" {
		return fmt.Errorf("unsupported telemetry schemaVersion")
	}
	if frame.RunID != run.RunID {
		return fmt.Errorf("telemetry runId does not match ingest path")
	}
	if frame.ScenarioID != run.ScenarioID {
		return fmt.Errorf("telemetry scenarioId does not match run")
	}
	if !allowedWorker(frame.WorkerKind) {
		return fmt.Errorf("telemetry workerKind is not supported")
	}
	if strings.TrimSpace(frame.WorkerID) == "" {
		return fmt.Errorf("telemetry workerId is required")
	}
	if frame.Sequence == 0 {
		return fmt.Errorf("telemetry sequence must be positive")
	}
	if strings.TrimSpace(frame.EmittedAt) == "" {
		return fmt.Errorf("telemetry emittedAt is required")
	}
	if strings.TrimSpace(frame.PayloadType) == "" {
		return fmt.Errorf("telemetry payloadType is required")
	}
	if len(frame.Payload) == 0 || string(frame.Payload) == "null" {
		return fmt.Errorf("telemetry payload is required")
	}
	return nil
}

func allowedScenario(value string) bool {
	switch value {
	case "nominal", "scheduler-drift", "checkpoint-pressure", "cloud-burst", "fabric-warning":
		return true
	default:
		return false
	}
}

func allowedWorkScript(value string) bool {
	switch value {
	case "nominal", "scheduler-drift", "checkpoint-pressure", "cloud-burst", "fabric-warning", "module-rerun":
		return true
	default:
		return false
	}
}

func allowedWorker(value SimopsWorkerKind) bool {
	switch value {
	case SimopsWorkerScheduler, SimopsWorkerStorage, SimopsWorkerBurst, SimopsWorkerFabric:
		return true
	default:
		return false
	}
}

func defaultRunID() string {
	return "RUN-" + strings.ToUpper(randomToken()[:12])
}
