/** @type {import('@stryker-mutator/api/core').PartialStrykerOptions} */
export default {
  mutate: [
    "src/domain/simulator-workbench/workbenchSnapshotSession.ts:89-333"
  ],
  testRunner: "vitest",
  vitest: {
    configFile: "vite.config.ts"
  },
  coverageAnalysis: "perTest",
  ignorePatterns: [
    "workers/**/target/**",
    ".stryker-tmp/**",
    "dist/**",
    "test-results/**",
    "playwright-report/**"
  ],
  reporters: ["clear-text", "progress"],
  thresholds: {
    high: 90,
    low: 80,
    break: 80
  },
  concurrency: 4
};
