import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { buildWorkbenchProjection, loadFixtureWorkbenchData } from "../../domain/simulator-workbench";
import { FleetBoardSurface } from "./FleetBoardSurface";

describe("FleetBoardSurface simulation capacity", () => {
  it("presents the separate local-game budget, selection, cost, and accessible purchase intent", () => {
    const markup = renderToStaticMarkup(
      <FleetBoardSurface projection={buildWorkbenchProjection(loadFixtureWorkbenchData())} />
    );

    expect(markup).toContain("6 Simulation Budget");
    expect(markup).toContain("No reactor selected");
    expect(markup).toContain("Buy Simulation Container Token (2 budget)");
    expect(markup).toContain("Local game state only");
    expect(markup).toContain("Choose reactor for local simulation capacity");
    expect(markup).toContain("Select a reactor to inspect its Reactor Slot Rail");
    expect(markup).toContain("disabled");
  });
});
