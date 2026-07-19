import { expect, test } from "@playwright/test";

test("Workbench cadence recovers fixture and stale states by replacing the whole Snapshot", async ({ page }) => {
  await page.clock.install({ time: new Date("2026-07-18T12:00:00Z") });
  const requestHeaders: Array<Record<string, string>> = [];
  let serveLive = false;
  let generation = 4;
  await page.route("**/api/simulator-workbench/snapshot", async (route) => {
    requestHeaders.push(route.request().headers());
    if (!serveLive) {
      await route.fulfill({ status: 503, body: "unavailable" });
      return;
    }
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(liveSnapshot(generation)) });
  });

  await page.goto("/");
  await page.getByRole("button", { name: "Status Workbench" }).click();
  await expect(page.getByText("Fixture fallback", { exact: true })).toBeVisible();
  await expect(page.getByText(/explicit local-demo fixture Snapshot/)).toBeVisible();
  const health = page.getByRole("region", { name: "Simulation health cards" });
  await expect(health.getByText("2/2 complete", { exact: true })).toBeVisible();
  await page.locator('button[data-unit-id="KAL-03"]').click();
  await expect(page.getByRole("heading", { name: "Kaleidos Unit 03" })).toBeVisible();
  const fixtureMeasured = page.getByRole("region", { name: "Measured State" });
  await fixtureMeasured.getByRole("button", { name: /Electric Output/ }).click();
  await expect(fixtureMeasured.getByRole("button", { name: /Electric Output/ })).toHaveAttribute("aria-pressed", "true");

  const measuredValue = page.getByRole("button", { name: /Flux Axial Low/ });
  await measuredValue.click();
  await expect(measuredValue).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByLabel("Bottom Explanation Rail").locator(".simwb-count")).toContainText("measured");

  const simulatedValue = page.getByRole("button", { name: /Simulated Forecast Margin/ });
  await simulatedValue.click();
  await expect(simulatedValue).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByLabel("Bottom Explanation Rail").locator(".simwb-count")).toContainText("simulated");

  const missingLineageValue = page.getByRole("button", { name: /Unmeasured Fuel\/Block Temperature Estimate/ });
  await missingLineageValue.click();
  await expect(missingLineageValue).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByText(/Lineage pending for VAL-KAL-01-IMPUTED-BLOCK-TEMP/)).toBeVisible();

  const unit01 = page.locator('button[data-unit-id="KAL-01"]');
  const unit02 = page.locator('button[data-unit-id="KAL-02"]');
  await unit02.click();
  await expect(unit02).toHaveAttribute("aria-pressed", "true");
  await expect(unit01).toHaveAttribute("aria-pressed", "false");
  await expect(page.getByText("Commercial Display Basis")).toBeVisible();

  await page.getByRole("button", { name: "Refresh live Snapshot" }).click();
  await expect(page.getByText("Fixture fallback", { exact: true })).toBeVisible();
  await expect(page.getByText(/Retaining the explicit whole-Snapshot fixture fallback/)).toBeVisible();

  serveLive = true;
  await page.clock.fastForward(10_000);
  await expect(page.getByText("Live generation 4")).toBeVisible();
  await expect(page.getByText(/accepted atomically/)).toBeVisible();
  await expect(page.getByText("Measured State").first()).toBeVisible();
  await expect(page.getByText("Imputed State").first()).toBeVisible();
  await expect(page.getByText("Simulated Result State").first()).toBeVisible();
  await expect(page.getByRole("heading", { name: "reactor-01" })).toBeVisible();
  await expect(page.getByRole("region", { name: "Imputed State" })
    .getByText("Retained generation projection marker", { exact: true })).toBeVisible();
  await expect(page.getByRole("region", { name: "Imputed State" })
    .getByRole("button", { name: /Retained generation projection marker/ })).toHaveAttribute("aria-pressed", "true");
  await expect(health.getByText("0/1 complete", { exact: true })).toBeVisible();
  await expect(health.getByText("1/1 nominal", { exact: true })).toBeVisible();
  await expect(health.getByText("committed", { exact: true })).toBeVisible();
  await expect(health.getByText("2/2 complete", { exact: true })).not.toBeVisible();

  serveLive = false;
  await page.getByRole("button", { name: "Refresh live Snapshot" }).click();
  await expect(page.getByText("Stale live generation 4")).toBeVisible();

  serveLive = true;
  generation = 5;
  await page.clock.fastForward(10_000);
  await expect(page.getByText("Live generation 5")).toBeVisible();

  expect(requestHeaders.length).toBeGreaterThanOrEqual(2);
  for (const headers of requestHeaders) {
    expect(headers["x-workbench-ingest-token"]).toBeUndefined();
    expect(headers["x-simops-ingest-token"]).toBeUndefined();
    expect(headers.authorization).toBeUndefined();
  }
});

