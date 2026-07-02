#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

export TARGET_BRANCH="${TARGET_BRANCH:-main}"
export MERGED_BRANCH="${MERGED_BRANCH:-${V2_BRANCH:-codex/v2-quality-docs}}"
export EXTRA_WORKTREE="${EXTRA_WORKTREE:-../radiant-prj-v2}"
export VERSION_TAG="${VERSION_TAG:-v2.0.0}"
export TAG_MESSAGE="${TAG_MESSAGE:-Version 2 checkpoint}"

args=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --v2-branch)
      args+=(--merged-branch "${2:-}")
      shift 2
      ;;
    --keep-v2-branch)
      args+=(--keep-merged-branch)
      shift
      ;;
    *)
      args+=("$1")
      shift
      ;;
  esac
done

exec "$script_dir/cleanup-version-hygiene.sh" "${args[@]}"
