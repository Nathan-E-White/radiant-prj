#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Create a recoverable work-in-progress checkpoint commit and optionally push it.

Usage:
  scripts/checkpoint-wip.sh [options]

Options:
  --dry-run       Show what would happen without staging, committing, or pushing.
  --skip-checks   Skip local verification commands.
  --no-push       Do not push the current branch after checkpointing.
  --push          Push the current branch after checkpointing. Default is no push.
  --allow-jd      Allow JD.mhtml to be staged. By default it is excluded.
  --signed        Request a signed commit.
  -h, --help      Show this help text.

Environment:
  WIP_MESSAGE   Commit message. Default: WIP checkpoint
  REMOTE        Git remote to push to. Default: origin

The script excludes JD.mhtml, node_modules, dist, generated, local certificates,
Vault development material, local environment files, and tool caches unless
--allow-jd is supplied.
USAGE
}

DRY_RUN=0
SKIP_CHECKS=0
PUSH=0
ALLOW_JD=0
SIGNED=0

WIP_MESSAGE="${WIP_MESSAGE:-WIP checkpoint}"
REMOTE="${REMOTE:-origin}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) DRY_RUN=1 ;;
    --skip-checks) SKIP_CHECKS=1 ;;
    --no-push) PUSH=0 ;;
    --push) PUSH=1 ;;
    --allow-jd) ALLOW_JD=1 ;;
    --signed) SIGNED=1 ;;
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

if [[ "$PUSH" -eq 1 ]] && ! git remote get-url "$REMOTE" >/dev/null 2>&1; then
  echo "Remote '$REMOTE' is not configured." >&2
  exit 1
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
  ':!.local'
  ':!.local/**'
  ':!*.key'
  ':!*.crt'
  ':!*.csr'
  ':!*.srl'
  ':!vault_ca.crt'
  ':!vault-token*'
)

if [[ "$ALLOW_JD" -eq 0 ]]; then
  exclude_pathspecs+=(':!JD.mhtml')
fi

run git add -A -- . "${exclude_pathspecs[@]}"

commit_args=(git commit -m "$WIP_MESSAGE")
if [[ "$SIGNED" -eq 1 ]]; then
  commit_args=(git commit -S -m "$WIP_MESSAGE")
fi

if [[ "$DRY_RUN" -eq 0 ]]; then
  if git diff --cached --quiet; then
    echo "No staged source changes; no WIP checkpoint commit created."
  else
    run "${commit_args[@]}"
  fi
else
  echo "Dry run: would commit staged changes if any."
fi

if [[ "$PUSH" -eq 1 ]]; then
  run git push --signed=if-asked "$REMOTE" "$current_branch"
else
  echo "WIP checkpoint remains local; push skipped."
fi

echo "WIP checkpoint flow complete on ${current_branch}."
