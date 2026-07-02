#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Create the version 2 checkpoint commit/tag and optionally push it.

Usage:
  scripts/checkpoint-v2.sh [options]

Options:
  --dry-run       Show what would happen without staging, committing, tagging, or pushing.
  --skip-checks   Skip local verification commands.
  --no-push       Create the local commit/tag but do not push to the remote.
  --allow-jd      Allow JD.mhtml to be staged. By default it is excluded.
  --force-tag     Recreate the local version tag if it already exists.
  --unsigned      Create unsigned commit/tag instead of requesting signatures.
  -h, --help      Show this help text.

Environment:
  CHECKPOINT_VERSION   Version tag to create. Default: v2.0.0
  CHECKPOINT_MESSAGE   Commit/tag message. Default: Version 2 checkpoint
  REMOTE               Git remote to push to. Default: origin

The script excludes JD.mhtml, node_modules, dist, generated, local environment
files, and tool caches unless --allow-jd is supplied.
USAGE
}

DRY_RUN=0
SKIP_CHECKS=0
PUSH=1
ALLOW_JD=0
FORCE_TAG=0
SIGNED=1

CHECKPOINT_VERSION="${CHECKPOINT_VERSION:-v2.0.0}"
CHECKPOINT_MESSAGE="${CHECKPOINT_MESSAGE:-Version 2 checkpoint}"
REMOTE="${REMOTE:-origin}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) DRY_RUN=1 ;;
    --skip-checks) SKIP_CHECKS=1 ;;
    --no-push) PUSH=0 ;;
    --allow-jd) ALLOW_JD=1 ;;
    --force-tag) FORCE_TAG=1 ;;
    --unsigned) SIGNED=0 ;;
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

if [[ "$SIGNED" -eq 1 ]]; then
  commit_signing="$(git config --bool --get commit.gpgsign || true)"
  tag_signing="$(git config --bool --get tag.gpgSign || true)"
  signing_format="$(git config --get gpg.format || true)"
  signing_key="$(git config --get user.signingkey || true)"

  if [[ "$commit_signing" != "true" ]]; then
    echo "commit.gpgsign must be true before creating a signed checkpoint. Use --unsigned to bypass." >&2
    exit 1
  fi

  if [[ "$tag_signing" != "true" ]]; then
    echo "tag.gpgSign must be true before creating a signed checkpoint. Use --unsigned to bypass." >&2
    exit 1
  fi

  if [[ -z "$signing_format" || -z "$signing_key" ]]; then
    echo "Git signing is missing gpg.format or user.signingkey. Use --unsigned to bypass." >&2
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
  ':!.vite'
  ':!.terraform'
  ':!terraform.tfstate*'
  ':!.env'
  ':!.env.*'
)

if [[ "$ALLOW_JD" -eq 0 ]]; then
  exclude_pathspecs+=(':!JD.mhtml')
fi

run git add -A -- . "${exclude_pathspecs[@]}"

commit_args=(git commit -m "$CHECKPOINT_MESSAGE")
tag_args=(git tag -a "$CHECKPOINT_VERSION" -m "$CHECKPOINT_MESSAGE")
if [[ "$SIGNED" -eq 1 ]]; then
  commit_args=(git commit -S -m "$CHECKPOINT_MESSAGE")
  tag_args=(git tag -s "$CHECKPOINT_VERSION" -m "$CHECKPOINT_MESSAGE")
fi

if [[ "$DRY_RUN" -eq 0 ]]; then
  if git diff --cached --quiet; then
    echo "No staged source changes; using existing HEAD for ${CHECKPOINT_VERSION}."
  else
    run "${commit_args[@]}"
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

run "${tag_args[@]}"

if [[ "$DRY_RUN" -eq 1 ]]; then
  echo "Dry run complete; no commit, tag, or push was created."
elif [[ "$PUSH" -eq 1 ]]; then
  run git push --signed=if-asked "$REMOTE" "$current_branch"
  if [[ "$FORCE_TAG" -eq 1 ]]; then
    run git push --signed=if-asked --force "$REMOTE" "$CHECKPOINT_VERSION"
  else
    run git push --signed=if-asked "$REMOTE" "$CHECKPOINT_VERSION"
  fi
else
  echo "Created local checkpoint only; push skipped by --no-push."
fi

if [[ "$DRY_RUN" -eq 0 ]]; then
  echo "Checkpoint ${CHECKPOINT_VERSION} is ready on ${current_branch}."
fi
