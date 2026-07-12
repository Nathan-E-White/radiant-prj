#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Run a local SimOps end-to-end smoke check with Docker Compose.

Usage:
  scripts/simops-local-smoke.sh [--timeout seconds]

This starts the local SimOps platform, builds the worker image, creates one
API-driven run, and waits for Redpanda-backed Timescale, MoQ, and Iceberg
telemetry fanout. REST event polling is checked only as recovery evidence.

Image metadata/content preflight:
  SIMOPS_SMOKE_IMAGE_PREFLIGHT_TIMEOUT=15  Timeout per image metadata lookup.
  SIMOPS_SMOKE_IMAGE_PULL_TIMEOUT=120      Timeout per image content pull.
  SIMOPS_SMOKE_IMAGE_PULL_RETRIES=2        Pull attempts before failing.
  SIMOPS_SMOKE_IMAGE_CACHE_ONLY=1          Require required images in local cache.
  SIMOPS_*_IMAGE=registry/name@sha256:...  Override bases/services with pinned
                                           digests, local mirrors, or cache tags.
USAGE
}

TIMEOUT="${SIMOPS_SMOKE_TIMEOUT:-120}"
IMAGE_PREFLIGHT_TIMEOUT="${SIMOPS_SMOKE_IMAGE_PREFLIGHT_TIMEOUT:-15}"
IMAGE_PULL_TIMEOUT="${SIMOPS_SMOKE_IMAGE_PULL_TIMEOUT:-120}"
IMAGE_PULL_RETRIES="${SIMOPS_SMOKE_IMAGE_PULL_RETRIES:-2}"
IMAGE_CACHE_ONLY="${SIMOPS_SMOKE_IMAGE_CACHE_ONLY:-0}"

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
if ! [[ "$IMAGE_PREFLIGHT_TIMEOUT" =~ ^[0-9]+$ ]] || [[ "$IMAGE_PREFLIGHT_TIMEOUT" -le 0 ]]; then
  echo "SIMOPS_SMOKE_IMAGE_PREFLIGHT_TIMEOUT must be a positive integer number of seconds." >&2
  exit 2
fi
if ! [[ "$IMAGE_PULL_TIMEOUT" =~ ^[0-9]+$ ]] || [[ "$IMAGE_PULL_TIMEOUT" -le 0 ]]; then
  echo "SIMOPS_SMOKE_IMAGE_PULL_TIMEOUT must be a positive integer number of seconds." >&2
  exit 2
fi
if ! [[ "$IMAGE_PULL_RETRIES" =~ ^[0-9]+$ ]] || [[ "$IMAGE_PULL_RETRIES" -le 0 ]]; then
  echo "SIMOPS_SMOKE_IMAGE_PULL_RETRIES must be a positive integer." >&2
  exit 2
fi

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

export PATH="$HOME/.orbstack/bin:/opt/homebrew/bin:/usr/local/bin:/Applications/Docker.app/Contents/Resources/bin:$PATH"
export SIMOPS_WORKER_AUTO_REMOVE="${SIMOPS_WORKER_AUTO_REMOVE:-true}"
export SIMOPS_WORKER_FRAME_OVERRIDE="${SIMOPS_WORKER_FRAME_OVERRIDE:-2}"
compose=(docker compose -f deploy/slurm-gateway.compose.yml)

export SIMOPS_GO_BUILDER_IMAGE="${SIMOPS_GO_BUILDER_IMAGE:-golang:1.26-alpine}"
export SIMOPS_GATEWAY_RUNTIME_IMAGE="${SIMOPS_GATEWAY_RUNTIME_IMAGE:-alpine:3.21}"
export SIMOPS_RUST_BUILDER_IMAGE="${SIMOPS_RUST_BUILDER_IMAGE:-rust:1-alpine}"
export SIMOPS_GENERATOR_RUNTIME_IMAGE="${SIMOPS_GENERATOR_RUNTIME_IMAGE:-gcr.io/distroless/static-debian13:nonroot}"
export SIMOPS_REDPANDA_IMAGE="${SIMOPS_REDPANDA_IMAGE:-docker.redpanda.com/redpandadata/redpanda:latest}"
export SIMOPS_TIMESCALE_IMAGE="${SIMOPS_TIMESCALE_IMAGE:-timescale/timescaledb:latest-pg17}"
export SIMOPS_MINIO_IMAGE="${SIMOPS_MINIO_IMAGE:-quay.io/minio/minio:latest}"
export SIMOPS_MINIO_MC_IMAGE="${SIMOPS_MINIO_MC_IMAGE:-quay.io/minio/mc:latest}"
export SIMOPS_PROMETHEUS_IMAGE="${SIMOPS_PROMETHEUS_IMAGE:-prom/prometheus:v2.55.1}"

