package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSimopsCreateRunReturnsMoQSubscription(t *testing.T) {
	app, _ := newSimopsTestGateway(t, "RUN-SIMOPS-001")

	req := signedRequest(http.MethodPost, "/api/simops/runs", `{"scenario_id":"scheduler-drift","worker_kinds":["scheduler"],"launch_mode":"resident"}`, "react-backend-client")
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected accepted, got %d: %s", rr.Code, rr.Body.String())
	}

	var response SimopsRunResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.RunID != "RUN-SIMOPS-001" {
		t.Fatalf("expected run id, got %q", response.RunID)
	}
	if response.Lifecycle != SimopsStreaming {
		t.Fatalf("expected streaming lifecycle, got %q", response.Lifecycle)
	}
	if response.MoQSubscription.Protocol != "moq-webtransport" {
		t.Fatalf("expected MoQ/WebTransport subscription, got %q", response.MoQSubscription.Protocol)
	}
	if response.MoQSubscription.Namespace != "radiant/simops/RUN-SIMOPS-001" {
		t.Fatalf("unexpected namespace %q", response.MoQSubscription.Namespace)
	}
	if len(response.MoQSubscription.Tracks) != 4 {
		t.Fatalf("expected lifecycle, artifact, telemetry, and quality tracks; got %d", len(response.MoQSubscription.Tracks))
	}
	if len(response.Artifacts) != 1 {
		t.Fatalf("expected one planned artifact, got %d", len(response.Artifacts))
	}
	artifact := response.Artifacts[0]
	if artifact.IcebergTable != "simops.telemetry_frames" {
		t.Fatalf("expected planned Iceberg table, got %#v", artifact)
	}
	if artifact.Kind != "iceberg-table-partition" {
		t.Fatalf("expected planned artifact kind, got %#v", artifact.Kind)
	}
	if artifact.Status != SimopsArtifactStatusReceived {
		t.Fatalf("expected initial artifact status received, got %q", artifact.Status)
	}
	if artifact.Location == "" {
		t.Fatalf("expected artifact location to be present: %#v", artifact)
	}
}

func TestSimopsIdempotencyPreventsDuplicateSpool(t *testing.T) {
	app, spooler := newSimopsTestGateway(t, "RUN-IDEMPOTENT")

	body := `{"scenario_id":"scheduler-drift","worker_kinds":["scheduler"],"idempotency_key":"same-click"}`
	first := signedRequest(http.MethodPost, "/api/simops/runs", body, "react-backend-client")
	second := signedRequest(http.MethodPost, "/api/simops/runs", body, "react-backend-client")

	firstRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(firstRR, first)
	if firstRR.Code != http.StatusAccepted {
		t.Fatalf("first request expected accepted, got %d: %s", firstRR.Code, firstRR.Body.String())
	}

	secondRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(secondRR, second)
	if secondRR.Code != http.StatusOK {
		t.Fatalf("second request expected ok replay, got %d: %s", secondRR.Code, secondRR.Body.String())
	}
	if spooler.starts != 1 {
		t.Fatalf("expected one spool start, got %d", spooler.starts)
	}
}

