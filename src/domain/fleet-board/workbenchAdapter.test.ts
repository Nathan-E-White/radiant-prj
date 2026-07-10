import { describe, expect, it } from "vitest";
import { buildWorkbenchProjection, loadFixtureWorkbenchData } from "../simulator-workbench";
import { buildFleetBoardWorkbenchModifiers } from "./workbenchAdapter";

describe("fleet board workbench adapter", () => {
  it("turns Workbench projection state into light gameplay modifiers while preserving Value Basis counts", () => {
    const projection = buildWorkbenchProjection(loadFixtureWorkbenchData(), { selectedUnitId: "KAL-02" });
    const modifiers = buildFleetBoardWorkbenchModifiers(projection);

    expect(modifiers.valueBasisCounts).toEqual({
      measured: projection.valueBasisSummary.measured,
      imputed: projection.valueBasisSummary.imputed,
      simulated: projection.valueBasisSummary.simulated
    });
    expect(modifiers.freshnessRisk).toBeGreaterThan(0);
    expect(modifiers.inspectorPressure).toBeGreaterThan(0);
    expect(modifiers.confidenceMultiplier).toBeGreaterThan(0);
    expect(modifiers.simulatedResultPressure).toBeGreaterThanOrEqual(0);
    expect(modifiers.selectedUnitId).toBe("KAL-02");
  });
});
