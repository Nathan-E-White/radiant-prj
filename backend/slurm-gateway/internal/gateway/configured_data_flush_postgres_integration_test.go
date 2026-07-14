//go:build postgresintegration

package gateway

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestPostgresConfiguredDataFlushPreservesPlatformAndStartsSubsequentRun(t *testing.T) {
	workbench := openConfiguredDataFlushPostgresTestStore(t)
	seedConfiguredDataFlushRecords(t, workbench.db)
	repository := &PostgresConfiguredDataFlushRepository{db: workbench.db}
	service := NewConfiguredDataFlushService(repository)

	plan, err := service.Plan(context.Background())
	if err != nil {
		t.Fatalf("plan flush: %v", err)
	}
	if !plan.Ready || plan.CurrentGeneration != 0 || plan.NextGeneration != 1 {
		t.Fatalf("unexpected flush plan: %#v", plan)
	}
	for _, target := range plan.Targets {
		want := int64(1)
		if target.Name == "workbench_twin_publications" {
			want = 2
		}
		if target.Rows != want {
			t.Fatalf("plan did not identify seeded target %s: %#v", target.Name, target)
		}
	}

	result, err := service.Apply(context.Background(), plan.PlanID)
	if err != nil {
		t.Fatalf("apply reviewed flush: %v", err)
	}
	if result.PreviousGeneration != 0 || result.Generation != 1 {
		t.Fatalf("flush did not open generation one: %#v", result)
	}
	snapshot, err := workbench.Snapshot()
	if err != nil {
		t.Fatalf("read post-flush Snapshot: %v", err)
	}
	if snapshot.Generation != 1 || len(snapshot.Measured) != 0 || len(snapshot.Results) != 0 || len(snapshot.Twin.Entities) != 0 || len(snapshot.Lineage) != 0 {
		t.Fatalf("post-flush Snapshot mixed generations: %#v", snapshot)
	}
	for table, want := range map[string]int64{
		"workbench_resident_sources":   1,
		"workbench_resident_tags":      1,
		"workbench_consumer_offsets":   1,
		"workbench_processed_messages": 1,
		"simops_runs":                  1,
		"simops_consumer_offsets":      1,
		"simops_processed_messages":    1,
	} {
		var got int64
		if err := workbench.db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&got); err != nil || got != want {
			t.Fatalf("protected table %s count=%d want=%d err=%v", table, got, want, err)
		}
	}
	var publications int64
	if err := workbench.db.QueryRow(`SELECT count(*) FROM workbench_twin_publications`).Scan(&publications); err != nil || publications != 0 {
		t.Fatalf("pre-flush publication recovery records survived count=%d err=%v", publications, err)
	}

	simopsStore := &PostgresSimopsStore{db: workbench.db}
	cfg := DefaultConfig().Simops
	controller := NewSimopsController(cfg, simopsStore, ContractSimopsSpooler{Mode: cfg.LaunchMode}, MemorySimopsEventLog{Store: simopsStore}, nil, nil, nil)
	if _, status, err := controller.Ingest(context.Background(), "run-before-flush", "protected-ingest-token", strings.NewReader("{}")); err == nil || status != 409 {
		t.Fatalf("pre-flush terminal Run token crossed generation boundary: status=%d err=%v", status, err)
	}
	controller.runID = func() string { return "run-after-flush" }
	response, status, err := controller.CreateRun(context.Background(), SimopsRunRequest{
		ScenarioID: "scheduler-drift", IdempotencyKey: "after-flush",
	}, "flush-integration")
	if err != nil || status != 202 || response.RunID != "run-after-flush" {
		t.Fatalf("subsequent Run required reprovisioning: response=%#v status=%d err=%v", response, status, err)
	}
	newRun, err := simopsStore.GetRun(response.RunID)
	if err != nil {
		t.Fatalf("read subsequent Run: %v", err)
	}
	accepted, status, err := controller.Ingest(context.Background(), response.RunID, newRun.IngestToken, strings.NewReader(telemetryBatch(response.RunID, "scheduler-01")))
	if err != nil || status != 202 || accepted != 1 {
		t.Fatalf("subsequent Run could not ingest: accepted=%d status=%d err=%v", accepted, status, err)
	}
}

