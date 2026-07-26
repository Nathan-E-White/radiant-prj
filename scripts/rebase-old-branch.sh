#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Suggest or retire old branches.

Usage:
  scripts/rebase-old-branch.sh --list [options]
  scripts/rebase-old-branch.sh --random [options]
  scripts/rebase-old-branch.sh --branch NAME --yes [options]
  scripts/rebase-old-branch.sh --branch NAME --continue [options]
  scripts/rebase-old-branch.sh --branch NAME --abort [options]
  scripts/rebase-old-branch.sh --branch NAME --skip [options]

Options:
  --list                 Print likely cleanup candidates without trial rebases.
  --random               Print one random branch name from the ranked candidate list.
  --limit N              Candidate count for --list. Default: 5.
  --branch NAME          Branch to rebase and delete.
  --target NAME          Branch to rebase onto. Default: main.
  --remote NAME          Git remote. Default: origin.
  --worktree PATH        Temporary worktree path. Default: .worktrees/rebase-old-<branch>.
  --archive-tag NAME     Tag created before deletion. Default: archive/<date>/<branch>.
  --allow-content-diff   Allow deletion even if the rebased branch differs from target.
  --dry-run              Print destructive steps without rebasing, pushing, or deleting.
  --yes                  Required for destructive actions.
  --continue             Continue a conflicted rebase, then finish retirement if clean.
  --abort                Abort a conflicted rebase and remove the temporary worktree.
  --skip                 Skip the current patch in a conflicted rebase, then continue.
  -h, --help             Show this help text.

The --list mode is a cheap suggestion engine. It ranks remote branches by stale
tip date, short unique history, and fork age. It does not prove a branch will
rebase cleanly; the real rebase path prints conflict details if Git disagrees.
The --random mode uses the same ranked top list, then prints only one branch
name so it can be used in command substitution or pipelines.
USAGE
}

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

note() {
  printf '==> %s\n' "$*"
}

run() {
  printf '+ '
  printf '%q ' "$@"
  printf '\n'
  if [[ "$DRY_RUN" -eq 0 ]]; then
    "$@"
  fi
}

slugify_ref() {
  printf '%s' "$1" | tr '/[:space:]' '--' | tr -cd '[:alnum:]_.-'
}

is_protected_branch() {
  case "$1" in
    main|master|develop|dev|release|production|prod) return 0 ;;
    *) return 1 ;;
  esac
}

branch_worktree_path() {
  local branch="$1"
  git worktree list --porcelain | awk -v ref="refs/heads/${branch}" '
    /^worktree / { path = substr($0, 10) }
    /^branch / && substr($0, 8) == ref { print path; exit }
  '
}

short_display() {
  local value="$1"
  local width="$2"
  if [[ "${#value}" -le "$width" ]]; then
    printf '%s' "$value"
  else
    printf '%s...' "${value:0:$((width - 3))}"
  fi
}

default_worktree_path() {
  printf '%s/.worktrees/rebase-old-%s' "$repo_root" "$(slugify_ref "$BRANCH_NAME")"
}

metadata_path() {
  printf '%s/%s.env' "$(metadata_dir)" "$(slugify_ref "$BRANCH_NAME")"
}

metadata_dir() {
  git rev-parse --git-path rebase-old-branch
}

write_metadata() {
  [[ "$DRY_RUN" -eq 0 ]] || return 0
  mkdir -p "$(metadata_dir)"
  {
    printf 'BRANCH_NAME=%q\n' "$BRANCH_NAME"
    printf 'TARGET_BRANCH=%q\n' "$TARGET_BRANCH"
    printf 'REMOTE_NAME=%q\n' "$REMOTE_NAME"
    printf 'ARCHIVE_TAG_NAME=%q\n' "$ARCHIVE_TAG_NAME"
    printf 'ALLOW_CONTENT_DIFF=%q\n' "$ALLOW_CONTENT_DIFF"
  } >"$(metadata_path)"
}

load_metadata() {
  local file
  file="$(metadata_path)"
  [[ -f "$file" ]] || return 0
  # shellcheck disable=SC1090
  source "$file"
}

remove_metadata() {
  local file
  file="$(metadata_path)"
  [[ -e "$file" ]] || return 0
  rm -f "$file"
}

