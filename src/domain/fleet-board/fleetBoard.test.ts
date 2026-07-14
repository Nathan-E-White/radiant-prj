import { describe, expect, it } from "vitest";
import {
  applyFleetBoardAction,
  createInitialFleetBoardState,
  fleetBoardDefaultConfig,
  summarizeFleetBoard
} from "./fleetBoard";

describe("fleet board game reducer", () => {
  it("runs a deterministic contract sprint with fuel production, reactor output, service credits, and scoring", () => {
    let state = createInitialFleetBoardState({ seed: "contract-sprint" });
    state = applyFleetBoardAction(state, {
      type: "placeFacility",
      facilityId: "fuel-1",
      facilityKind: "trisoFactory",
      position: { x: 2, y: 2 }
    });
    state = applyFleetBoardAction(state, {
      type: "placeFacility",
      facilityId: "reactor-1",
      facilityKind: "reactor",
      position: { x: 5, y: 2 }
    });
    state = applyFleetBoardAction(state, {
      type: "placeFacility",
      facilityId: "desal-1",
      facilityKind: "desalPlant",
      position: { x: 8, y: 2 }
    });
    state = applyFleetBoardAction(state, {
      type: "placeFacility",
      facilityId: "base-1",
      facilityKind: "armyBase",
      position: { x: 5, y: 5 }
    });

    for (let day = 0; day < 8; day += 1) {
      state = applyFleetBoardAction(state, { type: "tickDay" });
    }

    const summary = summarizeFleetBoard(state);
    expect(summary.day).toBe(8);
    expect(summary.fuelBlocks).toBeGreaterThan(0);
    expect(summary.electricMwe).toBeGreaterThan(0);
    expect(summary.thermalMwt).toBeGreaterThan(0);
    expect(summary.waterCredits).toBeGreaterThan(0);
    expect(summary.resilienceCredits).toBeGreaterThan(0);
    expect(summary.score).toBeGreaterThan(0);
    expect(summary.removed).toBe(false);
  });

  it("removes the player after staying too deep in debt for too long", () => {
    let state = createInitialFleetBoardState({ seed: "debt", cash: -320 });

    for (let day = 0; day < fleetBoardDefaultConfig.debtGraceDays + 1; day += 1) {
      state = applyFleetBoardAction(state, { type: "tickDay" });
    }

    expect(state.removed).toBe(true);
    expect(state.events.at(-1)?.kind).toBe("debtRemoval");
  });

  it("makes inspector and Trouble pawn motion deterministic for the same seed", () => {
    const run = () => {
      let state = createInitialFleetBoardState({ seed: "pawn-test" });
      state = applyFleetBoardAction(state, {
        type: "placeFacility",
        facilityId: "reactor-1",
        facilityKind: "reactor",
        position: { x: 5, y: 2 }
      });
      for (let day = 0; day < 12; day += 1) {
        state = applyFleetBoardAction(state, { type: "tickDay" });
      }
      return {
        pawns: state.pawns,
        events: state.events.map((event) => `${event.day}:${event.kind}:${event.facilityId ?? event.detail}`)
      };
    };

    expect(run()).toEqual(run());
  });

  it("supports refueling outage recovery when a reactor runs out of fuel", () => {
    let state = createInitialFleetBoardState({ seed: "refuel", fuelBlocks: 1 });
    state = applyFleetBoardAction(state, {
      type: "placeFacility",
      facilityId: "reactor-1",
      facilityKind: "reactor",
      position: { x: 4, y: 3 }
    });
    state = applyFleetBoardAction(state, { type: "tickDay" });
    state = applyFleetBoardAction(state, { type: "tickDay" });

    expect(state.facilities["reactor-1"]?.status).toBe("refueling");

    state = applyFleetBoardAction(state, { type: "refuelFacility", facilityId: "reactor-1" });

    expect(state.facilities["reactor-1"]?.status).toBe("active");
    expect(state.resources.fuelBlocks).toBeLessThanOrEqual(0);
    expect(state.events.at(-1)?.kind).toBe("refueled");
  });

  it("buys reactor-scoped Simulation Container Tokens from a separate bounded budget", () => {
    let state = createInitialFleetBoardState({ seed: "simulation-capacity" });
    for (const [facilityId, x] of [
      ["reactor-1", 2],
      ["reactor-2", 6],
      ["reactor-3", 10]
    ] as const) {
      state = applyFleetBoardAction(state, {
        type: "placeFacility",
        facilityId,
        facilityKind: "reactor",
        position: { x, y: 3 }
      });
    }

    expect(summarizeFleetBoard(state)).toEqual(
      expect.objectContaining({ simulationBudget: 6, simulationContainerTokens: 0 })
    );

    state = applyFleetBoardAction(state, { type: "buySimulationContainerToken", reactorId: "reactor-1" });
    state = applyFleetBoardAction(state, { type: "buySimulationContainerToken", reactorId: "reactor-1" });
    const fullRail = applyFleetBoardAction(state, {
      type: "buySimulationContainerToken",
      reactorId: "reactor-1"
    });

    expect(summarizeFleetBoard(state)).toEqual(
      expect.objectContaining({ simulationBudget: 2, simulationContainerTokens: 2 })
    );
    expect(Object.values(state.simulation.containerTokens)).toEqual([
      expect.objectContaining({ reactorId: "reactor-1" }),
      expect.objectContaining({ reactorId: "reactor-1" })
    ]);
    expect(fullRail.events.at(-1)).toEqual(
      expect.objectContaining({
        kind: "simulationPurchaseBlocked",
        facilityId: "reactor-1",
        detail: expect.stringContaining("Reactor Slot Rail is full")
      })
    );

    state = applyFleetBoardAction(state, { type: "buySimulationContainerToken", reactorId: "reactor-2" });
    const exhausted = applyFleetBoardAction(state, {
      type: "buySimulationContainerToken",
      reactorId: "reactor-3"
    });

    expect(summarizeFleetBoard(state)).toEqual(
      expect.objectContaining({ simulationBudget: 0, simulationContainerTokens: 3 })
    );
    expect(exhausted.events.at(-1)).toEqual(
      expect.objectContaining({
        kind: "simulationPurchaseBlocked",
        facilityId: "reactor-3",
        detail: expect.stringContaining("Simulation Budget is exhausted")
      })
    );
  });
});
