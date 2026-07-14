import { expect, test } from "@playwright/test";

test("player queues one local Simulation Job and earns a reactor-scoped Insight Token", async ({ page }) => {
  await page.goto("/tests/e2e/fixtures/fleet-board-capacity.html");

  const reactorSelector = page.getByLabel("Choose reactor for local simulation capacity");
  const buyButton = page.getByRole("button", { name: "Buy Simulation Container Token (2 budget)" });
  const queueButton = page.getByRole("button", { name: "Queue local Simulation Job" });
  const tickButton = page.getByRole("button", { name: "Tick Day" });

  await reactorSelector.selectOption("reactor-2");
  await queueButton.click();
  await expect(page.getByText("no idle Simulation Container Token", { exact: false })).toBeVisible();

  await buyButton.click();
  await expect(page.getByTestId("fleet-board-selected-reactor-jobs")).toContainText("1 idle");
  await queueButton.click();
  await expect(page.getByTestId("fleet-board-simulation-jobs")).toHaveText(
    "1 queued · 0 running · 0 completed · 0 Insight Tokens"
  );
  await expect(page.getByTestId("fleet-board-selected-reactor-jobs")).toContainText("1 queued");

  await tickButton.click();
  await expect(page.getByTestId("fleet-board-selected-reactor-jobs")).toContainText(
    "1 running · 2 advances remaining"
  );
  await tickButton.click();
  await expect(page.getByTestId("fleet-board-selected-reactor-jobs")).toContainText(
    "1 running · 1 advance remaining"
  );
  await tickButton.click();

  await expect(page.getByTestId("fleet-board-simulation-jobs")).toHaveText(
    "0 queued · 0 running · 1 completed · 1 Insight Token"
  );
  await expect(page.getByTestId("fleet-board-selected-reactor-jobs")).toContainText("1 idle · 1 Insight Token");
  await expect(page.getByText("simulationJobCompleted")).toBeVisible();
  await expect(
    page.locator('[aria-label="Fleet Board event log"]').getByText("produced a reactor-scoped Insight Token", {
      exact: false
    })
  ).toBeVisible();
});
