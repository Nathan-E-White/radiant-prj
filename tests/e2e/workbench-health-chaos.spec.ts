import { expect, test } from "@playwright/test";

test("React Chaos health-panel fault is contained inside the Workbench", async ({ page }) => {
  await page.goto("/tests/e2e/fixtures/workbench-health-chaos.html");

  await expect(page.getByRole("heading", { name: "Workbench shell remains available" })).toBeVisible();
  await expect(page.getByRole("status", { name: "Simulation Health unavailable" })).toContainText(
    "Simulation Health temporarily unavailable"
  );
  await expect(page.getByRole("region", { name: "Simulation health cards" })).not.toBeVisible();
});
