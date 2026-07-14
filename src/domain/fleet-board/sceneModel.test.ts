import { describe, expect, it } from "vitest";
import { buildWorkbenchProjection, loadFixtureWorkbenchData } from "../simulator-workbench";
import { applyFleetBoardAction, createInitialFleetBoardState } from "./fleetBoard";
import { buildFleetBoardSceneModel } from "./sceneModel";
import { buildFleetBoardWorkbenchModifiers } from "./workbenchAdapter";

describe("fleet board scene model", () => {
  it("builds a small Phaser-ready scene model from game state and Workbench projection", () => {
    const projection = buildWorkbenchProjection(loadFixtureWorkbenchData(), { selectedUnitId: "KAL-03" });
    const modifiers = buildFleetBoardWorkbenchModifiers(projection);
    let gameState = createInitialFleetBoardState({ seed: "scene-model", modifiers });
    gameState = applyFleetBoardAction(gameState, {
      type: "placeFacility",
      facilityId: "reactor-1",
      facilityKind: "reactor",
      position: { x: 5, y: 2 }
    });
    gameState = applyFleetBoardAction(gameState, {
      type: "placeFacility",
      facilityId: "triso-1",
      facilityKind: "trisoFactory",
      position: { x: 1, y: 2 }
    });

    const scene = buildFleetBoardSceneModel(projection, gameState, "reactor-1");

    expect(scene.selectedUnitId).toBe("KAL-03");
    expect(scene.selectedReactorId).toBe("reactor-1");
    expect(scene.facilities).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          id: "reactor-1",
          kind: "reactor",
          spriteKey: "reactor"
        })
      ])
    );
    expect(scene.pawns.map((pawn) => pawn.kind)).toEqual(["inspector", "trouble"]);
    expect(scene.routes).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          from: expect.objectContaining({ id: "reactor-1" }),
          to: expect.objectContaining({ kind: "trisoFactory" })
        })
      ])
    );
    expect(scene.resources.cash).toBe(gameState.resources.cash);
    expect(scene.valueBasisCounts).toEqual(projection.valueBasisSummary);
  });
});
