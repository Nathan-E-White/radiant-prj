import { createRoot } from "react-dom/client";
import { BoardSceneWorkbench } from "../../../src/components/fleet-board/BoardSceneWorkbench";
import { buildWorkbenchProjection, loadFixtureWorkbenchData } from "../../../src/domain/simulator-workbench";

createRoot(document.getElementById("root")!).render(
  <BoardSceneWorkbench
    projection={buildWorkbenchProjection(loadFixtureWorkbenchData(), { selectedUnitId: "KAL-03" })}
  />
);
