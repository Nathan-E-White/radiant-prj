import { Factory, TimerReset } from "lucide-react";
import type { ProjectedFleetUnit } from "../../domain/simulator-workbench";

export function FleetStrip({
  units,
  onSelectUnit
}: {
  units: ProjectedFleetUnit[];
  onSelectUnit: (unitId: string) => void;
}) {
  return (
    <section className="simwb-fleet-strip" aria-label="Commercial Kaleidos Fleet Strip">
      {units.map((unit) => (
        <button
          aria-pressed={unit.selected}
          className={unit.selected ? "simwb-fleet-card selected" : "simwb-fleet-card"}
          data-unit-id={unit.unitId}
          key={unit.unitId}
          onClick={() => onSelectUnit(unit.unitId)}
          type="button"
        >
          <span className="simwb-fleet-title">
            <Factory size={16} />
            <strong>{unit.unitId}</strong>
            {unit.freshnessWarningLabel && <small>{unit.freshnessWarningLabel}</small>}
          </span>
          <span className="simwb-fleet-phase">{unit.phaseLine}</span>
          <span className="simwb-fleet-output">{unit.outputLine}</span>
          <span className="simwb-fleet-mode">{unit.commercialMode}</span>
          <span className="simwb-fleet-value">
            <TimerReset size={15} />
            {unit.accruedDisplayLabel}
          </span>
        </button>
      ))}
    </section>
  );
}
