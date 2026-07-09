import type { WorkbenchValueBasis } from "../../api/simulatorWorkbench";
import type { WorkbenchProjection } from "../simulator-workbench";

export type FleetBoardWorkbenchModifiers = {
  selectedUnitId: string;
  freshnessRisk: number;
  inspectorPressure: number;
  confidenceMultiplier: number;
  simulatedResultPressure: number;
  valueBasisCounts: Record<WorkbenchValueBasis, number>;
};

export function buildFleetBoardWorkbenchModifiers(projection: WorkbenchProjection): FleetBoardWorkbenchModifiers {
  const degradedFleetUnits = projection.fleetUnits.filter((unit) => unit.freshnessWarningLabel).length;
  const freshnessRisk = clamp01(degradedFleetUnits / Math.max(1, projection.fleetUnits.length));
  const imputedValues = projection.groups.imputed.values;
  const averageConfidence =
    imputedValues.reduce((sum, value) => sum + value.confidencePct, 0) / Math.max(1, imputedValues.length);
  const confidenceMultiplier = clamp(0.82 + averageConfidence / 500, 0.82, 1.05);
  const simulatedResultPressure =
    projection.healthSummary.status === "complete"
      ? 0
      : projection.healthSummary.status === "pending"
        ? 0.18
        : 0.34;
  const inspectorPressure = clamp01(freshnessRisk * 0.55 + (1 - confidenceMultiplier) * 0.3 + simulatedResultPressure * 0.15);

  return {
    selectedUnitId: projection.selectedUnit.unitId,
    freshnessRisk,
    inspectorPressure,
    confidenceMultiplier,
    simulatedResultPressure,
    valueBasisCounts: {
      measured: projection.valueBasisSummary.measured,
      imputed: projection.valueBasisSummary.imputed,
      simulated: projection.valueBasisSummary.simulated
    }
  };
}

function clamp01(value: number): number {
  return clamp(value, 0, 1);
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}
