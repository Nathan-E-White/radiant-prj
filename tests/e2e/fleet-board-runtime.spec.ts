import { expect, test, type Locator } from "@playwright/test";

test("mounted Fleet Board keeps selection, drag, camera, and assets across scene updates", async ({ page }) => {
  await page.goto("/tests/e2e/fixtures/fleet-board-runtime.html");

  const canvas = page.locator('[data-testid="fleet-board-canvas"] canvas');
  await expect(canvas).toBeVisible();
  await canvas.evaluate((element) => {
    element.dataset.runtimeInstance = "original";
  });
  const box = await canvas.boundingBox();
  expect(box).not.toBeNull();
  if (!box) {
    throw new Error("Fleet Board runtime harness has no canvas bounds");
  }

  await page.mouse.click(box.x + 424, box.y + 196);
  await expect(page.getByTestId("harness-selection")).toHaveText("Selected reactor-1");

  await page.mouse.move(box.x + 114, box.y + 549);
  await page.mouse.down();
  await page.mouse.move(box.x + 360, box.y + 430, { steps: 4 });
  await page.evaluate(() => window.advanceFleetBoardScene());
  await expect(page.getByTestId("harness-day")).toHaveText("Day 1");
  await page.mouse.move(box.x + 560, box.y + 400, { steps: 4 });
  await page.mouse.up();

  await expect(page.getByTestId("harness-placements")).toHaveText("Placements 1");
  await expect(page.getByTestId("harness-selection")).toHaveText("Selected reactor-1");
  await expect(canvas).toHaveAttribute("data-runtime-instance", "original");

  await page.mouse.move(box.x + 760, box.y + 500);
  await page.mouse.down();
  await page.mouse.move(box.x + 680, box.y + 450, { steps: 4 });
  await page.mouse.up();
  await page.mouse.wheel(0, -120);
  const cameraRegionBefore = await sampleCameraRegion(canvas);

  await page.evaluate(() => window.advanceFleetBoardScene());
  await expect(page.getByTestId("harness-day")).toHaveText("Day 2");
  expect(await sampleCameraRegion(canvas)).toEqual(cameraRegionBefore);
  await expect(canvas).toHaveAttribute("data-runtime-instance", "original");
});

async function sampleCameraRegion(canvas: Locator) {
  return canvas.evaluate((element: HTMLCanvasElement) => {
    const context = element.getContext("2d");
    if (!context) {
      throw new Error("Fleet Board canvas has no 2D context");
    }
    return Array.from(context.getImageData(8, 8, 80, 80).data);
  });
}
