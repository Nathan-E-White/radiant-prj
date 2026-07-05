#!/usr/bin/env bash
set -euo pipefail

missing=0
for binary in docker kind tofu; do
  if ! command -v "$binary" >/dev/null 2>&1; then
    echo "missing: $binary"
    missing=1
  fi
done

if [ "$missing" -ne 0 ]; then
  echo "Please install missing binaries before running the orchestrator app."
  exit 1
fi

KALI_IMAGE="${RADIANT_KALI_OBSERVER_IMAGE:-kali-observer:local}"
if ! docker image inspect "$KALI_IMAGE" >/dev/null 2>&1; then
  echo "warning: monitor image '$KALI_IMAGE' is missing; run tools/build-kali-observer.sh"
fi

echo "preflight ok: docker, kind, tofu, and monitor prerequisites"
