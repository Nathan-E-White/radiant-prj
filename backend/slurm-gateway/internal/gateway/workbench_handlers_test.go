package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWorkbenchScadaIngestRequiresDeclaredMeasuredTag(t *testing.T) {
	app := newWorkbenchTestGateway(t)
	source := scadaSourceFixture()
	postWorkbenchJSON(t, app, "/internal/scada/sources", source)

	frame := scadaFrameFixture()
	postWorkbenchJSON(t, app, "/internal/scada/telemetry", ScadaTelemetryBatch{Frames: []ScadaTelemetryFrame{frame}})

	req := signedRequest(http.MethodGet, "/api/simulator-workbench/measured", "", "react-backend-client")
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected measured ok, got %d: %s", rr.Code, rr.Body.String())
	}
	var response struct {
		Frames []ScadaTelemetryFrame `json:"frames"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode measured: %v", err)
	}
	if len(response.Frames) != 1 || response.Frames[0].ValueBasis != WorkbenchValueMeasured {
		t.Fatalf("expected measured frame, got %#v", response.Frames)
	}
}

func TestWorkbenchSnapshotHandlerReturnsOneCoherentGeneration(t *testing.T) {
	app := newWorkbenchTestGateway(t)
	postWorkbenchJSON(t, app, "/internal/scada/sources", scadaSourceFixture())
	postWorkbenchJSON(t, app, "/internal/scada/telemetry", ScadaTelemetryBatch{Frames: []ScadaTelemetryFrame{scadaFrameFixture()}})

	req := signedRequest(http.MethodGet, "/api/simulator-workbench/snapshot", "", "react-backend-client")
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected coherent snapshot, got %d: %s", rr.Code, rr.Body.String())
	}
	var snapshot WorkbenchSnapshot
	if err := json.Unmarshal(rr.Body.Bytes(), &snapshot); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if snapshot.Generation == 0 || snapshot.State.SnapshotGeneration != snapshot.Generation || len(snapshot.Measured) != 1 {
		t.Fatalf("handler mixed or omitted snapshot generation: %#v", snapshot)
	}
}

func TestWorkbenchScadaIngestRejectsImputedFrame(t *testing.T) {
	app := newWorkbenchTestGateway(t)
	postWorkbenchJSON(t, app, "/internal/scada/sources", scadaSourceFixture())
	frame := scadaFrameFixture()
	frame.ValueBasis = WorkbenchValueImputed

	req := workbenchRequest(http.MethodPost, "/internal/scada/telemetry", mustJSON(t, ScadaTelemetryBatch{Frames: []ScadaTelemetryFrame{frame}}))
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected unprocessable entity, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "valueBasis=measured") {
		t.Fatalf("expected measured-basis error, got %s", rr.Body.String())
	}
}

func TestWorkbenchScadaIngestRejectsUndeclaredTag(t *testing.T) {
	app := newWorkbenchTestGateway(t)
	postWorkbenchJSON(t, app, "/internal/scada/sources", scadaSourceFixture())
	frame := scadaFrameFixture()
	frame.TagID = "TAG-NOT-DECLARED"

	req := workbenchRequest(http.MethodPost, "/internal/scada/telemetry", mustJSON(t, ScadaTelemetryBatch{Frames: []ScadaTelemetryFrame{frame}}))
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected unprocessable entity, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "resident source declaration") {
		t.Fatalf("expected undeclared-tag error, got %s", rr.Body.String())
	}
}

func TestWorkbenchScadaIngestRequiresToken(t *testing.T) {
	app := newWorkbenchTestGateway(t)
	req := httptest.NewRequest(http.MethodPost, "/internal/scada/sources", strings.NewReader(mustJSON(t, scadaSourceFixture())))
	rr := httptest.NewRecorder()

	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSimopsResultIngestAcceptsSimulatedResult(t *testing.T) {
	app, record := newSimopsResultTestRun(t, "RUN-RESULT-GOOD")

	req := httptest.NewRequest(http.MethodPost, "/internal/simops/runs/RUN-RESULT-GOOD/results", strings.NewReader(mustJSON(t, SimopsResultBatch{Results: []SimopsResultFrame{simopsResultFixture("RUN-RESULT-GOOD")}})))
	req.Header.Set("X-Simops-Ingest-Token", record.IngestToken)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected accepted, got %d: %s", rr.Code, rr.Body.String())
	}
	results, err := app.workbench.store.LatestResultFrames(10)
	if err != nil {
		t.Fatalf("latest results: %v", err)
	}
	if len(results) != 1 || results[0].ValueBasis != WorkbenchValueSimulated {
		t.Fatalf("expected simulated result, got %#v", results)
	}
	artifact, err := app.workbench.store.(artifactForgeResultArtifactReader).ArtifactForgeResultArtifact(record.RunID)
	if err != nil || artifact.Kind != ArtifactForgeEligibleArtifactKind || artifact.Status != ArtifactForgeArtifactCommitted || artifact.Integrity != ArtifactForgeIntegrityVerified {
		t.Fatalf("durably projected Simulated Result State omitted verified result artifact: artifact=%#v err=%v", artifact, err)
	}
}

func TestSimopsResultIngestRejectsTerminalRunToken(t *testing.T) {
	app, record := newSimopsResultTestRun(t, "RUN-RESULT-COMPLETE")
	if _, err := app.simops.store.UpdateRunLifecycle(record.RunID, SimopsComplete); err != nil {
		t.Fatalf("complete run: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/internal/simops/runs/RUN-RESULT-COMPLETE/results", strings.NewReader(mustJSON(t, SimopsResultBatch{Results: []SimopsResultFrame{simopsResultFixture(record.RunID)}})))
	req.Header.Set("X-Simops-Ingest-Token", record.IngestToken)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "run_not_writable") {
		t.Fatalf("terminal Run result token was not fenced, got %d: %s", rr.Code, rr.Body.String())
	}
	results, err := app.workbench.store.LatestResultFrames(10)
	if err != nil || len(results) != 0 {
		t.Fatalf("terminal Run ingest persisted results: results=%#v err=%v", results, err)
	}
}

func TestSimopsResultIngestRejectsImputedWorkerOutput(t *testing.T) {
	app, record := newSimopsResultTestRun(t, "RUN-RESULT-BAD")
	result := simopsResultFixture("RUN-RESULT-BAD")
	result.ValueBasis = WorkbenchValueImputed

	req := httptest.NewRequest(http.MethodPost, "/internal/simops/runs/RUN-RESULT-BAD/results", strings.NewReader(mustJSON(t, SimopsResultBatch{Results: []SimopsResultFrame{result}})))
	req.Header.Set("X-Simops-Ingest-Token", record.IngestToken)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected unprocessable entity, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "valueBasis=simulated") {
		t.Fatalf("expected simulated-basis error, got %s", rr.Body.String())
	}
}

func TestSimopsResultIngestRejectsWrongRunID(t *testing.T) {
	app, record := newSimopsResultTestRun(t, "RUN-RESULT-PATH")
	result := simopsResultFixture("RUN-RESULT-BODY")

	req := httptest.NewRequest(http.MethodPost, "/internal/simops/runs/RUN-RESULT-PATH/results", strings.NewReader(mustJSON(t, SimopsResultBatch{Results: []SimopsResultFrame{result}})))
	req.Header.Set("X-Simops-Ingest-Token", record.IngestToken)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected unprocessable entity, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "runId does not match") {
		t.Fatalf("expected run mismatch error, got %s", rr.Body.String())
	}
}

func TestSimopsResultIngestRejectsWrongToken(t *testing.T) {
	app, _ := newSimopsResultTestRun(t, "RUN-RESULT-TOKEN")
	result := simopsResultFixture("RUN-RESULT-TOKEN")

	req := httptest.NewRequest(http.MethodPost, "/internal/simops/runs/RUN-RESULT-TOKEN/results", strings.NewReader(mustJSON(t, SimopsResultBatch{Results: []SimopsResultFrame{result}})))
	req.Header.Set("X-Simops-Ingest-Token", "wrong-token")
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSimopsResultIngestRejectsOversizedPayload(t *testing.T) {
	app, record := newSimopsResultTestRun(t, "RUN-RESULT-LARGE")

	req := httptest.NewRequest(http.MethodPost, "/internal/simops/runs/RUN-RESULT-LARGE/results", strings.NewReader(strings.Repeat(" ", maxWorkbenchBodyBytes+1)))
	req.Header.Set("X-Simops-Ingest-Token", record.IngestToken)
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "payload exceeds") {
		t.Fatalf("expected payload limit error, got %s", rr.Body.String())
	}
}

func newWorkbenchTestGateway(t *testing.T) *Gateway {
	t.Helper()
	cfg := DefaultConfig()
	cfg.Workbench.InternalIngestToken = "test-workbench-token"
	app := NewGateway(cfg, MockSpooler{}, NewInMemoryJobStore(), NewMetrics())
	app.workbench = NewWorkbenchController(cfg.Workbench, NewInMemoryWorkbenchStore(), nil)
	return app
}

func newSimopsResultTestRun(t *testing.T, runID string) (*Gateway, SimopsRunRecord) {
	t.Helper()
	app, _ := newSimopsTestGateway(t, runID)
	app.workbench = NewWorkbenchController(app.cfg.Workbench, NewInMemoryWorkbenchStore(), nil)
	create := signedRequest(http.MethodPost, "/api/simops/runs", `{"scenario_id":"scheduler-drift","worker_kinds":["burst"]}`, "react-backend-client")
	app.Handler().ServeHTTP(httptest.NewRecorder(), create)
	record, err := app.simops.store.GetRun(runID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	return app, record
}

func postWorkbenchJSON(t *testing.T, app *Gateway, path string, payload any) {
	t.Helper()
	req := workbenchRequest(http.MethodPost, path, mustJSON(t, payload))
	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("%s expected accepted, got %d: %s", path, rr.Code, rr.Body.String())
	}
}

func workbenchRequest(method string, path string, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Workbench-Ingest-Token", "test-workbench-token")
	return req
}

func mustJSON(t *testing.T, payload any) string {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(raw)
}

func scadaSourceFixture() ScadaResidentSourceDeclaration {
	return ScadaResidentSourceDeclaration{
		SchemaVersion:   WorkbenchSourceSchemaVersion,
		SourceID:        "SRC-MIXED-STANDIN-001",
		DisplayName:     "Mixed public-safe resident source stand-ins",
		Lifecycle:       "resident",
		SyntheticStatus: WorkbenchSyntheticPublicStandin,
		Ingest: ScadaIngest{
			Topic:        "scada.telemetry.v1",
			EndpointKind: "gateway-http",
		},
		Tags: []ScadaSourceTag{{
			TagID:      "TAG-FLUX-CORE-A",
			AssetID:    "ASSET-CORE-A",
			SignalKind: ScadaSignalFlux,
			Unit:       "relative-flux",
			ValueBasis: WorkbenchValueMeasured,
		}},
	}
}

func scadaFrameFixture() ScadaTelemetryFrame {
	return ScadaTelemetryFrame{
		SchemaVersion:   WorkbenchScadaSchemaVersion,
		SourceID:        "SRC-MIXED-STANDIN-001",
		TagID:           "TAG-FLUX-CORE-A",
		AssetID:         "ASSET-CORE-A",
		SignalKind:      ScadaSignalFlux,
		SampledAt:       time.Date(2026, 7, 6, 15, 0, 0, 0, time.UTC),
		ObservedAt:      time.Date(2026, 7, 6, 15, 0, 1, 0, time.UTC),
		Sequence:        1,
		Unit:            "relative-flux",
		Value:           map[string]any{"scalar": 0.82},
		Quality:         "good",
		ValueBasis:      WorkbenchValueMeasured,
		SyntheticStatus: WorkbenchSyntheticPublicStandin,
	}
}

func simopsResultFixture(runID string) SimopsResultFrame {
	return SimopsResultFrame{
		SchemaVersion:   WorkbenchResultSchemaVersion,
		RunID:           runID,
		ScenarioID:      "scheduler-drift",
		WorkerID:        "burst-01",
		WorkerKind:      SimopsWorkerBurst,
		Sequence:        1,
		ProducedAt:      "2026-07-04T18:00:16.400Z",
		ReceivedAt:      "2026-07-04T18:00:16.488Z",
		ResultType:      "syntheticEngineeringState",
		ModelID:         WorkbenchDefaultTwinModelID,
		InputWindow:     SimopsResultInputWindow{Start: "2026-07-04T18:00:01Z", End: "2026-07-04T18:00:16Z"},
		ValueBasis:      WorkbenchValueSimulated,
		SyntheticStatus: WorkbenchSyntheticPublicStandin,
		Values: []SimopsResultValue{{
			ResultID:   "SIM-RESULT-CORE-MARGIN",
			EntityID:   "ASSET-CORE-A",
			ValueID:    WorkbenchSimulatedMarginValue,
			Label:      "Simulated forecast margin",
			Unit:       "percent",
			Value:      json.RawMessage(`{"scalar":16.1}`),
			Confidence: 0.71,
		}},
	}
}
