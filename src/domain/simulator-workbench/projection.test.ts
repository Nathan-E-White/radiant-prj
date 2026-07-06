import { describe, expect, it } from "vitest";
import { loadFixtureWorkbenchData } from "./fixtureAdapter";
import { buildWorkbenchProjection, type WorkbenchProjectionInput } from "./projection";

describe("simulator workbench projection", () => {
  it("groups measured, imputed, and simulated values without flattening them into metrics", () => {
    const projection = buildWorkbenchProjection(loadFixtureWorkbenchData());

    expect(Object.keys(projection.groups)).toEqual(["measured", "imputed", "simulated"]);
    expect(projection.groups.measured.values).toHaveLength(3);
    expect(projection.groups.imputed.values).toHaveLength(2);
    expect(projection.groups.simulated.values).toHaveLength(1);
    expect(projection.valueBasisSummary).toEqual({ measured: 3, imputed: 2, simulated: 1 });
    expect(projection.groups.measured.values.every((value) => value.valueBasis === "measured")).toBe(true);
    expect(projection.groups.imputed.values.every((value) => value.valueBasis === "imputed")).toBe(true);
    expect(projection.groups.simulated.values.every((value) => value.valueBasis === "simulated")).toBe(true);
  });

  it("selects a lineage-backed value and builds readable lineage steps", () => {
    const projection = buildWorkbenchProjection(loadFixtureWorkbenchData(), "VAL-IMPUTED-CORE-MARGIN");

    expect(projection.selectedValue?.valueId).toBe("VAL-IMPUTED-CORE-MARGIN");
    expect(projection.selectedLineage?.lineageId).toBe("LIN-CORE-MARGIN");
    expect(projection.selectedLineageMissing).toBe(false);
    expect(projection.lineageSteps.map((step) => step.basis)).toEqual(
      expect.arrayContaining(["measured", "imputed", "simulated"])
    );
  });

  it("falls back to the default selected value when the requested value is missing", () => {
    const projection = buildWorkbenchProjection(loadFixtureWorkbenchData(), "DOES-NOT-EXIST");

    expect(projection.selectedValue?.valueId).toBe(projection.defaultSelectedValueId);
    expect(projection.selectedValue?.valueBasis).toBe("imputed");
  });

  it("marks selected values that do not have lineage yet", () => {
    const input = withoutLineages(loadFixtureWorkbenchData());
    const projection = buildWorkbenchProjection(input, "VAL-MEASURED-FLUX-A");

    expect(projection.selectedValue?.valueId).toBe("VAL-MEASURED-FLUX-A");
    expect(projection.selectedLineage).toBeNull();
    expect(projection.selectedLineageMissing).toBe(true);
    expect(projection.lineageSteps).toEqual([]);
  });

  it("keeps simulation health summary compact and excludes detailed infrastructure state", () => {
    const projection = buildWorkbenchProjection(loadFixtureWorkbenchData());

    expect(projection.healthSummary.scope).toBe("summary");
    expect(projection.healthSummary).toMatchObject({
      status: "complete",
      label: "Complete",
      runCount: 1,
      completedRuns: 1
    });
    expect(Object.keys(projection.healthSummary)).not.toEqual(
      expect.arrayContaining(["workers", "redpandaOffsets", "timescaleProjection", "icebergReadback", "streamTracks"])
    );
  });
});

function withoutLineages(input: WorkbenchProjectionInput): WorkbenchProjectionInput {
  return {
    ...input,
    lineages: []
  };
}
