//go:build postgresintegration

package gateway

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPostgresArtifactForgeRecoveryAppliesOneEligibleOutcome(t *testing.T) {
	workbench := openConfiguredDataFlushPostgresTestStore(t)
	if _, err := workbench.db.Exec(`
		ALTER TABLE artifact_forge_requests ADD COLUMN IF NOT EXISTS last_activity_at TIMESTAMPTZ;
		ALTER TABLE artifact_forge_requests ADD COLUMN IF NOT EXISTS session_expires_at TIMESTAMPTZ;
		ALTER TABLE artifact_forge_requests ADD COLUMN IF NOT EXISTS retain_until TIMESTAMPTZ;
	`); err != nil {
		t.Fatalf("upgrade isolated Artifact Forge ledger: %v", err)
	}
	forgeStore := &PostgresArtifactForgeStore{db: workbench.db}
	simopsStore := &PostgresSimopsStore{db: workbench.db}
	cfg := DefaultConfig().Simops
	controller := NewSimopsController(cfg, simopsStore, ContractSimopsSpooler{Mode: cfg.LaunchMode}, MemorySimopsEventLog{Store: simopsStore}, IcebergArtifactPlanner{}, nil, nil)
	controller.runID = func() string { return "forge-postgres-recovery-run" }
	forge := NewArtifactForgeManager(forgeStore, controller, workbench)
	request := artifactForgeRequestFixture()

	accepted, created, err := forge.Request(context.Background(), request, "fleet-board-client")
	if err != nil || !created {
		t.Fatalf("accept persisted Artifact Forge request: record=%#v created=%v err=%v", accepted, created, err)
	}
	if _, err := simopsStore.UpdateRunLifecycle(accepted.RunID, SimopsComplete); err != nil {
		t.Fatal(err)
	}
	seedEligibleArtifactForgeProjection(t, workbench, accepted, request)

	restartedDB := reopenArtifactForgePostgresTestDatabase(t, workbench.db)
	restartedWorkbench := &PostgresWorkbenchStore{db: restartedDB}
	restartedSimops := &PostgresSimopsStore{db: restartedDB}
	restartedController := NewSimopsController(cfg, restartedSimops, ContractSimopsSpooler{Mode: cfg.LaunchMode}, MemorySimopsEventLog{Store: restartedSimops}, IcebergArtifactPlanner{}, nil, nil)
	restarted := NewArtifactForgeManager(&PostgresArtifactForgeStore{db: restartedDB}, restartedController, restartedWorkbench)
	recovered, err := restarted.Recover(context.Background(), request.GameSessionID, request.IdempotencyKey)
	if err != nil || recovered.Decision != ArtifactForgeOutcomeApplied || recovered.Outcome == nil {
		t.Fatalf("recover persisted eligible outcome: record=%#v err=%v", recovered, err)
	}
	replayed, err := restarted.Recover(context.Background(), request.GameSessionID, request.IdempotencyKey)
	if err != nil || replayed.Outcome == nil || replayed.Outcome.OutcomeID != recovered.Outcome.OutcomeID || artifactForgeEventCount(replayed, ArtifactForgeEventOutcomeApplied) != 1 {
		t.Fatalf("recovery replay changed the durable outcome: record=%#v err=%v", replayed, err)
	}
}

func reopenArtifactForgePostgresTestDatabase(t *testing.T, database *sql.DB) *sql.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("SIMOPS_POSTGRES_TEST_DSN"))
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Scheme == "" {
		t.Fatalf("SIMOPS_POSTGRES_TEST_DSN must be a URL: %q", dsn)
	}
	var schema string
	if err := database.QueryRow(`SELECT current_schema()`).Scan(&schema); err != nil {
		t.Fatalf("read isolated Artifact Forge schema: %v", err)
	}
	query := parsed.Query()
	query.Set("options", "-csearch_path="+schema)
	parsed.RawQuery = query.Encode()
	restarted, err := sql.Open("pgx", parsed.String())
	if err != nil {
		t.Fatalf("reopen isolated Artifact Forge database: %v", err)
	}
	if err := restarted.Ping(); err != nil {
		_ = restarted.Close()
		t.Fatalf("ping reopened Artifact Forge database: %v", err)
	}
	t.Cleanup(func() { _ = restarted.Close() })
	return restarted
}

func TestPostgresArtifactForgeLedgerRecoversAssociationAndProtectsAppliedOutcome(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("SIMOPS_POSTGRES_TEST_DSN"))
	if dsn == "" {
		t.Skip("set SIMOPS_POSTGRES_TEST_DSN to run Artifact Forge Postgres integration tests")
	}
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

func TestPostgresArtifactForgeEligibilityStoreContract(t *testing.T) {
	assertArtifactForgeEligibilityStoreContract(t, openPostgresSnapshotTestStore(t))
}

func TestPostgresArtifactForgeEligibilityMigrationBackfillsLegacyEvidence(t *testing.T) {
	store := openPostgresSnapshotTestStore(t)
	run := SimopsRunRecord{RunID: "run-forge-legacy-upgrade", ScenarioID: ArtifactForgeSchedulerDriftRecipe, Lifecycle: SimopsComplete}
	request := artifactForgeRequestFixture()
	record := ArtifactForgeRecord{RunID: run.RunID, ReactorID: request.ReactorID, GameSessionID: request.GameSessionID, SimulationRecipe: request.SimulationRecipe}
	result := artifactForgeResultFrame(run.RunID, run.ScenarioID)
	if written, err := store.SaveResultProjection("artifact-forge-legacy", SimopsResultProjection{Frame: result, RedpandaTopic: "artifact-forge-legacy-results", RedpandaOffset: 1}); err != nil || !written {
		t.Fatalf("seed legacy result written=%v err=%v", written, err)
	}
	seedArtifactForgeEligibilityLineage(t, store, record, request, result, 2)
	ctx, cancel := simopsSQLContext()
	defer cancel()
	if _, err := store.db.ExecContext(ctx, `ALTER TABLE artifact_forge_result_artifacts DROP COLUMN eligibility_evidence`); err != nil {
		t.Fatalf("simulate legacy schema: %v", err)
	}
	if err := ensureWorkbenchSnapshotSchema(ctx, store.db); err != nil {
		t.Fatalf("migrate legacy eligibility evidence: %v", err)
	}
	evidence, err := store.ReadArtifactForgeEligibility(run)
	if err != nil || evidence.Result == nil || evidence.Result.RunID != run.RunID || evidence.ExpectedValue == nil || evidence.Lineage == nil || !artifactForgeLineageHasInput(*evidence.Lineage, "simulation-run", run.RunID) {
		t.Fatalf("legacy eligibility evidence=%#v err=%v", evidence, err)
	}
}
