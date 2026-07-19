/** @type {import('@stryker-mutator/api/core').PartialStrykerOptions} */
export default {
  mutate: ["scripts/docker-packaging-lib.mjs"],
  testRunner: "vitest",
  coverageAnalysis: "perTest",
  reporters: ["clear-text", "progress"],
  tempDirName: ".stryker-tmp/docker-packaging",
  vitest: {
    configFile: "vite.config.ts"
  }
};
