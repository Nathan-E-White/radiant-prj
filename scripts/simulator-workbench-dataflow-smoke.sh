#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Run the backend-only Simulator Workbench dataflow smoke check.

Usage:
  scripts/simulator-workbench-dataflow-smoke.sh [--timeout seconds]

The smoke proves one resident measured SCADA unit, one SimOps telemetry unit,
one simulated result unit, and one imputed twin value reached Redpanda,
Postgres projection tables, Iceberg tables, and the read-only Workbench APIs.
USAGE
}

TIMEOUT="${WORKBENCH_DATAFLOW_SMOKE_TIMEOUT:-180}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --timeout)
      TIMEOUT="${2:?--timeout requires a value}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if ! [[ "$TIMEOUT" =~ ^[0-9]+$ ]] || [[ "$TIMEOUT" -le 0 ]]; then
  echo "--timeout must be a positive integer number of seconds." >&2
  exit 2
fi

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

export PATH="$HOME/.orbstack/bin:/opt/homebrew/bin:/usr/local/bin:/Applications/Docker.app/Contents/Resources/bin:$PATH"
compose=(docker compose -f deploy/slurm-gateway.compose.yml)

export SIMOPS_GO_BUILDER_IMAGE="${SIMOPS_GO_BUILDER_IMAGE:-golang:1.26-alpine}"
export SIMOPS_GATEWAY_RUNTIME_IMAGE="${SIMOPS_GATEWAY_RUNTIME_IMAGE:-alpine:3.21}"
export SIMOPS_RUST_BUILDER_IMAGE="${SIMOPS_RUST_BUILDER_IMAGE:-rust:1-alpine}"
export SIMOPS_GENERATOR_RUNTIME_IMAGE="${SIMOPS_GENERATOR_RUNTIME_IMAGE:-gcr.io/distroless/static-debian13:nonroot}"
export SCADA_RUST_BUILDER_IMAGE="${SCADA_RUST_BUILDER_IMAGE:-rust:1-alpine}"
export SCADA_STANDINS_RUNTIME_IMAGE="${SCADA_STANDINS_RUNTIME_IMAGE:-gcr.io/distroless/static-debian13:nonroot}"
export SIMOPS_REDPANDA_IMAGE="${SIMOPS_REDPANDA_IMAGE:-docker.redpanda.com/redpandadata/redpanda:latest}"
export SIMOPS_TIMESCALE_IMAGE="${SIMOPS_TIMESCALE_IMAGE:-timescale/timescaledb:latest-pg17}"
export SIMOPS_MINIO_IMAGE="${SIMOPS_MINIO_IMAGE:-quay.io/minio/minio:latest}"
export SIMOPS_MINIO_MC_IMAGE="${SIMOPS_MINIO_MC_IMAGE:-quay.io/minio/mc:latest}"

run() {
  printf '+ '
  printf '%q ' "$@"
  printf '\n'
  "$@"
}

require_command() {
  local command_name="$1"
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "${command_name} is required for scripts/simulator-workbench-dataflow-smoke.sh." >&2
    exit 127
  fi
}

wait_for_ready() {
  local label="$1"
  local predicate="$2"
  until "$predicate"; do
    if [[ "$SECONDS" -ge "$deadline" ]]; then
      echo "Timed out waiting for ${label}." >&2
      return 1
    fi
    sleep 2
  done
  echo "${label} is ready."
}

postgres_ready() {
  "${compose[@]}" exec -T postgres pg_isready -U radiant -d radiant >/dev/null 2>&1
}

redpanda_ready() {
  "${compose[@]}" exec -T redpanda rpk cluster health >/dev/null 2>&1
}

minio_ready() {
  "${compose[@]}" run --rm --no-deps minio-init "mc alias set local http://minio:9000 radiant radiant-password >/dev/null && mc ready local" >/dev/null 2>&1
}

slurm_gateway_ready() {
  curl -fsS http://127.0.0.1:8081/healthz >/dev/null 2>&1
}

workbench_projection_writer_ready() {
  curl -fsS http://127.0.0.1:9470/healthz >/dev/null 2>&1
}