print_conflict_report() {
  local current_sha current_subject

  printf '\nRebase stopped before anything was deleted.\n'
  printf '\nConflicted files:\n'
  if ! git -C "$WORKTREE_PATH" diff --name-only --diff-filter=U | sed 's/^/  - /'; then
    printf '  - unable to read conflicted files\n'
  fi

  printf '\nCurrent blocking commit:\n'
  if current_sha="$(git -C "$WORKTREE_PATH" rev-parse -q --verify REBASE_HEAD 2>/dev/null)"; then
    current_subject="$(git -C "$WORKTREE_PATH" show -s --format=%s "$current_sha")"
    printf '  %s  %s\n' "$current_sha" "$current_subject"
  else
    printf '  unavailable\n'
  fi

  printf '\nStatus:\n'
  git -C "$WORKTREE_PATH" status --short | sed 's/^/  /'

  printf '\nNext commands:\n'
  printf '  scripts/rebase-old-branch.sh --branch %q --continue --yes\n' "$BRANCH_NAME"
  printf '  scripts/rebase-old-branch.sh --branch %q --skip --yes\n' "$BRANCH_NAME"
  printf '  scripts/rebase-old-branch.sh --branch %q --abort\n' "$BRANCH_NAME"
}

rebase_in_progress() {
  [[ -d "$(git -C "$WORKTREE_PATH" rev-parse --git-path rebase-merge 2>/dev/null)" ]] ||
    [[ -d "$(git -C "$WORKTREE_PATH" rev-parse --git-path rebase-apply 2>/dev/null)" ]]
}

cleanup() {
  local status=$?
  trap - EXIT

  if [[ $status -ne 0 && "${KEEP_WORKTREE_ON_FAILURE:-0}" -eq 1 ]] && rebase_in_progress; then
    print_conflict_report >&2 || true
    printf '\nTemporary worktree left at: %s\n' "$WORKTREE_PATH" >&2
    exit "$status"
  fi

  if [[ -n "${TEMP_WORKTREE_ADDED:-}" && -d "${WORKTREE_PATH:-}" ]]; then
    if [[ "$DRY_RUN" -eq 0 ]]; then
      git worktree remove --force "$WORKTREE_PATH" >/dev/null 2>&1 || true
      git worktree prune >/dev/null 2>&1 || true
    fi
  fi

  if [[ $status -ne 0 ]]; then
    printf 'error: cleanup stopped before deleting %s.\n' "${BRANCH_NAME:-the branch}" >&2
  fi

  exit "$status"
}

candidate_records() {
  local target_ref="${REMOTE_NAME}/${TARGET_BRANCH}"

  git show-ref --verify --quiet "refs/remotes/${target_ref}" ||
    die "remote target '${target_ref}' does not exist; fetch first"

  git for-each-ref \
    --format='%(refname:short)|%(committerdate:unix)|%(committerdate:short)|%(objectname:short)' \
    "refs/remotes/${REMOTE_NAME}" |
    while IFS='|' read -r remote_ref tip_unix tip_date tip_short; do
      branch="${remote_ref#${REMOTE_NAME}/}"
      [[ "$remote_ref" != "$REMOTE_NAME" ]] || continue
      [[ "$branch" != "HEAD" ]] || continue
      [[ "$branch" != "$TARGET_BRANCH" ]] || continue
      is_protected_branch "$branch" && continue
      [[ -z "$(branch_worktree_path "$branch")" ]] || continue

      merge_base="$(git merge-base "$target_ref" "$remote_ref" 2>/dev/null)" || continue
      read -r behind ahead < <(git rev-list --left-right --count "${target_ref}...${remote_ref}")
      unique_commits="$(git rev-list --count "${merge_base}..${remote_ref}")"
      fork_unix="$(git show -s --format=%ct "$merge_base")"
      fork_date="$(git show -s --format=%cs "$merge_base")"

      printf '%s|%s|%s|%s|%s|%s|%s|%s|%s\n' \
        "$tip_short" "$tip_unix" "$unique_commits" "$fork_unix" "$branch" \
        "$ahead" "$behind" "$tip_date" "$fork_date"
    done |
    sort -t'|' -k2,2n -k3,3n -k4,4n |
    awk -F'|' -v limit="$LIST_LIMIT" '
      !seen[$1]++ {
        print
        count++
      }
      count == limit { exit }
    '
}

