#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Start the local Docker daemon for development.

Usage:
  scripts/docker-up.sh [--timeout seconds]

Options:
  --timeout seconds  Seconds to wait for Docker to become ready. Default: 90.
  -h, --help         Show this help text.

Environment:
  DOCKER_UP_TIMEOUT  Default timeout when --timeout is not supplied.

This helper targets Docker Desktop on macOS. It refuses to manage Linux or
system-level Docker services.
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

docker_ready() {
  docker info >/dev/null 2>&1
}

if docker_ready; then
  echo "Docker daemon is already available."
  exit 0
fi

if [[ "$(uname -s)" != "Darwin" ]]; then
  cat >&2 <<'EOF'
Docker daemon is not available.

This helper only starts Docker Desktop on macOS. Start your platform Docker
daemon manually, then rerun the command.
EOF
  exit 1
fi

if ! command -v open >/dev/null 2>&1; then
  echo "macOS 'open' command is not available; cannot start Docker Desktop." >&2
  exit 1
fi

if ! command -v osascript >/dev/null 2>&1; then
  echo "macOS 'osascript' command is not available; cannot check Docker Desktop state." >&2
  exit 1
fi

docker_desktop_running() {
  local running
  running="$(osascript -e 'application "Docker" is running' 2>/dev/null)" || return 1
  [[ "$running" == "true" ]]
}

start_docker_desktop() {
  if open -a Docker; then
    return 0
  fi
  if [[ -d "/Applications/Docker.app" ]]; then
    if ! open "/Applications/Docker.app" >/dev/null 2>&1; then
      echo "Could not start /Applications/Docker.app." >&2
      exit 1
    fi
    return 0
  fi
  echo "Could not start Docker Desktop. Is Docker.app installed?" >&2
  exit 1
}

echo "Starting Docker Desktop..."
start_docker_desktop

deadline=$((SECONDS + TIMEOUT))
last_launch="$SECONDS"
while [[ "$SECONDS" -le "$deadline" ]]; do
  if docker_ready; then
    echo "Docker daemon is available."
    exit 0
  fi
  if [[ $((SECONDS - last_launch)) -ge 5 ]] && ! docker_desktop_running; then
    echo "Docker Desktop is not running yet; retrying launch..."
    start_docker_desktop
    last_launch="$SECONDS"
  fi
  sleep 2
done

echo "Timed out waiting ${TIMEOUT}s for Docker daemon readiness." >&2
exit 1
