package gateway

import (
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
