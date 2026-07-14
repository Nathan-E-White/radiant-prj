package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestArtifactForgeIntentEndpointExposesRunAndOutcomeTrace(t *testing.T) {
	app, _ := newSimopsTestGateway(t, "RUN-FORGE-API")
	workbench := NewInMemoryWorkbenchStore()
	app.workbench = NewWorkbenchController(app.cfg.Workbench, workbench, nil)
	app.artifactForge = NewArtifactForgeManager(NewInMemoryArtifactForgeStore(), app.simops, workbench)
	body := `{"intent":"requestArtifactForge","gameSessionId":"game-session-api","reactorId":"reactor-api","simulationJobId":"local-job-api","simulationJobState":"completed","simulationRecipe":"scheduler-drift","idempotencyKey":"forge-api-click"}`

	accepted := httptest.NewRecorder()
	app.Handler().ServeHTTP(accepted, signedRequest(http.MethodPost, "/api/fleet-board/intents", body, "react-backend-client"))
	if accepted.Code != http.StatusAccepted {
		t.Fatalf("accepted intent status=%d body=%s", accepted.Code, accepted.Body.String())
	}
	var record ArtifactForgeRecord
	if err := json.Unmarshal(accepted.Body.Bytes(), &record); err != nil || record.RunID != "RUN-FORGE-API" || record.SimulationJobID == record.RunID {
		t.Fatalf("decode distinct Run association: record=%#v err=%v", record, err)
	}

	if _, err := app.simops.store.UpdateRunLifecycle(record.RunID, SimopsComplete); err != nil {
		t.Fatal(err)
	}
	request := ArtifactForgeRequest{GameSessionID: record.GameSessionID, ReactorID: record.ReactorID, SimulationJobID: record.SimulationJobID, SimulationJobState: "completed", SimulationRecipe: record.SimulationRecipe, IdempotencyKey: record.IdempotencyKey}
	seedEligibleArtifactForgeProjection(t, workbench, record, request)

	applied := httptest.NewRecorder()
	app.Handler().ServeHTTP(applied, signedRequest(http.MethodPost, "/api/fleet-board/intents", body, "react-backend-client"))
	if applied.Code != http.StatusOK {
		t.Fatalf("applied intent status=%d body=%s", applied.Code, applied.Body.String())
	}
	if err := json.Unmarshal(applied.Body.Bytes(), &record); err != nil || record.Outcome == nil || record.Decision != ArtifactForgeOutcomeApplied || record.Trace.ArtifactID == "" || record.Trace.LineageID == "" {
		t.Fatalf("visible outcome omitted trace: record=%#v err=%v", record, err)
	}
}

func TestArtifactForgeIntentEndpointRejectsIncompleteLocalJobWithoutRun(t *testing.T) {
	app, _ := newSimopsTestGateway(t, "RUN-MUST-NOT-START")
	workbench := NewInMemoryWorkbenchStore()
	app.workbench = NewWorkbenchController(app.cfg.Workbench, workbench, nil)
	app.artifactForge = NewArtifactForgeManager(NewInMemoryArtifactForgeStore(), app.simops, workbench)
	body := `{"intent":"requestArtifactForge","gameSessionId":"game-session-api","reactorId":"reactor-api","simulationJobId":"local-job-api","simulationJobState":"running","simulationRecipe":"scheduler-drift","idempotencyKey":"forge-api-rejected"}`

	rr := httptest.NewRecorder()
	app.Handler().ServeHTTP(rr, signedRequest(http.MethodPost, "/api/fleet-board/intents", body, "react-backend-client"))
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("incomplete local job status=%d body=%s", rr.Code, rr.Body.String())
	}
	var record ArtifactForgeRecord
	if err := json.Unmarshal(rr.Body.Bytes(), &record); err != nil || record.Decision != ArtifactForgeIntentRejected || record.RunID != "" || record.Message == "" {
		t.Fatalf("rejection was not explicit: record=%#v err=%v", record, err)
	}
}
