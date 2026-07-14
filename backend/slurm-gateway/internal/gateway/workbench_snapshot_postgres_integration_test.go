//go:build postgresintegration

package gateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPostgresTwinStatePublisherRecoversEventFailureFromPersistedPublication(t *testing.T) {
	store := openPostgresSnapshotTestStore(t)
	eventFailure := errors.New("ambiguous Redpanda delivery")
	events := &twinPublicationEventLog{err: eventFailure}
	publisher := NewTwinStatePublisher(store, events)
	publication := twinPublicationFixture("simops.results.v1", 4, 77)
	originalAsOf := publication.State.AsOf
	originalStep := publication.Lineage[0].ProcessingSteps[0]

	outcome, err := publisher.Publish(context.Background(), publication)
	var publicationErr *TwinStatePublicationError
	if !errors.As(err, &publicationErr) || publicationErr.Stage != TwinStatePublicationEventDelivery || !outcome.Persisted || outcome.Delivered {
		t.Fatalf("unexpected Postgres event failure outcome=%#v err=%v", outcome, err)
	}
	snapshot, err := store.Snapshot()
	if err != nil || snapshot.Generation != 1 || snapshot.Twin.SchemaVersion == "" || len(snapshot.Lineage) != len(publication.Lineage) {
		t.Fatalf("event failure did not leave one explained persisted state: %#v err=%v", snapshot, err)
	}

	events.err = nil
	publication.State.AsOf = publication.State.AsOf.Add(time.Hour)
	publication.Lineage[0].ProcessingSteps[0] = "caller drift after persistence"
	outcome, err = publisher.Publish(context.Background(), publication)
	if err != nil || outcome.Persisted || !outcome.Duplicate || !outcome.Delivered {
		t.Fatalf("unexpected Postgres retry outcome=%#v err=%v", outcome, err)
	}
	if !events.publications[1].State.AsOf.Equal(originalAsOf) || events.publications[1].Lineage[0].ProcessingSteps[0] != originalStep {
		t.Fatalf("Postgres retry did not deliver persisted publication: %#v", events.publications[1])
	}
	after, err := store.Snapshot()
	if err != nil || after.Generation != 1 {
		t.Fatalf("Postgres retry repersisted generation: %#v err=%v", after, err)
	}
	if err := publisher.Acknowledge(publication.Source); err != nil {
		t.Fatalf("acknowledge Postgres publication: %v", err)
	}
	tombstone, err := store.GetTwinStatePublication(publication.PublicationID)
	if err != nil || !tombstone.Acknowledged || tombstone.State.SchemaVersion != "" || len(tombstone.Lineage) != 0 {
		t.Fatalf("Postgres acknowledgement did not compact payload: publication=%#v err=%v", tombstone, err)
	}
}

func TestPostgresTwinProjectorHydratesNewestResultAcrossKeys(t *testing.T) {
	store := openPostgresSnapshotTestStore(t)
	cfg := DefaultConfig().Workbench
	frames := []SimopsResultFrame{
		simopsResultFixture("AAA-OLDER-RUN"),
		simopsResultFixture("ZZZ-NEWER-RUN"),
	}
	frames[0].ProducedAt = "2026-07-04T18:00:00Z"
	frames[0].Values[0].ResultID = "RESULT-OLDER"
	frames[1].ProducedAt = "2026-07-04T19:00:00Z"
	frames[1].Values[0].ResultID = "RESULT-NEWER"
	for index, frame := range frames {
		raw, _ := json.Marshal(frame)
		projection, err := ProjectSimopsResultFrame(cfg.ResultsTopic, 0, int64(index+1), raw)
		if err != nil {
			t.Fatalf("project result %d: %v", index, err)
		}
		if written, err := store.SaveResultProjection("hydrate-order", projection); err != nil || !written {
			t.Fatalf("save result %d written=%v err=%v", index, written, err)
		}
	}

	projector, err := NewTwinProjector(cfg, store, &twinPublicationEventLog{})
	if err != nil {
		t.Fatalf("hydrate projector: %v", err)
	}
	if projector.result == nil || projector.result.RunID != "ZZZ-NEWER-RUN" {
		t.Fatalf("hydrated stale result: %#v", projector.result)
	}
}

func TestPostgresWorkbenchSnapshotMigratesAnExistingStore(t *testing.T) {
	store := openPostgresSnapshotTestStore(t)
	ctx, cancel := simopsSQLContext()
	defer cancel()
	if err := ensureWorkbenchSnapshotSchema(ctx, store.db); err != nil {
		t.Fatalf("repeat Snapshot schema migration: %v", err)
	}
	var generation uint64
	if err := store.db.QueryRowContext(ctx, `SELECT generation FROM workbench_snapshot_generation WHERE singleton = TRUE`).Scan(&generation); err != nil || generation != 0 {
		t.Fatalf("migrated generation=%d err=%v", generation, err)
	}
}

