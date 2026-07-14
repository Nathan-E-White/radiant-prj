import type { WorkbenchValueBasis } from "../../api/simulatorWorkbench";
import type { WorkbenchProjection } from "../simulator-workbench";
import type { FleetBoardFacilityKind, FleetBoardPawnKind, FleetBoardState } from "./fleetBoard";

export type FleetBoardSpriteKey = FleetBoardFacilityKind | FleetBoardPawnKind | "routePulse";

export type FleetBoardSceneFacility = {
  id: string;
  kind: FleetBoardFacilityKind;
  label: string;
  status: string;
  spriteKey: FleetBoardFacilityKind;
  gridX: number;
  gridY: number;
};

export type FleetBoardScenePawn = {
  kind: FleetBoardPawnKind;
  spriteKey: FleetBoardPawnKind;
  gridX: number;
  gridY: number;
};

export type FleetBoardSceneReactorSlot = {
  id: string;
  slotIndex: number;
  status: "empty" | "installed";
  label: string;
  tokenId?: string;
};

export type FleetBoardSceneReactorSlotRail = {
  reactorId: string;
  gridX: number;
  gridY: number;
  label: "Reactor Slot Rail";
  slots: FleetBoardSceneReactorSlot[];
};

export type FleetBoardSceneModel = {
  selectedUnitId: string;
  selectedReactorId: string | null;
  day: number;
  grid: { columns: number; rows: number; tileSize: number };
  facilities: FleetBoardSceneFacility[];
  pawns: FleetBoardScenePawn[];
  routes: Array<{ from: FleetBoardSceneFacility; to: FleetBoardSceneFacility }>;
  reactorSlotRails: FleetBoardSceneReactorSlotRail[];
  resources: FleetBoardState["resources"];
  score: FleetBoardState["score"];
  valueBasisCounts: Record<WorkbenchValueBasis, number>;
};

export function buildFleetBoardSceneModel(
  projection: WorkbenchProjection,
  gameState: FleetBoardState,
  selectedReactorId: string | null = null
): FleetBoardSceneModel {
  const facilities = Object.values(gameState.facilities).map((facility) => ({
    id: facility.id,
    kind: facility.kind,
    label: facility.label,
    status: facility.status,
    spriteKey: facility.kind,
    gridX: facility.position.x,
    gridY: facility.position.y
  }));
  return {
    selectedUnitId: projection.selectedUnit.unitId,
    selectedReactorId,
    day: gameState.day,
    grid: { columns: 16, rows: 10, tileSize: 72 },
    facilities,
    pawns: Object.values(gameState.pawns).map((pawn) => ({
      kind: pawn.kind,
      spriteKey: pawn.kind,
      gridX: pawn.position.x,
      gridY: pawn.position.y
    })),
    routes: buildFleetBoardRoutes(facilities, gameState.config.routeRange),
    reactorSlotRails: buildReactorSlotRails(gameState, facilities),
    resources: gameState.resources,
    score: gameState.score,
    valueBasisCounts: projection.valueBasisSummary
  };
}

function buildReactorSlotRails(gameState: FleetBoardState, facilities: FleetBoardSceneFacility[]) {
  return facilities
    .filter((facility) => facility.kind === "reactor")
    .map((reactor): FleetBoardSceneReactorSlotRail => {
      const tokens = Object.values(gameState.simulation.containerTokens).filter(
        (token) => token.reactorId === reactor.id
      );
      return {
        reactorId: reactor.id,
        gridX: reactor.gridX,
        gridY: reactor.gridY - 0.58,
        label: "Reactor Slot Rail",
        slots: Array.from({ length: gameState.config.simulationContainerTokenCapPerReactor }, (_, slotIndex) => {
          const token = tokens[slotIndex];
          return {
            id: `${reactor.id}-simulation-slot-${slotIndex + 1}`,
            slotIndex,
            status: token ? "installed" : "empty",
            label: token ? "Simulation Container Token installed" : "Empty simulation slot",
            ...(token ? { tokenId: token.id } : {})
          };
        })
      };
    });
}

function buildFleetBoardRoutes(facilities: FleetBoardSceneFacility[], routeRange: number) {
  const reactors = facilities.filter((facility) => facility.kind === "reactor" && facility.status === "active");
  const routeTargets = facilities.filter(
    (facility) =>
      facility.status === "active" &&
      (facility.kind === "trisoFactory" ||
        facility.kind === "desalPlant" ||
        facility.kind === "armyBase" ||
        facility.kind === "battery")
  );

  return reactors.flatMap((reactor) =>
    routeTargets
      .filter((facility) => manhattanDistance(reactor, facility) <= routeRange)
      .map((facility) => ({ from: reactor, to: facility }))
  );
}

function manhattanDistance(
  left: { gridX: number; gridY: number },
  right: { gridX: number; gridY: number }
) {
  return Math.abs(left.gridX - right.gridX) + Math.abs(left.gridY - right.gridY);
}
