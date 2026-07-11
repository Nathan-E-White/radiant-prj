#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Run Docker Compose smoke checks for local development.

Usage:
  scripts/compose-smoke.sh [--timeout seconds]

Options:
  --timeout seconds  Seconds to wait for Docker to become ready. Default: 90.
  -h, --help         Show this help text.
USAGE
}

TIMEOUT="${DOCKER_UP_TIMEOUT:-90}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --timeout)
      if [[ $# -lt 2 ]]; then
        echo "--timeout requires a value." >&2
        usage >&2
        exit 2
      fi
      TIMEOUT="$2"
      shift
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
  shift
done

if ! [[ "$TIMEOUT" =~ ^[0-9]+$ ]] || [[ "$TIMEOUT" -le 0 ]]; then
  echo "--timeout must be a positive integer number of seconds." >&2
  exit 2
fi

run() {
  printf '+ '
  printf '%q ' "$@"
  printf '\n'
  "$@"
}

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

run scripts/docker-up.sh --timeout "$TIMEOUT"
run scripts/create-local-gateway-certs.sh
run docker compose -f docker-compose.yml config --quiet
run docker compose -f deploy/slurm-gateway.compose.yml config --quiet

echo "Docker Compose smoke check passed."
