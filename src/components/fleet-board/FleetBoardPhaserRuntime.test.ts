import { describe, expect, it, vi } from "vitest";
import type { FleetBoardSceneModel } from "../../domain/fleet-board";
import { createFleetBoardPhaserRuntime } from "./FleetBoardPhaserRuntime";

describe("Fleet Board Phaser runtime", () => {
  it("updates the mounted game without replacing its assets or interaction state", async () => {
    const interactionState = { cameraZoom: 1.2, selectedFacilityId: "reactor-1", dragging: true };
    const mountedGame = {
      update: vi.fn(),
      destroy: vi.fn(),
      interactionState
    };
    const startGame = vi.fn(async () => mountedGame);
    const runtime = createFleetBoardPhaserRuntime(startGame);
    const host = {} as HTMLDivElement;
    const initialScene = buildScene(0);
    const nextScene = buildScene(1);
    const onPlaceFacility = vi.fn();

    runtime.mount({ host, scene: initialScene, onPlaceFacility });
    runtime.mount({ host, scene: nextScene, onPlaceFacility });
    await runtime.ready();
    runtime.update({ scene: nextScene, onPlaceFacility });

    expect(startGame).toHaveBeenCalledTimes(1);
    expect(mountedGame.update).toHaveBeenCalledWith({
      scene: nextScene,
      onPlaceFacility
    });
    expect(mountedGame.destroy).not.toHaveBeenCalled();
    expect(mountedGame.interactionState).toBe(interactionState);

    runtime.destroy();
    runtime.destroy();
    expect(mountedGame.destroy).toHaveBeenCalledOnce();
  });

  it("applies the latest scene when an update arrives while Phaser is loading", async () => {
    const mountedGame = { update: vi.fn(), destroy: vi.fn() };
    let finishStarting!: (game: typeof mountedGame) => void;
    const startGame = vi.fn(
      () =>
        new Promise<typeof mountedGame>((resolve) => {
          finishStarting = resolve;
        })
    );
    const runtime = createFleetBoardPhaserRuntime(startGame);
    const onPlaceFacility = vi.fn();
    const nextScene = buildScene(1);

    runtime.mount({ host: {} as HTMLDivElement, scene: buildScene(0), onPlaceFacility });
    runtime.update({ scene: nextScene, onPlaceFacility });
    finishStarting(mountedGame);
    await runtime.ready();

    expect(mountedGame.update).toHaveBeenCalledOnce();
    expect(mountedGame.update).toHaveBeenCalledWith({ scene: nextScene, onPlaceFacility });
  });

  it("destroys a late-starting game exactly once after the canvas unmounts", async () => {
    const mountedGame = { update: vi.fn(), destroy: vi.fn() };
    let finishStarting!: (game: typeof mountedGame) => void;
    const runtime = createFleetBoardPhaserRuntime(
      () =>
        new Promise<typeof mountedGame>((resolve) => {
          finishStarting = resolve;
        })
    );

    runtime.mount({ host: {} as HTMLDivElement, scene: buildScene(0), onPlaceFacility: vi.fn() });
    runtime.destroy();
    runtime.destroy();
    finishStarting(mountedGame);
    await runtime.ready();

    expect(mountedGame.destroy).toHaveBeenCalledOnce();
    expect(mountedGame.update).not.toHaveBeenCalled();
  });
});

function buildScene(day: number): FleetBoardSceneModel {
  return {
    selectedUnitId: "KAL-03",
    day,
    grid: { columns: 16, rows: 10, tileSize: 72 },
    facilities: [],
    pawns: [],
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
