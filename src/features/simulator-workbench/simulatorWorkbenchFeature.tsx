import { useCallback, useEffect, useMemo, useState, useSyncExternalStore } from "react";
import type { ReactNode } from "react";
import { SimulatorWorkbenchSurface, type SimulationHealthPanelModel } from "../../components/simulator-workbench";
import type { ComputeJob } from "../../domain/types";
import {
  buildWorkbenchProjection,
  createBrowserWorkbenchSnapshotSession,
  createHealthTickDriver,
  projectHealthCards,
  workbenchReadLabel,
  type WorkbenchHealthRunState,
  type WorkbenchSelection,
  type WorkbenchProjection,
  type WorkbenchReadState,
  type WorkbenchSnapshotSession,
  type WorkbenchStateView
} from "../../domain/simulator-workbench";

const WORKBENCH_HEALTH_TICK_MS = 1250;

export function useSimulatorWorkbenchFeature(initialSelection: WorkbenchSelection = {}) {
  const [session] = useState<WorkbenchSnapshotSession>(() => createBrowserWorkbenchSnapshotSession());
  const readState = useSyncExternalStore(session.subscribe, session.getState, session.getState);
  const [selection, setSelection] = useState<WorkbenchSelection>(initialSelection);
  const data = readState.model?.input ?? null;
  const projection = useMemo(() => (data ? buildWorkbenchProjection(data, selection) : null), [data, selection]);
  const [healthPanelModel, setHealthPanelModel] = useState<SimulationHealthPanelModel>(() =>
    projectHealthCards({ generatedAt: "1970-01-01T00:00:00Z", activeSimulationRuns: [] }, new Date(0))
  );

  const refresh = useCallback(() => {
    void session.refresh();
  }, [session]);

  useEffect(() => {
    void session.start();
    return () => session.dispose();
  }, [session]);

  useEffect(() => {
    if (!data) return;
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
    readState,
    refresh,
    healthPanelModel,
    selection,
    selectUnit,
    selectValue
  };
}

export function StatusWorkbenchTab({
  projection,
  readState,
  onRefresh,
  onSelectUnit,
  onSelectValue,
  computeQueue,
  selectedJob,
  scenario,
  scenarioJobs,
  bundleState,
  orchestrationPanel
}: {
  projection: WorkbenchProjection | null;
  readState: WorkbenchReadState;
  onRefresh: () => void;
  onSelectUnit: (unitId: string, commercialBasisId: string) => void;
  onSelectValue: (valueId: string) => void;
  computeQueue: ReactNode;
  selectedJob: ComputeJob;
  scenario: string;
  scenarioJobs: ComputeJob[];
  bundleState: string;
  orchestrationPanel: ReactNode;
}) {
  if (!projection) {
    return (
      <section className="simwb-shell" aria-label="Status Workbench">
        <div className={`workbench-read-status ${readState.phase}`} role="status" aria-live="polite">
          <strong>{workbenchReadLabel(readState)}</strong>
          <span>{readState.message}</span>
          <button type="button" onClick={onRefresh}>Retry live Snapshot</button>
        </div>
      </section>
    );
  }
  return (
    <SimulatorWorkbenchSurface
      projection={projection}
      readState={readState}
      onRefresh={onRefresh}
      onSelectUnit={onSelectUnit}
      onSelectValue={onSelectValue}
      computeQueue={computeQueue}
      selectedJob={selectedJob}
      scenario={scenario}
      scenarioJobs={scenarioJobs}
      bundleState={bundleState}
      orchestrationPanel={orchestrationPanel}
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