func TestPostgresWorkbenchSnapshotKeepsOneMVCCReadMomentDuringCommit(t *testing.T) {
	store := openPostgresSnapshotTestStore(t)
	first := scadaProjectionFixture(t, "TAG-MVCC-BEFORE", 1)
	if written, err := store.SaveScadaProjection("mvcc-test", first); err != nil || !written {
		t.Fatalf("save initial frame written=%v err=%v", written, err)
	}

	boundary := make(chan struct{})
	resume := make(chan struct{})
	store.afterSnapshotGeneration = func() {
		close(boundary)
		<-resume
	}
	type snapshotResult struct {
		snapshot WorkbenchSnapshot
		err      error
	}
	result := make(chan snapshotResult, 1)
	go func() {
		snapshot, err := store.Snapshot()
		result <- snapshotResult{snapshot: snapshot, err: err}
	}()
	<-boundary
	second := scadaProjectionFixture(t, "TAG-MVCC-AFTER", 2)
	if written, err := store.SaveScadaProjection("mvcc-test", second); err != nil || !written {
		close(resume)
		t.Fatalf("save interleaved frame written=%v err=%v", written, err)
	}
	resultRaw, _ := json.Marshal(simopsResultFixture("RUN-MVCC-AFTER"))
	resultProjection, _ := ProjectSimopsResultFrame("snapshot-mvcc-result", 0, 3, resultRaw)
	if written, err := store.SaveResultProjection("mvcc-test", resultProjection); err != nil || !written {
		close(resume)
		t.Fatalf("save interleaved result written=%v err=%v", written, err)
	}
	twinState, twinLineage := BuildTwinStateFromData([]ScadaTelemetryFrame{second.Frame}, resultProjection.Frame, time.Now().UTC())
	twinProjection := TwinStateProjection{State: twinState, Lineage: twinLineage, LineagePresent: true, RedpandaTopic: "snapshot-mvcc-twin", RedpandaOffset: 4}
	if written, err := store.SaveTwinStateProjection("mvcc-test", twinProjection); err != nil || !written {
		close(resume)
		t.Fatalf("save interleaved Twin transition written=%v err=%v", written, err)
	}
	close(resume)
	during := <-result
	store.afterSnapshotGeneration = nil
	if during.err != nil {
		t.Fatalf("read interleaved Snapshot: %v", during.err)
	}
	if during.snapshot.Generation != 1 || during.snapshot.State.SnapshotGeneration != during.snapshot.Generation || len(during.snapshot.Measured) != 1 || during.snapshot.Measured[0].TagID != first.Frame.TagID || len(during.snapshot.Results) != 0 || during.snapshot.Twin.SchemaVersion != "" || len(during.snapshot.Lineage) != 0 {
		t.Fatalf("Snapshot crossed its generation boundary: %#v", during.snapshot)
	}
	after, err := store.Snapshot()
	if err != nil || after.Generation != 4 || after.State.SnapshotGeneration != after.Generation || len(after.Measured) != 2 || len(after.Results) != 1 || after.Twin.SchemaVersion == "" || len(after.Lineage) != len(twinLineage) {
		t.Fatalf("later Snapshot did not observe committed generation: %#v err=%v", after, err)
	}
}

