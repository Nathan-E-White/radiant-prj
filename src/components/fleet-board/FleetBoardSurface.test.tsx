import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { buildWorkbenchProjection, loadFixtureWorkbenchData } from "../../domain/simulator-workbench";
import { FleetBoardSurface } from "./FleetBoardSurface";

describe("FleetBoardSurface local simulation economy", () => {
  it("presents accessible capacity and job intents plus local-only lifecycle summaries", () => {
    const markup = renderToStaticMarkup(
      <FleetBoardSurface projection={buildWorkbenchProjection(loadFixtureWorkbenchData())} />
    );

    expect(markup).toContain("6 Simulation Budget");
    expect(markup).toContain("No reactor selected");
    expect(markup).toContain("Buy Simulation Container Token (2 budget)");
    expect(markup).toContain("Local game state only");
    expect(markup).toContain("Choose reactor for local simulation capacity");
    expect(markup).toContain("Select a reactor to inspect its Reactor Slot Rail");
    expect(markup).toContain("Queue local Simulation Job");
    expect(markup).toContain("0 queued · 0 running · 0 completed · 0 Insight Tokens");
    expect(markup).toContain("not a SimOps Run or backend artifact");
    expect(markup).toContain("disabled");
  });
});
