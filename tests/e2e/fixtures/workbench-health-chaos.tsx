import { createRoot } from "react-dom/client";
import withChaos from "react-chaos";
import {
  SimulationHealthErrorBoundary,
  SimulationHealthPanel,
  type SimulationHealthPanelModel
} from "../../../src/components/simulator-workbench";

Math.random = () => 1;

const model: SimulationHealthPanelModel = {
  lifecycle: { title: "Lifecycle", summary: "1/1 complete", detail: "live", status: "healthy" },
  worker: { title: "Worker", summary: "1/1 nominal", detail: "live", status: "healthy" },
  artifact: { title: "Artifact", summary: "committed", detail: "live", status: "healthy" },
  streamFreshness: { title: "Stream freshness", summary: "fresh", detail: "live", status: "healthy" }
};

const ChaoticSimulationHealthPanel = withChaos(
  SimulationHealthPanel,
  10,
  "Injected Simulation Health render fault",
  true
);

function Harness() {
  return (
    <main>
      <h1>Workbench shell remains available</h1>
      <SimulationHealthErrorBoundary>
        <ChaoticSimulationHealthPanel model={model} />
      </SimulationHealthErrorBoundary>
    </main>
  );
}

createRoot(document.getElementById("root")!).render(<Harness />);