list_candidates() {
  local target_ref="${REMOTE_NAME}/${TARGET_BRANCH}"

  printf 'Likely cleanup candidates for %s (cheap metadata only):\n\n' "$target_ref"
  printf '%-4s %-72s %8s %8s %8s %12s %12s %s\n' \
    "#" "branch" "ahead" "behind" "unique" "tip-date" "fork-date" "why"

  candidate_records |
    awk -F'|' '
      END {
        for (i = 1; i <= NR; i++) {
          split(rows[i], f, "|")
          branch = f[5]
          if (length(branch) > 72) {
            branch = substr(branch, 1, 69) "..."
          }
          why = "old tip, " f[3] " unique commit(s)"
          printf "%-4d %-72s %8s %8s %8s %12s %12s %s\n",
            i, branch, f[6], f[7], f[3], f[8], f[9], why
        }
      }
      { rows[NR] = $0 }
    '

  printf '\nUse --branch <name> --dry-run to inspect one, then --branch <name> --yes to retire it.\n'
}

random_candidate() {
  local records count pick

  records="$(candidate_records)"
  [[ -n "$records" ]] || die "no cleanup candidates found"

  count="$(printf '%s\n' "$records" | wc -l | tr -d '[:space:]')"
  pick=$((RANDOM % count + 1))

  printf '%s\n' "$records" | sed -n "${pick}p" | cut -d'|' -f5
}

finish_retirement() {
  local rebased_head target_head
  load_metadata

  if [[ "$ALLOW_CONTENT_DIFF" -ne 1 ]]; then
    if ! git -C "$WORKTREE_PATH" diff --quiet "$remote_target_ref"...HEAD; then
      die "rebased branch still has file differences from '${REMOTE_NAME}/${TARGET_BRANCH}'; use --allow-content-diff only if that is intentional"
    fi
  fi

  rebased_head="$(git -C "$WORKTREE_PATH" rev-parse HEAD)"
  target_head="$(git rev-parse "$remote_target_ref")"

  note "Rebased branch tip: ${rebased_head}"
  note "Target branch tip:  ${target_head}"

  note "Archiving the rebased tip as ${ARCHIVE_TAG_NAME}"
  run git -C "$WORKTREE_PATH" tag -a "$ARCHIVE_TAG_NAME" -m "Archive retired branch ${BRANCH_NAME}" "$rebased_head"
  run git -C "$WORKTREE_PATH" push "$REMOTE_NAME" "refs/tags/${ARCHIVE_TAG_NAME}"

  note "Deleting ${REMOTE_NAME}/${BRANCH_NAME}"
  run git push "$REMOTE_NAME" --delete "$BRANCH_NAME"

  if git show-ref --verify --quiet "refs/heads/${BRANCH_NAME}"; then
    note "Deleting local branch ${BRANCH_NAME}"
    run git branch -D "$BRANCH_NAME"
  else
    note "No local branch named ${BRANCH_NAME} exists"
  fi

  note "Removing temporary worktree"
  remove_metadata
  run git worktree remove --force "$WORKTREE_PATH"
  TEMP_WORKTREE_ADDED=
  run git worktree prune

  note "Retired ${BRANCH_NAME}. Archive tag: ${ARCHIVE_TAG_NAME}"
}

DRY_RUN=0
YES=0
LIST=0
RANDOM_CANDIDATE=0
LIST_LIMIT=5
ALLOW_CONTENT_DIFF=0
REBASE_ACTION=""
POSITIONAL=()

