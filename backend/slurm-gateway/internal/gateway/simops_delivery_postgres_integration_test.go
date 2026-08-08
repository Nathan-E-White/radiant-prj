//go:build postgresintegration

package gateway

import (
	"testing"
	"time"
)

func TestPostgresDeliveryAttemptPersistsUnknownAcrossStoreReopen(t *testing.T) {
	workbench := openConfiguredDataFlushPostgresTestStore(t)
	store := &PostgresSimopsStore{db: workbench.db}
	now := time.Now().UTC()
	run := SimopsRunRecord{RunID: "delivery-postgres-restart", ScenarioID: "delivery", Lifecycle: SimopsStreaming, Source: "test", WorkScript: "test", LaunchMode: "contract", RuntimeLimitSec: 1, SubmittedBy: "test", IngestToken: "token", CreatedAt: now, UpdatedAt: now}
	if _, _, err := store.CreateRun(run, nil, nil); err != nil {
		t.Fatalf("create run: %v", err)
	}
	request := DeliveryAttemptRequest{RunID: run.RunID, Target: "simops.telemetry_frames", Coordinates: []DeliveryCoordinate{{Topic: "simops.telemetry.v1", Partition: 0, Offset: 41}}}
	attempt, created, err := store.CreateDeliveryAttempt(request)
	if err != nil || !created {
		t.Fatalf("create attempt: created=%v err=%v", created, err)
	}
	if err := store.PrepareDeliveryAttempt(attempt.AttemptID, "file:///warehouse/attempt"); err != nil {
		t.Fatalf("prepare attempt: %v", err)
	}
	if err := store.MarkDeliveryAttemptUnknown(attempt.AttemptID, "timeout after append"); err != nil {
		t.Fatalf("mark unknown: %v", err)
	}

	restarted := &PostgresSimopsStore{db: reopenArtifactForgePostgresTestDatabase(t, workbench.db)}
	recovered, created, err := restarted.CreateDeliveryAttempt(request)
	if err != nil || created {
		t.Fatalf("recover attempt: created=%v err=%v", created, err)
	}
	if recovered.AttemptID != attempt.AttemptID || recovered.State != DeliveryAttemptUnknown || recovered.Location != "file:///warehouse/attempt" || recovered.Reason != "timeout after append" {
		t.Fatalf("restart lost delivery state: %#v", recovered)
	}
	if err := restarted.ResolveDeliveryAttempt(recovered.AttemptID, VerifiedDeliveryEvidence{Assurance: DeliveryAssuranceManifestWritten, Reconciliation: DeliveryReconciliationVerified, ObservedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("resolve recovered attempt: %v", err)
	}
	resolved, err := restarted.GetDeliveryAttempt(recovered.AttemptID)
	if err != nil || resolved.State != DeliveryAttemptResolved || resolved.Evidence == nil {
		t.Fatalf("read resolved attempt: %#v err=%v", resolved, err)
	}
}
