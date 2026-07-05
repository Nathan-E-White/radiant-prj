#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Run a local SimOps end-to-end smoke check with Docker Compose.

Usage:
  scripts/simops-local-smoke.sh [--timeout seconds]

This starts the local SimOps platform, builds the worker image, creates one
API-driven run, and waits for worker frames plus a committed manifest artifact.
USAGE
}

TIMEOUT="${SIMOPS_SMOKE_TIMEOUT:-120}"

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

compose=(docker compose -f deploy/slurm-gateway.compose.yml)

run() {
  printf '+ '
  printf '%q ' "$@"
  printf '\n'
  "$@"
}

run scripts/docker-up.sh --timeout "$TIMEOUT"
run "${compose[@]}" --profile simops-buckets build slurm-gateway simops-bucket-scheduler
run "${compose[@]}" up -d postgres redpanda minio minio-init simops-stream-gateway simops-iceberg-writer slurm-gateway

deadline=$((SECONDS + TIMEOUT))
until curl -fsS http://127.0.0.1:8081/healthz >/dev/null; do
  if [[ "$SECONDS" -ge "$deadline" ]]; then
    echo "Timed out waiting for slurm-gateway health." >&2
    exit 1
  fi
  sleep 2
done

run_id=""
idempotency_key="simops-smoke-$(date +%s)"
payload=$(printf '{"scenario_id":"scheduler-drift","worker_kinds":["scheduler"],"launch_mode":"auto","runtime_limit_sec":30,"idempotency_key":"%s"}' "$idempotency_key")

while [[ "$SECONDS" -lt "$deadline" ]]; do
  response="$(curl -fsS -X POST http://127.0.0.1:8081/api/simops/runs \
    -H 'Content-Type: application/json' \
    -H 'Accept: application/json' \
    --data "$payload" || true)"
  run_id="$(node -e 'const fs=require("fs"); const raw=fs.readFileSync(0,"utf8"); try { const parsed=JSON.parse(raw); process.stdout.write(parsed.run_id || ""); } catch {}' <<<"$response")"
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

while [[ "$SECONDS" -lt "$deadline" ]]; do
  status="$(curl -fsS "http://127.0.0.1:8081/api/simops/runs/${run_id}")"
  if node -e '
const parsed = JSON.parse(process.argv[1]);
const frames = parsed.workers.reduce((sum, worker) => sum + worker.frames, 0);
const committed = parsed.artifacts.some((artifact) => artifact.status === "committed");
process.exit(frames > 0 && committed ? 0 : 1);
' "$status"; then
    events="$(curl -fsS "http://127.0.0.1:8081/api/simops/runs/${run_id}/events")"
    node -e '
const parsed = JSON.parse(process.argv[1]);
if (!Array.isArray(parsed.events) || parsed.events.length === 0) process.exit(1);
' "$events"
    echo "SimOps local smoke passed for ${run_id}."
    exit 0
  fi
  sleep 2
done

echo "Timed out waiting for SimOps smoke run frames and committed artifact." >&2
exit 1
