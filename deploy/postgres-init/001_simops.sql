CREATE EXTENSION IF NOT EXISTS timescaledb;

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
  observed_lifecycle TEXT,
  observed_reason TEXT,
  observed_message TEXT,
  runtime TEXT,
  runtime_id TEXT,
  observed_exit_code INTEGER,
  observed_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (run_id, worker_id)
);

ALTER TABLE simops_workers
  ADD COLUMN IF NOT EXISTS observed_lifecycle TEXT,
  ADD COLUMN IF NOT EXISTS observed_reason TEXT,
  ADD COLUMN IF NOT EXISTS observed_message TEXT,
  ADD COLUMN IF NOT EXISTS runtime TEXT,
  ADD COLUMN IF NOT EXISTS runtime_id TEXT,
  ADD COLUMN IF NOT EXISTS observed_exit_code INTEGER,
  ADD COLUMN IF NOT EXISTS observed_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS simops_spool_commands (
  command_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES simops_runs(run_id) ON DELETE CASCADE,
  worker_id TEXT NOT NULL,
  mode TEXT NOT NULL,
  state TEXT NOT NULL,
  message TEXT NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

ALTER TABLE simops_spool_commands
  ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}'::jsonb;

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

CREATE TABLE IF NOT EXISTS simops_telemetry_frames (
  received_at TIMESTAMPTZ NOT NULL,
  emitted_at TIMESTAMPTZ NOT NULL,
  run_id TEXT NOT NULL REFERENCES simops_runs(run_id) ON DELETE CASCADE,
  scenario_id TEXT NOT NULL,
  worker_id TEXT NOT NULL,
  worker_kind TEXT NOT NULL,
  sequence BIGINT NOT NULL,
  payload_type TEXT NOT NULL,
  quality TEXT,
  source_lag_ms DOUBLE PRECISION,
  collector_lag_ms DOUBLE PRECISION,
  dropped_frame_count BIGINT NOT NULL DEFAULT 0,
  frame JSONB NOT NULL,
  redpanda_topic TEXT NOT NULL,
  redpanda_partition INTEGER NOT NULL,
  redpanda_offset BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

SELECT create_hypertable('simops_telemetry_frames', 'received_at', if_not_exists => TRUE);

CREATE INDEX IF NOT EXISTS idx_simops_telemetry_frames_run_received
  ON simops_telemetry_frames (run_id, received_at DESC);

CREATE INDEX IF NOT EXISTS idx_simops_telemetry_frames_worker_received
  ON simops_telemetry_frames (run_id, worker_id, received_at DESC);

CREATE INDEX IF NOT EXISTS idx_simops_telemetry_frames_payload_quality
  ON simops_telemetry_frames (payload_type, quality);

CREATE TABLE IF NOT EXISTS simops_processed_messages (
  consumer_name TEXT NOT NULL,
  redpanda_topic TEXT NOT NULL,
  redpanda_partition INTEGER NOT NULL,
  redpanda_offset BIGINT NOT NULL,
  processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (consumer_name, redpanda_topic, redpanda_partition, redpanda_offset)
);

CREATE TABLE IF NOT EXISTS simops_consumer_offsets (
  consumer_name TEXT NOT NULL,
  redpanda_topic TEXT NOT NULL,
  redpanda_partition INTEGER NOT NULL,
  redpanda_offset BIGINT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (consumer_name, redpanda_topic, redpanda_partition)
);

CREATE TABLE IF NOT EXISTS workbench_resident_sources (
  source_id TEXT PRIMARY KEY,
  declaration JSONB NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS workbench_resident_tags (
  tag_id TEXT PRIMARY KEY,
  source_id TEXT NOT NULL REFERENCES workbench_resident_sources(source_id) ON DELETE CASCADE,
  asset_id TEXT NOT NULL,
  signal_kind TEXT NOT NULL,
  unit TEXT NOT NULL,
  value_basis TEXT NOT NULL CHECK (value_basis = 'measured'),
  tag JSONB NOT NULL
);

CREATE TABLE IF NOT EXISTS scada_measured_frames (
  observed_at TIMESTAMPTZ NOT NULL,
  sampled_at TIMESTAMPTZ NOT NULL,
  source_id TEXT NOT NULL,
  tag_id TEXT NOT NULL,
  asset_id TEXT NOT NULL,
  signal_kind TEXT NOT NULL,
  sequence BIGINT NOT NULL,
  unit TEXT NOT NULL,
  quality TEXT NOT NULL,
  value_basis TEXT NOT NULL CHECK (value_basis = 'measured'),
  synthetic_status TEXT NOT NULL,
  value JSONB NOT NULL,
  frame JSONB NOT NULL,
  redpanda_topic TEXT NOT NULL,
  redpanda_partition INTEGER NOT NULL,
  redpanda_offset BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

SELECT create_hypertable('scada_measured_frames', 'observed_at', if_not_exists => TRUE);

CREATE INDEX IF NOT EXISTS idx_scada_measured_redpanda_coord
  ON scada_measured_frames (redpanda_topic, redpanda_partition, redpanda_offset);

CREATE INDEX IF NOT EXISTS idx_scada_measured_frames_tag_observed
  ON scada_measured_frames (tag_id, observed_at DESC);

CREATE TABLE IF NOT EXISTS simops_result_values (
  produced_at TIMESTAMPTZ NOT NULL,
  received_at TIMESTAMPTZ NOT NULL,
  run_id TEXT NOT NULL REFERENCES simops_runs(run_id) ON DELETE CASCADE,
  scenario_id TEXT NOT NULL,
  worker_id TEXT NOT NULL,
  worker_kind TEXT NOT NULL,
  sequence BIGINT NOT NULL,
  result_type TEXT NOT NULL,
  model_id TEXT NOT NULL,
  input_window_start TIMESTAMPTZ NOT NULL,
  input_window_end TIMESTAMPTZ NOT NULL,
  value_basis TEXT NOT NULL CHECK (value_basis = 'simulated'),
  synthetic_status TEXT NOT NULL,
  result_id TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  value_id TEXT NOT NULL,
  label TEXT NOT NULL,
  unit TEXT NOT NULL,
  value JSONB NOT NULL,
  confidence DOUBLE PRECISION NOT NULL,
  frame JSONB NOT NULL,
  redpanda_topic TEXT NOT NULL,
  redpanda_partition INTEGER NOT NULL,
  redpanda_offset BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

SELECT create_hypertable('simops_result_values', 'produced_at', if_not_exists => TRUE);

CREATE INDEX IF NOT EXISTS idx_simops_result_redpanda_coord_value
  ON simops_result_values (redpanda_topic, redpanda_partition, redpanda_offset, value_id);

CREATE INDEX IF NOT EXISTS idx_simops_result_values_run_produced
  ON simops_result_values (run_id, produced_at DESC);

CREATE INDEX IF NOT EXISTS idx_simops_result_values_basis
  ON simops_result_values (value_basis);

CREATE TABLE IF NOT EXISTS digital_twin_state_values (
  as_of TIMESTAMPTZ NOT NULL,
  twin_id TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  display_name TEXT NOT NULL,
  value_id TEXT NOT NULL,
  label TEXT NOT NULL,
  value_basis TEXT NOT NULL CHECK (value_basis IN ('measured', 'imputed', 'simulated')),
  unit TEXT NOT NULL,
  value JSONB NOT NULL,
  confidence DOUBLE PRECISION NOT NULL,
  freshness JSONB NOT NULL,
  lineage_id TEXT NOT NULL,
  source_ids JSONB NOT NULL,
  state JSONB NOT NULL,
  redpanda_topic TEXT NOT NULL,
  redpanda_partition INTEGER NOT NULL,
  redpanda_offset BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (twin_id, entity_id, value_id)
);

CREATE INDEX IF NOT EXISTS idx_digital_twin_state_values_basis
  ON digital_twin_state_values (value_basis);

CREATE INDEX IF NOT EXISTS idx_digital_twin_state_values_as_of
  ON digital_twin_state_values (as_of DESC);

CREATE TABLE IF NOT EXISTS digital_twin_lineage (
  lineage_id TEXT PRIMARY KEY,
  value_id TEXT NOT NULL,
  value_basis TEXT NOT NULL,
  lineage JSONB NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_digital_twin_lineage_value
  ON digital_twin_lineage (value_id);

CREATE TABLE IF NOT EXISTS workbench_processed_messages (
  consumer_name TEXT NOT NULL,
  redpanda_topic TEXT NOT NULL,
  redpanda_partition INTEGER NOT NULL,
  redpanda_offset BIGINT NOT NULL,
  processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (consumer_name, redpanda_topic, redpanda_partition, redpanda_offset)
);

CREATE TABLE IF NOT EXISTS workbench_consumer_offsets (
  consumer_name TEXT NOT NULL,
  redpanda_topic TEXT NOT NULL,
  redpanda_partition INTEGER NOT NULL,
  redpanda_offset BIGINT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (consumer_name, redpanda_topic, redpanda_partition)
);

CREATE TABLE IF NOT EXISTS workbench_snapshot_generation (
  singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
  generation BIGINT NOT NULL CHECK (generation >= 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO workbench_snapshot_generation (singleton, generation)
VALUES (TRUE, 0)
ON CONFLICT (singleton) DO NOTHING;

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

CREATE TABLE IF NOT EXISTS iceberg_tables (
  catalog_name TEXT NOT NULL,
  table_namespace TEXT NOT NULL,
  table_name TEXT NOT NULL,
  iceberg_type TEXT NOT NULL,
  metadata_location TEXT,
  previous_metadata_location TEXT,
  PRIMARY KEY (catalog_name, table_namespace, table_name)
);

CREATE TABLE IF NOT EXISTS iceberg_namespace_properties (
  catalog_name TEXT NOT NULL,
  namespace TEXT NOT NULL,
  property_key TEXT NOT NULL,
  property_value TEXT,
  PRIMARY KEY (catalog_name, namespace, property_key)
);