run() {
  printf '+ '
  printf '%q ' "$@"
  printf '\n'
  "$@"
}

require_command() {
  local command_name="$1"
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "${command_name} is required for scripts/simops-local-smoke.sh." >&2
    exit 127
  fi
}

run_with_timeout() {
  local timeout_seconds="$1"
  shift
  local start="$SECONDS"
  "$@" &
  local pid=$!
  while kill -0 "$pid" 2>/dev/null; do
    if (( SECONDS - start >= timeout_seconds )); then
      kill "$pid" 2>/dev/null || true
      wait "$pid" 2>/dev/null || true
      return 124
    fi
    sleep 1
  done
  wait "$pid"
}

image_cached() {
  docker image inspect "$1" >/dev/null 2>&1
}

check_image_metadata() {
  local image="$1"
  if image_cached "$image"; then
    echo "Image cache ok: ${image}"
    return 0
  fi
  if [[ "$IMAGE_CACHE_ONLY" == "1" || "$IMAGE_CACHE_ONLY" == "true" ]]; then
    echo "Docker image cache miss before SimOps smoke build: ${image}" >&2
    echo "Preload the image with docker pull/load, or override it with a pinned digest/local mirror via the SIMOPS_*_IMAGE variables shown by --help." >&2
    return 1
  fi
  if run_with_timeout "$IMAGE_PREFLIGHT_TIMEOUT" docker manifest inspect "$image" >/dev/null 2>&1; then
    echo "Registry metadata ok: ${image}"
    return 0
  fi
  local status=$?
  echo "Docker base-image metadata preflight failed before SimOps services started: ${image}" >&2
  if [[ "$status" -eq 124 ]]; then
    echo "Registry metadata lookup timed out after ${IMAGE_PREFLIGHT_TIMEOUT}s; this is a Docker registry/base-image problem, not a SimOps service failure." >&2
  else
    echo "Registry metadata lookup failed; this is a Docker registry/base-image problem, not a SimOps service failure." >&2
  fi
  cat >&2 <<'EOF'
Remediation options:
- Set a pinned digest or local mirror with SIMOPS_GO_BUILDER_IMAGE, SIMOPS_GATEWAY_RUNTIME_IMAGE, SIMOPS_RUST_BUILDER_IMAGE, SIMOPS_GENERATOR_RUNTIME_IMAGE, SIMOPS_REDPANDA_IMAGE, SIMOPS_TIMESCALE_IMAGE, SIMOPS_MINIO_IMAGE, or SIMOPS_MINIO_MC_IMAGE.
- Preload the image cache with docker pull/docker load and rerun with SIMOPS_SMOKE_IMAGE_CACHE_ONLY=1.
EOF
  return 1
}

