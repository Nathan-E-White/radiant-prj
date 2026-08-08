import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { buildWorkbenchProjection, loadFixtureWorkbenchData } from "../../domain/simulator-workbench";
import { BoardSceneWorkbench } from "./BoardSceneWorkbench";

describe("BoardSceneWorkbench", () => {
  it("renders scene controls, atlas inspection, navigator review, and prototype verdicts", () => {
    const markup = renderToStaticMarkup(
      <BoardSceneWorkbench projection={buildWorkbenchProjection(loadFixtureWorkbenchData())} />
    );

    expect(markup).toContain("Board Scene Workbench");
    expect(markup).toContain("Starter board");
    expect(markup).toContain("Queued Simulation Job");
    expect(markup).toContain("Terminal removed");
    expect(markup).toContain("Seed");
    expect(markup).toContain("Day");
    expect(markup).toContain("Selected reactor");
    expect(markup).toContain("Camera");
    expect(markup).toContain("Density");
    expect(markup).toContain("Reduced motion");
    expect(markup).toContain("Asset atlas");
    expect(markup).toContain("simulation-container-token");
    expect(markup).toContain("Board Navigator");
    expect(markup).toContain("Reactor Slot Rail");
    expect(markup).toContain("Fleet Board map mode");
    expect(markup).toContain("accepted");
    expect(markup).toContain("rejected");
    expect(markup).toContain("deferred");
  });
});
