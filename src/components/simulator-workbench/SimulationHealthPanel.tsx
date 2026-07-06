import type { SimulatorWorkbenchState } from "../../api/simulatorWorkbench";

export function SimulationHealthPanel({ state }: { state: SimulatorWorkbenchState }) {
  return (
    <section aria-label="Simulation health scaffold">
      <h2>Health</h2>
      <ul>
        {state.activeSimulationRuns.map((run) => (
          <li key={run.runId}>
            {run.runId} {run.health} {run.artifactStatus}
          </li>
        ))}
      </ul>
    </section>
  );
}
