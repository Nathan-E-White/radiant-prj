import { chmodSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { delimiter, join } from "node:path";
import { tmpdir } from "node:os";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const repoRoot = fileURLToPath(new URL("..", import.meta.url)).replace(/\/$/, "");
const scriptPath = join(repoRoot, "scripts", "hygiene-clean.mjs");
const tempDir = mkdtempSync(join(tmpdir(), "radiant-hygiene-clean-test-"));
const fixtureRoot = join(tempDir, "repo");
const fixtureWorktree = join(tempDir, "worktree");
const cacheOne = join(tempDir, "go-cache");
const cacheTwo = join(tempDir, "go-mod-cache");
const fakeGit = join(tempDir, "git");
const gitLog = join(tempDir, "git.log");

function assert(condition, message) {
  if (!condition) throw new Error(message);
}

function run(args, envOverrides = {}) {
  return spawnSync(process.execPath, [scriptPath, ...args], {
    cwd: repoRoot,
    encoding: "utf8",
    env: {
      ...process.env,
      RADIANT_REPO_ROOT: fixtureRoot,
      RADIANT_NAMED_GO_CACHES: `${cacheOne}${delimiter}${cacheTwo}`,
      GIT_BIN: fakeGit,
      ...envOverrides
    }
  });
}

try {
  for (const path of [
    join(fixtureRoot, "dist"),
    join(fixtureRoot, "workers", "simops-generator", "target"),
    join(fixtureWorktree, "node_modules"),
    cacheOne,
    cacheTwo
  ]) mkdirSync(path, { recursive: true });
  writeFileSync(join(fixtureRoot, "app.tsbuildinfo"), "build\n");
  writeFileSync(join(fixtureRoot, "dist", "bundle.js"), "dist\n");
  writeFileSync(join(fixtureRoot, "workers", "simops-generator", "target", "worker"), "target\n");
  writeFileSync(join(fixtureWorktree, "node_modules", "dep.js"), "dependency\n");
  writeFileSync(join(cacheOne, "cache.bin"), "cache\n");
  writeFileSync(join(cacheTwo, "module.zip"), "cache\n");
  writeFileSync(fakeGit, `#!/usr/bin/env bash
set -euo pipefail
printf '%s\\n' "$*" >> "${gitLog}"
if [[ "$*" == "worktree list --porcelain" ]]; then
  cat <<'EOF'
worktree ${fixtureRoot}
HEAD 1111111111111111111111111111111111111111
branch refs/heads/main

worktree ${fixtureWorktree}
HEAD 2222222222222222222222222222222222222222
branch refs/heads/codex/other
EOF
  exit 0
fi
exit 1
`);
  chmodSync(fakeGit, 0o755);

  const plan = run([]);
  assert(plan.status === 0, plan.stderr);
  assert(plan.stdout.includes("Mode: dry-run"), "no-flag invocation should be a dry-run");
  assert(plan.stdout.includes("Generated outputs"), "plan should list generated outputs");
  assert(plan.stdout.includes("worktree-deps"), "plan should list worktree dependencies");
  assert(existsSync(join(fixtureRoot, "dist")), "dry-run must not remove generated output");

  const unsafe = run(["--execute"]);
  assert(unsafe.status !== 0, "execute without categories should fail");
  assert(existsSync(join(fixtureRoot, "dist")), "refused execution must not remove paths");

  const broadCache = run(["--named-caches", "--execute"], { RADIANT_NAMED_GO_CACHES: "/tmp" });
  assert(broadCache.status !== 0, "broad cache roots should be refused");

  const generated = run(["--generated", "--execute"]);
  assert(generated.status === 0, generated.stderr);
  assert(!existsSync(join(fixtureRoot, "dist")), "generated output should be removed when selected");
  assert(!existsSync(join(fixtureRoot, "app.tsbuildinfo")), "tsbuildinfo should be removed when selected");
  assert(existsSync(join(fixtureRoot, "workers", "simops-generator", "target")), "unselected target must remain");

  const rest = run(["--rust-targets", "--worktree-deps", "--named-caches", "--execute"]);
  assert(rest.status === 0, rest.stderr);
  assert(!existsSync(join(fixtureRoot, "workers", "simops-generator", "target")), "selected target should be removed");
  assert(!existsSync(join(fixtureWorktree, "node_modules")), "selected worktree dependency install should be removed");
  assert(!existsSync(cacheOne) && !existsSync(cacheTwo), "selected named caches should be removed");
  assert(readFileSync(gitLog, "utf8").includes("worktree list --porcelain"), "cleanup should inspect Git worktrees");

  console.log("Guarded hygiene cleanup checks passed.");
} finally {
  rmSync(tempDir, { recursive: true, force: true });
}
