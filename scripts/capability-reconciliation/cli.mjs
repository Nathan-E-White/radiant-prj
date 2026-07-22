#!/usr/bin/env node
import { readFile } from "node:fs/promises";
import path from "node:path";
import { loadHistoricalLedger, loadHistoricalRange } from "./git-adapter.mjs";
import { reconcileCapabilities } from "./reconcile.mjs";
import { verifyCapability } from "../capability-ledger/ledger.mjs";

const options = parse(process.argv.slice(2));
const root = path.resolve(options.root ?? process.cwd());
try {
  const { revision, ledger: historicalLedger } = options.range ? loadHistoricalRange({ range: options.range, root }) : loadHistoricalLedger({ baseline: options.baseline, root });
  const [currentLedger, manifest] = await Promise.all([readJson(path.resolve(root, "config/capability-ledger.json")), readJson(path.resolve(root, "config/repository-verification.json"))]);
  console.log(JSON.stringify(await reconcileCapabilities({ historicalLedger, historicalRevision: revision, currentLedger, manifest, root, verifyCapability })));
} catch (error) { console.error(JSON.stringify({ valid: false, problems: [error.message] })); process.exitCode = 1; }

function parse(args) {
  const parsed = {};
  for (let index = 0; index < args.length; index += 1) {
    if (args[index] === "--baseline") parsed.baseline = args[++index];
    else if (args[index] === "--range") parsed.range = args[++index];
    else if (args[index] === "--root") parsed.root = args[++index];
    else throw new Error(`unknown argument: ${args[index]}`);
  }
  if (!parsed.baseline && !parsed.range) throw new Error("--baseline or --range is required");
  if (parsed.baseline && parsed.range) throw new Error("--baseline and --range cannot be combined");
  return parsed;
}
async function readJson(file) { return JSON.parse(await readFile(file, "utf8")); }
