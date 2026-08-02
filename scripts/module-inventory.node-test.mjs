import assert from "node:assert/strict";
import test from "node:test";
import { discoverModuleRoots, verifyInventory } from "./module-inventory.mjs";

test("inventory discovers first-party manifests while excluding vendored roots", () => {
  assert.deepEqual(discoverModuleRoots(["package.json", "workers/a/Cargo.toml", "lib/x/go.mod", "lib/x/vendor/y/go.mod", "infra/a/main.tf"]), [".", "infra/a", "lib/x", "workers/a"]);
});
test("inventory rejects missing, stale, and undocumented mappings", () => {
  assert.deepEqual(verifyInventory({ roots: ["new/module"], inventory: { mappings: {}, requiredClaims: ["missing"] }, claimIds: new Set() }), ["new/module: add an executable verification claim or documented exclusion", "required claim missing is absent"]);
});
