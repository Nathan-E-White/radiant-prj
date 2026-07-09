import { expect, test, type Page } from "@playwright/test";

test("Fleet Board is the default Simulator Workbench experience and accepts board interaction", async ({ page }) => {
  await page.goto("/");

  await expect(page.getByRole("heading", { name: /Kaleidos/i })).toBeVisible();
  await expect(page.getByRole("button", { name: "Simulator Workbench" })).toBeVisible();

  await page.getByRole("button", { name: "Simulator Workbench" }).click();

  await expect(page.getByRole("region", { name: "Fleet Board" })).toBeVisible();
  await expect(page.getByText("30-day contract sprint")).toBeVisible();
  await expect(page.getByText("Measured State")).toBeVisible();
  await expect(page.getByText("Imputed State")).toBeVisible();
  await expect(page.getByText("Simulated Result State")).toBeVisible();

  const canvas = page.locator('[data-testid="fleet-board-canvas"] canvas');
  await expect(canvas).toBeVisible();
  await expect.poll(() => canvasHasNonBlankPixels(page), { timeout: 15_000 }).toBe(true);

  await expect(page.getByTestId("fleet-board-facility-count")).toContainText("4 facilities");
  const box = await canvas.boundingBox();
  expect(box).not.toBeNull();
  if (!box) {
    throw new Error("Fleet Board canvas has no bounding box");
  }

  await page.mouse.move(box.x + 112, box.y + 548);
  await page.mouse.down();
  await page.mouse.move(box.x + 780, box.y + 470, { steps: 8 });
  await page.mouse.up();
  await expect(page.getByTestId("fleet-board-facility-count")).toContainText("5 facilities");

  await page.getByRole("button", { name: /Tick Day/i }).click();
  await expect(page.getByText("Day 1/30")).toBeVisible();
  await expect(page.getByText("reactorGenerated").first()).toBeVisible();

  await expect(page.getByRole("button", { name: "Compute Workbench" })).toBeVisible();
  await expect(page.getByRole("button", { name: "SimOps Control" })).toBeVisible();
});

async function canvasHasNonBlankPixels(page: Page) {
  return page.locator('[data-testid="fleet-board-canvas"] canvas').evaluate((canvas: HTMLCanvasElement) => {
    const context = canvas.getContext("2d");
    if (!context || canvas.width === 0 || canvas.height === 0) {
      return false;
    }
    const sample = context.getImageData(0, 0, canvas.width, canvas.height).data;
    for (let index = 0; index < sample.length; index += 4) {
      if (sample[index] !== 0 || sample[index + 1] !== 0 || sample[index + 2] !== 0 || sample[index + 3] !== 0) {
        return true;
      }
    }
    return false;
  });
}
