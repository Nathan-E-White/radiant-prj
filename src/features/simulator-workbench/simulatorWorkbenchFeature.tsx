import { useCallback, useEffect, useMemo, useState, useSyncExternalStore } from "react";
import type { ReactNode } from "react";
import { SimulatorWorkbenchSurface } from "../../components/simulator-workbench";
import type { ComputeJob } from "../../domain/types";
import {
  createBrowserWorkbenchSnapshotSession,
  workbenchReadLabel,
  type WorkbenchSelection,
  type WorkbenchProjection,
  type WorkbenchReadState,
  type WorkbenchSnapshotSession
} from "../../domain/simulator-workbench";

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

  return result;
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
  onSelectUnit: (unitId: string) => void;
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
      healthPanelModel={readState.model!.healthPanelModel}
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