func TestPostgresTwinTransitionRollsBackLineageAndStateThenRecovers(t *testing.T) {
	store := openPostgresSnapshotTestStore(t)
	state, lineage := BuildTwinStateFromData([]ScadaTelemetryFrame{scadaFrameFixture()}, simopsResultFixture("RUN-POSTGRES-TRANSITION"), time.Now().UTC())
	initial := TwinStateProjection{State: state, Lineage: lineage, LineagePresent: true, RedpandaTopic: "twin-transition", RedpandaOffset: 1}
	if written, err := store.SaveTwinStateProjection("twin-transition-test", initial); err != nil || !written {
		t.Fatalf("save initial Twin transition written=%v err=%v", written, err)
	}
	before, err := store.Snapshot()
	if err != nil || before.Generation != 1 || len(before.Lineage) != len(lineage) {
		t.Fatalf("read initial Twin transition: %#v err=%v", before, err)
	}
	targetLineageID := lineage[0].LineageID
	wantStep := postgresSnapshotLineageStep(t, before.Lineage, targetLineageID)

	nextState, _ := cloneWorkbenchValue(state)
	nextLineage, _ := cloneWorkbenchValue(lineage)
	nextState.AsOf = nextState.AsOf.Add(time.Second)
	nextLineage[0].ProcessingSteps[0] = "committed replacement lineage"
	invalidState, _ := cloneWorkbenchValue(nextState)
	interrupted := TwinStateProjection{State: invalidState, Lineage: nextLineage, LineagePresent: true, RedpandaTopic: "twin-transition", RedpandaOffset: 2}
	interrupted.State.Entities[0].Values[0].Value = map[string]any{"invalid": func() {}}
	if _, err := store.SaveTwinStateProjection("twin-transition-test", interrupted); err == nil {
		t.Fatal("expected interrupted Twin transition to fail")
	}
	afterFailure, err := store.Snapshot()
	if err != nil || afterFailure.Generation != before.Generation || postgresSnapshotLineageStep(t, afterFailure.Lineage, targetLineageID) != wantStep || !afterFailure.Twin.AsOf.Equal(before.Twin.AsOf) {
		t.Fatalf("interrupted transition became partly visible: before=%#v after=%#v err=%v", before, afterFailure, err)
	}

	interrupted.State = nextState
	if written, err := store.SaveTwinStateProjection("twin-transition-test", interrupted); err != nil || !written {
		t.Fatalf("retry Twin transition written=%v err=%v", written, err)
	}
	afterRecovery, err := store.Snapshot()
	if err != nil || afterRecovery.Generation != before.Generation+1 || postgresSnapshotLineageStep(t, afterRecovery.Lineage, targetLineageID) != nextLineage[0].ProcessingSteps[0] || !afterRecovery.Twin.AsOf.Equal(nextState.AsOf) {
		t.Fatalf("recovered transition was not atomic: %#v err=%v", afterRecovery, err)
	}
	interrupted.State = invalidState
	if written, err := store.SaveTwinStateProjection("twin-transition-test", interrupted); err != nil || written {
		t.Fatalf("invalid replay should be ignored after commit written=%v err=%v", written, err)
	}

	reduced := interrupted
	reduced.State = nextState
	reduced.RedpandaOffset = 3
	reduced.Lineage = nextLineage[:1]
	if written, err := store.SaveTwinStateProjection("twin-transition-test", reduced); err != nil || !written {
		t.Fatalf("save reduced Twin transition written=%v err=%v", written, err)
	}
	afterReduced, err := store.Snapshot()
	if err != nil || afterReduced.Generation != 3 || len(afterReduced.Lineage) != 1 || afterReduced.Lineage[0].LineageID != nextLineage[0].LineageID {
		t.Fatalf("stale Postgres lineage survived replacement: %#v err=%v", afterReduced, err)
	}
	legacy := reduced
	legacy.RedpandaOffset = 4
	legacy.Lineage = nil
	legacy.LineagePresent = false
	if written, err := store.SaveTwinStateProjection("twin-transition-test", legacy); err != nil || !written {
		t.Fatalf("save legacy state-only transition written=%v err=%v", written, err)
	}
	afterLegacy, err := store.Snapshot()
	if err != nil || afterLegacy.Generation != 4 || len(afterLegacy.Lineage) != 1 {
		t.Fatalf("legacy Postgres transition discarded separately published lineage: %#v err=%v", afterLegacy, err)
	}
	explicitEmpty := legacy
	explicitEmpty.RedpandaOffset = 5
	explicitEmpty.LineagePresent = true
	if written, err := store.SaveTwinStateProjection("twin-transition-test", explicitEmpty); err != nil || !written {
		t.Fatalf("save explicitly empty Postgres lineage transition written=%v err=%v", written, err)
	}
	afterEmpty, err := store.Snapshot()
	if err != nil || afterEmpty.Generation != 5 || len(afterEmpty.Lineage) != 0 {
		t.Fatalf("explicit empty Postgres lineage did not clear active set: %#v err=%v", afterEmpty, err)
	}
}

func postgresSnapshotLineageStep(t *testing.T, lineage []DigitalTwinValueLineage, lineageID string) string {
	t.Helper()
	for _, record := range lineage {
		if record.LineageID == lineageID && len(record.ProcessingSteps) > 0 {
			return record.ProcessingSteps[0]
		}
	}
	t.Fatalf("missing lineage %q", lineageID)
	return ""
}

