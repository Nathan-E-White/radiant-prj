#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

export CHECKPOINT_VERSION="${CHECKPOINT_VERSION:-v2.0.0}"
export CHECKPOINT_MESSAGE="${CHECKPOINT_MESSAGE:-Version 2 checkpoint}"

exec "$script_dir/checkpoint-version.sh" "$@"
