import { useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import { FleetBoardCanvas } from "../../../src/components/fleet-board/FleetBoardCanvas";
import type { FleetBoardSceneModel } from "../../../src/domain/fleet-board";

declare global {
  interface Window {
    advanceFleetBoardScene: () => void;
  }
}

function FleetBoardRuntimeHarness() {
  const [day, setDay] = useState(0);
  const [selectedReactorId, setSelectedReactorId] = useState<string | null>(null);
  const [placements, setPlacements] = useState(0);
  const scene = useMemo(() => buildScene(day, selectedReactorId), [day, selectedReactorId]);
  window.advanceFleetBoardScene = () => setDay((current) => current + 1);

  return (
    <main>
      <p data-testid="harness-day">Day {day}</p>
      <p data-testid="harness-selection">Selected {selectedReactorId ?? "none"}</p>
      <p data-testid="harness-placements">Placements {placements}</p>
      <FleetBoardCanvas
        scene={scene}
        onPlaceFacility={() => setPlacements((current) => current + 1)}
        onSelectReactor={setSelectedReactorId}
      />
    </main>
  );
}

function buildScene(day: number, selectedReactorId: string | null): FleetBoardSceneModel {
  return {
    selectedUnitId: "KAL-03",
    selectedReactorId,
    day,
    grid: { columns: 16, rows: 10, tileSize: 72 },
    facilities: [
      {
        id: "reactor-1",
        kind: "reactor",
        label: "Reactor 1",
        status: "active",
        spriteKey: "reactor",
        gridX: 5,
        gridY: 2
      }
    ],
    pawns: [],
    routes: [],
    reactorSlotRails: [],
    resources: {
      fuelBlocks: 0,
      electricMwe: 0,
      thermalMwt: 0,
      waterCredits: 0,
      resilienceCredits: 0,
      cash: 0
    },
    score: { water: 0, resilience: 0, cash: 0, continuity: 0, total: 0 },
    valueBasisCounts: { measured: 0, simulated: 0, imputed: 0 }
  };
}

createRoot(document.getElementById("root")!).render(<FleetBoardRuntimeHarness />);
