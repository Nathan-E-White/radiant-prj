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
    gameState = applyFleetBoardAction(gameState, {
      type: "buySimulationContainerToken",
      reactorId: "reactor-1"
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
    expect(scene.reactorSlotRails).toEqual([
      expect.objectContaining({
        reactorId: "reactor-1",
        label: "Reactor Slot Rail",
        slots: [
          expect.objectContaining({ slotIndex: 0, status: "idle" }),
          expect.objectContaining({ slotIndex: 1, status: "empty" })
        ]
      })
    ]);
    expect(scene.resources.cash).toBe(gameState.resources.cash);
    expect(scene.valueBasisCounts).toEqual(projection.valueBasisSummary);
  });

  it("projects queued and running jobs plus completed reactor-scoped Insight Tokens", () => {
    const projection = buildWorkbenchProjection(loadFixtureWorkbenchData(), { selectedUnitId: "KAL-03" });
    let gameState = createInitialFleetBoardState({ seed: "job-scene" });
    gameState = applyFleetBoardAction(gameState, {
      type: "placeFacility",
      facilityId: "reactor-1",
      facilityKind: "reactor",
      position: { x: 5, y: 2 }
    });
    gameState = applyFleetBoardAction(gameState, { type: "buySimulationContainerToken", reactorId: "reactor-1" });
    gameState = applyFleetBoardAction(gameState, { type: "queueSimulationJob", reactorId: "reactor-1" });

    expect(buildFleetBoardSceneModel(projection, gameState).reactorSlotRails[0]?.slots[0]).toEqual(
      expect.objectContaining({ status: "queued", label: "Simulation Job queued" })
    );

    gameState = applyFleetBoardAction(gameState, { type: "tickDay" });
    expect(buildFleetBoardSceneModel(projection, gameState).reactorSlotRails[0]?.slots[0]).toEqual(
      expect.objectContaining({ status: "running", label: "Simulation Job running · 2 advances remaining" })
    );

    gameState = applyFleetBoardAction(gameState, { type: "tickDay" });
    gameState = applyFleetBoardAction(gameState, { type: "tickDay" });
    expect(buildFleetBoardSceneModel(projection, gameState).insightTokenBadges).toEqual([
      expect.objectContaining({ reactorId: "reactor-1", count: 1, label: "1 Insight Token" })
    ]);
  });
});
