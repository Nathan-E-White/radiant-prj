import { expect, test, type Locator } from "@playwright/test";
import { canvasHasNonBlankPixels } from "./fleet-board-canvas-helpers";

test("player queues one local Simulation Job and earns a reactor-scoped Insight Token", async ({ page }) => {
  await page.goto("/tests/e2e/fixtures/fleet-board-capacity.html");

  const reactorSelector = page.getByLabel("Choose reactor for local simulation capacity");
  const buyButton = page.getByRole("button", { name: "Buy Simulation Container Token (2 budget)" });
  const queueButton = page.getByRole("button", { name: "Queue local Simulation Job" });
  const tickButton = page.getByRole("button", { name: "Tick Day" });
  const canvas = page.locator('[data-testid="fleet-board-canvas"] canvas');
  await expect(canvas).toBeVisible();
  await expect.poll(() => canvasHasNonBlankPixels(canvas), { timeout: 15_000 }).toBe(true);

  await reactorSelector.selectOption("reactor-2");
  await queueButton.click();
  await expect(page.getByText("no idle Simulation Container Token", { exact: false })).toBeVisible();

  await buyButton.click();
  await expect(page.getByTestId("fleet-board-selected-reactor-jobs")).toContainText("1 idle");
  const idleRail = await sampleReactorSimulation(canvas);
  await queueButton.click();
  await expect(page.getByTestId("fleet-board-simulation-jobs")).toHaveText(
    "1 queued · 0 running · 0 completed · 0 Insight Tokens"
  );
  await expect(page.getByTestId("fleet-board-selected-reactor-jobs")).toContainText("1 queued");
  await expect.poll(() => sampleReactorSimulation(canvas)).not.toBe(idleRail);
  const queuedRail = await sampleReactorSimulation(canvas);

  await tickButton.click();
  await expect(page.getByTestId("fleet-board-selected-reactor-jobs")).toContainText(
    "1 running · 2 advances remaining"
  );
  await expect.poll(() => sampleReactorSimulation(canvas)).not.toBe(queuedRail);
  const runningTwoRail = await sampleReactorSimulation(canvas);
  await tickButton.click();
  await expect(page.getByTestId("fleet-board-selected-reactor-jobs")).toContainText(
    "1 running · 1 advance remaining"
  );
  await expect.poll(() => sampleReactorSimulation(canvas)).not.toBe(runningTwoRail);
  await tickButton.click();

  await expect(page.getByTestId("fleet-board-simulation-jobs")).toHaveText(
    "0 queued · 0 running · 1 completed · 1 Insight Token"
  );
  await expect(page.getByTestId("fleet-board-selected-reactor-jobs")).toContainText("1 idle · 1 Insight Token");
  await expect.poll(() => sampleReactorSimulation(canvas)).not.toBe(idleRail);
  await expect(page.getByText("simulationJobCompleted")).toBeVisible();
  await expect(
    page.locator('[aria-label="Fleet Board event log"]').getByText("produced a reactor-scoped Insight Token", {
      exact: false
    })
  ).toBeVisible();
});

async function sampleReactorSimulation(canvas: Locator) {
  return canvas.evaluate((element: HTMLCanvasElement) => {
    const context = element.getContext("2d");
    if (!context) {
      throw new Error("Fleet Board canvas has no 2D context");
    }
    const pixels = context.getImageData(368, 134, 90, 42).data;
    let hash = 2166136261;
    for (const value of pixels) {
      hash ^= value;
      hash = Math.imul(hash, 16777619);
    }
    return hash >>> 0;
  });
}
