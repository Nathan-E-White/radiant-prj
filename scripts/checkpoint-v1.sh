#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Create a version 1 checkpoint commit/tag and optionally push it to GitHub.

Usage:
  scripts/checkpoint-v1.sh [options]

Options:
  --dry-run       Show what would happen without staging, committing, tagging, or pushing.
  --skip-checks   Skip local verification commands.
  --no-push       Create the local commit/tag but do not push to the remote.
  --allow-jd      Allow JD.mhtml to be staged. By default it is excluded.
  --force-tag     Recreate the local version tag if it already exists.
  --skip-signing-check
                 Skip signing preflight checks. Commit/tag commands still request signatures.
  -h, --help      Show this help text.

Environment:
  CHECKPOINT_VERSION   Version tag to create. Default: v1.0.0
  CHECKPOINT_MESSAGE   Commit/tag message. Default: Version 1 checkpoint
  REMOTE               Git remote to push to. Default: origin

The script intentionally excludes JD.mhtml, node_modules, dist, generated,
and local environment files unless --allow-jd is supplied.
USAGE
}

DRY_RUN=0
SKIP_CHECKS=0
PUSH=1
ALLOW_JD=0
FORCE_TAG=0
SKIP_SIGNING_CHECK=0

CHECKPOINT_VERSION="${CHECKPOINT_VERSION:-v1.0.0}"
CHECKPOINT_MESSAGE="${CHECKPOINT_MESSAGE:-Version 1 checkpoint}"
REMOTE="${REMOTE:-origin}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)
      DRY_RUN=1
      ;;
    --skip-checks)
      SKIP_CHECKS=1
      ;;
    --no-push)
      PUSH=0
      ;;
    --allow-jd)
      ALLOW_JD=1
      ;;
    --force-tag)
      FORCE_TAG=1
      ;;
    --skip-signing-check)
      SKIP_SIGNING_CHECK=1
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

run() {
  printf '+ '
  printf '%q ' "$@"
  printf '\n'
  if [[ "$DRY_RUN" -eq 0 ]]; then
    "$@"
  fi
}

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

current_branch="$(git branch --show-current)"
if [[ -z "$current_branch" ]]; then
  echo "Refusing to checkpoint from detached HEAD." >&2
  exit 1
fi

if ! git remote get-url "$REMOTE" >/dev/null 2>&1; then
  echo "Remote '$REMOTE' is not configured." >&2
  exit 1
fi

if [[ "$SKIP_SIGNING_CHECK" -eq 0 ]]; then
  commit_signing="$(git config --bool --get commit.gpgsign || true)"
  tag_signing="$(git config --bool --get tag.gpgSign || true)"
  signing_format="$(git config --get gpg.format || true)"
  signing_key="$(git config --get user.signingkey || true)"

  if [[ "$commit_signing" != "true" ]]; then
    echo "commit.gpgsign must be true before creating a checkpoint." >&2
    exit 1
  fi

  if [[ "$tag_signing" != "true" ]]; then
    echo "tag.gpgSign must be true before creating a checkpoint." >&2
    exit 1
  fi

  if [[ -z "$signing_format" || -z "$signing_key" ]]; then
    echo "Git signing is missing gpg.format or user.signingkey." >&2
    exit 1
  fi
fi

if [[ "$SKIP_CHECKS" -eq 0 ]]; then
  run bun run ci
  run bun run build
fi

exclude_pathspecs=(
  ':!node_modules'
  ':!dist'
  ':!generated'
  ':!.env'
  ':!.env.*'
)

if [[ "$ALLOW_JD" -eq 0 ]]; then
  exclude_pathspecs+=(':!JD.mhtml')
fi

run git add -A -- . "${exclude_pathspecs[@]}"

if [[ "$DRY_RUN" -eq 0 ]]; then
  if git diff --cached --quiet; then
    echo "No staged source changes; using existing HEAD for ${CHECKPOINT_VERSION}."
  else
    run git commit -S -m "$CHECKPOINT_MESSAGE"
  fi
else
  echo "Dry run: would commit staged changes if any."
fi

if git rev-parse -q --verify "refs/tags/${CHECKPOINT_VERSION}" >/dev/null; then
  if [[ "$FORCE_TAG" -eq 1 ]]; then
    run git tag -d "$CHECKPOINT_VERSION"
  else
    echo "Tag ${CHECKPOINT_VERSION} already exists. Use --force-tag to recreate it." >&2
    exit 1
  fi
fi

run git tag -s "$CHECKPOINT_VERSION" -m "$CHECKPOINT_MESSAGE"

if [[ "$PUSH" -eq 1 ]]; then
  run git push --signed=if-asked "$REMOTE" "$current_branch"
  if [[ "$FORCE_TAG" -eq 1 ]]; then
    run git push --signed=if-asked --force "$REMOTE" "$CHECKPOINT_VERSION"
  else
    run git push --signed=if-asked "$REMOTE" "$CHECKPOINT_VERSION"
  fi
else
  echo "Created local checkpoint only; push skipped by --no-push."
fi

echo "Checkpoint ${CHECKPOINT_VERSION} is ready on ${current_branch}."
