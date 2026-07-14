import { describe, expect, it, vi } from "vitest";
import type { FleetBoardSceneModel } from "../../domain/fleet-board";
import {
  createFleetBoardPhaserRuntime,
  type FleetBoardPhaserMount
} from "./FleetBoardPhaserRuntime";

describe("Fleet Board Phaser runtime", () => {
  it("updates one mounted game so its loaded assets and Phaser-owned interactions stay mounted", async () => {
    const mountedGame = {
      update: vi.fn(),
      destroy: vi.fn()
    };
    const startedMounts: FleetBoardPhaserMount[] = [];
    const startGame = vi.fn(async (mount: FleetBoardPhaserMount) => {
      startedMounts.push(mount);
      return mountedGame;
    });
    const runtime = createFleetBoardPhaserRuntime(startGame);
    const host = {} as HTMLDivElement;
    const initialScene = buildScene(0);
    const nextScene = buildScene(1);
    const onPlaceFacility = vi.fn();
    const onSelectReactor = vi.fn();

    runtime.mount({ host, scene: initialScene, onPlaceFacility, onSelectReactor });
    runtime.mount({ host, scene: nextScene, onPlaceFacility, onSelectReactor });
    await runtime.ready();
    runtime.update({ scene: nextScene, onPlaceFacility, onSelectReactor });

    expect(startGame).toHaveBeenCalledTimes(1);
    expect(mountedGame.update).toHaveBeenCalledWith({
      scene: nextScene,
      onPlaceFacility,
      onSelectReactor
    });
    expect(mountedGame.destroy).not.toHaveBeenCalled();
    expect(startedMounts[0]?.scene.selectedReactorId).toBe("reactor-1");

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

    runtime.mount({ host: {} as HTMLDivElement, scene: buildScene(0), onPlaceFacility, onSelectReactor: vi.fn() });
    runtime.update({ scene: nextScene, onPlaceFacility, onSelectReactor: vi.fn() });
    finishStarting(mountedGame);
    await runtime.ready();

    expect(mountedGame.update).toHaveBeenCalledOnce();
    expect(mountedGame.update).toHaveBeenCalledWith({
      scene: nextScene,
      onPlaceFacility,
      onSelectReactor: expect.any(Function)
    });
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

    runtime.mount({
      host: {} as HTMLDivElement,
      scene: buildScene(0),
      onPlaceFacility: vi.fn(),
      onSelectReactor: vi.fn()
    });
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
    selectedReactorId: "reactor-1",
    day,
    grid: { columns: 16, rows: 10, tileSize: 72 },
    facilities: [],
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