pull_image_content() {
  local image="$1"
  if image_cached "$image"; then
    echo "Image content already cached: ${image}"
    return 0
  fi
  if [[ "$IMAGE_CACHE_ONLY" == "1" || "$IMAGE_CACHE_ONLY" == "true" ]]; then
    echo "Docker image cache miss before SimOps smoke service startup: ${image}" >&2
    echo "Preload the image with docker pull/load, or override it with a pinned digest/local mirror via the SIMOPS_*_IMAGE variables shown by --help." >&2
    return 1
  fi

  local attempt
  for ((attempt = 1; attempt <= IMAGE_PULL_RETRIES; attempt++)); do
    echo "Pulling Docker image content (${attempt}/${IMAGE_PULL_RETRIES}): ${image}"
    if run_with_timeout "$IMAGE_PULL_TIMEOUT" docker pull "$image"; then
      echo "Image content ok: ${image}"
      return 0
    fi
  done

  echo "Docker image content pull failed before SimOps services started: ${image}" >&2
  echo "This is a Docker registry/base-image problem, not a SimOps service failure." >&2
  cat >&2 <<'EOF'
Remediation options:
- Set a pinned digest or local mirror with SIMOPS_GO_BUILDER_IMAGE, SIMOPS_GATEWAY_RUNTIME_IMAGE, SIMOPS_RUST_BUILDER_IMAGE, SIMOPS_GENERATOR_RUNTIME_IMAGE, SIMOPS_REDPANDA_IMAGE, SIMOPS_TIMESCALE_IMAGE, SIMOPS_MINIO_IMAGE, or SIMOPS_MINIO_MC_IMAGE.
- Preload the image cache with docker pull/docker load and rerun with SIMOPS_SMOKE_IMAGE_CACHE_ONLY=1.
EOF
  return 1
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

preflight_images=(
  "$SIMOPS_GO_BUILDER_IMAGE"
  "$SIMOPS_GATEWAY_RUNTIME_IMAGE"
  "$SIMOPS_RUST_BUILDER_IMAGE"
  "$SIMOPS_GENERATOR_RUNTIME_IMAGE"
  "$SIMOPS_REDPANDA_IMAGE"
  "$SIMOPS_TIMESCALE_IMAGE"
  "$SIMOPS_MINIO_IMAGE"
  "$SIMOPS_MINIO_MC_IMAGE"
)

require_command docker
require_command curl
require_command node

run scripts/docker-up.sh --timeout "$TIMEOUT"
run scripts/create-local-gateway-certs.sh
echo "Checking Docker base-image metadata before SimOps services start..."
for image in "${preflight_images[@]}"; do
  check_image_metadata "$image"
done
echo "Checking Docker image content before SimOps services start..."
for image in "${preflight_images[@]}"; do
  pull_image_content "$image"
done
run "${compose[@]}" --profile simops-buckets build slurm-gateway simops-bucket-scheduler simops-timescale-writer simops-moq-gateway simops-iceberg-writer
deadline=$((SECONDS + TIMEOUT))
run "${compose[@]}" up -d postgres redpanda minio
wait_for_ready "Postgres" postgres_ready
wait_for_ready "Redpanda" redpanda_ready
wait_for_ready "MinIO" minio_ready
run "${compose[@]}" run --rm --no-deps minio-init
run "${compose[@]}" up -d simops-moq-gateway simops-timescale-writer simops-iceberg-writer slurm-gateway
wait_for_ready "slurm-gateway health" slurm_gateway_ready

run_id=""
idempotency_key="simops-smoke-$(date +%s)"
payload=$(printf '{"scenario_id":"scheduler-drift","worker_kinds":["scheduler"],"launch_mode":"auto","runtime_limit_sec":30,"idempotency_key":"%s"}' "$idempotency_key")

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
  echo "Could not create SimOps smoke run." >&2
  exit 1
fi

echo "Created smoke run ${run_id}."

psql_scalar() {
  "${compose[@]}" exec -T postgres psql -U radiant -d radiant -tAc "$1" | tr -d '[:space:]'
}

minio_parquet_count() {
  "${compose[@]}" run --rm --no-deps minio-init "mc alias set local http://minio:9000 radiant radiant-password >/dev/null && mc find local/radiant-simops/warehouse --name '*.parquet' | wc -l" | tr -d '[:space:]'
}

while [[ "$SECONDS" -lt "$deadline" ]]; do
  status="$(curl -fsS "http://127.0.0.1:8081/api/simops/runs/${run_id}")"
  if node scripts/simops-smoke-json.mjs status-ready <<<"$status"; then
    timescale_rows="$(psql_scalar "SELECT COUNT(*) FROM simops_telemetry_frames WHERE run_id = '${run_id}';" || true)"
    iceberg_tables="$(psql_scalar "SELECT COUNT(*) FROM iceberg_tables WHERE table_namespace = 'simops' AND table_name = 'telemetry_frames' AND metadata_location IS NOT NULL;" || true)"
    parquet_files="$(minio_parquet_count || true)"
    if ! "${compose[@]}" run --rm --no-deps --entrypoint /app/simops-webtransport-probe simops-moq-gateway --endpoint https://simops-moq-gateway:9443/moq/simops --run-id "$run_id" --timeout 10s --ca-cert /run/secrets/simops_moq_gateway_ca_crt --server-name simops-moq-gateway; then
      sleep 2
      continue
    fi
    if [[ "${timescale_rows:-0}" -lt 1 || "${iceberg_tables:-0}" -lt 1 || "${parquet_files:-0}" -lt 1 ]]; then
      sleep 2
      continue
    fi
    events="$(curl -fsS "http://127.0.0.1:8081/api/simops/runs/${run_id}/events")"
    node scripts/simops-smoke-json.mjs events-nonempty <<<"$events"
    echo "SimOps local smoke passed for ${run_id}: Timescale rows=${timescale_rows}, Iceberg tables=${iceberg_tables}, Parquet files=${parquet_files}."
    exit 0
  fi
  sleep 2
done

echo "Timed out waiting for Redpanda-backed SimOps telemetry fanout." >&2
exit 1
