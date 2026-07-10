import type { SimulatorWorkbenchState } from "../../api/simulatorWorkbench";

export function SimulatorWorkbenchShell({ state }: { state: SimulatorWorkbenchState }) {
  return (
    <section aria-label="Status Workbench scaffold">
      <h1>Status Workbench</h1>
      <p>{state.scenarioId}</p>
    </section>
  );
}
