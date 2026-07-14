import { expect, test } from "@playwright/test";

test("reactor Insight Tokens absorb Trouble and Inspector pressure but not refueling", async ({ page }) => {
  await page.goto("/tests/e2e/fixtures/fleet-board-pressure.html");

  await page.getByLabel("Choose reactor for local simulation capacity").selectOption("reactor-2");
  const buyButton = page.getByRole("button", { name: "Buy Simulation Container Token (2 budget)" });
  const queueButton = page.getByRole("button", { name: "Queue local Simulation Job" });
  const tickButton = page.getByRole("button", { name: "Tick Day" });
  const refuelButton = page.getByRole("button", { name: "Refuel Reactor" });

  await buyButton.click();
  await buyButton.click();
  await queueButton.click();
  await queueButton.click();
  await tickButton.click();
  await tickButton.click();
  await tickButton.click();
  await expect(page.getByTestId("fleet-board-simulation-jobs")).toHaveText(
    "0 queued · 0 running · 2 completed · 2 Insight Tokens"
  );

  await queueButton.click();
  await tickButton.click();
  await tickButton.click();
  await expect(page.getByText("Trouble pressure absorbed", { exact: false })).toBeVisible();
  await expect(page.getByTestId("fleet-board-selected-reactor-jobs")).toContainText("1 Insight Token");

  await tickButton.click();
  await expect(page.getByText("Inspector pressure absorbed", { exact: false })).toBeVisible();
  await expect(page.getByTestId("fleet-board-simulation-jobs")).toHaveText(
    "0 queued · 0 running · 3 completed · 1 Insight Token"
  );

  for (let click = 0; click < 30; click += 1) {
    await refuelButton.click();
  }
  await tickButton.click();
  await expect(page.getByText("refuelingNeeded")).toBeVisible();
  await expect(page.getByTestId("fleet-board-selected-reactor-jobs")).toContainText("1 Insight Token");
});
