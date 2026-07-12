import type { WorkbenchValueBasis } from "../../api/simulatorWorkbench";
import type { WorkbenchProjection } from "../simulator-workbench";
import type { FleetBoardFacilityKind, FleetBoardPawnKind, FleetBoardState } from "./fleetBoard";

export type FleetBoardSceneFacility = {
  id: string;
  kind: FleetBoardFacilityKind;
  label: string;
  status: string;
  spriteKey: string;
  gridX: number;
  gridY: number;
};

export type FleetBoardScenePawn = {
  kind: FleetBoardPawnKind;
  spriteKey: string;
  gridX: number;
  gridY: number;
};

export type FleetBoardSceneModel = {
  selectedUnitId: string;
  day: number;
  grid: { columns: number; rows: number; tileSize: number };
  facilities: FleetBoardSceneFacility[];
  pawns: FleetBoardScenePawn[];
  resources: FleetBoardState["resources"];
  score: FleetBoardState["score"];
  valueBasisCounts: Record<WorkbenchValueBasis, number>;
};

const spriteKeys: Record<FleetBoardFacilityKind | FleetBoardPawnKind, string> = {
  trisoFactory: "trisoFactory",
  reactor: "reactor",
  desalPlant: "desalPlant",
  armyBase: "armyBase",
  battery: "battery",
  inspector: "inspector",
  trouble: "trouble"
};

export function buildFleetBoardSceneModel(
  projection: WorkbenchProjection,
  gameState: FleetBoardState
): FleetBoardSceneModel {
  return {
    selectedUnitId: projection.selectedUnit.unitId,
    day: gameState.day,
    grid: { columns: 16, rows: 10, tileSize: 72 },
    facilities: Object.values(gameState.facilities).map((facility) => ({
      id: facility.id,
      kind: facility.kind,
      label: facility.label,
      status: facility.status,
      spriteKey: spriteKeys[facility.kind],
      gridX: facility.position.x,
      gridY: facility.position.y
    })),
    pawns: Object.values(gameState.pawns).map((pawn) => ({
      kind: pawn.kind,
      spriteKey: spriteKeys[pawn.kind],
      gridX: pawn.position.x,
      gridY: pawn.position.y
    })),
    resources: gameState.resources,
    score: gameState.score,
    valueBasisCounts: projection.valueBasisSummary
  };
}
