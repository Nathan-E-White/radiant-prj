#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Stop the local Docker daemon for development.

Usage:
  scripts/docker-down.sh [--timeout seconds]

Options:
  --timeout seconds  Seconds to wait for Docker to become unavailable. Default: 60.
  -h, --help         Show this help text.

Environment:
  DOCKER_DOWN_TIMEOUT  Default timeout when --timeout is not supplied.

This helper targets Docker Desktop on macOS. It refuses to stop Linux or
system-level Docker services.
USAGE
}

TIMEOUT="${DOCKER_DOWN_TIMEOUT:-60}"

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

docker_ready() {
  docker info >/dev/null 2>&1
}

if [[ "$(uname -s)" != "Darwin" ]]; then
  cat >&2 <<'EOF'
Refusing to stop Docker automatically on this platform.

This helper only quits Docker Desktop on macOS. Stop your platform Docker
daemon manually if that is what you intend.
EOF
  exit 1
fi

if ! command -v osascript >/dev/null 2>&1; then
  echo "macOS 'osascript' command is not available; cannot quit Docker Desktop." >&2
  exit 1
fi

echo "Quitting Docker Desktop..."
if ! osascript -e 'if application "Docker" is running then tell application "Docker" to quit' >/dev/null; then
  echo "Could not ask Docker Desktop to quit." >&2
  exit 1
fi

deadline=$((SECONDS + TIMEOUT))
while [[ "$SECONDS" -le "$deadline" ]]; do
  if ! docker_ready; then
    echo "Docker daemon is unavailable."
    exit 0
  fi
  sleep 2
done

echo "Timed out waiting ${TIMEOUT}s for Docker daemon shutdown." >&2
exit 1
