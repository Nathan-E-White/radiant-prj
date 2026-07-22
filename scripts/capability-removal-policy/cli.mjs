#!/usr/bin/env node
import { readFile } from "node:fs/promises";
import path from "node:path";
import { loadHistoricalLedger } from "../capability-reconciliation/git-adapter.mjs";
import { verifyCapability } from "../capability-ledger/ledger.mjs";
import { loadChangedPaths } from "./git-adapter.mjs";
import { evaluateCapabilityRemovalPolicy } from "./policy.mjs";

const options = parse(process.argv.slice(2));
const root = path.resolve(options.root ?? process.cwd());
try {
  const [{ ledger: previousLedger }, currentLedger, currentManifest, evidence] = await Promise.all([
    Promise.resolve(loadHistoricalLedger({ baseline: options.base, root })),
    readJson(path.resolve(root, "config/capability-ledger.json")),
    readJson(path.resolve(root, "config/repository-verification.json")),
    readJson(path.resolve(root, "config/capability-change-evidence.json")),
  ]);
  const successorClaims = currentLedger.capabilities.filter(({ lifecycle }) => lifecycle === "superseded").map(({ successorId }) => currentLedger.capabilities.find(({ id }) => id === successorId)?.verificationClaim).filter(Boolean);
  const verificationReports = await Promise.all(successorClaims.map((claim) => verifyCapability({ ledger: currentLedger, manifest: currentManifest, capabilityId: currentLedger.capabilities.find(({ verificationClaim }) => verificationClaim === claim)?.id, root })));
  const verifiedClaimIds = verificationReports.filter(({ exitCode }) => exitCode === 0).flatMap(({ results }) => results.filter(({ status }) => status === "pass").map(({ claimId }) => claimId));
  const report = evaluateCapabilityRemovalPolicy({ previousLedger, currentLedger, currentManifest, verifiedClaimIds, evidence, changes: loadChangedPaths({ base: options.base, root }) });
  console.log(JSON.stringify(report));
  process.exitCode = report.exitCode;
} catch (error) { console.error(JSON.stringify({ valid: false, problems: [error.message] })); process.exitCode = 1; }

function parse(args) {
  const parsed = {};
  for (let index = 0; index < args.length; index += 1) {
    if (args[index] === "--base") parsed.base = args[++index];
    else if (args[index] === "--root") parsed.root = args[++index];
    else throw new Error(`unknown argument: ${args[index]}`);
  }
  if (!parsed.base) throw new Error("--base is required");
  return parsed;
}
async function readJson(file) { return JSON.parse(await readFile(file, "utf8")); }
