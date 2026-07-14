import { createRoot } from "react-dom/client";
import { FleetBoardSurface } from "../../../src/components/fleet-board/FleetBoardSurface";
import { buildWorkbenchProjection, loadFixtureWorkbenchData } from "../../../src/domain/simulator-workbench";

createRoot(document.getElementById("root")!).render(
  <FleetBoardSurface
    projection={buildWorkbenchProjection(loadFixtureWorkbenchData(), { selectedUnitId: "KAL-02" })}
  />
);