test("Workbench session exposes initial live success and typed terminal errors without credentials", async ({ page }) => {
  const requestHeaders: Array<Record<string, string>> = [];
  let outcome: "live" | "auth" | "schema" = "live";
  await page.route("**/api/simulator-workbench/snapshot", async (route) => {
    requestHeaders.push(route.request().headers());
    if (outcome === "auth") {
      await route.fulfill({ status: 401, body: "denied" });
      return;
    }
    const snapshot = liveSnapshot(6);
    if (outcome === "schema") snapshot.state.schemaVersion = "simulator-workbench.state.v2";
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(snapshot) });
  });

  await page.goto("/");
  await page.getByRole("button", { name: "Status Workbench" }).click();
  await expect(page.getByText("Live generation 6")).toBeVisible();

  outcome = "auth";
  await page.reload();
  await page.getByRole("button", { name: "Status Workbench" }).click();
  await expect(page.getByText("Live Snapshot error")).toBeVisible();
  await expect(page.getByText("Workbench authorization failed (401).")).toBeVisible();

  outcome = "schema";
  await page.reload();
  await page.getByRole("button", { name: "Status Workbench" }).click();
  await expect(page.getByText("Live Snapshot error")).toBeVisible();
  await expect(page.getByText("Workbench state schema is not supported.")).toBeVisible();

  expect(requestHeaders.length).toBeGreaterThanOrEqual(3);
  for (const headers of requestHeaders) {
    expect(headers["x-workbench-ingest-token"]).toBeUndefined();
    expect(headers["x-simops-ingest-token"]).toBeUndefined();
    expect(headers.authorization).toBeUndefined();
  }
});

test("Workbench retains stale live data, rejects generation regression, and recovers", async ({ page }) => {
  let generation = 8;
  let unavailable = false;
  await page.route("**/api/simulator-workbench/snapshot", async (route) => {
    if (unavailable) {
      await route.fulfill({ status: 503, body: "unavailable" });
      return;
    }
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(liveSnapshot(generation)) });
  });

  await page.goto("/");
  await page.getByRole("button", { name: "Status Workbench" }).click();
  await expect(page.getByText("Live generation 8")).toBeVisible();
  const imputedState = page.getByRole("region", { name: "Imputed State" });
  const retainedProjection = imputedState.getByText("Retained generation projection marker", { exact: true });
  await expect(retainedProjection).toBeVisible();
  const selectedUnit = page.locator('button[data-unit-id="reactor-01"]');
  await selectedUnit.click();
  await expect(selectedUnit).toHaveAttribute("aria-pressed", "true");
  const simulatedState = page.getByRole("region", { name: "Simulated Result State" });
  const selectedForecast = simulatedState.getByRole("button", { name: /Forecast margin/ });
  await selectedForecast.click();
  await expect(selectedForecast).toHaveAttribute("aria-pressed", "true");
  await expect(selectedUnit).toHaveAttribute("aria-pressed", "true");
  const health = page.getByRole("region", { name: "Simulation health cards" });
  await expect(health.getByText("0/1 complete", { exact: true })).toBeVisible();
  await expect(health.getByText("committed", { exact: true })).toBeVisible();

  unavailable = true;
  await page.getByRole("button", { name: "Refresh live Snapshot" }).click();
  await expect(page.getByText("Stale live generation 8")).toBeVisible();
  await expect(page.getByText(/Retaining live generation 8 as stale/)).toBeVisible();
  await expect(health.getByText("0/1 complete", { exact: true })).toBeVisible();
  await expect(health.getByText("committed", { exact: true })).toBeVisible();
  await expect(retainedProjection).toBeVisible();
  await expect(selectedForecast).toHaveAttribute("aria-pressed", "true");

  unavailable = false;
  generation = 7;
  await page.getByRole("button", { name: "Refresh live Snapshot" }).click();
  await expect(page.getByText("Stale live generation 8")).toBeVisible();
  await expect(page.getByText(/generation regressed from 8 to 7/)).toBeVisible();

  generation = 8;
  await page.getByRole("button", { name: "Refresh live Snapshot" }).click();
  await expect(page.getByText("Live generation 8")).toBeVisible();
  await expect(page.getByText(/generation 8 accepted atomically/)).toBeVisible();

  generation = 9;
  await page.getByRole("button", { name: "Refresh live Snapshot" }).click();
  await expect(page.getByText("Live generation 9")).toBeVisible();
  await expect(health.getByText("2/2 complete", { exact: true })).toBeVisible();
  await expect(health.getByText("2/2 nominal", { exact: true })).toBeVisible();
  await expect(health.getByText("staged", { exact: true })).toBeVisible();
  await expect(imputedState.getByText("Recovered generation projection marker", { exact: true })).toBeVisible();
  await expect(retainedProjection).not.toBeVisible();
  await expect(health.getByText("0/1 complete", { exact: true })).not.toBeVisible();
  await expect(health.getByText("committed", { exact: true })).not.toBeVisible();
  await expect(simulatedState.getByRole("button", { name: /Forecast margin/ })).toHaveAttribute("aria-pressed", "true");
  await expect(selectedUnit).toHaveAttribute("aria-pressed", "true");
});

