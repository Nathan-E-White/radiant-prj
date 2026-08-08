package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDeliveryAttemptStoreCreatesOneStableAttemptAndEvidence(t *testing.T) {
	store := NewInMemorySimopsStore()
	run := SimopsRunRecord{RunID: "RUN-DELIVERY", ScenarioID: "delivery", Lifecycle: SimopsStreaming, Source: "test", WorkScript: "test", LaunchMode: "contract", RuntimeLimitSec: 1, SubmittedBy: "test", IngestToken: "token", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if _, _, err := store.CreateRun(run, nil, nil); err != nil {
		t.Fatalf("create run: %v", err)
	}

	request := DeliveryAttemptRequest{
		RunID: "RUN-DELIVERY", Target: "simops.telemetry_frames",
		Coordinates: []DeliveryCoordinate{{Topic: "simops.telemetry.v1", Partition: 2, Offset: 41}, {Topic: "simops.telemetry.v1", Partition: 2, Offset: 42}},
	}
	first, created, err := store.CreateDeliveryAttempt(request)
	if err != nil || !created {
		t.Fatalf("create attempt: created=%v err=%v", created, err)
	}
	second, created, err := store.CreateDeliveryAttempt(request)
	if err != nil || created {
		t.Fatalf("replay attempt: created=%v err=%v", created, err)
	}
	if first.AttemptID == "" || second.AttemptID != first.AttemptID || first.State != DeliveryAttemptPending {
		t.Fatalf("expected stable pending identity, first=%#v second=%#v", first, second)
	}

	evidence := VerifiedDeliveryEvidence{AttemptID: first.AttemptID, Assurance: DeliveryAssuranceManifestWritten, Coordinates: request.Coordinates, Reconciliation: DeliveryReconciliationVerified, ObservedAt: time.Now().UTC()}
	if err := store.ResolveDeliveryAttempt(first.AttemptID, evidence); err != nil {
		t.Fatalf("resolve attempt: %v", err)
	}
	resolved, err := store.GetDeliveryAttempt(first.AttemptID)
	if err != nil {
		t.Fatalf("get resolved attempt: %v", err)
	}
	if resolved.State != DeliveryAttemptResolved || resolved.Evidence == nil || resolved.Evidence.Assurance != DeliveryAssuranceManifestWritten {
		t.Fatalf("expected durable resolved evidence, got %#v", resolved)
	}
}

func TestArtifactIntentProcessorDoesNotReplaceUnknownDeliveryAttempt(t *testing.T) {
	store := NewInMemorySimopsStore()
	run := SimopsRunRecord{RunID: "RUN-PROCESSOR-UNKNOWN", ScenarioID: "delivery", Lifecycle: SimopsStreaming, Source: "test", WorkScript: "test", LaunchMode: "contract", RuntimeLimitSec: 1, SubmittedBy: "test", IngestToken: "token", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if _, _, err := store.CreateRun(run, nil, nil); err != nil {
		t.Fatalf("create run: %v", err)
	}
	writer := &uncertainDeliveryWriter{err: errors.New("timeout after send")}
	processor := NewSimopsArtifactIntentProcessorWithDeliveryStore(writer, nil, "simops.telemetry.v1", 1, time.Now, store)
	event := SimopsEvent{RunID: run.RunID, EventType: SimopsEventWorkerTelemetry, RedpandaTopic: "simops.telemetry.v1", RedpandaPartition: 1, RedpandaOffset: 8}
	if _, err := processor.ProcessEvent(context.Background(), event); err == nil {
		t.Fatal("first uncertain write must fail")
	}
	if writer.writes != 1 {
		t.Fatalf("expected one attempted write, got %d", writer.writes)
	}
	if _, err := processor.ProcessEvent(context.Background(), event); !errors.Is(err, ErrUnknownDeliveryAttempt) {
		t.Fatalf("replay must require reconciliation, got %v", err)
	}
	if writer.writes != 1 {
		t.Fatalf("unknown replay attempted a replacement append: %d writes", writer.writes)
	}
}

func TestArtifactIntentProcessorReplayAfterVerifiedDeliveryWritesOnce(t *testing.T) {
	store := NewInMemorySimopsStore()
	run := SimopsRunRecord{RunID: "RUN-PROCESSOR-REPLAY", ScenarioID: "delivery", Lifecycle: SimopsStreaming, Source: "test", WorkScript: "test", LaunchMode: "contract", RuntimeLimitSec: 1, SubmittedBy: "test", IngestToken: "token", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if _, _, err := store.CreateRun(run, nil, nil); err != nil {
		t.Fatalf("create run: %v", err)
	}
	writer := &uncertainDeliveryWriter{}
	processor := NewSimopsArtifactIntentProcessorWithDeliveryStore(writer, nil, "simops.telemetry.v1", 1, time.Now, store)
	event := SimopsEvent{RunID: run.RunID, EventType: SimopsEventWorkerTelemetry, RedpandaTopic: "simops.telemetry.v1", RedpandaPartition: 1, RedpandaOffset: 10}
	if _, err := processor.ProcessEvent(context.Background(), event); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if _, err := processor.ProcessEvent(context.Background(), event); err != nil {
		t.Fatalf("replay: %v", err)
	}
	if writer.writes != 1 {
		t.Fatalf("verified replay wrote %d times", writer.writes)
	}
	attempts, err := store.ListDeliveryAttempts(run.RunID)
	if err != nil || len(attempts) != 1 || attempts[0].Evidence == nil {
		t.Fatalf("expected one verified attempt, attempts=%#v err=%v", attempts, err)
	}
}

func TestManifestWriterReconcilesOriginalAttemptFromStableIdentity(t *testing.T) {
	writer := &ManifestSimopsArtifactWriter{base: &simopsArtifactWriterBase{manifestDir: t.TempDir(), now: time.Now}}
	plan, err := writer.Prepare(SimopsArtifactWritePlan{Artifact: SimopsArtifactRecord{RunID: "RUN-MANIFEST-RECOVERY", ArtifactID: "artifact"}, DeliveryAttemptID: "delivery-original", Topic: "simops.telemetry.v1", Partition: "run_id=RUN-MANIFEST-RECOVERY", Sequence: 1, EventCount: 1})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if err := writer.WriteArtifact(plan.Artifact.RunID, plan); err != nil {
		t.Fatalf("write: %v", err)
	}
	evidence, resolved, err := writer.ReconcileDeliveryAttempt(DeliveryAttempt{AttemptID: "delivery-original", Location: plan.Artifact.Location, Coordinates: []DeliveryCoordinate{{Topic: "simops.telemetry.v1", Partition: 0, Offset: 7}}})
	if err != nil || !resolved {
		t.Fatalf("reconcile manifest: resolved=%v err=%v", resolved, err)
	}
	if evidence.Assurance != DeliveryAssuranceManifestWritten || evidence.AttemptID != "delivery-original" {
		t.Fatalf("unexpected evidence %#v", evidence)
	}
}

func TestRunReaderExposesSanitizedVerifiedDeliveryEvidence(t *testing.T) {
	app, _ := newSimopsTestGateway(t, "RUN-DELIVERY-READER")
	create := signedRequest(http.MethodPost, "/api/simops/runs", `{"scenario_id":"scheduler-drift","worker_kinds":["scheduler"]}`, "react-backend-client")
	app.Handler().ServeHTTP(httptest.NewRecorder(), create)
	attempt, _, err := app.simops.store.CreateDeliveryAttempt(DeliveryAttemptRequest{RunID: "RUN-DELIVERY-READER", Target: "simops.telemetry_frames", Coordinates: []DeliveryCoordinate{{Topic: "simops.telemetry.v1", Partition: 2, Offset: 44}}})
	if err != nil {
		t.Fatalf("create attempt: %v", err)
	}
	if err := app.simops.store.ResolveDeliveryAttempt(attempt.AttemptID, VerifiedDeliveryEvidence{Assurance: DeliveryAssuranceManifestWritten, Reconciliation: DeliveryReconciliationVerified, ObservedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("resolve attempt: %v", err)
	}

	request := signedRequest(http.MethodGet, "/api/simops/runs/RUN-DELIVERY-READER", "", "react-backend-client")
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("get run: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if bytes := recorder.Body.Bytes(); string(bytes) == "" || string(bytes) == "{}" {
		t.Fatalf("expected run response")
	}
	var response SimopsRunResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.DeliveryAttempts) != 1 || response.DeliveryAttempts[0].AttemptID != attempt.AttemptID || response.DeliveryAttempts[0].Assurance != DeliveryAssuranceManifestWritten {
		t.Fatalf("unexpected delivery response %#v", response.DeliveryAttempts)
	}
	if response.DeliveryAttempts[0].Location != "" || response.DeliveryAttempts[0].Reason != "" {
		t.Fatalf("reader leaked storage-facing detail %#v", response.DeliveryAttempts[0])
	}
}

type uncertainDeliveryWriter struct {
	err    error
	writes int
}

func (w *uncertainDeliveryWriter) Prepare(plan SimopsArtifactWritePlan) (SimopsArtifactWritePlan, error) {
	return plan, nil
}
func (w *uncertainDeliveryWriter) WriteArtifact(string, SimopsArtifactWritePlan) error {
	w.writes++
	return w.err
}
func (w *uncertainDeliveryWriter) Commit(string) error { return nil }

func TestDeliveryAttemptStoreRejectsReplacementWhileUnknown(t *testing.T) {
	store := NewInMemorySimopsStore()
	run := SimopsRunRecord{RunID: "RUN-UNKNOWN", ScenarioID: "delivery", Lifecycle: SimopsStreaming, Source: "test", WorkScript: "test", LaunchMode: "contract", RuntimeLimitSec: 1, SubmittedBy: "test", IngestToken: "token", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if _, _, err := store.CreateRun(run, nil, nil); err != nil {
		t.Fatalf("create run: %v", err)
	}
	request := DeliveryAttemptRequest{RunID: run.RunID, Target: "simops.telemetry_frames", Coordinates: []DeliveryCoordinate{{Topic: "simops.telemetry.v1", Partition: 0, Offset: 9}}}
	attempt, _, err := store.CreateDeliveryAttempt(request)
	if err != nil {
		t.Fatalf("create attempt: %v", err)
	}
	if err := store.MarkDeliveryAttemptUnknown(attempt.AttemptID, "writer result unavailable"); err != nil {
		t.Fatalf("mark unknown: %v", err)
	}
	replayed, created, err := store.CreateDeliveryAttempt(request)
	if err != nil || created || replayed.AttemptID != attempt.AttemptID || replayed.State != DeliveryAttemptUnknown {
		t.Fatalf("unknown attempt must be recovered rather than replaced: attempt=%#v created=%v err=%v", replayed, created, err)
	}
}
