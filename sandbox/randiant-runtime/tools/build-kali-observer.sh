#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMAGE="${RADIANT_KALI_OBSERVER_IMAGE:-kali-observer:local}"
DOCKERFILE_DIR="${SCRIPT_DIR}/../infra/docker/kali-observer"

if [ -z "${1:-}" ]; then
  docker build -t "$IMAGE" -f "$DOCKERFILE_DIR/Dockerfile" "$DOCKERFILE_DIR"
  exit 0
fi

if [ "$1" = "--no-cache" ]; then
  docker build --no-cache -t "$IMAGE" -f "$DOCKERFILE_DIR/Dockerfile" "$DOCKERFILE_DIR"
  exit 0
fi

echo "usage: $0 [--no-cache]"
exit 1
