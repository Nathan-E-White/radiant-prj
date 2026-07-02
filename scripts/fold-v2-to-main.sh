#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Fold the v2 branch back into the target branch using the target branch worktree.

Usage:
  scripts/fold-v2-to-main.sh [options]

Options:
  --dry-run             Show what would happen without merging or pushing.
  --skip-checks         Skip local verification commands.
  --push                Push the target branch after merge. Default is no push.
  --no-push             Do not push the target branch.
  --source-branch NAME  Branch to merge. Default: current branch.
  --target-branch NAME  Branch to merge into. Default: main.
  --message TEXT        Merge commit message.
  -h, --help            Show this help text.

Environment:
  SOURCE_BRANCH   Branch to merge. Default: current branch.
  TARGET_BRANCH   Branch to merge into. Default: main.
  REMOTE          Git remote to push to. Default: origin

The script is worktree-aware. If main is checked out in a sibling worktree, the
merge is performed there instead of trying to switch branches in this worktree.
USAGE
}

DRY_RUN=0
SKIP_CHECKS=0
PUSH=0
SOURCE_BRANCH="${SOURCE_BRANCH:-}"
TARGET_BRANCH="${TARGET_BRANCH:-main}"
REMOTE="${REMOTE:-origin}"
MERGE_MESSAGE="${MERGE_MESSAGE:-Fold v2 quality documentation into main}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) DRY_RUN=1 ;;
    --skip-checks) SKIP_CHECKS=1 ;;
    --push) PUSH=1 ;;
    --no-push) PUSH=0 ;;
    --source-branch)
      SOURCE_BRANCH="${2:-}"
      shift
      ;;
    --target-branch)
      TARGET_BRANCH="${2:-}"
      shift
      ;;
    --message)
      MERGE_MESSAGE="${2:-}"
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

source_worktree="$(git rev-parse --show-toplevel)"
cd "$source_worktree"

if [[ -z "$SOURCE_BRANCH" ]]; then
  SOURCE_BRANCH="$(git branch --show-current)"
fi

if [[ -z "$SOURCE_BRANCH" ]]; then
  echo "Could not determine source branch." >&2
  exit 1
fi

if [[ "$SOURCE_BRANCH" == "$TARGET_BRANCH" ]]; then
  echo "Source and target branches are both '$TARGET_BRANCH'; nothing to fold." >&2
  exit 1
fi

if ! git show-ref --verify --quiet "refs/heads/${SOURCE_BRANCH}"; then
  echo "Source branch '${SOURCE_BRANCH}' does not exist." >&2
  exit 1
fi

if ! git show-ref --verify --quiet "refs/heads/${TARGET_BRANCH}"; then
  echo "Target branch '${TARGET_BRANCH}' does not exist." >&2
  exit 1
fi

if [[ -n "$(git status --porcelain)" ]]; then
  if [[ "$DRY_RUN" -eq 1 ]]; then
    echo "Dry run: source worktree has uncommitted changes; an actual fold requires scripts/checkpoint-wip.sh first."
  else
    echo "Source worktree has uncommitted changes. Run scripts/checkpoint-wip.sh first." >&2
    exit 1
  fi
fi

target_worktree="$(
  git worktree list --porcelain | awk -v target="refs/heads/${TARGET_BRANCH}" '
    /^worktree / { current = substr($0, 10) }
    /^branch / && substr($0, 8) == target { print current; exit }
  '
)"

if [[ -z "$target_worktree" ]]; then
  echo "No worktree found for target branch '${TARGET_BRANCH}'." >&2
  exit 1
fi

if [[ -n "$(git -C "$target_worktree" status --porcelain)" ]]; then
  if [[ "$DRY_RUN" -eq 1 ]]; then
    echo "Dry run: target worktree '${target_worktree}' has uncommitted changes; an actual fold requires a clean target."
  else
    echo "Target worktree '${target_worktree}' has uncommitted changes." >&2
    exit 1
  fi
fi

if [[ "$PUSH" -eq 1 ]] && ! git -C "$target_worktree" remote get-url "$REMOTE" >/dev/null 2>&1; then
  echo "Remote '$REMOTE' is not configured in target worktree." >&2
  exit 1
fi

if [[ "$SKIP_CHECKS" -eq 0 ]]; then
  run bun run ci
  run bun run build
fi

run git -C "$target_worktree" merge --no-ff "$SOURCE_BRANCH" -m "$MERGE_MESSAGE"

if [[ "$DRY_RUN" -eq 1 ]]; then
  echo "Dry run complete; no merge or push was performed."
elif [[ "$PUSH" -eq 1 ]]; then
  run git -C "$target_worktree" push --signed=if-asked "$REMOTE" "$TARGET_BRANCH"
else
  echo "Fold completed locally; push skipped."
fi

if [[ "$DRY_RUN" -eq 0 ]]; then
  echo "Fold flow complete: ${SOURCE_BRANCH} -> ${TARGET_BRANCH}."
fi