func TestSimopsInternalIngestRejectsBadToken(t *testing.T) {
	app, _ := newSimopsTestGateway(t, "RUN-INGEST-BAD")
	create := signedRequest(http.MethodPost, "/api/simops/runs", `{"scenario_id":"scheduler-drift","worker_kinds":["scheduler"]}`, "react-backend-client")
	app.Handler().ServeHTTP(httptest.NewRecorder(), create)

	body := telemetryBatch("RUN-INGEST-BAD", "scheduler-01")
	req := httptest.NewRequest(http.MethodPost, "/internal/simops/runs/RUN-INGEST-BAD/ingest", strings.NewReader(body))
	req.Header.Set("X-Simops-Ingest-Token", "wrong")
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSimopsInternalIngestAcceptsTelemetryBatch(t *testing.T) {
	app, _ := newSimopsTestGateway(t, "RUN-INGEST-GOOD")
	create := signedRequest(http.MethodPost, "/api/simops/runs", `{"scenario_id":"scheduler-drift","worker_kinds":["scheduler"]}`, "react-backend-client")
	createRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(createRR, create)
	if createRR.Code != http.StatusAccepted {
		t.Fatalf("create expected accepted, got %d: %s", createRR.Code, createRR.Body.String())
	}

	record, err := app.simops.store.GetRun("RUN-INGEST-GOOD")
	if err != nil {
		t.Fatalf("get run: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/internal/simops/runs/RUN-INGEST-GOOD/ingest", strings.NewReader(telemetryBatch("RUN-INGEST-GOOD", "scheduler-01")))
	req.Header.Set("X-Simops-Ingest-Token", record.IngestToken)
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected ingest accepted, got %d: %s", rr.Code, rr.Body.String())
	}

	status := signedRequest(http.MethodGet, "/api/simops/runs/RUN-INGEST-GOOD", "", "react-backend-client")
	statusRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(statusRR, status)
	if statusRR.Code != http.StatusOK {
		t.Fatalf("status expected ok, got %d: %s", statusRR.Code, statusRR.Body.String())
	}

	var response SimopsRunResponse
	if err := json.Unmarshal(statusRR.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if len(response.Workers) != 1 || response.Workers[0].Frames != 1 {
		t.Fatalf("expected one ingested frame, got %#v", response.Workers)
	}
}

func TestSimopsRunEventsEndpointReturnsPersistedEvents(t *testing.T) {
	app, _ := newSimopsTestGateway(t, "RUN-EVENTS")
	create := signedRequest(http.MethodPost, "/api/simops/runs", `{"scenario_id":"scheduler-drift","worker_kinds":["scheduler"]}`, "react-backend-client")
	createRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(createRR, create)
	if createRR.Code != http.StatusAccepted {
		t.Fatalf("create expected accepted, got %d: %s", createRR.Code, createRR.Body.String())
	}

	events := signedRequest(http.MethodGet, "/api/simops/runs/RUN-EVENTS/events", "", "react-backend-client")
	eventsRR := httptest.NewRecorder()
	app.Handler().ServeHTTP(eventsRR, events)
	if eventsRR.Code != http.StatusOK {
		t.Fatalf("events expected ok, got %d: %s", eventsRR.Code, eventsRR.Body.String())
	}

	var response struct {
		RunID  string        `json:"run_id"`
		Events []SimopsEvent `json:"events"`
	}
	if err := json.Unmarshal(eventsRR.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode events: %v", err)
	}
	if response.RunID != "RUN-EVENTS" {
		t.Fatalf("unexpected run id %q", response.RunID)
	}
	if len(response.Events) == 0 || response.Events[0].EventType != "run.lifecycle" {
		t.Fatalf("expected lifecycle event, got %#v", response.Events)
	}
}

func TestSimopsInternalIngestRejectsOversizedBody(t *testing.T) {
	app, _ := newSimopsTestGateway(t, "RUN-INGEST-LARGE")
	create := signedRequest(http.MethodPost, "/api/simops/runs", `{"scenario_id":"scheduler-drift","worker_kinds":["scheduler"]}`, "react-backend-client")
	app.Handler().ServeHTTP(httptest.NewRecorder(), create)

	record, err := app.simops.store.GetRun("RUN-INGEST-LARGE")
	if err != nil {
		t.Fatalf("get run: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/internal/simops/runs/RUN-INGEST-LARGE/ingest", strings.NewReader(strings.Repeat(" ", maxSimopsBodyBytes+1)))
	req.Header.Set("X-Simops-Ingest-Token", record.IngestToken)
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d: %s", rr.Code, rr.Body.String())
	}
}

func newSimopsTestGateway(t *testing.T, runID string) (*Gateway, *countingSimopsSpooler) {
	t.Helper()
	cfg := DefaultConfig()
	cfg.Simops.MoQWebTransportURL = "https://localhost:9443/moq/simops"
	cfg.Simops.StreamTokenTTL = time.Minute
	store := NewInMemorySimopsStore()
	spooler := &countingSimopsSpooler{delegate: ContractSimopsSpooler{Mode: "resident"}}
	controller := NewSimopsController(
		cfg.Simops,
		store,
		spooler,
		MemorySimopsEventLog{Store: store},
		IcebergArtifactPlanner{Warehouse: "s3://radiant-simops/warehouse", Bucket: "radiant-simops", Catalog: "postgres-sql"},
		nil,
		nil,
	)
	controller.runID = func() string { return runID }
	controller.now = func() time.Time { return time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC) }

	app := NewGateway(cfg, MockSpooler{}, NewInMemoryJobStore(), NewMetrics())
	app.simops = controller
	return app, spooler
}

type countingSimopsSpooler struct {
	delegate ContractSimopsSpooler
	starts   int
}

func (s *countingSimopsSpooler) StartRun(ctx context.Context, run SimopsRunRecord, workers []SimopsWorkerKind) ([]SimopsWorkerRecord, []SimopsSpoolCommand, error) {
	s.starts++
	return s.delegate.StartRun(ctx, run, workers)
}

func (s *countingSimopsSpooler) StopRun(ctx context.Context, runID string) error {
	return s.delegate.StopRun(ctx, runID)
}

func telemetryBatch(runID string, workerID string) string {
	return `{"frames":[{"schemaVersion":"simops.telemetry.v1","runId":"` + runID + `","scenarioId":"scheduler-drift","workerId":"` + workerID + `","workerKind":"scheduler","sequence":1,"emittedAt":"2026-07-04T12:00:00.000Z","payloadType":"schedulerCoScheduling","payload":{}}]}`
}
