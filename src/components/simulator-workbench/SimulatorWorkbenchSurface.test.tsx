import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import { buildWorkbenchProjection, loadFixtureWorkbenchData } from "../../domain/simulator-workbench";
import { SimulatorWorkbenchSurface } from "./SimulatorWorkbenchSurface";

describe("SimulatorWorkbenchSurface", () => {
  it("renders the integrated presentational twin screen with fleet, viewport, value bases, and explanation rail", () => {
    const markup = renderToStaticMarkup(
      <SimulatorWorkbenchSurface
        onSelectUnit={vi.fn()}
        onSelectValue={vi.fn()}
        projection={buildWorkbenchProjection(loadFixtureWorkbenchData())}
      />
    );

    expect(markup).toContain("KAL-01");
    expect(markup).toContain("KAL-05");
    expect(markup).toContain("Measured State");
    expect(markup).toContain("Imputed State");
    expect(markup).toContain("Simulated Result State");
    expect(markup).toContain("Kaleidos Unit twin topology overlay");
    expect(markup).toContain('id="core"');
    expect(markup).toContain('id="heatExchangers"');
    expect(markup).toContain("Engineering Lineage");
    expect(markup).toContain("Core Power Distribution Estimate");
    expect(markup.toLowerCase()).not.toContain("revenue");
    expect(markup).not.toContain("/api/simulator-workbench");
  });

  it("renders commercial display basis when a fleet commercial value is selected", () => {
    const projection = buildWorkbenchProjection(loadFixtureWorkbenchData(), {
      selectedUnitId: "KAL-03",
      selectedCommercialBasisId: "CDB-KAL-03-DESALINATION"
    });
    const markup = renderToStaticMarkup(
      <SimulatorWorkbenchSurface onSelectUnit={vi.fn()} onSelectValue={vi.fn()} projection={projection} />
    );

    expect(markup).toContain("Commercial Display Basis");
    expect(markup).toContain("Accrued Display Value");
    expect(markup).toContain("$18.4k (est)");
    expect(markup).toContain("desalination heat");
    expect(markup).toContain("not billing");
    expect(markup).toContain("not settlement");
    expect(markup).not.toContain("run launch");
    expect(markup).not.toContain("Redpanda");
  });
});
