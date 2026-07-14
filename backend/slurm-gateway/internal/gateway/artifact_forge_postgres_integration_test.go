//go:build postgresintegration

package gateway

import (
	"errors"
	"os"
	"testing"
	"time"
)

func TestPostgresArtifactForgeLedgerRecoversAssociationAndProtectsAppliedOutcome(t *testing.T) {
	dsn := os.Getenv("RADIANT_POSTGRES_TEST_DSN")
	store, err := NewPostgresArtifactForgeStore(dsn)
	if err != nil {
		t.Fatalf("open Artifact Forge ledger: %v", err)
	}
	t.Cleanup(func() {
		_, _ = store.db.Exec(`DELETE FROM artifact_forge_requests WHERE request_id = 'forge-postgres-recovery'`)
		_ = store.db.Close()
	})
	now := time.Now().UTC()
	record := ArtifactForgeRecord{
		RequestID: "forge-postgres-recovery", GameSessionID: "game-postgres-recovery", ReactorID: "reactor-postgres",
		SimulationJobID: "local-job-postgres", SimulationRecipe: ArtifactForgeSchedulerDriftRecipe, IdempotencyKey: "retry-key",
		RunID: "run-postgres-recovery", Decision: ArtifactForgeAwaitingRun, Message: "awaiting", Trace: ArtifactForgeTrace{SimulationJobID: "local-job-postgres", RunID: "run-postgres-recovery"}, CreatedAt: now, UpdatedAt: now,
	}
	setArtifactForgeRetention(&record, now)
	if err := store.Save(record); err != nil {
		t.Fatalf("save association: %v", err)
	}

	restarted, err := NewPostgresArtifactForgeStore(dsn)
	if err != nil {
		t.Fatalf("restart Artifact Forge ledger: %v", err)
	}
	defer restarted.db.Close()
	recovered, err := restarted.Find(record.GameSessionID, record.IdempotencyKey)
	if err != nil || recovered.RunID != record.RunID || recovered.SimulationJobID != record.SimulationJobID {
		t.Fatalf("recover association: record=%#v err=%v", recovered, err)
	}

	recovered.Decision = ArtifactForgeOutcomeApplied
	recovered.Outcome = &ArtifactForgeGameOutcome{OutcomeID: "outcome-postgres", Type: ArtifactForgeSimulatedMarginCard, Version: 1, ReactorID: recovered.ReactorID, ArtifactID: "artifact-postgres", LineageID: "lineage-postgres", ValueID: WorkbenchSimulatedMarginValue}
	recovered.UpdatedAt = time.Now().UTC()
	if err := restarted.Save(recovered); err != nil {
		t.Fatalf("atomically save applied outcome: %v", err)
	}
	recovered.Outcome = nil
	if err := restarted.Save(recovered); err == nil {
		t.Fatal("applied outcome consumption marker was removed")
	}
	recovered.Outcome = &ArtifactForgeGameOutcome{OutcomeID: "different-outcome", Type: ArtifactForgeSimulatedMarginCard, Version: 1}
	if err := restarted.Save(recovered); err == nil {
		t.Fatal("applied outcome consumption marker was replaced")
	}
	final, err := store.Find(record.GameSessionID, record.IdempotencyKey)
	if err != nil || final.Outcome == nil || final.Outcome.OutcomeID != "outcome-postgres" {
		t.Fatalf("applied outcome was not durable: record=%#v err=%v", final, err)
	}
	final.LastActivityAt = now.Add(-9 * 24 * time.Hour)
	final.SessionExpiresAt = final.LastActivityAt.Add(24 * time.Hour)
	final.RetainUntil = now.Add(-time.Hour)
	if err := store.Save(final); err != nil {
		t.Fatalf("age retained ledger: %v", err)
	}
	if removed, err := store.PruneExpired(now); err != nil || removed != 1 {
		t.Fatalf("prune expired ledger removed=%d err=%v", removed, err)
	}
	if _, err := store.Find(record.GameSessionID, record.IdempotencyKey); !errors.Is(err, ErrArtifactForgeNotFound) {
		t.Fatalf("expired Postgres ledger still present: %v", err)
	}
}

func TestPostgresArtifactForgeResultArtifactCommitsWithProjection(t *testing.T) {
	store := openPostgresSnapshotTestStore(t)
	frame := artifactForgeResultFrame("run-postgres-forge-artifact", ArtifactForgeSchedulerDriftRecipe)
	projection := SimopsResultProjection{ProducedAt: time.Now().UTC(), ReceivedAt: time.Now().UTC(), Frame: frame, RedpandaTopic: "artifact-forge-results", RedpandaOffset: 1}
	written, err := store.SaveResultProjection("artifact-forge-result-writer", projection)
	if err != nil || !written {
		t.Fatalf("commit result projection: written=%v err=%v", written, err)
	}
	artifact, err := store.ArtifactForgeResultArtifact(frame.RunID)
	if err != nil || artifact.Status != ArtifactForgeArtifactCommitted || artifact.SchemaVersion != WorkbenchResultSchemaVersion || artifact.Integrity != ArtifactForgeIntegrityVerified || !artifact.Complete {
		t.Fatalf("durable projection omitted verified artifact metadata: artifact=%#v err=%v", artifact, err)
	}
}
