/** @type {import('@stryker-mutator/api/core').PartialStrykerOptions} */
export default {
  mutate: ["scripts/capability-ledger/ledger.mjs"],
  testRunner: "vitest",
  coverageAnalysis: "perTest",
  reporters: ["clear-text", "progress"],
  tempDirName: ".stryker-tmp/capability-ledger",
  vitest: { configFile: "vite.config.ts" },
  thresholds: { high: 90, low: 75, break: 75 }
};
