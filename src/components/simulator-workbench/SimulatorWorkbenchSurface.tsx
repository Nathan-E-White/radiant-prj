import { FleetStrip } from "./FleetStrip";
import { LineagePanel } from "./LineagePanel";
import { MeasuredStatePanel } from "./MeasuredStatePanel";
import { SimulationResultsPanel } from "./SimulationResultsPanel";
import { TwinStatePanel } from "./TwinStatePanel";
import { TwinViewport } from "./TwinViewport";
import { FleetBoardSurface } from "../fleet-board";
import type { WorkbenchProjection } from "../../domain/simulator-workbench";

export function SimulatorWorkbenchSurface({
  projection,
  onSelectUnit,
  onSelectValue
}: {
  projection: WorkbenchProjection;
  onSelectUnit: (unitId: string, commercialBasisId: string) => void;
  onSelectValue: (valueId: string) => void;
}) {
  const selectedValueId = projection.selectedValue?.valueId ?? "";
  const selectedUnitValues = [
    ...projection.groups.measured.values,
    ...projection.groups.imputed.values,
    ...projection.groups.simulated.values
  ];

  return (
    <section className="simwb-shell" aria-label="Simulator Workbench">
      <div className="simwb-head">
        <div>
          <p className="eyebrow">Simulator Workbench</p>
          <h2>{projection.selectedUnit.displayName}</h2>
        </div>
        <div className="simwb-context">
          <span>{projection.selectedUnit.phaseLine}</span>
          <span>Twin: {projection.twinId}</span>
          <span>Generated: {formatTime(projection.generatedAt)}</span>
        </div>
      </div>

      <FleetStrip units={projection.fleetUnits} onSelectUnit={onSelectUnit} />
      <FleetBoardSurface projection={projection} />

      <div className="simwb-grid">
        <aside className="simwb-stack">
          <MeasuredStatePanel
            group={projection.groups.measured}
            selectedValueId={selectedValueId}
            onSelectValue={onSelectValue}
          />
        </aside>

        <TwinViewport
          model={projection.viewport}
          selectedValue={projection.selectedValue}
          values={selectedUnitValues}
          onSelectValue={onSelectValue}
        />

        <aside className="simwb-stack">
          <TwinStatePanel
            group={projection.groups.imputed}
            selectedValueId={selectedValueId}
            onSelectValue={onSelectValue}
          />
          <SimulationResultsPanel
            group={projection.groups.simulated}
            healthSummary={projection.healthSummary}
            selectedValueId={selectedValueId}
            onSelectValue={onSelectValue}
          />
        </aside>
      </div>

      <LineagePanel explanation={projection.explanation} />
    </section>
  );
}

function formatTime(value: string): string {
  return new Date(value).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}
