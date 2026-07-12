#!/usr/bin/env node
import { existsSync, lstatSync, readdirSync, rmSync } from "node:fs";
import { homedir, tmpdir } from "node:os";
import { delimiter, join, relative, resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(process.env.RADIANT_REPO_ROOT || fileURLToPath(new URL("..", import.meta.url)));
const gitBin = process.env.GIT_BIN || "git";
const categories = ["generated", "rust-targets", "worktree-deps", "named-caches"];
const categoryLabels = {
  generated: "Generated outputs",
  "rust-targets": "Rust worker targets",
  "worktree-deps": "Worktree dependency installs",
  "named-caches": "Named temp caches"
};
const generatedNames = ["dist", "generated", "storybook-static", ".local"];
const skippedNames = new Set([".git", ".worktrees", "node_modules", "target", "dist", "generated", "storybook-static", ".local"]);

function usage() {
  console.log(`Usage: bun run hygiene:clean [category flags] [--execute]

Category flags:
  --generated       generated outputs and *.tsbuildinfo files
  --rust-targets    workers/*/target directories
  --worktree-deps   node_modules in registered Git worktrees
  --named-caches    named Radiant Go caches

Without category flags, all categories are shown as a dry-run plan.
Nothing is removed unless --execute and at least one category flag are supplied.`);
}

function fail(message) {
  console.error(`Refusing cleanup: ${message}`);
  process.exitCode = 2;
}

function parseArgs(args) {
  const selected = new Set();
  let execute = false;
  for (const arg of args) {
    if (arg === "--execute") {
      execute = true;
    } else if (arg === "--help" || arg === "-h") {
      usage();
      process.exit(0);
    } else if (arg.startsWith("--") && categories.includes(arg.slice(2))) {
      selected.add(arg.slice(2));
    } else {
      throw new Error(`unknown option ${arg}`);
    }
  }
  return { selected: selected.size ? selected : new Set(categories), explicitCategory: selected.size > 0, execute };
}

function run(bin, args) {
  const result = spawnSync(bin, args, { cwd: repoRoot, encoding: "utf8" });
  if (result.error || result.status !== 0) {
    throw new Error(result.error?.message || result.stderr?.trim() || `${bin} exited ${result.status}`);
  }
  return result.stdout;
}

function parseWorktrees(raw) {
  const paths = [];
  for (const block of raw.split(/\n\s*\n/)) {
    const path = block.match(/^worktree (.+)$/m)?.[1];
    if (path) paths.push(resolve(path));
  }
  return paths;
}

function worktreeRoots() {
  return parseWorktrees(run(gitBin, ["worktree", "list", "--porcelain"]));
}

function exists(path) {
  try {
    return existsSync(path);
  } catch {
    return false;
  }
}

function safeEntries(root) {
  try {
    return readdirSync(root, { withFileTypes: true });
  } catch {
    return [];
  }
}

function findTsBuildInfo(root) {
  const matches = [];
  function walk(current) {
    for (const entry of safeEntries(current)) {
      const path = join(current, entry.name);
      if (entry.isDirectory()) {
        if (!skippedNames.has(entry.name)) walk(path);
      } else if (entry.isFile() && entry.name.endsWith(".tsbuildinfo")) {
        matches.push(path);
      }
    }
  }
  walk(root);
  return matches;
}

function present(paths) {
  return paths.filter(exists);
}

function pathsFor(category, roots) {
  if (category === "generated") {
    return roots.flatMap((root) => [
      ...generatedNames.map((name) => join(root, name)),
      ...findTsBuildInfo(root)
    ]);
  }
  if (category === "rust-targets") {
    return roots.flatMap((root) => safeEntries(join(root, "workers"))
      .filter((entry) => entry.isDirectory())
      .map((entry) => join(root, "workers", entry.name, "target")));
  }
  if (category === "worktree-deps") {
    return roots.filter((root) => root !== repoRoot).map((root) => join(root, "node_modules"));
  }
  return (process.env.RADIANT_NAMED_GO_CACHES || ["/tmp/radiant-go-cache", "/tmp/radiant-go-mod-cache"].join(delimiter))
    .split(delimiter)
    .filter(Boolean)
    .map((path) => resolve(path));
}

function label(path, root) {
  const relativePath = relative(root, path);
  return relativePath && !relativePath.startsWith("..") ? relativePath : path;
}

function assertSafe(path, category, roots) {
  if (category === "named-caches") {
    const forbiddenRoots = new Set(["/", resolve(process.env.HOME || homedir()), resolve(tmpdir()), repoRoot, ...roots]);
    if (forbiddenRoots.has(path)) throw new Error(`refusing to remove a broad cache root: ${path}`);
    return;
  }
  const allowed = roots.some((root) => path === root || path.startsWith(`${root}/`));
  if (!allowed) throw new Error(`path outside the registered Radiant roots: ${path}`);
  if (path === repoRoot || roots.includes(path)) throw new Error(`refusing to remove a repository or worktree root: ${path}`);
  try {
    if (lstatSync(path).isSymbolicLink()) return;
  } catch {
    // A path can disappear between planning and execution; rmSync remains idempotent.
  }
}

function main() {
  let options;
  try {
    options = parseArgs(process.argv.slice(2));
  } catch (error) {
    fail(error.message);
    return;
  }
  if (options.execute && !options.explicitCategory) {
    fail("--execute requires one or more explicit category flags; no generic wipe is available");
    return;
  }

  let roots;
  try {
    roots = worktreeRoots();
  } catch (error) {
    fail(`could not inspect registered Git worktrees: ${error.message}`);
    return;
  }
  if (!roots.includes(repoRoot)) roots.unshift(repoRoot);

  console.log("Radiant guarded hygiene cleanup");
  console.log(options.execute ? "Mode: execute (selected paths will be removed after being printed)." : "Mode: dry-run (no paths will be changed).");
  console.log("");

  for (const category of categories) {
    if (!options.selected.has(category)) continue;
    const paths = present(pathsFor(category, roots));
    console.log(`${categoryLabels[category]} (--${category}):`);
    if (!paths.length) {
      console.log("  - none found");
      continue;
    }
    for (const path of paths) {
      assertSafe(path, category, roots);
      const action = options.execute ? "Removing" : "Would remove";
      console.log(`  - ${action}: ${label(path, repoRoot)} (${path})`);
      if (options.execute) rmSync(path, { recursive: true, force: true });
    }
  }
}

main();