func TestPostgresConfiguredDataFlushFailureRollsBackPriorGeneration(t *testing.T) {
	workbench := openConfiguredDataFlushPostgresTestStore(t)
	seedConfiguredDataFlushRecords(t, workbench.db)
	service := NewConfiguredDataFlushService(&PostgresConfiguredDataFlushRepository{db: workbench.db})
	if _, err := workbench.db.Exec(`
		CREATE FUNCTION reject_lineage_flush() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'injected flush failure'; END $$;
		CREATE TRIGGER reject_lineage_flush BEFORE DELETE ON digital_twin_lineage
		FOR EACH STATEMENT EXECUTE FUNCTION reject_lineage_flush()
	`); err != nil {
		t.Fatalf("install failure injection: %v", err)
	}
	plan, err := service.Plan(context.Background())
	if err != nil {
		t.Fatalf("plan flush: %v", err)
	}
	if _, err := service.Apply(context.Background(), plan.PlanID); err == nil {
		t.Fatal("injected failure unexpectedly committed")
	}
	var generation uint64
	if err := workbench.db.QueryRow(`SELECT generation FROM workbench_snapshot_generation WHERE singleton = TRUE`).Scan(&generation); err != nil || generation != 0 {
		t.Fatalf("failed flush advanced generation=%d err=%v", generation, err)
	}
	for _, table := range []string{"simops_events", "scada_measured_frames", "digital_twin_lineage"} {
		var count int
		if err := workbench.db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("failed flush partially cleared %s count=%d err=%v", table, count, err)
		}
	}
}

func TestPostgresConfiguredDataFlushRejectsActiveResources(t *testing.T) {
	workbench := openConfiguredDataFlushPostgresTestStore(t)
	seedConfiguredDataFlushRecords(t, workbench.db)
	if _, err := workbench.db.Exec(`UPDATE simops_runs SET lifecycle = 'streaming'`); err != nil {
		t.Fatalf("activate run: %v", err)
	}
	if _, err := workbench.db.Exec(`UPDATE reactor_telemetry_worker_sets SET worker_set = jsonb_set(worker_set, '{lifecycle}', '"active"')`); err != nil {
		t.Fatalf("activate worker set: %v", err)
	}
	service := NewConfiguredDataFlushService(&PostgresConfiguredDataFlushRepository{db: workbench.db})
	plan, err := service.Plan(context.Background())
	if err != nil || plan.Ready || len(plan.Blockers) != 2 {
		t.Fatalf("active resources did not block plan: %#v err=%v", plan, err)
	}
	if _, err := service.Apply(context.Background(), plan.PlanID); !errors.Is(err, ErrConfiguredDataFlushBlocked) {
		t.Fatalf("active-resource plan was applied: %v", err)
	}
}

func TestPostgresConfiguredDataFlushFailsClosedForMalformedWorkerSetLifecycle(t *testing.T) {
	workbench := openConfiguredDataFlushPostgresTestStore(t)
	seedConfiguredDataFlushRecords(t, workbench.db)
	if _, err := workbench.db.Exec(`UPDATE reactor_telemetry_worker_sets SET worker_set = worker_set - 'lifecycle'`); err != nil {
		t.Fatalf("remove worker-set lifecycle: %v", err)
	}
	plan, err := NewConfiguredDataFlushService(&PostgresConfiguredDataFlushRepository{db: workbench.db}).Plan(context.Background())
	if err != nil || plan.Ready || len(plan.Blockers) != 1 {
		t.Fatalf("malformed worker set did not fail closed: %#v err=%v", plan, err)
	}
}

