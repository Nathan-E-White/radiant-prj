import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";

const adrDir = "docs/adr";
const problems = [];
const repeatedWordPattern = /\b([A-Za-z][A-Za-z'-]{1,})\s+\1\b/gi;

function requireText(path, text, requiredText, description = requiredText) {
  if (!text.includes(requiredText)) {
    problems.push(`${path}: missing ${description}`);
  }
}

for (const name of readdirSync(adrDir).sort()) {
  const path = join(adrDir, name);
  if (!statSync(path).isFile() || (!name.endsWith(".md") && !name.endsWith(".txt"))) {
    continue;
  }

  const text = readFileSync(path, "utf8");
  const lines = text.split(/\r?\n/);

  lines.forEach((line, index) => {
    const lineNumber = index + 1;

    if (/[ \t]$/.test(line)) {
      problems.push(`${path}:${lineNumber} trailing whitespace`);
    }

    const repeatedMatches = [...line.matchAll(repeatedWordPattern)];
    for (const match of repeatedMatches) {
      problems.push(`${path}:${lineNumber} repeated word: ${match[0]}`);
    }
  });

  if (name.endsWith(".md") && !/^# .+/m.test(text)) {
    problems.push(`${path}: missing top-level title`);
  }

  if (name.endsWith(".md") && !/^## Status$/m.test(text)) {
    problems.push(`${path}: missing Status heading`);
  }
}

const backendGameBoundaryPath = join(adrDir, "adr-0007.md");
let backendGameBoundary = "";

try {
  backendGameBoundary = readFileSync(backendGameBoundaryPath, "utf8");
} catch {
  problems.push(`${backendGameBoundaryPath}: missing accepted backend game boundary`);
}

if (backendGameBoundary) {
  for (const heading of [
    "## Backend Execution Boundary",
    "## Stream And Identity Boundary",
    "## Persistence And Lifecycle Boundary",
    "## Security Boundary",
    "## Configured Data Flush And Recovery",
    "## Live Read Boundary",
    "## Sequencing Gates",
  ]) {
    requireText(backendGameBoundaryPath, backendGameBoundary, heading);
  }

  for (const term of [
    "Simulation Job",
    "SimOps Run",
    "Resident Source",
    "Run-Scoped Simulation Worker",
    "operational telemetry",
    "Simulated Result State",
    "Twin State",
    "Lineage",
    "Gateway-Only Worker Ingest",
    "Artifact Forge",
    "Reactor Telemetry Worker Set",
    "Configured Data Flush",
    "Workbench Snapshot",
  ]) {
    requireText(backendGameBoundaryPath, backendGameBoundary, term, `boundary term ${term}`);
  }

  for (const gate of ["Prerequisite", "Anytime", "Deferred", "#62", "#63", "#64", "#65", "#66", "#67", "#68", "#69", "#70"]) {
    requireText(backendGameBoundaryPath, backendGameBoundary, gate, `sequencing gate ${gate}`);
  }
}

const phaserDecisionPath = join(adrDir, "adr-0006.md");
const phaserDecision = readFileSync(phaserDecisionPath, "utf8");
requireText(phaserDecisionPath, phaserDecision, "ADR-0007", "revision link to the backend game boundary");

const glossaryPath = "CONTEXT.md";
const glossary = readFileSync(glossaryPath, "utf8");
for (const term of ["**SimOps Run**:", "**Artifact Forge**:", "**Reactor Telemetry Worker Set**:", "**Configured Data Flush**:", "**Workbench Snapshot**:"]) {
  requireText(glossaryPath, glossary, term, `glossary term ${term}`);
}

if (problems.length) {
  console.error("ADR style check failed:");
  for (const problem of problems) {
    console.error(`- ${problem}`);
  }
  process.exit(1);
}

console.log("ADR style check passed.");
