import { readdir, readFile, stat } from "node:fs/promises";
import path from "node:path";
import { spawnSync } from "node:child_process";

const rasterExtensions = new Set([".avif", ".gif", ".jpeg", ".jpg", ".png", ".webp"]);

export async function verifyBrowserDelivery({ root = process.cwd(), budgets, run = runProcess } = {}) {
  const failures = [];
  let productionBuildPassed = false;
  for (const command of [["bun", "run", "typecheck"], ["bun", "run", "test"], ["bun", "run", "build:production"]]) {
    const [program, ...args] = command;
    const result = await run(program, args, { cwd: root });
    if (result.error) failures.push(`${args.at(-1)} could not start: ${result.error.message}`);
    else if (result.status !== 0) failures.push(`${args.at(-1)} exited ${result.status ?? "unknown"}: ${conciseOutput(result)}`);
    else if (args.at(-1) === "build:production") productionBuildPassed = true;
  }

  let outputs;
  if (!productionBuildPassed) {
    failures.push("production build failed; output budgets were not evaluated");
  } else {
    try {
      outputs = await measureOutputs(path.join(root, "dist"));
      checkBudget(failures, "entry asset", outputs.entryBytes, budgets.maxEntryBytes);
      checkBudget(failures, "largest lazy chunk", outputs.lazyChunkBytes, budgets.maxLazyChunkBytes);
      checkBudget(failures, "largest raster asset", outputs.rasterBytes, budgets.maxRasterBytes);
      checkBudget(failures, "total production output", outputs.totalBytes, budgets.maxTotalBytes);
    } catch (error) {
      failures.push(`production output could not be measured: ${error.message}`);
    }
  }

  return { exitCode: failures.length ? 1 : 0, failures, outputs };
}

async function measureOutputs(distDirectory) {
  const files = await collectFiles(distDirectory);
  const named = files.map((file) => ({ ...file, relative: path.relative(distDirectory, file.path).replaceAll(path.sep, "/") }));
  const index = named.find(({ relative }) => relative === "index.html");
  if (!index) throw new Error("dist/index.html is missing");
  const indexText = await readFile(index.path, "utf8");
  const entryPaths = new Set([...indexText.matchAll(/(?:src|href)=["']\/?([^"']+\.(?:js|css))["']/g)].map((match) => match[1]));
  const entryBytes = named.filter(({ relative }) => entryPaths.has(relative)).reduce((total, file) => total + file.bytes, 0);
  const lazyChunkBytes = Math.max(0, ...named.filter(({ relative }) => relative.endsWith(".js") && !entryPaths.has(relative)).map(({ bytes }) => bytes));
  const rasterBytes = Math.max(0, ...named.filter(({ relative }) => rasterExtensions.has(path.extname(relative).toLowerCase())).map(({ bytes }) => bytes));
  return { entryBytes, lazyChunkBytes, rasterBytes, totalBytes: named.reduce((total, file) => total + file.bytes, 0) };
}

async function collectFiles(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const files = await Promise.all(entries.map(async (entry) => {
    const entryPath = path.join(directory, entry.name);
    if (entry.isDirectory()) return collectFiles(entryPath);
    return [{ path: entryPath, bytes: (await stat(entryPath)).size }];
  }));
  return files.flat();
}

function checkBudget(failures, label, actual, maximum) {
  if (!Number.isInteger(maximum) || maximum < 0) {
    failures.push(`${label} budget is invalid: ${String(maximum)}`);
  } else if (actual > maximum) {
    failures.push(`${label} is ${actual} bytes; budget is ${maximum} bytes`);
  }
}

function conciseOutput({ stdout = "", stderr = "" }) {
  return `${stderr}\n${stdout}`.trim().split(/\r?\n/).filter(Boolean).slice(-40).join(" | ");
}

function runProcess(command, args, options) {
  return spawnSync(command, args, { ...options, encoding: "utf8" });
}
