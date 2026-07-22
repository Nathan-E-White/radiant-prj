#!/usr/bin/env node
import { readFile } from "node:fs/promises";
import path from "node:path";
import { parse as parseYaml } from "yaml";
import { verifyPipelinePolicy } from "./policy.mjs";

const root = path.resolve(process.argv[2] ?? process.cwd());
try {
  const policy = await readJson("config/pipeline-policy.json");
  const ledger = await readJson("config/capability-ledger.json");
  const workflows = Object.fromEntries(await Promise.all(policy.policies.map(async ({ workflow }) => [workflow, parseYaml(await readText(workflow))])));
  const report = verifyPipelinePolicy({ policy, ledger, workflows });
  console.log(JSON.stringify(report));
  process.exitCode = report.exitCode;
} catch (error) { console.error(JSON.stringify({ valid: false, problems: [error.message] })); process.exitCode = 1; }
function readText(file) { return readFile(path.resolve(root, file), "utf8"); }
async function readJson(file) { return JSON.parse(await readText(file)); }
