import { useEffect, useMemo, useState } from "react";
import { SimulatorWorkbenchSurface, type SimulationHealthPanelModel } from "../../components/simulator-workbench";
import {
  buildWorkbenchProjection,
  createHealthTickDriver,
  loadFixtureWorkbenchData,
  projectHealthCards,
  type WorkbenchHealthRunState,
  type WorkbenchSelection,
  type WorkbenchProjection,
  type WorkbenchStateView
} from "../../domain/simulator-workbench";

const WORKBENCH_HEALTH_TICK_MS = 1250;

export function useSimulatorWorkbenchFeature(initialSelection: WorkbenchSelection = {}) {
  const data = useMemo(() => loadFixtureWorkbenchData(), []);
  const [selection, setSelection] = useState<WorkbenchSelection>(initialSelection);
  const projection = useMemo(() => buildWorkbenchProjection(data, selection), [data, selection]);
  const [healthPanelModel, setHealthPanelModel] = useState<SimulationHealthPanelModel>(() =>
    projectHealthCards(stateViewFromWorkbenchState(data.state), new Date(data.state.generatedAt))
  );

  useEffect(() => {
    const driver = createHealthTickDriver({
      intervalMs: WORKBENCH_HEALTH_TICK_MS,
      fixtures: buildHealthPanelSequence(data.state),
      initialNow: new Date(data.state.generatedAt),
      onTick: setHealthPanelModel
    });
    return () => driver.stop();
  }, [data]);

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
    healthPanelModel,
    selection,
    selectUnit,
    selectValue
  };
}

export function SimulatorWorkbenchTab({
  projection,
  healthPanelModel,
  onSelectUnit,
  onSelectValue
}: {
  projection: WorkbenchProjection;
  healthPanelModel: SimulationHealthPanelModel;
  onSelectUnit: (unitId: string, commercialBasisId: string) => void;
  onSelectValue: (valueId: string) => void;
}) {
  return (
    <SimulatorWorkbenchSurface
      projection={projection}
      healthPanelModel={healthPanelModel}
      onSelectUnit={onSelectUnit}
      onSelectValue={onSelectValue}
    />
  );
}

function buildHealthPanelSequence(state: {
  generatedAt: string;
  activeSimulationRuns: Array<WorkbenchHealthRunState>;
}): Array<WorkbenchStateView> {
  const base = stateViewFromWorkbenchState(state);
  return [base, withRunningRun(base, "RUN-KAL-01-SCHED-DRIFT"), withMissingArtifact(base), withCriticalWorker(base)];
}

function stateViewFromWorkbenchState(state: {
  generatedAt: string;
  activeSimulationRuns: Array<WorkbenchHealthRunState>;
}): WorkbenchStateView {
  return {
    generatedAt: state.generatedAt,
    activeSimulationRuns: [...state.activeSimulationRuns]
  };
}

function withRunningRun(base: WorkbenchStateView, runId: string): WorkbenchStateView {
  const selected = base.activeSimulationRuns.find((run) => run.runId === runId);
  const replacement: WorkbenchHealthRunState = selected
    ? { ...selected, lifecycle: "running", health: selected.health }
    : {
        runId,
        lifecycle: "running",
        health: "nominal",
        artifactStatus: "fixture-reference",
        scenarioId: "simulator-ws"
      };

  return {
    ...base,
    activeSimulationRuns: updateRuns(base.activeSimulationRuns, runId, replacement)
  };
}

function withMissingArtifact(base: WorkbenchStateView): WorkbenchStateView {
  const [first, ...rest] = base.activeSimulationRuns;
  if (!first) return base;
  return {
    ...base,
    activeSimulationRuns: [
      {
        ...first,
        artifactStatus: "missing"
      },
      ...rest
    ]
  };
}

function withCriticalWorker(base: WorkbenchStateView): WorkbenchStateView {
  const [first, ...rest] = base.activeSimulationRuns;
  if (!first) return base;
  return {
    ...base,
    activeSimulationRuns: [
      {
        ...first,
        health: "critical"
      },
      ...rest
    ]
  };
}

function updateRuns(
  runs: Array<WorkbenchHealthRunState>,
  runId: string,
  replacement: WorkbenchHealthRunState
): Array<WorkbenchHealthRunState> {
  if (!runs.some((run) => run.runId === runId)) {
    return [...runs, replacement];
  }
  return runs.map((run) => (run.runId === runId ? replacement : run));
}
