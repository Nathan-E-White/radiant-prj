package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDynamicReactorIntentLaunchesWorkersWithoutExposingCredentials(t *testing.T) {
	app, runtime := reactorTelemetryGatewayFixture(t)
	payload := `{"intent":"registerDynamicReactor","gameSessionId":"session-http","reactorId":"reactor-http","idempotencyKey":"register-http"}`
	req := workbenchRequest(http.MethodPost, "/api/fleet-board/intents", payload)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusAccepted {
		t.Fatalf("expected accepted registration, got %d: %s", res.Code, res.Body.String())
	}
	if strings.Contains(res.Body.String(), "ingestToken") || strings.Contains(res.Body.String(), "ingestBaseUrl") || strings.Contains(res.Body.String(), "test-only-signing-key") {
		t.Fatalf("browser-facing response exposed a worker credential: %s", res.Body.String())
	}
	var response struct {
		Created bool                      `json:"created"`
		Set     ReactorTelemetryWorkerSet `json:"workerSet"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Created || response.Set.ReactorID != "reactor-http" || len(response.Set.Workers) != 3 {
		t.Fatalf("unexpected registration response: %#v", response)
	}
	if len(runtime.launches) != 1 {
		t.Fatalf("expected one runtime launch, got %#v", runtime.launches)
	}

	retry := workbenchRequest(http.MethodPost, "/api/fleet-board/intents", payload)
	retryRes := httptest.NewRecorder()
	app.Handler().ServeHTTP(retryRes, retry)
	if retryRes.Code != http.StatusOK || len(runtime.launches) != 1 {
		t.Fatalf("idempotent HTTP retry launched parallel workers: %d %#v", retryRes.Code, runtime.launches)
	}
}

func TestDynamicSourceCredentialIsBoundToSourceAndReactor(t *testing.T) {
	app, runtime := reactorTelemetryGatewayFixture(t)
	set, _, err := app.reactorTelemetry.RegisterDynamicReactor(context.Background(), RegisterDynamicReactorRequest{
		GameSessionID: "session-auth", ReactorID: "reactor-auth", IdempotencyKey: "register-auth",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	worker := runtime.launches[0].Workers[0]
	source := BuildReactorResidentSource(worker)

	accepted := dynamicScadaRequest(t, "/internal/scada/sources", source, worker.Gateway.IngestToken)
	acceptedRes := httptest.NewRecorder()
	app.Handler().ServeHTTP(acceptedRes, accepted)
	if acceptedRes.Code != http.StatusAccepted {
		t.Fatalf("bound source declaration rejected: %d %s", acceptedRes.Code, acceptedRes.Body.String())
	}

	wrongSource := source
	wrongSource.SourceID = set.Workers[1].SourceID
	denied := dynamicScadaRequest(t, "/internal/scada/sources", wrongSource, worker.Gateway.IngestToken)
	deniedRes := httptest.NewRecorder()
	app.Handler().ServeHTTP(deniedRes, denied)
	if deniedRes.Code != http.StatusUnauthorized {
		t.Fatalf("source credential authorized a different source: %d", deniedRes.Code)
	}

	frames := BuildReactorTelemetryFrames(worker, 1, time.Date(2026, 7, 14, 10, 30, 0, 0, time.UTC))
	frameReq := dynamicScadaRequest(t, "/internal/scada/telemetry", ScadaTelemetryBatch{Frames: frames}, worker.Gateway.IngestToken)
	frameRes := httptest.NewRecorder()
	app.Handler().ServeHTTP(frameRes, frameReq)
	if frameRes.Code != http.StatusAccepted {
		t.Fatalf("bound measured frames rejected: %d %s", frameRes.Code, frameRes.Body.String())
	}

	frames[0].ReactorID = "reactor-other"
	wrongReactor := dynamicScadaRequest(t, "/internal/scada/telemetry", ScadaTelemetryBatch{Frames: frames}, worker.Gateway.IngestToken)
	wrongReactorRes := httptest.NewRecorder()
	app.Handler().ServeHTTP(wrongReactorRes, wrongReactor)
	if wrongReactorRes.Code != http.StatusUnauthorized {
		t.Fatalf("source credential authorized a different reactor: %d", wrongReactorRes.Code)
	}
}

func TestRemoveDynamicReactorRevokesCredentialBeforeRuntimeCleanup(t *testing.T) {
	app, runtime := reactorTelemetryGatewayFixture(t)
	set, _, err := app.reactorTelemetry.RegisterDynamicReactor(context.Background(), RegisterDynamicReactorRequest{
		GameSessionID: "session-revoke", ReactorID: "reactor-revoke", IdempotencyKey: "register-revoke",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	worker := set.Workers[0]
	runtime.stopErr = context.DeadlineExceeded
	payload := `{"intent":"removeDynamicReactor","gameSessionId":"session-revoke","reactorId":"reactor-revoke","idempotencyKey":"remove-revoke"}`
	req := workbenchRequest(http.MethodPost, "/api/fleet-board/intents", payload)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusBadGateway {
		t.Fatalf("cleanup failure must remain visible, got %d: %s", res.Code, res.Body.String())
	}

	source := BuildReactorResidentSource(worker)
	ingest := dynamicScadaRequest(t, "/internal/scada/sources", source, worker.Gateway.IngestToken)
	ingestRes := httptest.NewRecorder()
	app.Handler().ServeHTTP(ingestRes, ingest)
	if ingestRes.Code != http.StatusUnauthorized {
		t.Fatalf("revoked credential remained usable during failed cleanup: %d", ingestRes.Code)
	}
}

func reactorTelemetryGatewayFixture(t *testing.T) (*Gateway, *recordingReactorTelemetryRuntime) {
	t.Helper()
	cfg := DefaultConfig()
	cfg.RequireClientCert = false
	app := NewGateway(cfg, MockSpooler{}, NewInMemoryJobStore(), NewMetrics())
	workbench := NewWorkbenchController(cfg.Workbench, NewInMemoryWorkbenchStore(), nil)
	runtime := &recordingReactorTelemetryRuntime{}
	telemetryCfg := DefaultReactorTelemetryConfig()
	telemetryCfg.IngestBaseURL = "http://gateway:8080"
	telemetryCfg.CredentialSigningKey = "test-only-signing-key"
	app.workbench = workbench
	app.reactorTelemetry = NewReactorTelemetryManager(telemetryCfg, NewInMemoryReactorTelemetryStore(), runtime, workbench)
	return app, runtime
}

func dynamicScadaRequest(t *testing.T, path string, payload any, token string) *http.Request {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Workbench-Ingest-Token", token)
	return req
}
