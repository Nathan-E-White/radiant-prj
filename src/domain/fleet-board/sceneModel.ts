import type { WorkbenchValueBasis } from "../../api/simulatorWorkbench";
import type { WorkbenchProjection } from "../simulator-workbench";
import {
  summarizeReactorSimulation,
  type FleetBoardFacilityKind,
  type FleetBoardPawnKind,
  type FleetBoardState
} from "./fleetBoard";

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
  status: "empty" | "idle" | "queued" | "running";
  label: string;
  tokenId?: string;
  jobId?: string;
  advancesRemaining?: number;
};

export type FleetBoardSceneReactorSlotRail = {
  reactorId: string;
  gridX: number;
  gridY: number;
  label: "Reactor Slot Rail";
  slots: FleetBoardSceneReactorSlot[];
};

export type FleetBoardSceneInsightTokenBadge = {
  id: string;
  reactorId: string;
  gridX: number;
  gridY: number;
  count: number;
  label: string;
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
  insightTokenBadges: FleetBoardSceneInsightTokenBadge[];
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
    insightTokenBadges: buildInsightTokenBadges(gameState, facilities),
    resources: gameState.resources,
    score: gameState.score,
    valueBasisCounts: projection.valueBasisSummary
  };
}

function buildReactorSlotRails(gameState: FleetBoardState, facilities: FleetBoardSceneFacility[]) {
  return facilities
    .filter((facility) => facility.kind === "reactor")
    .map((reactor): FleetBoardSceneReactorSlotRail => {
      const reactorSimulation = summarizeReactorSimulation(gameState, reactor.id);
      return {
        reactorId: reactor.id,
        gridX: reactor.gridX,
        gridY: reactor.gridY - 0.58,
        label: "Reactor Slot Rail",
        slots: Array.from({ length: gameState.config.simulationContainerTokenCapPerReactor }, (_, slotIndex) => {
          const slot = reactorSimulation.slots[slotIndex];
          return {
            id: `${reactor.id}-simulation-slot-${slotIndex + 1}`,
            slotIndex,
            status: slot?.status ?? "empty",
            label: simulationSlotLabel(slot),
            ...(slot
              ? {
                  tokenId: slot.tokenId,
                  ...(slot.jobId ? { jobId: slot.jobId } : {}),
                  ...(slot.advancesRemaining === undefined
                    ? {}
                    : { advancesRemaining: slot.advancesRemaining })
                }
              : {})
          };
        })
      };
    });
}

function simulationSlotLabel(slot: ReturnType<typeof summarizeReactorSimulation>["slots"][number] | undefined) {
  if (!slot) {
    return "Empty simulation slot";
  }
  if (slot.status === "idle") {
    return "Simulation Container Token idle";
  }
  if (slot.status === "queued") {
    return "Simulation Job queued";
  }
  return `Simulation Job running · ${slot.advancesRemaining} advances remaining`;
}

function buildInsightTokenBadges(gameState: FleetBoardState, facilities: FleetBoardSceneFacility[]) {
  return facilities.flatMap((facility): FleetBoardSceneInsightTokenBadge[] => {
    if (facility.kind !== "reactor") {
      return [];
    }
    const count = summarizeReactorSimulation(gameState, facility.id).insightTokens;
    if (count === 0) {
      return [];
    }
    return [
      {
        id: `${facility.id}-insight-token-badge`,
        reactorId: facility.id,
        gridX: facility.gridX - 0.52,
        gridY: facility.gridY - 0.58,
        count,
        label: `${count} Insight Token${count === 1 ? "" : "s"}`
      }
    ];
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
