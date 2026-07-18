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

  unavailable = true;
  await page.getByRole("button", { name: "Refresh live Snapshot" }).click();
  await expect(page.getByText("Stale live generation 8")).toBeVisible();
  await expect(page.getByText(/Retaining live generation 8 as stale/)).toBeVisible();

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
});

test("Workbench unmount cancels its pending Snapshot request", async ({ page }) => {
  await page.goto("/tests/e2e/fixtures/workbench-session-unmount.html");
  await expect(page.locator("body")).toHaveAttribute("data-request-started", "true");

  await page.getByRole("button", { name: "Unmount Workbench" }).click();

  await expect(page.getByText("Workbench session unmounted")).toBeVisible();
  await expect(page.locator("body")).toHaveAttribute("data-request-aborted", "true");
});

function liveSnapshot(generation = 4) {
  return {
    generation,
    state: {
      schemaVersion: "simulator-workbench.state.v1",
      generatedAt: "2026-07-14T11:00:00Z",
      snapshotGeneration: generation,
      scenarioId: "scheduler-drift",
      valueBasisSummary: { measured: 1, imputed: 1, simulated: 1 },
      measuredStateRefs: ["scada_measured_frames"],
      twinStateRef: "digital_twin_state_values",
      lineageRefs: ["digital_twin_lineage"],
      activeSimulationRuns: [{ runId: "run-1", scenarioId: "scheduler-drift", lifecycle: "streaming", valueBasis: "simulated", health: "nominal", artifactStatus: "committed" }],
      panels: [
        { panelId: "measured", title: "Measured State", valueBasis: "measured" },
        { panelId: "imputed", title: "Imputed State", valueBasis: "imputed" },
        { panelId: "simulated", title: "Simulated Result State", valueBasis: "simulated" }
      ]
    },
    measured: [{ schemaVersion: "scada.telemetry.v1", sourceId: "source-1", reactorId: "reactor-01", tagId: "TAG-CORE", assetId: "reactor-01-core", signalKind: "flux", sampledAt: "2026-07-14T10:59:59Z", observedAt: "2026-07-14T11:00:00Z", sequence: 1, unit: "relative", value: { scalar: 0.81 }, quality: "good", valueBasis: "measured", syntheticStatus: "public-safe-standin" }],
    twin: {
      schemaVersion: "digital-twin.state.v1",
      twinId: "twin-live-1",
      asOf: "2026-07-14T11:00:00Z",
      entities: [{ entityId: "reactor-01", displayName: "Reactor 01", values: [
        { valueId: "margin-imputed", label: "Core margin", valueBasis: "imputed", unit: "percent", value: { scalar: 14 }, confidence: 0.7, freshness: { ageSec: 4, status: "fresh" }, lineageId: "lin-imputed", sourceIds: ["TAG-CORE"] },
        { valueId: "margin-simulated", label: "Forecast margin", valueBasis: "simulated", unit: "percent", value: { scalar: 16 }, confidence: 0.6, freshness: { ageSec: 3, status: "fresh" }, lineageId: "lin-simulated", sourceIds: ["run-1"] }
      ] }]
    },
    lineage: [
      { schemaVersion: "digital-twin.lineage.v1", lineageId: "lin-imputed", valueId: "margin-imputed", valueBasis: "imputed", inputs: [{ sourceKind: "scada-tag", sourceId: "TAG-CORE", valueBasis: "measured" }], processingSteps: ["project"], artifacts: [] },
      { schemaVersion: "digital-twin.lineage.v1", lineageId: "lin-simulated", valueId: "margin-simulated", valueBasis: "simulated", inputs: [{ sourceKind: "simulation-run", sourceId: "run-1", valueBasis: "simulated" }], processingSteps: ["project"], artifacts: [] }
    ],
    results: []
  };
}
