package gateway

import (
	"context"
	"errors"
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
