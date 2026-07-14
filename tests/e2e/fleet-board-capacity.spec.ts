import { expect, test, type Locator } from "@playwright/test";

test("player buys bounded reactor-scoped Simulation Container Tokens as local game state", async ({ page }) => {
  await page.goto("/tests/e2e/fixtures/fleet-board-capacity.html");

  const canvas = page.locator('[data-testid="fleet-board-canvas"] canvas');
  await expect(canvas).toBeVisible();
  await expect.poll(() => canvasHasNonBlankPixels(canvas), { timeout: 15_000 }).toBe(true);
  await canvas.evaluate((element) => {
    element.dataset.runtimeInstance = "capacity";
  });
  const buyButton = page.getByRole("button", { name: "Buy Simulation Container Token (2 budget)" });
  await expect(buyButton).toBeDisabled();
  await expect(page.getByTestId("fleet-board-simulation-budget")).toContainText("6 Simulation Budget");
  await expect(page.getByText("Local game state only", { exact: false })).toBeVisible();

  const box = await canvas.boundingBox();
  expect(box).not.toBeNull();
  if (!box) {
    throw new Error("Fleet Board capacity harness has no canvas bounds");
  }
  await page.mouse.click(box.x + 424, box.y + 196);
  await expect(page.getByTestId("fleet-board-selected-reactor")).toHaveText("Selected reactor-2");
  await expect(buyButton).toBeEnabled();
  const emptyRail = await sampleRail(canvas);

  await buyButton.click();
  await expect(page.getByTestId("fleet-board-simulation-budget")).toContainText("4 Simulation Budget");
  await expect(page.getByTestId("fleet-board-simulation-container-tokens")).toContainText("1 Simulation Container Token");
  await expect(page.getByText("simulationContainerPurchased")).toBeVisible();
  await expect(
    page.locator('[aria-label="Fleet Board event log"]').getByText("local game state only", { exact: false })
  ).toBeVisible();
  expect(await sampleRail(canvas)).not.toEqual(emptyRail);
  await expect(canvas).toHaveAttribute("data-runtime-instance", "capacity");

  await buyButton.click();
  await expect(page.getByTestId("fleet-board-simulation-budget")).toContainText("2 Simulation Budget");
  await expect(page.getByTestId("fleet-board-simulation-container-tokens")).toContainText("2 Simulation Container Tokens");

  await buyButton.click();
  await expect(page.getByTestId("fleet-board-simulation-budget")).toContainText("2 Simulation Budget");
  await expect(page.getByTestId("fleet-board-simulation-container-tokens")).toContainText("2 Simulation Container Tokens");
  await expect(page.getByText("Reactor Slot Rail is full", { exact: false })).toBeVisible();
});

async function canvasHasNonBlankPixels(canvas: Locator) {
  return canvas.evaluate((element: HTMLCanvasElement) => {
    const context = element.getContext("2d");
    if (!context) {
      return false;
    }
    return Array.from(context.getImageData(0, 0, 120, 120).data).some((channel) => channel !== 0);
  });
}

async function sampleRail(canvas: Locator) {
  return canvas.evaluate((element: HTMLCanvasElement) => {
    const context = element.getContext("2d");
    if (!context) {
      throw new Error("Fleet Board canvas has no 2D context");
    }
    return Array.from(context.getImageData(390, 135, 70, 38).data);
  });
}
