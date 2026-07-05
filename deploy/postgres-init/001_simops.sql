CREATE TABLE IF NOT EXISTS simops_runs (
  run_id TEXT PRIMARY KEY,
  scenario_id TEXT NOT NULL,
  lifecycle TEXT NOT NULL,
  source TEXT NOT NULL,
  work_script TEXT NOT NULL,
  launch_mode TEXT NOT NULL,
  runtime_limit_sec INTEGER NOT NULL,
  idempotency_key TEXT,
  submitted_by TEXT NOT NULL,
  ingest_token TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  UNIQUE (submitted_by, idempotency_key)
);

CREATE TABLE IF NOT EXISTS simops_workers (
  run_id TEXT NOT NULL REFERENCES simops_runs(run_id) ON DELETE CASCADE,
  worker_id TEXT NOT NULL,
  worker_kind TEXT NOT NULL,
  lifecycle TEXT NOT NULL,
  launch_mode TEXT NOT NULL,
  endpoint TEXT,
  frames INTEGER NOT NULL DEFAULT 0,
  labels JSONB NOT NULL DEFAULT '{}'::jsonb,
  updated_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (run_id, worker_id)
);

CREATE TABLE IF NOT EXISTS simops_spool_commands (
  command_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES simops_runs(run_id) ON DELETE CASCADE,
  worker_id TEXT NOT NULL,
  mode TEXT NOT NULL,
  state TEXT NOT NULL,
  message TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS simops_events (
  event_id BIGSERIAL PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES simops_runs(run_id) ON DELETE CASCADE,
  worker_id TEXT,
  event_type TEXT NOT NULL,
  lifecycle TEXT,
  frame JSONB,
  occurred_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS simops_artifacts (
  artifact_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES simops_runs(run_id) ON DELETE CASCADE,
  kind TEXT NOT NULL,
  media_type TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'received',
  location TEXT NOT NULL,
  iceberg_table TEXT,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE SCHEMA IF NOT EXISTS iceberg_catalog;

CREATE TABLE IF NOT EXISTS iceberg_catalog.catalog_metadata (
  catalog_name TEXT NOT NULL,
  table_namespace TEXT NOT NULL,
  table_name TEXT NOT NULL,
  metadata_location TEXT NOT NULL,
  previous_metadata_location TEXT,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (catalog_name, table_namespace, table_name)
);
