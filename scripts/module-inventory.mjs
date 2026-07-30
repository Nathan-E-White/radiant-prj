import { readFile } from "node:fs/promises";
import { spawnSync } from "node:child_process";
import path from "node:path";

const ignored = ["third_party/", "vendor/", "generated/", ".cache/"];

export function discoverModuleRoots(files) {
  return [...new Set(files.flatMap((file) => {
    if (ignored.some((prefix) => file.includes(`/${prefix}`) || file.startsWith(prefix))) return [];
    if (file === "package.json") return ["."];
    if (file.endsWith("/Cargo.toml") || file.endsWith("/go.mod")) return [path.posix.dirname(file)];
    if (file.endsWith(".tf") && file.startsWith("infra/")) return [path.posix.dirname(file)];
    return [];
  }))].sort();
}

export function verifyInventory({ roots, inventory, claimIds }) {
  const failures = roots.flatMap((root) => {
    const mapping = inventory.mappings[root];
    if (!mapping) return [`${root}: add an executable verification claim or documented exclusion`];
    if (mapping.claim && !claimIds.has(mapping.claim)) return [`${root}: mapped claim ${mapping.claim} is stale`];
    if (mapping.exclusion && (!mapping.exclusion.rationale || !mapping.exclusion.owner)) return [`${root}: exclusion requires rationale and owner`];
    return [];
  });
  for (const claim of inventory.requiredClaims) if (!claimIds.has(claim)) failures.push(`required claim ${claim} is absent`);
  return failures;
}

async function main() {
  const root = process.cwd();
  const tracked = spawnSync("git", ["ls-files"], { cwd: root, encoding: "utf8" });
  if (tracked.status !== 0) throw new Error(tracked.stderr.trim());
  const [inventory, manifest] = await Promise.all([
    readFile(path.join(root, "config/module-inventory.json"), "utf8").then(JSON.parse),
    readFile(path.join(root, "config/repository-verification.json"), "utf8").then(JSON.parse),
  ]);
  const roots = discoverModuleRoots(tracked.stdout.trim().split("\n").filter(Boolean));
  const failures = verifyInventory({ roots, inventory, claimIds: new Set(manifest.claims.map(({ id }) => id)) });
  if (failures.length) throw new Error(`Module inventory failed:\n${failures.join("\n")}`);
  console.log(`Module inventory: ${roots.length} first-party roots mapped to claims or documented exclusions.`);
}

if (process.argv[1] && path.resolve(process.argv[1]) === new URL(import.meta.url).pathname) await main();
