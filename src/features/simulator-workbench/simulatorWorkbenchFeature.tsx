import { useMemo, useState } from "react";
import { SimulatorWorkbenchSurface } from "../../components/simulator-workbench";
import {
  buildWorkbenchProjection,
  loadFixtureWorkbenchData,
  type WorkbenchSelection,
  type WorkbenchProjection
} from "../../domain/simulator-workbench";

export function useSimulatorWorkbenchFeature(initialSelection: WorkbenchSelection = {}) {
  const data = useMemo(() => loadFixtureWorkbenchData(), []);
  const [selection, setSelection] = useState<WorkbenchSelection>(initialSelection);
  const projection = useMemo(() => buildWorkbenchProjection(data, selection), [data, selection]);

  function selectUnit(unitId: string, commercialBasisId: string) {
    setSelection((current) => ({
      ...current,
      selectedUnitId: unitId,
      selectedCommercialBasisId: commercialBasisId
    }));
  }

  function selectValue(valueId: string) {
    setSelection((current) => ({
      ...current,
      selectedValueId: valueId
    }));
  }

  return {
    projection,
    selection,
    selectUnit,
    selectValue
  };
}

export function SimulatorWorkbenchTab({
  projection,
  onSelectUnit,
  onSelectValue
}: {
  projection: WorkbenchProjection;
  onSelectUnit: (unitId: string, commercialBasisId: string) => void;
  onSelectValue: (valueId: string) => void;
}) {
  return (
    <SimulatorWorkbenchSurface
      projection={projection}
      onSelectUnit={onSelectUnit}
      onSelectValue={onSelectValue}
    />
  );
}
