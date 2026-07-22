#!/usr/bin/env node
import { readFile } from "node:fs/promises";
import path from "node:path";
import { affectedCapabilities, validateLedger, verifyCapability } from "./ledger.mjs";

const options = parse(process.argv.slice(2));
const root = path.resolve(options.root ?? process.cwd());
const ledger = await readJson(path.resolve(root, options.ledger ?? "config/capability-ledger.json"));
const manifest = await readJson(path.resolve(root, options.manifest ?? "config/repository-verification.json"));
const problems = validateLedger(ledger, { manifest, root });
if (problems.length) fail({ valid: false, problems });
if (options.changedPaths.length) console.log(JSON.stringify({ valid: true, affectedCapabilities: affectedCapabilities(ledger, options.changedPaths) }));
else if (options.capabilityId) {
  const report = await verifyCapability({ ledger, manifest, capabilityId: options.capabilityId, root });
  console.log(JSON.stringify(report));
  process.exitCode = report.exitCode;
} else console.log(JSON.stringify({ valid: true, capabilities: ledger.capabilities.map(({ id }) => id).sort() }));

async function readJson(file) { try { return JSON.parse(await readFile(file, "utf8")); } catch (error) { fail({ valid: false, problems: [`unable to read JSON ${file}: ${error.message}`] }); } }
function parse(args) {
  const parsed = { changedPaths: [] };
  for (let index = 0; index < args.length; index += 1) {
    const argument = args[index];
    if (argument === "--root") parsed.root = args[++index];
    else if (argument === "--ledger") parsed.ledger = args[++index];
    else if (argument === "--manifest") parsed.manifest = args[++index];
    else if (argument === "--capability") parsed.capabilityId = args[++index];
    else if (argument === "--changed-path") parsed.changedPaths.push(args[++index]);
    else fail({ valid: false, problems: [`unknown argument: ${argument}`] });
  }
  if (parsed.capabilityId && parsed.changedPaths.length) fail({ valid: false, problems: ["--capability and --changed-path cannot be combined"] });
  return parsed;
}
function fail(result) { console.error(JSON.stringify(result)); process.exit(1); }
