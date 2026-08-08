import { expect, test } from "@playwright/test";
import { canvasHasNonBlankPixels } from "./fleet-board-canvas-helpers";

test("contributors inspect stable board scenes with the navigator beside the render", async ({ page }) => {
  await page.goto("/tests/e2e/fixtures/board-scene-workbench.html");

  await expect(page.getByRole("region", { name: "Board Scene Workbench" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Queued Simulation Job" })).toBeVisible();
  await page.getByRole("button", { name: "Queued Simulation Job" }).click();

  await expect(page.getByTestId("board-scene-seed")).toHaveValue("job-queued");
  await expect(page.getByTestId("board-scene-day")).toHaveValue("0");
  await expect(page.getByTestId("board-scene-selected-reactor")).toHaveValue("reactor-2");
  await expect(page.getByText("Simulation Job queued")).toBeVisible();
  await expect(page.getByRole("region", { name: "Board Navigator" })).toContainText("Reactor Slot Rail");
  await expect(page.getByRole("region", { name: "Board Navigator" })).toContainText("Routes");
  await expect(page.getByRole("region", { name: "Board Navigator" })).toContainText("Reactor -> TRISO Supply");
  await expect(page.getByRole("region", { name: "Asset atlas" })).toContainText("reactor-slot-rail-queued");

  const canvas = page.locator('[data-testid="board-scene-canvas"] canvas');
  await expect(canvas).toBeVisible();
  await expect.poll(() => canvasHasNonBlankPixels(canvas), { timeout: 15_000 }).toBe(true);
  await expect(canvas).toHaveScreenshot("board-scene-workbench-job-queued.png");

  await page.getByTestId("board-scene-seed").fill("custom-seed-42");
  await page.getByTestId("board-scene-day").fill("2");
  await page.getByRole("button", { name: "compact" }).click();
  await page.getByTestId("board-scene-reduced-motion").uncheck();
  await page.getByTestId("board-scene-selected-reactor").selectOption("");
  await expect(page.getByTestId("board-scene-workbench")).toHaveClass(/board-scene-density-compact/);
  await expect(page.getByTestId("board-scene-seed")).toHaveValue("custom-seed-42");
  await expect(page.getByTestId("board-scene-day")).toHaveValue("2");
  await expect(page.getByTestId("board-scene-selected-reactor")).toHaveValue("");
  await expect(page.getByTestId("board-scene-summary")).toContainText("running");
  await expect.poll(() => canvasHasNonBlankPixels(canvas), { timeout: 15_000 }).toBe(true);

  await page.getByTestId("board-scene-selected-reactor").selectOption("reactor-2");
  await page.getByTestId("board-scene-reduced-motion").check();
  await expect(page.getByTestId("board-scene-selected-reactor")).toHaveValue("reactor-2");

  await page.getByRole("button", { name: "Terminal removed" }).click();
  await expect(page.getByTestId("board-scene-day")).toHaveValue("3");
  await expect(page.getByTestId("board-scene-summary")).toContainText("removed");
});
