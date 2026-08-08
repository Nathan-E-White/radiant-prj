import { expect, test, type Locator } from "@playwright/test";
import { canvasHasNonBlankPixels } from "./fleet-board-canvas-helpers";

test("mounted Fleet Board keeps selection, drag, camera, and assets across scene updates", async ({ page }) => {
  await page.goto("/tests/e2e/fixtures/fleet-board-runtime.html");

  const canvas = page.locator('[data-testid="fleet-board-canvas"] canvas');
  await expect(canvas).toBeVisible();
  await expect.poll(() => canvasHasNonBlankPixels(canvas), { timeout: 15_000 }).toBe(true);
  await canvas.evaluate((element) => {
    element.dataset.runtimeInstance = "original";
  });
  const box = await canvas.boundingBox();
  expect(box).not.toBeNull();
  if (!box) {
    throw new Error("Fleet Board runtime harness has no canvas bounds");
  }

  const reactorCenter = canvasPoint(box, 424, 196);
  await expect
    .poll(
      async () => {
        await page.mouse.click(reactorCenter.x, reactorCenter.y);
        return page.getByTestId("harness-selection").textContent();
      },
      { timeout: 10_000 }
    )
    .toBe("Selected reactor-1");

  const dragStart = canvasPoint(box, 114, 549);
  const dragMid = canvasPoint(box, 360, 430);
  const dragEnd = canvasPoint(box, 560, 400);
  await page.mouse.move(dragStart.x, dragStart.y);
  await page.mouse.down();
  await page.mouse.move(dragMid.x, dragMid.y, { steps: 4 });
  await page.evaluate(() => window.advanceFleetBoardScene());
  await expect(page.getByTestId("harness-day")).toHaveText("Day 1");
  await page.mouse.move(dragEnd.x, dragEnd.y, { steps: 4 });
  await page.mouse.up();

  await expect(page.getByTestId("harness-placements")).toHaveText("Placements 1");
  await expect(page.getByTestId("harness-selection")).toHaveText("Selected reactor-1");
  await expect(canvas).toHaveAttribute("data-runtime-instance", "original");

  const panStart = canvasPoint(box, 760, 500);
  const panEnd = canvasPoint(box, 680, 450);
  await page.mouse.move(panStart.x, panStart.y);
  await page.mouse.down();
  await page.mouse.move(panEnd.x, panEnd.y, { steps: 4 });
  await page.mouse.up();
  await page.mouse.wheel(0, -120);
  const cameraRegionBefore = await sampleCameraRegion(canvas);

  await page.evaluate(() => window.advanceFleetBoardScene());
  await expect(page.getByTestId("harness-day")).toHaveText("Day 2");
  expect(await sampleCameraRegion(canvas)).toEqual(cameraRegionBefore);
  await expect(canvas).toHaveAttribute("data-runtime-instance", "original");
});

function canvasPoint(box: NonNullable<Awaited<ReturnType<Locator["boundingBox"]>>>, x: number, y: number) {
  return {
    x: box.x + (x / 980) * box.width,
    y: box.y + (y / 640) * box.height
  };
}

async function sampleCameraRegion(canvas: Locator) {
  return canvas.evaluate((element: HTMLCanvasElement) => {
    const context = element.getContext("2d");
    if (!context) {
      throw new Error("Fleet Board canvas has no 2D context");
    }
    return Array.from(context.getImageData(8, 8, 80, 80).data);
  });
}