func openPostgresSnapshotTestStore(t *testing.T) *PostgresWorkbenchStore {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("SIMOPS_POSTGRES_TEST_DSN"))
	if dsn == "" {
		t.Skip("set SIMOPS_POSTGRES_TEST_DSN to run Postgres Snapshot integration tests")
	}
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Scheme == "" {
		t.Fatalf("SIMOPS_POSTGRES_TEST_DSN must be a URL: %q", dsn)
	}
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open Postgres test admin connection: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	schema := fmt.Sprintf("workbench_snapshot_test_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		_ = admin.Close()
		t.Fatalf("create isolated Postgres test schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.ExecContext(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
		_ = admin.Close()
	})
	query := parsed.Query()
	query.Set("options", "-csearch_path="+schema)
	parsed.RawQuery = query.Encode()
	db, err := sql.Open("pgx", parsed.String())
	if err != nil {
		t.Fatalf("open isolated Postgres test store: %v", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		t.Fatalf("ping isolated Postgres test store: %v", err)
	}
	if err := ensureWorkbenchSnapshotSchema(ctx, db); err != nil {
		_ = db.Close()
		t.Fatalf("migrate isolated Postgres test store: %v", err)
	}
	if _, err := db.ExecContext(ctx, postgresSnapshotTestSchema); err != nil {
		_ = db.Close()
		t.Fatalf("create isolated Postgres Workbench tables: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &PostgresWorkbenchStore{db: db}
}

func scadaProjectionFixture(t *testing.T, tagID string, offset int64) ScadaProjection {
	t.Helper()
	frame := scadaFrameFixture()
	frame.TagID = tagID
	frame.Sequence = uint64(offset)
	frame.ObservedAt = frame.ObservedAt.Add(time.Duration(offset) * time.Second)
	raw, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("marshal SCADA fixture: %v", err)
	}
	projection, err := ProjectScadaFrame("snapshot-mvcc", 0, offset, raw)
	if err != nil {
		t.Fatalf("project SCADA fixture: %v", err)
	}
	return projection
}

const postgresSnapshotTestSchema = `
CREATE TABLE workbench_processed_messages (
  consumer_name TEXT NOT NULL, redpanda_topic TEXT NOT NULL, redpanda_partition INTEGER NOT NULL,
  redpanda_offset BIGINT NOT NULL, PRIMARY KEY (consumer_name, redpanda_topic, redpanda_partition, redpanda_offset)
);
CREATE TABLE workbench_consumer_offsets (
  consumer_name TEXT NOT NULL, redpanda_topic TEXT NOT NULL, redpanda_partition INTEGER NOT NULL,
  redpanda_offset BIGINT NOT NULL, updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (consumer_name, redpanda_topic, redpanda_partition)
);
CREATE TABLE scada_measured_frames (
  observed_at TIMESTAMPTZ NOT NULL, sampled_at TIMESTAMPTZ NOT NULL, source_id TEXT NOT NULL,
  tag_id TEXT NOT NULL, asset_id TEXT NOT NULL, signal_kind TEXT NOT NULL, sequence BIGINT NOT NULL,
  unit TEXT NOT NULL, quality TEXT NOT NULL, value_basis TEXT NOT NULL, synthetic_status TEXT NOT NULL,
  value JSONB NOT NULL, frame JSONB NOT NULL, redpanda_topic TEXT NOT NULL,
  redpanda_partition INTEGER NOT NULL, redpanda_offset BIGINT NOT NULL
);
CREATE TABLE simops_result_values (
  produced_at TIMESTAMPTZ NOT NULL, received_at TIMESTAMPTZ NOT NULL, run_id TEXT NOT NULL,
  scenario_id TEXT NOT NULL, worker_id TEXT NOT NULL, worker_kind TEXT NOT NULL, sequence BIGINT NOT NULL,
  result_type TEXT NOT NULL, model_id TEXT NOT NULL, input_window_start TIMESTAMPTZ NOT NULL,
  input_window_end TIMESTAMPTZ NOT NULL, value_basis TEXT NOT NULL, synthetic_status TEXT NOT NULL,
  result_id TEXT NOT NULL, entity_id TEXT NOT NULL, value_id TEXT NOT NULL, label TEXT NOT NULL,
  unit TEXT NOT NULL, value JSONB NOT NULL, confidence DOUBLE PRECISION NOT NULL, frame JSONB NOT NULL,
  redpanda_topic TEXT NOT NULL, redpanda_partition INTEGER NOT NULL, redpanda_offset BIGINT NOT NULL
);
CREATE TABLE digital_twin_state_values (
  as_of TIMESTAMPTZ NOT NULL, twin_id TEXT NOT NULL, entity_id TEXT NOT NULL, display_name TEXT NOT NULL,
  value_id TEXT NOT NULL, label TEXT NOT NULL, value_basis TEXT NOT NULL, unit TEXT NOT NULL,
  value JSONB NOT NULL, confidence DOUBLE PRECISION NOT NULL, freshness JSONB NOT NULL,
  lineage_id TEXT NOT NULL, source_ids JSONB NOT NULL, state JSONB NOT NULL,
  redpanda_topic TEXT NOT NULL, redpanda_partition INTEGER NOT NULL, redpanda_offset BIGINT NOT NULL,
  PRIMARY KEY (twin_id, entity_id, value_id)
);
CREATE TABLE digital_twin_lineage (
  lineage_id TEXT PRIMARY KEY, value_id TEXT NOT NULL, value_basis TEXT NOT NULL,
  lineage JSONB NOT NULL, updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);`
