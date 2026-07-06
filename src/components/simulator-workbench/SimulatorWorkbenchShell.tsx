import type { SimulatorWorkbenchState } from "../../api/simulatorWorkbench";

export function SimulatorWorkbenchShell({ state }: { state: SimulatorWorkbenchState }) {
  return (
    <section aria-label="Simulator Workbench scaffold">
      <h1>Simulator Workbench</h1>
      <p>{state.scenarioId}</p>
    </section>
  );
}
