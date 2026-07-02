#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Finalize version push/tag hygiene and optionally clean a merged branch/worktree.

Usage:
  scripts/cleanup-version-hygiene.sh --version VERSION [options]

Options:
  --version NAME          Version tag to create or verify.
  --tag NAME              Alias for --version.
  --dry-run               Show what would happen without fetching, tagging, pushing, or removing.
  --skip-checks           Skip local verification commands.
  --no-push               Do not push the target branch or tag.
  --unsigned              Create an unsigned annotated tag if the tag is missing.
  --force-tag             Recreate the local tag if it exists at a different commit.
  --keep-worktree         Leave the extra worktree in place.
  --keep-merged-branch    Leave the merged branch in place.
  --target-branch NAME    Branch to push and tag. Default: main.
  --merged-branch NAME    Merged branch to clean up. Optional.
  --worktree PATH         Extra worktree path to remove. Optional.
  --message TEXT          Tag message. Default: Version <version> checkpoint.
  -h, --help              Show this help text.

Environment:
  VERSION_TAG             Version tag to create or verify. Required unless --version is supplied.
  TARGET_BRANCH           Branch to push and tag. Default: main.
  MERGED_BRANCH           Merged branch to clean up. Optional.
  EXTRA_WORKTREE          Extra worktree path to remove. Optional.
  TAG_MESSAGE             Tag message. Default: Version <version> checkpoint.
  REMOTE                  Git remote to push to. Default: origin.
USAGE
}

DRY_RUN=0
SKIP_CHECKS=0
PUSH=1
SIGNED=1
FORCE_TAG=0
KEEP_WORKTREE=0
KEEP_MERGED_BRANCH=0

REMOTE="${REMOTE:-origin}"
TARGET_BRANCH="${TARGET_BRANCH:-main}"
MERGED_BRANCH="${MERGED_BRANCH:-}"
EXTRA_WORKTREE="${EXTRA_WORKTREE:-}"
VERSION_TAG="${VERSION_TAG:-}"
TAG_MESSAGE="${TAG_MESSAGE:-}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version|--tag)
      VERSION_TAG="${2:-}"
      shift
      ;;
    --dry-run) DRY_RUN=1 ;;
    --skip-checks) SKIP_CHECKS=1 ;;
    --no-push) PUSH=0 ;;
    --unsigned) SIGNED=0 ;;
    --force-tag) FORCE_TAG=1 ;;
    --keep-worktree) KEEP_WORKTREE=1 ;;
    --keep-merged-branch) KEEP_MERGED_BRANCH=1 ;;
    --target-branch)
      TARGET_BRANCH="${2:-}"
      shift
      ;;
    --merged-branch)
      MERGED_BRANCH="${2:-}"
      shift
      ;;
    --worktree)
      EXTRA_WORKTREE="${2:-}"
      shift
      ;;
    --message)
      TAG_MESSAGE="${2:-}"
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

if [[ -z "$VERSION_TAG" ]]; then
  echo "A version tag is required. Supply --version, --tag, or VERSION_TAG." >&2
  usage >&2
  exit 2
fi

if [[ -z "$TAG_MESSAGE" ]]; then
  TAG_MESSAGE="Version ${VERSION_TAG} checkpoint"
fi

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
if [[ "$current_branch" != "$TARGET_BRANCH" ]]; then
  echo "Expected to run from '${TARGET_BRANCH}', but current branch is '${current_branch:-detached HEAD}'." >&2
  exit 1
fi

if [[ -n "$(git status --porcelain)" ]]; then
  if [[ "$DRY_RUN" -eq 1 ]]; then
    echo "Dry run: target worktree has uncommitted changes; actual cleanup requires a clean worktree."
  else
    echo "Target worktree has uncommitted changes. Commit or stash them before cleanup." >&2
    exit 1
  fi
fi

if ! git remote get-url "$REMOTE" >/dev/null 2>&1; then
  echo "Remote '${REMOTE}' is not configured." >&2
  exit 1
fi

if ! git show-ref --verify --quiet "refs/heads/${TARGET_BRANCH}"; then
  echo "Target branch '${TARGET_BRANCH}' does not exist." >&2
  exit 1
fi

if [[ -n "$MERGED_BRANCH" ]]; then
  if git show-ref --verify --quiet "refs/heads/${MERGED_BRANCH}"; then
    if ! git merge-base --is-ancestor "$MERGED_BRANCH" "$TARGET_BRANCH"; then
      echo "Merged branch '${MERGED_BRANCH}' is not merged into '${TARGET_BRANCH}'." >&2
      exit 1
    fi
  else
    echo "Merged branch '${MERGED_BRANCH}' is already absent; branch cleanup will be skipped."
  fi
