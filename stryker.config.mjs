/** @type {import('@stryker-mutator/api/core').PartialStrykerOptions} */
export default {
  mutate: [
    "src/domain/simulator-workbench/workbenchSnapshotSession.ts:33-120",
    "src/domain/simulator-workbench/liveWorkbench.ts:170-179",
    "src/domain/simulator-workbench/liveWorkbench.ts:210-222",
    "src/domain/simulator-workbench/liveWorkbench.ts:228-233"
  ],
  testRunner: "vitest",
  vitest: {
    configFile: "vite.config.ts"
  },
  coverageAnalysis: "perTest",
  reporters: ["clear-text", "progress"],
  thresholds: {
    high: 90,
    low: 80,
    break: 80
  },
  concurrency: 4
};
