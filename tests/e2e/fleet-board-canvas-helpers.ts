import type { Locator } from "@playwright/test";

export function canvasHasNonBlankPixels(canvas: Locator) {
  return canvas.evaluate((element: HTMLCanvasElement) => {
    const context = element.getContext("2d");
    if (!context) {
      return false;
    }
    return Array.from(context.getImageData(0, 0, 120, 120).data).some((channel) => channel !== 0);
  });
}