twin_projector_ready() {
  curl -fsS http://127.0.0.1:9480/healthz >/dev/null 2>&1
}

workbench_iceberg_writer_ready() {
  curl -fsS http://127.0.0.1:9490/healthz >/dev/null 2>&1
}

ensure_topic() {
  local topic="$1"
  "${compose[@]}" exec -T redpanda rpk topic create "$topic" --partitions 1 --replicas 1 >/dev/null 2>&1 || true
}

topic_ready() {
  local topic="$1"
  "${compose[@]}" exec -T redpanda rpk topic describe "$topic" >/dev/null 2>&1
}

psql_scalar() {
  "${compose[@]}" exec -T postgres psql -U radiant -d radiant -tAc "$1" | tr -d '[:space:]'
}

close_stale_active_runs() {
  "${compose[@]}" exec -T postgres psql -U radiant -d radiant -c "UPDATE simops_runs SET lifecycle = 'stopped', updated_at = now() WHERE lifecycle IN ('created','starting','streaming','degraded'); UPDATE simops_workers SET lifecycle = 'stopped', updated_at = now() WHERE lifecycle IN ('created','starting','streaming','degraded'); UPDATE simops_spool_commands SET state = 'stopped', updated_at = now() WHERE state IN ('created','starting','streaming','degraded');" >/dev/null
}

minio_parquet_count() {
  "${compose[@]}" run --rm --no-deps minio-init "mc alias set local http://minio:9000 radiant radiant-password >/dev/null && mc find local/radiant-simops/warehouse --name '*.parquet' | wc -l" | tr -d '[:space:]'
}

require_command docker
require_command curl
require_command node

run scripts/docker-up.sh --timeout "$TIMEOUT"
run "${compose[@]}" --profile simops-buckets build slurm-gateway simops-bucket-burst simops-timescale-writer simops-iceberg-writer workbench-projection-writer twin-projector workbench-iceberg-writer scada-standins
deadline=$((SECONDS + TIMEOUT))
run "${compose[@]}" up -d postgres redpanda minio
wait_for_ready "Postgres" postgres_ready
run "${compose[@]}" exec -T postgres psql -U radiant -d radiant -f /docker-entrypoint-initdb.d/001_simops.sql
close_stale_active_runs
wait_for_ready "Redpanda" redpanda_ready
for topic in scada.telemetry.v1 simops.telemetry.v1 simops.results.v1 digital-twin.state.v1; do
  ensure_topic "$topic"
done
wait_for_ready "MinIO" minio_ready
run "${compose[@]}" run --rm --no-deps minio-init
run "${compose[@]}" up -d simops-timescale-writer simops-iceberg-writer workbench-projection-writer twin-projector workbench-iceberg-writer slurm-gateway
wait_for_ready "slurm-gateway health" slurm_gateway_ready
wait_for_ready "workbench-projection-writer health" workbench_projection_writer_ready
wait_for_ready "twin-projector health" twin_projector_ready
wait_for_ready "workbench-iceberg-writer health" workbench_iceberg_writer_ready

run "${compose[@]}" run --rm --no-deps scada-standins --source-id SRC-MIXED-STANDIN-001 --ingest-base-url http://slurm-gateway:8080 --ingest-token workbench-local-token --interval-ms 100 --max-frames 1

idempotency_key="workbench-dataflow-smoke-$(date +%s)"
payload=$(printf '{"scenario_id":"scheduler-drift","worker_kinds":["burst"],"launch_mode":"auto","runtime_limit_sec":30,"idempotency_key":"%s"}' "$idempotency_key")
run_id=""

while [[ "$SECONDS" -lt "$deadline" ]]; do
  response="$(curl -fsS -X POST http://127.0.0.1:8081/api/simops/runs \
    -H 'Content-Type: application/json' \
    -H 'Accept: application/json' \
    --data "$payload" || true)"
  run_id="$(node scripts/simops-smoke-json.mjs run-id <<<"$response" || true)"
  if [[ -n "$run_id" ]]; then
    break
  fi
  sleep 2
