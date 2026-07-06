import { describe, expect, it } from "vitest";
import { loadFixtureWorkbenchData } from "./fixtureAdapter";
import { buildWorkbenchProjection, type WorkbenchProjectionInput } from "./projection";

describe("simulator workbench projection", () => {
  it("groups measured, imputed, and simulated values without flattening them into metrics", () => {
    const projection = buildWorkbenchProjection(loadFixtureWorkbenchData());

    expect(Object.keys(projection.groups)).toEqual(["measured", "imputed", "simulated"]);
    expect(projection.groups.measured.values.length).toBeGreaterThan(4);
    expect(projection.groups.imputed.values.length).toBeGreaterThan(4);
    expect(projection.groups.simulated.values.length).toBeGreaterThanOrEqual(1);
    expect(projection.valueBasisSummary).toEqual({ measured: 7, imputed: 7, simulated: 2 });
    expect(projection.groups.measured.values.every((value) => value.valueBasis === "measured")).toBe(true);
    expect(projection.groups.imputed.values.every((value) => value.valueBasis === "imputed")).toBe(true);
    expect(projection.groups.simulated.values.every((value) => value.valueBasis === "simulated")).toBe(true);
  });

  it("selects a lineage-backed value and builds readable lineage steps", () => {
    const projection = buildWorkbenchProjection(loadFixtureWorkbenchData(), {
      selectedValueId: "VAL-KAL-01-IMPUTED-CORE-MARGIN"
    });

    expect(projection.selectedValue?.valueId).toBe("VAL-KAL-01-IMPUTED-CORE-MARGIN");
    expect(projection.selectedLineage?.lineageId).toBe("LIN-KAL-01-CORE-MARGIN");
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
    const projection = buildWorkbenchProjection(input, { selectedValueId: "VAL-KAL-01-MEASURED-FLUX-LOW" });

    expect(projection.selectedValue?.valueId).toBe("VAL-KAL-01-MEASURED-FLUX-LOW");
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
      runCount: 2,
      completedRuns: 2
    });
    expect(Object.keys(projection.healthSummary)).not.toEqual(
      expect.arrayContaining(["workers", "redpandaOffsets", "timescaleProjection", "icebergReadback", "streamTracks"])
    );
  });

  it("selects a fleet unit and scopes panels, viewport, and values to that Kaleidos Unit", () => {
    const projection = buildWorkbenchProjection(loadFixtureWorkbenchData(), { selectedUnitId: "KAL-03" });

    expect(projection.selectedUnit.unitId).toBe("KAL-03");
    expect(projection.fleetUnits.find((unit) => unit.unitId === "KAL-03")?.selected).toBe(true);
    expect(projection.groups.measured.values.every((value) => value.unitId === "KAL-03")).toBe(true);
    expect(projection.groups.imputed.values.every((value) => value.unitId === "KAL-03")).toBe(true);
    expect(projection.viewport.layers.some((layer) => layer.entityId === "secondaryHeatUse")).toBe(true);
    expect(projection.viewport.layers.every((layer) => layer.unitId === "KAL-03")).toBe(true);
  });

  it("builds compact fleet cards with degraded freshness only when the source is late or stale", () => {
    const projection = buildWorkbenchProjection(loadFixtureWorkbenchData());

    expect(projection.fleetUnits.map((unit) => unit.unitId)).toEqual(["KAL-01", "KAL-02", "KAL-03", "KAL-04", "KAL-05"]);
    expect(projection.fleetUnits.find((unit) => unit.unitId === "KAL-01")).toMatchObject({
      phaseLine: "online generation | B2B 184d",
      outputLine: "0.94 MWe | 2.6 MWt",
      accruedDisplayLabel: "$18.4k (est)",
      freshnessWarningLabel: null
    });
    expect(projection.fleetUnits.find((unit) => unit.unitId === "KAL-02")?.freshnessWarningLabel).toBe("late 4m");
    expect(projection.fleetUnits.find((unit) => unit.unitId === "KAL-04")).toMatchObject({
      outputLine: "0.18 MWth residual heat",
      accruedDisplayLabel: "no commercial output"
    });
  });

  it("uses commercial display basis in the explanation rail when a fleet value is selected", () => {
    const projection = buildWorkbenchProjection(loadFixtureWorkbenchData(), {
      selectedUnitId: "KAL-03",
      selectedCommercialBasisId: "CDB-KAL-03-DESALINATION"
    });

    expect(projection.explanation.kind).toBe("commercial");
    expect(projection.explanation.title).toBe("Accrued Display Value");
    expect(projection.explanation.items).toEqual(
      expect.arrayContaining([
        { label: "Commercial mode", value: "desalination heat" },
        { label: "Delivered heat", value: "18.7 MWhth" },
        { label: "Rate assumption", value: "display-only desalination heat service estimate" }
      ])
    );
    expect(projection.explanation.exclusions).toEqual(
      expect.arrayContaining(["not billing", "not settlement", "not tariff", "not market-cleared", "not dispatch"])
    );
    expect(JSON.stringify(projection.explanation).toLowerCase()).not.toContain("revenue");
  });

  it("keeps cooldown heat as reactor-state context with no commercial output", () => {
    const projection = buildWorkbenchProjection(loadFixtureWorkbenchData(), {
      selectedUnitId: "KAL-04",
      selectedCommercialBasisId: "CDB-KAL-04-RESILIENCE"
    });

    expect(projection.selectedUnit.outputLine).toBe("0.18 MWth residual heat");
    expect(projection.selectedUnit.accruedDisplayLabel).toBe("no commercial output");
    expect(projection.groups.imputed.values.map((value) => value.label)).toContain("Cooldown Heat");
    expect(projection.explanation.items).toEqual(
      expect.arrayContaining([
        { label: "Commercial mode", value: "resilience backup" },
        { label: "Delivered heat", value: "0 MWhth" }
      ])
    );
  });

  it("requires multiple measured flux stand-ins before showing a core distribution estimate", () => {
    const input = loadFixtureWorkbenchData();
    const singleFluxInput: WorkbenchProjectionInput = {
      ...input,
      measured: input.measured.filter(
        (frame) => !frame.tagId.startsWith("TAG-KAL-01-FLUX-") || frame.tagId === "TAG-KAL-01-FLUX-LOW"
      )
    };

    expect(
      buildWorkbenchProjection(input, { selectedUnitId: "KAL-01" }).groups.imputed.values.map((value) => value.label)
    ).toContain("Core Power Distribution Estimate");
    expect(
      buildWorkbenchProjection(singleFluxInput, { selectedUnitId: "KAL-01" }).groups.imputed.values.map((value) => value.label)
    ).not.toContain("Core Power Distribution Estimate");
  });
});

function withoutLineages(input: WorkbenchProjectionInput): WorkbenchProjectionInput {
  return {
    ...input,
    lineages: []
  };
}