fi

if [[ "$SKIP_CHECKS" -eq 0 ]]; then
  run bun run ci
  run bun run build
fi

run git fetch --tags "$REMOTE"

if git show-ref --verify --quiet "refs/remotes/${REMOTE}/${TARGET_BRANCH}"; then
  behind_count="$(git rev-list --count "${TARGET_BRANCH}..${REMOTE}/${TARGET_BRANCH}")"
  if [[ "$behind_count" != "0" ]]; then
    echo "'${TARGET_BRANCH}' is behind '${REMOTE}/${TARGET_BRANCH}'. Pull or merge before cleanup." >&2
    exit 1
  fi
fi

target_head="$(git rev-parse "$TARGET_BRANCH")"
tag_exists=0
if git rev-parse -q --verify "refs/tags/${VERSION_TAG}" >/dev/null; then
  tag_exists=1
  tag_target="$(git rev-list -n 1 "$VERSION_TAG")"
  if [[ "$tag_target" != "$target_head" ]]; then
    if [[ "$FORCE_TAG" -eq 1 ]]; then
      run git tag -d "$VERSION_TAG"
      tag_exists=0
    else
      echo "Tag '${VERSION_TAG}' points at ${tag_target}, not ${target_head}. Use --force-tag to recreate it." >&2
      exit 1
    fi
  else
    echo "Tag '${VERSION_TAG}' already points at ${TARGET_BRANCH} HEAD."
  fi
fi

if [[ "$tag_exists" -eq 0 ]]; then
  if [[ "$SIGNED" -eq 1 ]]; then
    tag_signing="$(git config --bool --get tag.gpgSign || true)"
    signing_format="$(git config --get gpg.format || true)"
    signing_key="$(git config --get user.signingkey || true)"

    if [[ "$tag_signing" != "true" ]]; then
      echo "tag.gpgSign must be true before creating a signed tag. Use --unsigned to bypass." >&2
      exit 1
    fi

    if [[ -z "$signing_format" || -z "$signing_key" ]]; then
      echo "Git signing is missing gpg.format or user.signingkey. Use --unsigned to bypass." >&2
      exit 1
    fi

    run git tag -s "$VERSION_TAG" -m "$TAG_MESSAGE"
  else
    run git tag -a "$VERSION_TAG" -m "$TAG_MESSAGE"
  fi
fi

if [[ "$PUSH" -eq 1 ]]; then
  run git push --signed=if-asked "$REMOTE" "$TARGET_BRANCH"
  if [[ "$FORCE_TAG" -eq 1 ]]; then
    run git push --signed=if-asked --force "$REMOTE" "$VERSION_TAG"
  else
    run git push --signed=if-asked "$REMOTE" "$VERSION_TAG"
  fi
else
  echo "Push skipped by --no-push."
fi

if [[ "$KEEP_WORKTREE" -eq 0 && -n "$EXTRA_WORKTREE" ]]; then
  if git -C "$EXTRA_WORKTREE" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    extra_root="$(git -C "$EXTRA_WORKTREE" rev-parse --show-toplevel)"
    if [[ -n "$(git -C "$extra_root" status --porcelain)" ]]; then
      echo "Extra worktree '${extra_root}' has uncommitted tracked/untracked changes; refusing to remove it." >&2
      exit 1
    fi
    run git worktree remove --force "$extra_root"
  else
    echo "Extra worktree '${EXTRA_WORKTREE}' is already absent."
  fi
elif [[ -n "$EXTRA_WORKTREE" ]]; then
  echo "Extra worktree cleanup skipped by --keep-worktree."
else
  echo "No extra worktree supplied; worktree cleanup skipped."
fi

if [[ "$KEEP_MERGED_BRANCH" -eq 0 && -n "$MERGED_BRANCH" ]]; then
  if git show-ref --verify --quiet "refs/heads/${MERGED_BRANCH}"; then
    run git branch -d "$MERGED_BRANCH"
  else
    echo "Merged branch '${MERGED_BRANCH}' is already absent."
  fi
elif [[ -n "$MERGED_BRANCH" ]]; then
  echo "Merged branch cleanup skipped by --keep-merged-branch."
else
  echo "No merged branch supplied; branch cleanup skipped."
fi

if [[ "$DRY_RUN" -eq 1 ]]; then
  echo "Dry run complete; no refs, remote state, branches, or worktrees were changed."
else
  echo "Version hygiene cleanup complete: handled '${VERSION_TAG}' on '${TARGET_BRANCH}'."
fi
