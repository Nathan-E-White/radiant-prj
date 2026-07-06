import type { SimulatorWorkbenchState } from "../../api/simulatorWorkbench";

export function SimulationResultsPanel({ state }: { state: SimulatorWorkbenchState }) {
  return (
    <section aria-label="Simulation results scaffold">
      <h2>Simulated</h2>
      <p>{state.activeSimulationRuns.length} run references</p>
    </section>
  );
}
