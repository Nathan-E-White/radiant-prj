import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";

const adrDir = "docs/adr";
const problems = [];
const repeatedWordPattern = /\b([A-Za-z][A-Za-z'-]{1,})\s+\1\b/gi;

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

if (problems.length) {
  console.error("ADR style check failed:");
  for (const problem of problems) {
    console.error(`- ${problem}`);
  }
  process.exit(1);
}

console.log("ADR style check passed.");
