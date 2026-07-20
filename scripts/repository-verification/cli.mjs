#!/usr/bin/env node

import { readFile } from "node:fs/promises";
import path from "node:path";

import { formatVerificationReport, verifyRepository } from "./verifier.mjs";

const options = parseArguments(process.argv.slice(2));
const root = path.resolve(options.root ?? process.cwd());
const manifestPath = path.resolve(root, options.manifest ?? "config/repository-verification.json");

let manifest;
try {
  manifest = JSON.parse(await readFile(manifestPath, "utf8"));
} catch (error) {
  console.error(`[FAIL] manifest.load: Unable to load verification manifest\n  evidence: ${manifestPath}\n  expected: valid JSON verification manifest\n  observed: ${error.message}\nRepository verification: 0 passed, 1 failed.`);
  process.exitCode = 1;
}

if (manifest) {
  const report = await verifyRepository({ root, manifest, claimIds: options.claims });
  const output = formatVerificationReport(report);
  if (report.exitCode === 0) console.log(output);
  else console.error(output);
  process.exitCode = report.exitCode;
}

function parseArguments(args) {
  const parsed = { claims: [] };
  for (let index = 0; index < args.length; index += 1) {
    switch (args[index]) {
      case "--root":
        parsed.root = args[++index];
        break;
      case "--manifest":
        parsed.manifest = args[++index];
        break;
      case "--claim":
        parsed.claims.push(args[++index]);
        break;
      default:
        console.error(`Unknown argument: ${args[index]}`);
        process.exit(2);
    }
  }
  return parsed;
}
