import assert from "node:assert/strict";
import test from "node:test";

import { inspectStorybookCatalog } from "./verify-storybook-catalog.mjs";

const indexHtml = '<!doctype html><html><body><div id="root"></div></body></html>';

test("Storybook catalog evidence requires compiled Simulation Health and Simulator Workbench stories", () => {
  const observation = inspectStorybookCatalog({
    indexHtml,
    index: { entries: {
      nominal: { type: "story", title: "Simulator Workbench / Simulation Health Summary", exportName: "Nominal" },
      stale: { type: "story", title: "Simulator Workbench / Simulation Health Summary", exportName: "LifecycleRunningWithStaleStream" },
      degraded: { type: "story", title: "Simulator Workbench / Simulation Health Summary", exportName: "ArtifactPipelineDegraded" },
      critical: { type: "story", title: "Simulator Workbench / Simulation Health Summary", exportName: "CriticalWorkerAndArtifacts" },
    } },
  });

  assert.deepEqual(observation, {
    storyCount: 4,
    titles: ["Simulator Workbench / Simulation Health Summary"],
  });
});

test("Storybook catalog evidence rejects a build that omits a required presentation surface", () => {
  assert.match(
    inspectStorybookCatalog({ indexHtml, index: { entries: { health: { type: "story", title: "Simulation Health Summary" } } } }),
    /Simulator Workbench/,
  );
});
