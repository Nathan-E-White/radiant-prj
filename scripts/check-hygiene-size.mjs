import { chmodSync, mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { delimiter, join } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const repoRoot = fileURLToPath(new URL("..", import.meta.url)).replace(/\/$/, "");
const scriptPath = join(repoRoot, "scripts", "hygiene-size.mjs");

function assert(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

function assertIncludes(value, expected, message) {
  assert(value.includes(expected), `${message}\nMissing: ${expected}\nOutput:\n${value}`);
}

const tempDir = mkdtempSync(join(tmpdir(), "radiant-hygiene-size-test-"));
const fixtureRoot = join(tempDir, "repo");
const fixtureWorktree = join(tempDir, "worktree");
const fixtureHome = join(tempDir, "home");
const gitLogPath = join(tempDir, "git.log");
const dockerLogPath = join(tempDir, "docker.log");
const goLogPath = join(tempDir, "go.log");
const fakeGit = join(tempDir, "git");
const fakeDocker = join(tempDir, "docker");
const fakeGo = join(tempDir, "go");
const namedGoCache = join(tempDir, "radiant-go-cache");
const namedGoModCache = join(tempDir, "radiant-go-mod-cache");
const goBuildCache = join(tempDir, "go-build-cache");
const goModuleCache = join(tempDir, "go-module-cache");

function runAudit(envOverrides = {}) {
  return spawnSync(process.execPath, [scriptPath], {
    cwd: repoRoot,
    encoding: "utf8",
    env: {
      ...process.env,
      RADIANT_REPO_ROOT: fixtureRoot,
      RADIANT_NAMED_GO_CACHES: `${namedGoCache}${delimiter}${namedGoModCache}`,
      GIT_BIN: fakeGit,
      DOCKER_BIN: fakeDocker,
      GO_BIN: fakeGo,
      HOME: fixtureHome,
      ...envOverrides
    }
  });
}

try {
  mkdirSync(join(fixtureRoot, "node_modules", "pkg"), { recursive: true });
  mkdirSync(join(fixtureRoot, "dist"), { recursive: true });
  mkdirSync(join(fixtureRoot, "test-results"), { recursive: true });
  mkdirSync(join(fixtureRoot, "target", "debug"), { recursive: true });
  mkdirSync(join(fixtureRoot, "workers", "simops-generator", "target", "debug"), { recursive: true });
  mkdirSync(join(fixtureWorktree, "node_modules", "dep"), { recursive: true });
  mkdirSync(join(fixtureWorktree, "generated"), { recursive: true });
  mkdirSync(join(fixtureHome, ".bun", "install", "cache"), { recursive: true });
  mkdirSync(namedGoCache, { recursive: true });
  mkdirSync(namedGoModCache, { recursive: true });
  mkdirSync(goBuildCache, { recursive: true });
  mkdirSync(goModuleCache, { recursive: true });

  writeFileSync(join(fixtureRoot, "node_modules", "pkg", "index.js"), "console.log('fixture');\n");
  writeFileSync(join(fixtureRoot, "dist", "bundle.js"), "bundle\n");
  writeFileSync(join(fixtureRoot, "test-results", "results.json"), "{}\n");
  writeFileSync(join(fixtureRoot, "target", "debug", "app"), "app\n");
  writeFileSync(join(fixtureRoot, "app.tsbuildinfo"), "{}\n");
  writeFileSync(join(fixtureRoot, "workers", "simops-generator", "target", "debug", "worker"), "worker\n");
  writeFileSync(join(fixtureWorktree, "node_modules", "dep", "index.js"), "dep\n");
  writeFileSync(join(fixtureWorktree, "generated", "artifact.json"), "{}\n");
  writeFileSync(join(fixtureHome, ".bun", "install", "cache", "pkg.tgz"), "bun-cache\n");
  writeFileSync(join(namedGoCache, "cache.bin"), "go-cache\n");
  writeFileSync(join(namedGoModCache, "mod.zip"), "go-mod-cache\n");
  writeFileSync(join(goBuildCache, "build.bin"), "global-go-cache\n");
  writeFileSync(join(goModuleCache, "module.zip"), "global-go-mod-cache\n");

  writeFileSync(
    fakeGit,
    `#!/usr/bin/env bash
set -euo pipefail
printf '%s\\n' "$*" >> "${gitLogPath}"
if [[ "$*" == "worktree list --porcelain" ]]; then
  cat <<'GIT'
worktree ${fixtureRoot}
HEAD 1111111111111111111111111111111111111111
branch refs/heads/main

worktree ${fixtureWorktree}
HEAD 2222222222222222222222222222222222222222
branch refs/heads/codex/dev-hygiene

GIT
  exit 0
fi
if [[ "$1" == "-C" && "$3" == "status" && "$4" == "--short" ]]; then
  if [[ "$2" == "${fixtureWorktree}" ]]; then
    printf ' M package.json\\n'
  fi
  exit 0
fi
echo "unexpected git args: $*" >&2
exit 1
`
  );

  writeFileSync(
    fakeDocker,
    `#!/usr/bin/env bash
set -euo pipefail
printf '%s\\n' "$*" >> "${dockerLogPath}"
if [[ "$*" == "--context orbstack system df" ]]; then
  cat <<'DOCKER'
TYPE            TOTAL     ACTIVE    SIZE      RECLAIMABLE
Images          2         1         512MB     128MB (25%)
Build Cache     4         0         1.2GB     1.2GB
DOCKER
  exit 0
fi
echo "unexpected docker args: $*" >&2
exit 1
`
  );

  writeFileSync(
    fakeGo,
    `#!/usr/bin/env bash
set -euo pipefail
printf '%s\\n' "$*" >> "${goLogPath}"
if [[ "$*" == "env GOCACHE GOMODCACHE" ]]; then
  printf '%s\\n%s\\n' "${goBuildCache}" "${goModuleCache}"
  exit 0
fi
echo "unexpected go args: $*" >&2
exit 1
`
  );

  chmodSync(fakeGit, 0o755);
  chmodSync(fakeDocker, 0o755);
  chmodSync(fakeGo, 0o755);

  const result = runAudit();

  assert(result.status === 0, `hygiene size check failed: ${result.stderr}\n${result.stdout}`);

  assertIncludes(result.stdout, "Radiant hygiene size audit", "report should have a title");
  assertIncludes(result.stdout, "Read-only: reports storage only", "report should declare read-only behavior");
  assertIncludes(result.stdout, "Repo-local storage", "report should include repo-local storage");
  assertIncludes(result.stdout, "node_modules", "report should include dependency installs");
  assertIncludes(result.stdout, "dist", "report should include generated/build output");
  assertIncludes(result.stdout, "test-results", "report should include test output");
  assertIncludes(result.stdout, "target", "report should include Rust target output");
  assertIncludes(result.stdout, "app.tsbuildinfo", "report should include tsbuildinfo files");
  assertIncludes(result.stdout, "workers/simops-generator/target", "report should include Rust worker target output");
  assertIncludes(result.stdout, "Registered Git worktrees", "report should include worktrees");
  assertIncludes(result.stdout, fixtureWorktree, "report should include the second worktree path");
  assertIncludes(result.stdout, "State: clean (0 changed paths)", "report should include clean worktree state");
  assertIncludes(result.stdout, "State: dirty (1 changed path)", "report should include dirty worktree state");
  assertIncludes(result.stdout, "storybook-static: skipped (not present)", "report should mark absent output paths as skipped");
  assertIncludes(result.stdout, "External toolchain caches", "report should include external caches");
  assertIncludes(result.stdout, namedGoCache, "report should include named Go cache");
  assertIncludes(result.stdout, ".bun/install/cache", "report should include Bun cache");
  assertIncludes(result.stdout, "Docker/OrbStack storage", "report should include Docker/OrbStack storage");
  assertIncludes(result.stdout, "Images", "report should print docker system df output");

  const gitLog = readFileSync(gitLogPath, "utf8");
  const dockerLog = readFileSync(dockerLogPath, "utf8");
  const goLog = readFileSync(goLogPath, "utf8");

  assertIncludes(gitLog, "worktree list --porcelain", "script should query registered worktrees");
  assertIncludes(gitLog, `-C ${fixtureWorktree} status --short`, "script should query worktree status");
  assertIncludes(dockerLog, "--context orbstack system df", "script should only request Docker storage reporting");
  assertIncludes(goLog, "env GOCACHE GOMODCACHE", "script should query Go cache locations");

  for (const forbidden of [" prune", " rm ", " rmdir ", " unlink "]) {
    assert(!gitLog.includes(forbidden), `git command log contains forbidden operation: ${forbidden}`);
    assert(!dockerLog.includes(forbidden), `docker command log contains forbidden operation: ${forbidden}`);
    assert(!goLog.includes(forbidden), `go command log contains forbidden operation: ${forbidden}`);
  }

  const unavailableTools = runAudit({
    DOCKER_BIN: join(tempDir, "missing-docker"),
    GO_BIN: join(tempDir, "missing-go")
  });
  assert(
    unavailableTools.status === 0,
    `hygiene size should pass when optional tools are unavailable: ${unavailableTools.stderr}\n${unavailableTools.stdout}`
  );
  assertIncludes(unavailableTools.stdout, "Global Go caches", "report should still include Go cache section");
  assertIncludes(unavailableTools.stdout, "skipped (spawnSync", "report should mark unavailable tools as skipped");
  assertIncludes(unavailableTools.stdout, "Docker/OrbStack storage", "report should still include Docker section");

  console.log("Hygiene size audit checks passed.");
} finally {
  rmSync(tempDir, { recursive: true, force: true });
}