REMOTE_NAME="${REMOTE:-origin}"
TARGET_BRANCH="${TARGET_BRANCH:-main}"
BRANCH_NAME="${BRANCH:-}"
WORKTREE_PATH="${WORKTREE:-}"
ARCHIVE_TAG_NAME="${ARCHIVE_TAG:-}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --list)
      LIST=1
      ;;
    --random|--random-candidate)
      LIST=1
      RANDOM_CANDIDATE=1
      ;;
    --limit)
      [[ $# -ge 2 && "${2:-}" =~ ^[0-9]+$ && "$2" -gt 0 ]] || die "--limit requires a positive number"
      LIST_LIMIT="$2"
      shift
      ;;
    --branch)
      [[ $# -ge 2 && -n "${2:-}" ]] || die "--branch requires a value"
      BRANCH_NAME="${2:-}"
      shift
      ;;
    --target|--target-branch)
      [[ $# -ge 2 && -n "${2:-}" ]] || die "$1 requires a value"
      TARGET_BRANCH="${2:-}"
      shift
      ;;
    --remote)
      [[ $# -ge 2 && -n "${2:-}" ]] || die "--remote requires a value"
      REMOTE_NAME="${2:-}"
      shift
      ;;
    --worktree)
      [[ $# -ge 2 && -n "${2:-}" ]] || die "--worktree requires a value"
      WORKTREE_PATH="${2:-}"
      shift
      ;;
    --archive-tag)
      [[ $# -ge 2 && -n "${2:-}" ]] || die "--archive-tag requires a value"
      ARCHIVE_TAG_NAME="${2:-}"
      shift
      ;;
    --allow-content-diff)
      ALLOW_CONTENT_DIFF=1
      ;;
    --dry-run)
      DRY_RUN=1
      ;;
    --yes|-y)
      YES=1
      ;;
    --continue|--abort|--skip)
      [[ -z "$REBASE_ACTION" ]] || die "choose only one of --continue, --abort, or --skip"
      REBASE_ACTION="${1#--}"
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      [[ "$1" != -* ]] || die "unknown option: $1"
      POSITIONAL+=("$1")
      ;;
  esac
  shift
done

if [[ "${#POSITIONAL[@]}" -gt 0 ]]; then
  if [[ "${#POSITIONAL[@]}" -eq 3 && -z "${REMOTE:-}" ]]; then
    REMOTE_NAME="${POSITIONAL[0]}"
    BRANCH_NAME="${POSITIONAL[1]}"
    WORKTREE_PATH="${POSITIONAL[2]}"
  elif [[ "${#POSITIONAL[@]}" -eq 2 ]]; then
    BRANCH_NAME="${POSITIONAL[0]}"
    WORKTREE_PATH="${POSITIONAL[1]}"
  elif [[ "${#POSITIONAL[@]}" -eq 1 ]]; then
    BRANCH_NAME="${POSITIONAL[0]}"
  else
    usage >&2
    die "expected --branch NAME, BRANCH, BRANCH WORKTREE, or REMOTE BRANCH WORKTREE"
  fi
fi

[[ -n "$TARGET_BRANCH" ]] || die "--target cannot be empty"
[[ -n "$REMOTE_NAME" ]] || die "--remote cannot be empty"

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

git remote get-url "$REMOTE_NAME" >/dev/null 2>&1 || die "remote '${REMOTE_NAME}' is not configured"

if [[ "$LIST" -eq 1 ]]; then
  [[ -z "$REBASE_ACTION" ]] || die "--list cannot be combined with --${REBASE_ACTION}"
  if [[ "$RANDOM_CANDIDATE" -eq 1 ]]; then
    [[ "$DRY_RUN" -eq 0 ]] || die "--random prints a branch name; do not combine it with --dry-run"
    git fetch --quiet --prune "$REMOTE_NAME" "$TARGET_BRANCH"
    random_candidate
  else
    note "Fetching ${TARGET_BRANCH} and pruning stale remote refs from ${REMOTE_NAME}"
    run git fetch --prune "$REMOTE_NAME" "$TARGET_BRANCH"
    list_candidates
  fi
  exit 0
fi

[[ -n "$BRANCH_NAME" ]] || {
  usage >&2
  die "--branch is required unless --list is used"
}

if is_protected_branch "$BRANCH_NAME"; then
  die "refusing to retire protected branch '${BRANCH_NAME}'"
fi

[[ "$BRANCH_NAME" != "$TARGET_BRANCH" ]] || die "branch and target are both '${TARGET_BRANCH}'"

if [[ -z "$WORKTREE_PATH" ]]; then
  WORKTREE_PATH="$(default_worktree_path)"
fi

if [[ -z "$ARCHIVE_TAG_NAME" ]]; then
  ARCHIVE_TAG_NAME="archive/$(date +%Y-%m-%d)/$(slugify_ref "$BRANCH_NAME")"
fi

remote_branch_ref="refs/remotes/${REMOTE_NAME}/${BRANCH_NAME}"
remote_target_ref="refs/remotes/${REMOTE_NAME}/${TARGET_BRANCH}"

if [[ "$REBASE_ACTION" == "abort" ]]; then
  [[ -d "$WORKTREE_PATH" ]] || die "worktree path does not exist: ${WORKTREE_PATH}"
  load_metadata
  note "Aborting rebase in ${WORKTREE_PATH}"
  git -C "$WORKTREE_PATH" rebase --abort >/dev/null 2>&1 || true
  run git worktree remove --force "$WORKTREE_PATH"
  run git worktree prune
  note "Aborted ${BRANCH_NAME}; no branch or tag was deleted."
  exit 0
fi

if [[ -n "$REBASE_ACTION" ]]; then
  [[ "$YES" -eq 1 ]] || die "--${REBASE_ACTION} can finish destructive cleanup; pass --yes"
  [[ -d "$WORKTREE_PATH" ]] || die "worktree path does not exist: ${WORKTREE_PATH}"
  load_metadata
  git show-ref --verify --quiet "$remote_target_ref" || die "remote target '${REMOTE_NAME}/${TARGET_BRANCH}' does not exist"

  trap cleanup EXIT
  KEEP_WORKTREE_ON_FAILURE=1
  TEMP_WORKTREE_ADDED=1

  note "Running git rebase --${REBASE_ACTION} in ${WORKTREE_PATH}"
  if ! git -C "$WORKTREE_PATH" rebase "--${REBASE_ACTION}"; then
    exit 1
  fi

  KEEP_WORKTREE_ON_FAILURE=0
  finish_retirement
  exit 0
fi

if [[ -n "$(git status --porcelain)" ]]; then
  note "Current worktree has local changes; leaving them untouched"
fi

current_branch="$(git branch --show-current)"
if [[ "$current_branch" == "$BRANCH_NAME" ]]; then
  die "branch '${BRANCH_NAME}' is checked out here; switch away from it before retiring it"
fi

existing_worktree="$(branch_worktree_path "$BRANCH_NAME")"
if [[ -n "$existing_worktree" ]]; then
  die "branch '${BRANCH_NAME}' is already checked out at ${existing_worktree}"
fi

note "Fetching ${TARGET_BRANCH} and ${BRANCH_NAME} from ${REMOTE_NAME}"
run git fetch --prune "$REMOTE_NAME" "$TARGET_BRANCH" "$BRANCH_NAME"

git show-ref --verify --quiet "$remote_branch_ref" || die "remote branch '${REMOTE_NAME}/${BRANCH_NAME}' does not exist"
git show-ref --verify --quiet "$remote_target_ref" || die "remote target '${REMOTE_NAME}/${TARGET_BRANCH}' does not exist"

if [[ -e "$WORKTREE_PATH" ]]; then
  die "worktree path already exists: ${WORKTREE_PATH}"
fi

if git rev-parse -q --verify "refs/tags/${ARCHIVE_TAG_NAME}" >/dev/null; then
  die "archive tag already exists: ${ARCHIVE_TAG_NAME}"
fi

if [[ "$YES" -ne 1 && "$DRY_RUN" -ne 1 ]]; then
  die "destructive cleanup requires --yes; run with --dry-run first if you want the tape measure"
fi

trap cleanup EXIT

note "Creating temporary worktree at ${WORKTREE_PATH}"
run git worktree add --detach "$WORKTREE_PATH" "$remote_branch_ref"
TEMP_WORKTREE_ADDED=1
write_metadata

note "Rebasing ${BRANCH_NAME} onto ${REMOTE_NAME}/${TARGET_BRANCH}"
KEEP_WORKTREE_ON_FAILURE=1
if [[ "$DRY_RUN" -eq 0 ]]; then
  if ! git -C "$WORKTREE_PATH" rebase "$remote_target_ref"; then
    exit 1
  fi
else
  run git -C "$WORKTREE_PATH" rebase "$remote_target_ref"
fi
KEEP_WORKTREE_ON_FAILURE=0

if [[ "$DRY_RUN" -eq 1 ]]; then
  rebased_head="<rebased-${BRANCH_NAME}-head>"
  note "Rebased branch tip: <rebased-${BRANCH_NAME}-head>"
  note "Target branch tip:  $(git rev-parse "$remote_target_ref")"
  note "Archiving the rebased tip as ${ARCHIVE_TAG_NAME}"
  run git -C "$WORKTREE_PATH" tag -a "$ARCHIVE_TAG_NAME" -m "Archive retired branch ${BRANCH_NAME}" "$rebased_head"
  run git -C "$WORKTREE_PATH" push "$REMOTE_NAME" "refs/tags/${ARCHIVE_TAG_NAME}"
  note "Deleting ${REMOTE_NAME}/${BRANCH_NAME}"
  run git push "$REMOTE_NAME" --delete "$BRANCH_NAME"
  if git show-ref --verify --quiet "refs/heads/${BRANCH_NAME}"; then
    note "Deleting local branch ${BRANCH_NAME}"
    run git branch -D "$BRANCH_NAME"
  else
    note "No local branch named ${BRANCH_NAME} exists"
  fi
  note "Removing temporary worktree"
  [[ "$DRY_RUN" -ne 0 ]] || remove_metadata
  run git worktree remove --force "$WORKTREE_PATH"
  TEMP_WORKTREE_ADDED=
  run git worktree prune
  note "Retired ${BRANCH_NAME}. Archive tag: ${ARCHIVE_TAG_NAME}"
else
  finish_retirement
fi
