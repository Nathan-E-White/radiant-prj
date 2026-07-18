import { useState } from "react";
import { createRoot } from "react-dom/client";
import { useSimulatorWorkbenchFeature } from "../../../src/features/simulator-workbench/simulatorWorkbenchFeature";

window.fetch = (_input, init) => new Promise((_resolve, reject) => {
  document.body.dataset.requestStarted = "true";
  init?.signal?.addEventListener("abort", () => {
    document.body.dataset.requestAborted = "true";
    reject(new DOMException("aborted", "AbortError"));
  }, { once: true });
});

function WorkbenchSessionConsumer() {
  useSimulatorWorkbenchFeature();
  return <span>Workbench session mounted</span>;
}

function Harness() {
  const [mounted, setMounted] = useState(true);
  return (
    <main>
      <button type="button" onClick={() => setMounted(false)}>Unmount Workbench</button>
      {mounted ? <WorkbenchSessionConsumer /> : <span>Workbench session unmounted</span>}
    </main>
  );
}

createRoot(document.getElementById("root")!).render(<Harness />);
