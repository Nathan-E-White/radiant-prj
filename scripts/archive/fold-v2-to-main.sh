#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

export TARGET_BRANCH="${TARGET_BRANCH:-main}"
export MERGE_MESSAGE="${MERGE_MESSAGE:-Fold v2 quality documentation into main}"

exec "$script_dir/fold-branch.sh" "$@"
