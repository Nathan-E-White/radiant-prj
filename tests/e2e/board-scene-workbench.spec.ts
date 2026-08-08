import { expect, test } from "@playwright/test";
import { canvasHasNonBlankPixels } from "./fleet-board-canvas-helpers";

test("contributors inspect stable board scenes with the navigator beside the render", async ({ page }) => {
  await page.goto("/tests/e2e/fixtures/board-scene-workbench.html");

  await expect(page.getByRole("region", { name: "Board Scene Workbench" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Queued Simulation Job" })).toBeVisible();
  await page.getByRole("button", { name: "Queued Simulation Job" }).click();

  await expect(page.getByTestId("board-scene-seed")).toHaveText("job-queued");
  await expect(page.getByTestId("board-scene-day")).toHaveText("Day 0");
  await expect(page.getByTestId("board-scene-selected-reactor")).toHaveText("reactor-2");
  await expect(page.getByText("Simulation Job queued")).toBeVisible();
  await expect(page.getByRole("region", { name: "Board Navigator" })).toContainText("Reactor Slot Rail");
  await expect(page.getByRole("region", { name: "Asset atlas" })).toContainText("reactor-slot-rail-queued");

  const canvas = page.locator('[data-testid="board-scene-canvas"] canvas');
  await expect(canvas).toBeVisible();
  await expect.poll(() => canvasHasNonBlankPixels(canvas), { timeout: 15_000 }).toBe(true);
  await expect(canvas).toHaveScreenshot("board-scene-workbench-job-queued.png");

  await page.getByRole("button", { name: "Terminal removed" }).click();
  await expect(page.getByTestId("board-scene-day")).toHaveText("Day 3");
  await expect(page.getByTestId("board-scene-summary")).toContainText("removed");
});