done

if [[ -z "$run_id" ]]; then
  echo "Could not create Workbench dataflow smoke run." >&2
  exit 1
fi

echo "Created Workbench dataflow smoke run ${run_id}."

while [[ "$SECONDS" -lt "$deadline" ]]; do
  scada_rows="$(psql_scalar "SELECT COUNT(*) FROM scada_measured_frames WHERE redpanda_topic = 'scada.telemetry.v1' AND value_basis = 'measured';" || true)"
  telemetry_rows="$(psql_scalar "SELECT COUNT(*) FROM simops_telemetry_frames WHERE run_id = '${run_id}' AND redpanda_topic = 'simops.telemetry.v1';" || true)"
  result_rows="$(psql_scalar "SELECT COUNT(*) FROM simops_result_values WHERE run_id = '${run_id}' AND redpanda_topic = 'simops.results.v1' AND value_basis = 'simulated';" || true)"
  twin_rows="$(psql_scalar "SELECT COUNT(*) FROM digital_twin_state_values WHERE redpanda_topic = 'digital-twin.state.v1' AND value_basis = 'imputed' AND source_ids::text LIKE '%${run_id}%';" || true)"
  lineage_rows="$(psql_scalar "SELECT COUNT(*) FROM digital_twin_lineage WHERE value_id = 'VAL-IMPUTED-CORE-MARGIN' AND value_basis = 'imputed' AND lineage::text LIKE '%${run_id}%';" || true)"
  iceberg_tables="$(psql_scalar "SELECT COUNT(*) FROM iceberg_tables WHERE metadata_location IS NOT NULL AND ((table_namespace = 'simops' AND table_name = 'telemetry_frames') OR (table_namespace = 'scada' AND table_name = 'measured_frames') OR (table_namespace = 'simops' AND table_name = 'simulated_results') OR (table_namespace = 'digital_twin' AND table_name = 'state_values'));" || true)"
  parquet_files="$(minio_parquet_count || true)"

  if topic_ready scada.telemetry.v1 &&
    topic_ready simops.telemetry.v1 &&
    topic_ready simops.results.v1 &&
    topic_ready digital-twin.state.v1 &&
    [[ "${scada_rows:-0}" -ge 1 ]] &&
    [[ "${telemetry_rows:-0}" -ge 1 ]] &&
    [[ "${result_rows:-0}" -ge 1 ]] &&
    [[ "${twin_rows:-0}" -ge 1 ]] &&
    [[ "${lineage_rows:-0}" -ge 1 ]] &&
    [[ "${iceberg_tables:-0}" -ge 4 ]] &&
    [[ "${parquet_files:-0}" -ge 4 ]]; then
    curl -fsS http://127.0.0.1:8081/api/simulator-workbench/state | node scripts/workbench-dataflow-json.mjs state-ready
    curl -fsS http://127.0.0.1:8081/api/simulator-workbench/measured | node scripts/workbench-dataflow-json.mjs frames-ready
    curl -fsS http://127.0.0.1:8081/api/simulator-workbench/twin | node scripts/workbench-dataflow-json.mjs twin-ready
    curl -fsS http://127.0.0.1:8081/api/simulator-workbench/lineage/VAL-IMPUTED-CORE-MARGIN | node scripts/workbench-dataflow-json.mjs lineage-ready
    echo "Simulator Workbench dataflow smoke passed for ${run_id}: SCADA=${scada_rows}, telemetry=${telemetry_rows}, results=${result_rows}, imputed=${twin_rows}, Iceberg tables=${iceberg_tables}, Parquet files=${parquet_files}."
    exit 0
  fi
  sleep 2
done

echo "Timed out waiting for full Simulator Workbench backend dataflow proof." >&2
echo "Last observed counts: SCADA=${scada_rows:-0}, telemetry=${telemetry_rows:-0}, results=${result_rows:-0}, imputed=${twin_rows:-0}, lineage=${lineage_rows:-0}, Iceberg tables=${iceberg_tables:-0}, Parquet files=${parquet_files:-0}." >&2
exit 1