test("Workbench replacement reconciles a selection removed by the new Snapshot", async ({ page }) => {
  let generation = 10;
  let reactorId = "reactor-01";
  await page.route("**/api/simulator-workbench/snapshot", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(liveSnapshot(generation, reactorId))
    });
  });

  await page.goto("/");
  await page.getByRole("button", { name: "Status Workbench" }).click();
  await expect(page.getByRole("heading", { name: "reactor-01" })).toBeVisible();
  await page.getByRole("region", { name: "Simulated Result State" })
    .getByRole("button", { name: /Forecast margin/ }).click();

  generation = 11;
  reactorId = "reactor-02";
  await page.getByRole("button", { name: "Refresh live Snapshot" }).click();

  await expect(page.getByText("Live generation 11")).toBeVisible();
  await expect(page.getByRole("heading", { name: "reactor-02" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "reactor-01" })).not.toBeVisible();
  await expect(page.getByRole("region", { name: "Imputed State" })
    .getByRole("button", { name: /Recovered generation projection marker/ })).toHaveAttribute("aria-pressed", "true");
});

test("Workbench unmount cancels its pending Snapshot request", async ({ page }) => {
  await page.goto("/tests/e2e/fixtures/workbench-session-unmount.html");
  await expect(page.locator("body")).toHaveAttribute("data-request-started", "true");

  await page.getByRole("button", { name: "Unmount Workbench" }).click();

  await expect(page.getByText("Workbench session unmounted")).toBeVisible();
  await expect(page.locator("body")).toHaveAttribute("data-request-aborted", "true");
});

function liveSnapshot(generation = 4, reactorId = "reactor-01") {
  const imputedValueId = `margin-imputed:${reactorId}`;
  const simulatedValueId = `margin-simulated:${reactorId}`;
  const imputedLineageId = `lin-imputed:${reactorId}`;
  const simulatedLineageId = `lin-simulated:${reactorId}`;
  return {
    generation,
    state: {
      schemaVersion: "simulator-workbench.state.v1",
      generatedAt: generation >= 9 ? "2026-07-18T12:00:25Z" : "2026-07-18T11:59:55Z",
      snapshotGeneration: generation,
      scenarioId: "scheduler-drift",
      valueBasisSummary: { measured: 1, imputed: 1, simulated: 1 },
      measuredStateRefs: ["scada_measured_frames"],
      twinStateRef: "digital_twin_state_values",
      lineageRefs: ["digital_twin_lineage"],
      activeSimulationRuns: generation >= 9
        ? [
            { runId: "recovered-run-a", scenarioId: "recovered-scenario", lifecycle: "completed", valueBasis: "simulated", health: "nominal", artifactStatus: "staged" },
            { runId: "recovered-run-b", scenarioId: "recovered-scenario", lifecycle: "completed", valueBasis: "simulated", health: "nominal", artifactStatus: "staged" }
          ]
        : [{ runId: "run-1", scenarioId: "scheduler-drift", lifecycle: "streaming", valueBasis: "simulated", health: "nominal", artifactStatus: "committed" }],
      panels: [
        { panelId: "measured", title: "Measured State", valueBasis: "measured" },
        { panelId: "imputed", title: "Imputed State", valueBasis: "imputed" },
        { panelId: "simulated", title: "Simulated Result State", valueBasis: "simulated" }
      ]
    },
    measured: [{ schemaVersion: "scada.telemetry.v1", sourceId: "source-1", reactorId, tagId: "TAG-CORE", assetId: `${reactorId}-core`, signalKind: "flux", sampledAt: "2026-07-14T10:59:59Z", observedAt: "2026-07-14T11:00:00Z", sequence: 1, unit: "relative", value: { scalar: 0.81 }, quality: "good", valueBasis: "measured", syntheticStatus: "public-safe-standin" }],
    twin: {
      schemaVersion: "digital-twin.state.v1",
      twinId: "twin-live-1",
      asOf: "2026-07-14T11:00:00Z",
      entities: [{ entityId: reactorId, displayName: reactorId, values: [
        { valueId: imputedValueId, label: generation >= 9 ? "Recovered generation projection marker" : "Retained generation projection marker", valueBasis: "imputed", unit: "percent", value: { scalar: generation >= 9 ? 19 : 14 }, confidence: 0.7, freshness: { ageSec: 4, status: "fresh" }, lineageId: imputedLineageId, sourceIds: ["TAG-CORE"] },
        { valueId: simulatedValueId, label: "Forecast margin", valueBasis: "simulated", unit: "percent", value: { scalar: 16 }, confidence: 0.6, freshness: { ageSec: 3, status: "fresh" }, lineageId: simulatedLineageId, sourceIds: ["run-1"] }
      ] }]
    },
    lineage: [
      { schemaVersion: "digital-twin.lineage.v1", lineageId: imputedLineageId, valueId: imputedValueId, valueBasis: "imputed", inputs: [{ sourceKind: "scada-tag", sourceId: "TAG-CORE", valueBasis: "measured" }], processingSteps: ["project"], artifacts: [] },
      { schemaVersion: "digital-twin.lineage.v1", lineageId: simulatedLineageId, valueId: simulatedValueId, valueBasis: "simulated", inputs: [{ sourceKind: "simulation-run", sourceId: "run-1", valueBasis: "simulated" }], processingSteps: ["project"], artifacts: [] }
    ],
    results: []
  };
}
