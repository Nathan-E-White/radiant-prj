//go:build postgresintegration

package gateway

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestPostgresReactorTelemetryRecoversIdentityAndRevocationAcrossRestart(t *testing.T) {
	workbenchStore := openPostgresSnapshotTestStore(t)
	if err := ensureReactorTelemetrySchema(context.Background(), workbenchStore.db); err != nil {
		t.Fatalf("create reactor telemetry schema: %v", err)
	}
	store := &PostgresReactorTelemetryStore{db: workbenchStore.db}
	firstRuntime := &recordingReactorTelemetryRuntime{}
	first := NewReactorTelemetryManager(DefaultReactorTelemetryConfig(), store, firstRuntime, nil)
	request := RegisterDynamicReactorRequest{
		GameSessionID: "postgres-session", ReactorID: "postgres-reactor", IdempotencyKey: "postgres-register",
	}
	set, created, err := first.RegisterDynamicReactor(context.Background(), request)
	if err != nil || !created {
		t.Fatalf("register set created=%v err=%v", created, err)
	}
	token := firstRuntime.launches[0].Workers[0].Gateway.IngestToken
	sourceID := firstRuntime.launches[0].Workers[0].SourceID

	restartedRuntime := &recordingReactorTelemetryRuntime{}
	restarted := NewReactorTelemetryManager(DefaultReactorTelemetryConfig(), store, restartedRuntime, nil)
	recovered, recoveredCreated, err := restarted.RegisterDynamicReactor(context.Background(), request)
	if err != nil || recoveredCreated || recovered.SetID != set.SetID || len(restartedRuntime.launches) != 0 {
		t.Fatalf("restart did not recover persisted identity: set=%#v created=%v err=%v", recovered, recoveredCreated, err)
	}
	if !restarted.AuthorizeSourceCredential(token, sourceID, request.ReactorID) {
		t.Fatal("valid persisted source association did not authorize after restart")
	}

	if _, err := restarted.RemoveDynamicReactor(context.Background(), RemoveDynamicReactorRequest{
		GameSessionID: request.GameSessionID, ReactorID: request.ReactorID, IdempotencyKey: "postgres-remove",
	}); err != nil {
		t.Fatalf("remove set: %v", err)
	}
	afterRemoval := NewReactorTelemetryManager(DefaultReactorTelemetryConfig(), store, &recordingReactorTelemetryRuntime{}, nil)
	if afterRemoval.AuthorizeSourceCredential(token, sourceID, request.ReactorID) {
		t.Fatal("revoked credential became valid after another restart")
	}
}

func TestPostgresReactorTelemetryMeasuredRetentionPreservesSourceDeclaration(t *testing.T) {
	store := openPostgresSnapshotTestStore(t)
	if _, err := store.db.Exec(`
		CREATE TABLE workbench_resident_sources (
			source_id TEXT PRIMARY KEY, declaration JSONB NOT NULL, updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE TABLE workbench_resident_tags (
			tag_id TEXT PRIMARY KEY, source_id TEXT NOT NULL REFERENCES workbench_resident_sources(source_id) ON DELETE CASCADE,
			asset_id TEXT NOT NULL, signal_kind TEXT NOT NULL, unit TEXT NOT NULL, value_basis TEXT NOT NULL, tag JSONB NOT NULL
		)
	`); err != nil {
		t.Fatalf("create resident source test schema: %v", err)
	}
	controller := NewWorkbenchController(DefaultConfig().Workbench, store, nil)
	worker := ReactorTelemetryWorker{SourceID: "postgres-dynamic-source", ReactorID: "postgres-dynamic-reactor", WorkerIndex: 0}
	source := BuildReactorResidentSource(worker)
	if _, err := controller.RegisterSource(source); err != nil {
		t.Fatalf("register source: %v", err)
	}
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	for index, frame := range BuildReactorTelemetryFrames(worker, 1, now.Add(48*time.Hour)) {
		raw, _ := json.Marshal(frame)
		projection, err := ProjectScadaFrame("postgres-retention", 0, int64(index+1), raw)
		if err != nil {
			t.Fatalf("project frame: %v", err)
		}
		if _, err := store.SaveScadaProjection("postgres-retention", projection); err != nil {
			t.Fatalf("save frame: %v", err)
		}
	}
	if _, err := store.db.Exec(`UPDATE scada_measured_frames SET created_at = $1 WHERE source_id = $2`, now.Add(-2*time.Hour), worker.SourceID); err != nil {
		t.Fatalf("backdate ingestion time: %v", err)
	}
	controller.now = func() time.Time { return now }
	controller.dynamicMeasuredRetention = time.Hour
	measured, err := controller.Measured()
	if err != nil || len(measured) != 0 {
		t.Fatalf("expired dynamic measured state remained visible: %#v err=%v", measured, err)
	}
	if _, err := store.GetResidentTag(source.Tags[0].TagID); err != nil {
		t.Fatalf("retention removed protected source declaration: %v", err)
	}
}
