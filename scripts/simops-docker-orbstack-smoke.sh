#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Run a focused Docker/OrbStack SimOps runtime smoke check.

Usage:
  scripts/simops-docker-orbstack-smoke.sh [--timeout seconds] [--build auto|always|never]

This smoke proves runtime launch semantics through the normal SimOps API path:
worker container launch, gateway-only ingest credentials, observed Docker
lifecycle sync, successful-run cleanup, failed-run retention, and smoke-forced
cleanup. It intentionally does not prove the full lakehouse fanout path.

Set SIMOPS_SMOKE_BUILD=always to force a from-source image rebuild.
The smoke defaults SIMOPS_WORKER_CLEANUP_TTL to 0s so succeeded workers prove
the configured cleanup policy without waiting for the local-dev TTL.
USAGE
}

TIMEOUT="${SIMOPS_SMOKE_TIMEOUT:-120}"
BUILD_MODE="${SIMOPS_SMOKE_BUILD:-auto}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --timeout)
      TIMEOUT="${2:?--timeout requires a value}"
      shift 2
      ;;
    --build)
      BUILD_MODE="${2:?--build requires a value}"
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
case "$BUILD_MODE" in
  auto|always|never) ;;
  *)
    echo "--build must be one of: auto, always, never." >&2
    exit 2
    ;;
esac

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

export PATH="$HOME/.orbstack/bin:/opt/homebrew/bin:/usr/local/bin:/Applications/Docker.app/Contents/Resources/bin:$PATH"
export SIMOPS_WORKER_AUTO_REMOVE="${SIMOPS_WORKER_AUTO_REMOVE:-false}"
export SIMOPS_WORKER_FRAME_OVERRIDE="${SIMOPS_WORKER_FRAME_OVERRIDE:-2}"
export SIMOPS_WORKER_CLEANUP_TTL="${SIMOPS_WORKER_CLEANUP_TTL:-0s}"
export COMPOSE_PARALLEL_LIMIT="${COMPOSE_PARALLEL_LIMIT:-1}"

docker_cmd=(docker)
if [[ -z "${SIMOPS_DOCKER_CONTEXT:-}" ]] && command -v docker >/dev/null 2>&1 && docker context inspect orbstack >/dev/null 2>&1 && docker --context orbstack info >/dev/null 2>&1; then
  SIMOPS_DOCKER_CONTEXT="orbstack"
fi
if [[ -n "${SIMOPS_DOCKER_CONTEXT:-}" ]]; then
  export DOCKER_CONTEXT="$SIMOPS_DOCKER_CONTEXT"
  docker_cmd=(docker --context "$SIMOPS_DOCKER_CONTEXT")
fi
compose=("${docker_cmd[@]}" compose -f deploy/slurm-gateway.compose.yml)
created_runs=()

run() {
  printf '+ '
  printf '%q ' "$@"
  printf '\n'
  "$@"
}

require_command() {
  local command_name="$1"
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "${command_name} is required for scripts/simops-docker-orbstack-smoke.sh." >&2
    exit 127
  fi
}

