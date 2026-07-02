#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Finalize v2 push/tag hygiene and remove the extra v2 worktree.

Usage:
  scripts/cleanup-v2-hygiene.sh [options]

Options:
  --dry-run             Show what would happen without fetching, tagging, pushing, or removing.
  --skip-checks         Skip local verification commands.
  --no-push             Do not push the target branch or tag.
  --unsigned            Create an unsigned annotated tag if the tag is missing.
  --force-tag           Recreate the local tag if it exists at a different commit.
  --keep-worktree       Leave the extra v2 worktree in place.
  --keep-v2-branch      Leave the merged v2 branch in place.
  --target-branch NAME  Branch to push and tag. Default: main.
  --v2-branch NAME      Merged v2 branch to clean up. Default: codex/v2-quality-docs.
  --worktree PATH       Extra worktree path. Default: ../radiant-prj-v2.
  --tag NAME            Version tag. Default: v2.0.0.
  --message TEXT        Tag message. Default: Version 2 checkpoint.
  -h, --help            Show this help text.

Environment:
  REMOTE                Git remote to push to. Default: origin

Default behavior:
  1. Verify the current worktree is the clean target branch.
  2. Run local verification.
  3. Fetch remote refs and tags.
  4. Create or verify the v2 tag at the target branch HEAD.
  5. Push the target branch and tag.
  6. Remove the extra v2 worktree and delete the merged v2 branch.
USAGE
}

DRY_RUN=0
SKIP_CHECKS=0
PUSH=1
SIGNED=1
FORCE_TAG=0
KEEP_WORKTREE=0
KEEP_V2_BRANCH=0

REMOTE="${REMOTE:-origin}"
TARGET_BRANCH="${TARGET_BRANCH:-main}"
V2_BRANCH="${V2_BRANCH:-codex/v2-quality-docs}"
EXTRA_WORKTREE="${EXTRA_WORKTREE:-../radiant-prj-v2}"
VERSION_TAG="${VERSION_TAG:-v2.0.0}"
TAG_MESSAGE="${TAG_MESSAGE:-Version 2 checkpoint}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) DRY_RUN=1 ;;
    --skip-checks) SKIP_CHECKS=1 ;;
    --no-push) PUSH=0 ;;
    --unsigned) SIGNED=0 ;;
    --force-tag) FORCE_TAG=1 ;;
    --keep-worktree) KEEP_WORKTREE=1 ;;
    --keep-v2-branch) KEEP_V2_BRANCH=1 ;;
    --target-branch)
      TARGET_BRANCH="${2:-}"
      shift
      ;;
    --v2-branch)
      V2_BRANCH="${2:-}"
      shift
      ;;
    --worktree)
      EXTRA_WORKTREE="${2:-}"
      shift
      ;;
    --tag)
      VERSION_TAG="${2:-}"
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

if git show-ref --verify --quiet "refs/heads/${V2_BRANCH}"; then
  if ! git merge-base --is-ancestor "$V2_BRANCH" "$TARGET_BRANCH"; then
    echo "V2 branch '${V2_BRANCH}' is not merged into '${TARGET_BRANCH}'." >&2
    exit 1
  fi
else
  echo "V2 branch '${V2_BRANCH}' is already absent; branch cleanup will be skipped."
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

if [[ "$KEEP_WORKTREE" -eq 0 ]]; then
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
else
  echo "Extra worktree cleanup skipped by --keep-worktree."
fi

if [[ "$KEEP_V2_BRANCH" -eq 0 ]]; then
  if git show-ref --verify --quiet "refs/heads/${V2_BRANCH}"; then
    run git branch -d "$V2_BRANCH"
  else
    echo "V2 branch '${V2_BRANCH}' is already absent."
  fi
else
  echo "V2 branch cleanup skipped by --keep-v2-branch."
fi

if [[ "$DRY_RUN" -eq 1 ]]; then
  echo "Dry run complete; no refs, remote state, branches, or worktrees were changed."
else
  echo "V2 hygiene cleanup complete: pushed '${TARGET_BRANCH}', handled '${VERSION_TAG}', and cleaned '${V2_BRANCH}'."
fi

