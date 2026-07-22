import { execFileSync } from "node:child_process";

export function loadHistoricalLedger({ baseline, root = process.cwd(), run = execFileSync } = {}) {
  if (!baseline) throw new Error("a Git baseline is required");
  const revision = run("git", ["rev-parse", "--verify", baseline], { cwd: root, encoding: "utf8" }).trim();
  const source = run("git", ["show", `${revision}:config/capability-ledger.json`], { cwd: root, encoding: "utf8" });
  try { return { revision, ledger: JSON.parse(source) }; } catch (error) { throw new Error(`historical capability ledger at ${revision} is invalid JSON: ${error.message}`); }
}

export function loadHistoricalRange({ range, root = process.cwd(), run = execFileSync } = {}) {
  if (!range || !range.includes("..")) throw new Error("a Git range in <from>..<to> form is required");
  const revisions = run("git", ["rev-list", "--reverse", range], { cwd: root, encoding: "utf8" }).trim().split("\n").filter(Boolean);
  const capabilities = new Map();
  for (const revision of revisions) {
    try {
      const source = run("git", ["show", `${revision}:config/capability-ledger.json`], { cwd: root, encoding: "utf8" });
      const ledger = JSON.parse(source);
      for (const capability of ledger.capabilities ?? []) capabilities.set(capability.id, { ...capability, historicalRevision: revision });
    } catch { /* The ledger predates part of the requested range; those commits have no retained-capability commitment. */ }
  }
  if (capabilities.size === 0) throw new Error(`no historical capability ledger was found in range ${range}`);
  return { revision: range, ledger: { schemaVersion: "radiant.capability-ledger.v1", capabilities: [...capabilities.values()] } };
}