func openConfiguredDataFlushPostgresTestStore(t *testing.T) *PostgresWorkbenchStore {
	t.Helper()
	store := openPostgresSnapshotTestStore(t)
	if _, err := store.db.Exec(configuredDataFlushPostgresTestSchema); err != nil {
		t.Fatalf("create configured flush schema: %v", err)
	}
	return store
}

func seedConfiguredDataFlushRecords(t *testing.T, db *sql.DB) {
	t.Helper()
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	statements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO simops_runs VALUES ($1,$2,'complete','local-demo','fixture.sh','contract',60,$3,$4,$5,$6,$6)`, []any{"run-before-flush", "scenario-before", "before-flush", "flush-integration", "protected-ingest-token", now}},
		{`INSERT INTO simops_events (run_id,event_type,occurred_at) VALUES ('run-before-flush','run.lifecycle',$1)`, []any{now}},
		{`INSERT INTO simops_telemetry_frames (received_at,run_id,frame) VALUES ($1,'run-before-flush','{}')`, []any{now}},
		{`INSERT INTO scada_measured_frames (observed_at,sampled_at,source_id,tag_id,asset_id,signal_kind,sequence,unit,quality,value_basis,synthetic_status,value,frame,redpanda_topic,redpanda_partition,redpanda_offset) VALUES ($1,$1,'source-protected','tag-protected','asset','flux',1,'a.u.','good','measured','public-safe-standin','{}','{}','scada',0,1)`, []any{now}},
		{`INSERT INTO simops_result_values (produced_at,received_at,run_id,scenario_id,worker_id,worker_kind,sequence,result_type,model_id,input_window_start,input_window_end,value_basis,synthetic_status,result_id,entity_id,value_id,label,unit,value,confidence,frame,redpanda_topic,redpanda_partition,redpanda_offset) VALUES ($1,$1,'run-before-flush','scenario','worker','scheduler',1,'summary','model',$1,$1,'simulated','public-safe-standin','result','entity','value','label','a.u.','{}',1,'{}','results',0,1)`, []any{now}},
		{`INSERT INTO digital_twin_state_values (as_of,twin_id,entity_id,display_name,value_id,label,value_basis,unit,value,confidence,freshness,lineage_id,source_ids,state,redpanda_topic,redpanda_partition,redpanda_offset) VALUES ($1,'twin','entity','Entity','value','Value','imputed','a.u.','{}',1,'{}','lineage','[]','{}','twin',0,1)`, []any{now}},
		{`INSERT INTO digital_twin_lineage VALUES ('lineage','value','imputed','{}',$1)`, []any{now}},
		{`INSERT INTO workbench_twin_publications VALUES ('publication-acknowledged','{"acknowledged":true}',$1)`, []any{now}},
		{`INSERT INTO workbench_twin_publications VALUES ('publication-pending','{}',$1)`, []any{now}},
		{`INSERT INTO reactor_telemetry_worker_sets VALUES ('session','reactor','register','{"setId":"set-removed","lifecycle":"removed"}',$1)`, []any{now}},
		{`INSERT INTO workbench_resident_sources VALUES ('source-protected','{}',$1)`, []any{now}},
		{`INSERT INTO workbench_resident_tags VALUES ('tag-protected','source-protected','asset','flux','a.u.','measured','{}')`, nil},
		{`INSERT INTO workbench_processed_messages VALUES ('consumer','scada',0,1)`, nil},
		{`INSERT INTO workbench_consumer_offsets VALUES ('consumer','scada',0,1,$1)`, []any{now}},
		{`INSERT INTO simops_processed_messages VALUES ('consumer','simops',0,1,$1)`, []any{now}},
		{`INSERT INTO simops_consumer_offsets VALUES ('consumer','simops',0,1,$1)`, []any{now}},
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement.query, statement.args...); err != nil {
			t.Fatalf("seed configured flush record with %q: %v", statement.query, err)
		}
	}
}

const configuredDataFlushPostgresTestSchema = `
CREATE TABLE simops_runs (
  run_id TEXT PRIMARY KEY, scenario_id TEXT NOT NULL, lifecycle TEXT NOT NULL, source TEXT NOT NULL,
  work_script TEXT NOT NULL, launch_mode TEXT NOT NULL, runtime_limit_sec INTEGER NOT NULL,
  idempotency_key TEXT, submitted_by TEXT NOT NULL, ingest_token TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL,
  UNIQUE (submitted_by, idempotency_key)
);
CREATE TABLE simops_workers (
  run_id TEXT NOT NULL REFERENCES simops_runs(run_id) ON DELETE CASCADE, worker_id TEXT NOT NULL,
  worker_kind TEXT NOT NULL, lifecycle TEXT NOT NULL, launch_mode TEXT NOT NULL, endpoint TEXT,
  frames INTEGER NOT NULL DEFAULT 0, labels JSONB NOT NULL DEFAULT '{}', observed_lifecycle TEXT,
  observed_reason TEXT, observed_message TEXT, runtime TEXT, runtime_id TEXT,
  observed_exit_code INTEGER, observed_at TIMESTAMPTZ, updated_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (run_id, worker_id)
);
CREATE TABLE simops_spool_commands (
  command_id TEXT PRIMARY KEY, run_id TEXT NOT NULL REFERENCES simops_runs(run_id) ON DELETE CASCADE,
  worker_id TEXT NOT NULL, mode TEXT NOT NULL, state TEXT NOT NULL, message TEXT NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}', created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE simops_events (
  event_id BIGSERIAL PRIMARY KEY, run_id TEXT NOT NULL REFERENCES simops_runs(run_id) ON DELETE CASCADE,
  worker_id TEXT, event_type TEXT NOT NULL, lifecycle TEXT, frame JSONB, occurred_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE simops_artifacts (
  artifact_id TEXT PRIMARY KEY, run_id TEXT NOT NULL REFERENCES simops_runs(run_id) ON DELETE CASCADE,
  kind TEXT NOT NULL, media_type TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'received',
  location TEXT NOT NULL, iceberg_table TEXT, created_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE simops_telemetry_frames (
  received_at TIMESTAMPTZ NOT NULL, run_id TEXT NOT NULL REFERENCES simops_runs(run_id) ON DELETE CASCADE,
  frame JSONB NOT NULL
);
CREATE TABLE simops_processed_messages (
  consumer_name TEXT NOT NULL, redpanda_topic TEXT NOT NULL, redpanda_partition INTEGER NOT NULL,
  redpanda_offset BIGINT NOT NULL, processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (consumer_name, redpanda_topic, redpanda_partition, redpanda_offset)
);
CREATE TABLE simops_consumer_offsets (
  consumer_name TEXT NOT NULL, redpanda_topic TEXT NOT NULL, redpanda_partition INTEGER NOT NULL,
  redpanda_offset BIGINT NOT NULL, updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (consumer_name, redpanda_topic, redpanda_partition)
);
CREATE TABLE workbench_resident_sources (
  source_id TEXT PRIMARY KEY, declaration JSONB NOT NULL, updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE workbench_resident_tags (
  tag_id TEXT PRIMARY KEY, source_id TEXT NOT NULL REFERENCES workbench_resident_sources(source_id) ON DELETE CASCADE,
  asset_id TEXT NOT NULL, signal_kind TEXT NOT NULL, unit TEXT NOT NULL, value_basis TEXT NOT NULL, tag JSONB NOT NULL
);
CREATE TABLE reactor_telemetry_worker_sets (
  game_session_id TEXT NOT NULL, reactor_id TEXT NOT NULL, register_idempotency_key TEXT NOT NULL,
  worker_set JSONB NOT NULL, updated_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (game_session_id, reactor_id), UNIQUE (game_session_id, register_idempotency_key)
);`