cleanup_runs() {
  if [[ "${SIMOPS_SMOKE_FORCE_CLEANUP:-1}" != "1" ]]; then
    return
  fi
  for run_id in "${created_runs[@]:-}"; do
    curl -fsS -X POST "http://127.0.0.1:8081/api/simops/runs/${run_id}/stop" >/dev/null 2>&1 || true
  done
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

apply_postgres_schema() {
  run "${compose[@]}" exec -T postgres psql -v ON_ERROR_STOP=1 -U radiant -d radiant -f /docker-entrypoint-initdb.d/001_simops.sql >/dev/null
}

container_ids_for_run() {
  "${docker_cmd[@]}" ps -a \
    --filter "label=simops.run_id=$1" \
    --filter "label=simops.runtime_adapter=docker-sdk" \
    --filter "label=simops.role=ordinary-worker" \
    --format '{{.ID}}'
}

wait_for_container() {
  local run_id="$1"
  local container_id=""

  while [[ "$SECONDS" -lt "$deadline" ]]; do
    container_id="$(container_ids_for_run "$run_id" | head -n 1 || true)"
    if [[ -n "$container_id" ]]; then
      printf '%s' "$container_id"
      return 0
    fi
    sleep 1
  done

  echo "Timed out waiting for Docker worker container for ${run_id}." >&2
  return 1
}

wait_for_no_containers() {
  local run_id="$1"

  while [[ "$SECONDS" -lt "$deadline" ]]; do
    if [[ -z "$(container_ids_for_run "$run_id")" ]]; then
      echo "Docker cleanup proof for ${run_id}: no labeled worker containers remain."
      return 0
    fi
    sleep 1
  done

  echo "Timed out waiting for Docker cleanup for ${run_id}. Remaining containers:" >&2
  container_ids_for_run "$run_id" >&2 || true
  return 1
}

create_run() {
  local scenario_id="$1"
  local suffix="$2"
  local idempotency_key="simops-docker-orbstack-${suffix}-$(date +%s)"
  local payload
  local response
  local run_id

  payload=$(printf '{"scenario_id":"%s","worker_kinds":["scheduler"],"launch_mode":"auto","runtime_limit_sec":30,"idempotency_key":"%s"}' "$scenario_id" "$idempotency_key")
  if ! response="$(curl -fsS -X POST http://127.0.0.1:8081/api/simops/runs \
    -H 'Content-Type: application/json' \
    -H 'Accept: application/json' \
    --data "$payload")"; then
    echo "Failed to create Docker/OrbStack smoke run for scenario ${scenario_id}." >&2
    return 1
  fi
  run_id="$(node scripts/simops-smoke-json.mjs run-id <<<"$response")"
  if [[ -z "$run_id" ]]; then
    echo "Smoke run creation response did not include a run id: ${response}" >&2
    return 1
  fi
  echo "Created Docker/OrbStack smoke run ${run_id} for scenario ${scenario_id}." >&2
  printf '%s' "$run_id"
}

wait_for_runtime_state() {
  local run_id="$1"
  local state="$2"
  local require_frames="${3:-}"
  local status
  local node_args=(runtime-worker "$state")

  if [[ "$require_frames" == "--frames" ]]; then
    node_args+=("--frames")
  fi

  while [[ "$SECONDS" -lt "$deadline" ]]; do
    status="$(curl -fsS "http://127.0.0.1:8081/api/simops/runs/${run_id}")"
    if node scripts/simops-smoke-json.mjs "${node_args[@]}" <<<"$status"; then
      echo
      return 0
    fi
    sleep 2
  done

  echo "Timed out waiting for observed worker state ${state} on ${run_id}." >&2
  return 1
}

stop_run() {
  local run_id="$1"
  run curl -fsS -X POST "http://127.0.0.1:8081/api/simops/runs/${run_id}/stop" >/dev/null
}

assert_failed_container_logs() {
  local container_id="$1"
  local run_id="$2"
  local logs
  local first_line

  logs="$("${docker_cmd[@]}" logs "$container_id" 2>&1 || true)"
  if [[ -z "$logs" || "$logs" != *"simops-generator:"* ]]; then
    echo "Expected retained failed worker ${container_id} for ${run_id} to expose simops-generator logs." >&2
    echo "$logs" >&2
    return 1
  fi
  first_line="$(printf '%s\n' "$logs" | head -n 1)"
  echo "Failed-worker log proof for ${run_id}: ${first_line}"
}

image_exists() {
  "${docker_cmd[@]}" image inspect "$1" >/dev/null 2>&1
}

required_images_exist() {
  image_exists deploy-slurm-gateway:latest && image_exists radiant-simops-generator:latest
}

build_images() {
  run "${compose[@]}" build slurm-gateway
  run "${compose[@]}" --profile simops-buckets build simops-bucket-scheduler
}

ensure_images() {
  case "$BUILD_MODE" in
    always)
      build_images
      ;;
    never)
      if ! required_images_exist; then
        echo "Required smoke images are missing. Re-run with --build auto or --build always." >&2
        exit 1
      fi
      echo "Using existing smoke images because build mode is never."
      ;;
    auto)
      if required_images_exist; then
        echo "Using existing smoke images. Set SIMOPS_SMOKE_BUILD=always to rebuild from source."
      else
        build_images
      fi
      ;;
  esac
}

require_command docker
require_command curl
require_command node

trap cleanup_runs EXIT

echo "Docker context: $("${docker_cmd[@]}" context show)"
"${docker_cmd[@]}" version --format 'Docker version: {{.Server.Version}}'
"${compose[@]}" version
if [[ "${SIMOPS_DOCKER_CONTEXT:-}" != "orbstack" ]]; then
  run scripts/docker-up.sh --timeout "$TIMEOUT"
fi

deadline=$((SECONDS + TIMEOUT))
ensure_images
run "${compose[@]}" up -d postgres redpanda minio
wait_for_ready "Postgres" postgres_ready
apply_postgres_schema
wait_for_ready "Redpanda" redpanda_ready
wait_for_ready "MinIO" minio_ready
run "${compose[@]}" run --rm --no-deps minio-init
run "${compose[@]}" up -d slurm-gateway
wait_for_ready "slurm-gateway health" slurm_gateway_ready

success_run="$(create_run scheduler-drift success)"
created_runs+=("$success_run")
success_container="$(wait_for_container "$success_run")"
"${docker_cmd[@]}" inspect "$success_container" | node scripts/simops-smoke-json.mjs container-proof
echo
wait_for_runtime_state "$success_run" succeeded --frames
events="$(curl -fsS "http://127.0.0.1:8081/api/simops/runs/${success_run}/events")"
node scripts/simops-smoke-json.mjs events-nonempty <<<"$events"
echo "Gateway ingest proof for ${success_run}: worker frames and run events arrived through the gateway."
echo "Successful-run cleanup policy proof for ${success_run}: waiting for zero-TTL cleanup."
wait_for_no_containers "$success_run"

echo "Failure-path proof uses checkpoint-pressure to exercise an inspectable worker-process failure."
failed_run="$(create_run checkpoint-pressure failure)"
created_runs+=("$failed_run")
failed_container="$(wait_for_container "$failed_run")"
wait_for_runtime_state "$failed_run" failed
if [[ -z "$(container_ids_for_run "$failed_run")" ]]; then
  echo "Expected failed run ${failed_run} to retain its worker container before forced cleanup." >&2
  exit 1
fi
assert_failed_container_logs "$failed_container" "$failed_run"
echo "Failed-run retention proof for ${failed_run}: retained worker container ${failed_container} before forced cleanup."
stop_run "$failed_run"
wait_for_no_containers "$failed_run"

echo "Docker/OrbStack SimOps runtime smoke passed: launch, gateway ingest, lifecycle sync, retention, and cleanup verified."
