#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Prune Docker/OrbStack storage by explicit category.

Dry-run is the default. Use --execute to run Docker prune commands.

Usage:
  scripts/docker-prune-hygiene.sh [options]

Options:
  --images              Prune unused images with docker image prune --all.
  --build-cache         Prune Docker builder cache.
  --containers          Prune stopped containers.
  --volumes             Prune unused local volumes. Requires --confirm-volumes.
  --all                 Select images, build cache, stopped containers, and volumes.
                        Executing volume pruning still requires --confirm-volumes.
  --confirm-volumes     Acknowledge volume pruning risk when --volumes is set.
  --execute             Run the selected Docker prune commands.
  --dry-run             Print selected commands without running them. Default.
  --context NAME        Docker context to target. Default: orbstack.
  -h, --help            Show this help text.

Environment:
  DOCKER_BIN            Docker executable. Default: docker.
  DOCKER_CONTEXT        Docker context. Default: orbstack.

Examples:
  scripts/docker-prune-hygiene.sh --all
  scripts/docker-prune-hygiene.sh --images --build-cache --execute
  scripts/docker-prune-hygiene.sh --volumes --confirm-volumes --execute
USAGE
}

DOCKER_BIN="${DOCKER_BIN:-docker}"
DOCKER_CONTEXT="${DOCKER_CONTEXT:-orbstack}"
EXECUTE=0
PRUNE_IMAGES=0
PRUNE_BUILD_CACHE=0
PRUNE_CONTAINERS=0
PRUNE_VOLUMES=0
CONFIRM_VOLUMES=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --images) PRUNE_IMAGES=1 ;;
    --build-cache) PRUNE_BUILD_CACHE=1 ;;
    --containers) PRUNE_CONTAINERS=1 ;;
    --volumes) PRUNE_VOLUMES=1 ;;
    --all)
      PRUNE_IMAGES=1
      PRUNE_BUILD_CACHE=1
      PRUNE_CONTAINERS=1
      PRUNE_VOLUMES=1
      ;;
    --confirm-volumes) CONFIRM_VOLUMES=1 ;;
    --execute) EXECUTE=1 ;;
    --dry-run) EXECUTE=0 ;;
    --context)
      if [[ $# -lt 2 || -z "${2:-}" ]]; then
        echo "--context requires a non-empty value." >&2
        exit 2
      fi
      DOCKER_CONTEXT="${2:-}"
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

if [[ -z "$DOCKER_CONTEXT" ]]; then
  echo "Docker context cannot be empty." >&2
  exit 2
fi

selected_count=$((PRUNE_IMAGES + PRUNE_BUILD_CACHE + PRUNE_CONTAINERS + PRUNE_VOLUMES))
if [[ "$selected_count" -eq 0 ]]; then
  echo "Select at least one prune category: --images, --build-cache, --containers, --volumes, or --all." >&2
  exit 2
fi

if [[ "$EXECUTE" -eq 1 && "$PRUNE_VOLUMES" -eq 1 && "$CONFIRM_VOLUMES" -ne 1 ]]; then
  echo "Volume pruning requires --confirm-volumes because Docker volumes may contain local runtime data." >&2
  exit 2
fi

print_command() {
  printf '%q --context %q' "$DOCKER_BIN" "$DOCKER_CONTEXT"
  printf ' %q' "$@"
  printf '\n'
}

run_docker() {
  print_command "$@"
  if [[ "$EXECUTE" -eq 1 ]]; then
    "$DOCKER_BIN" --context "$DOCKER_CONTEXT" "$@"
  fi
}

if [[ "$EXECUTE" -eq 1 ]]; then
  echo "Docker storage before pruning:"
  run_docker system df
else
  echo "Dry run: no Docker prune command was executed."
  echo "Selected Docker context: ${DOCKER_CONTEXT}"
fi

if [[ "$PRUNE_IMAGES" -eq 1 ]]; then
  run_docker image prune --all --force
fi

if [[ "$PRUNE_BUILD_CACHE" -eq 1 ]]; then
  run_docker builder prune --force
fi

if [[ "$PRUNE_CONTAINERS" -eq 1 ]]; then
  run_docker container prune --force
fi

if [[ "$PRUNE_VOLUMES" -eq 1 ]]; then
  run_docker volume prune --force
fi

if [[ "$EXECUTE" -eq 1 ]]; then
  echo "Docker storage after pruning:"
  run_docker system df
else
  echo "Rerun with --execute to run the selected prune commands."
fi
