import { describe, expect, it } from "vitest";
import {
  applyFleetBoardAction,
  createInitialFleetBoardState,
  fleetBoardDefaultConfig,
  summarizeFleetBoard,
  summarizeReactorSimulation
} from "./fleetBoard";

describe("fleet board reducer compatibility", () => {
  it("preserves the deterministic 30-day contract-sprint rules", () => {
    let state = createInitialFleetBoardState({ seed: "contract-sprint" });
    for (const facility of [
      { facilityId: "fuel-1", facilityKind: "trisoFactory" as const, position: { x: 2, y: 2 } },
      { facilityId: "reactor-1", facilityKind: "reactor" as const, position: { x: 5, y: 2 } },
      { facilityId: "desal-1", facilityKind: "desalPlant" as const, position: { x: 8, y: 2 } },
      { facilityId: "base-1", facilityKind: "armyBase" as const, position: { x: 5, y: 5 } }
    ]) {
      state = applyFleetBoardAction(state, { type: "placeFacility", ...facility });
    }

    for (let day = 0; day < 8; day += 1) {
      state = applyFleetBoardAction(state, { type: "tickDay" });
    }

    expect(summarizeFleetBoard(state)).toEqual(
      expect.objectContaining({
        day: 8,
        removed: false,
        complete: false,
        simulationBudget: 6
      })
    );
    expect(state.resources).toEqual(
      expect.objectContaining({
        fuelBlocks: expect.any(Number),
        electricMwe: expect.any(Number),
        thermalMwt: expect.any(Number)
      })
    );
    expect(state.score.total).toBeGreaterThan(0);
  });

  it("keeps seeded pawn movement and events deterministic", () => {
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

  it("preserves debt removal and refueling recovery", () => {
    let debtState = createInitialFleetBoardState({ seed: "debt", cash: -320 });
    for (let day = 0; day < fleetBoardDefaultConfig.debtGraceDays + 1; day += 1) {
      debtState = applyFleetBoardAction(debtState, { type: "tickDay" });
    }
    expect(debtState.removed).toBe(true);
    expect(debtState.events.at(-1)?.kind).toBe("debtRemoval");

    let refuelState = createInitialFleetBoardState({ seed: "refuel", fuelBlocks: 1 });
    refuelState = applyFleetBoardAction(refuelState, {
      type: "placeFacility",
      facilityId: "reactor-1",
      facilityKind: "reactor",
      position: { x: 4, y: 3 }
    });
    refuelState = applyFleetBoardAction(refuelState, { type: "tickDay" });
    refuelState = applyFleetBoardAction(refuelState, { type: "tickDay" });
    expect(refuelState.facilities["reactor-1"]?.status).toBe("refueling");

    refuelState = applyFleetBoardAction(refuelState, { type: "refuelFacility", facilityId: "reactor-1" });
    expect(refuelState.facilities["reactor-1"]?.status).toBe("active");
    expect(refuelState.events.at(-1)?.kind).toBe("refueled");
  });

  it("preserves bounded reactor-scoped simulation capacity", () => {
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
    state = applyFleetBoardAction(state, { type: "buySimulationContainerToken", reactorId: "reactor-1" });
    state = applyFleetBoardAction(state, { type: "buySimulationContainerToken", reactorId: "reactor-1" });
    const fullRail = applyFleetBoardAction(state, {
      type: "buySimulationContainerToken",
      reactorId: "reactor-1"
    });
    expect(fullRail.events.at(-1)).toEqual(
      expect.objectContaining({ kind: "simulationPurchaseBlocked", facilityId: "reactor-1" })
    );

    state = applyFleetBoardAction(state, { type: "buySimulationContainerToken", reactorId: "reactor-2" });
    expect(summarizeFleetBoard(state)).toEqual(
      expect.objectContaining({ simulationBudget: 0, simulationContainerTokens: 3 })
    );
  });

  it("preserves the queued, running, and completed Simulation Job lifecycle", () => {
    let state = createInitialFleetBoardState({ seed: "simulation-job" });
    state = applyFleetBoardAction(state, {
      type: "placeFacility",
      facilityId: "reactor-1",
      facilityKind: "reactor",
      position: { x: 5, y: 2 }
    });
    state = applyFleetBoardAction(state, { type: "buySimulationContainerToken", reactorId: "reactor-1" });
    state = applyFleetBoardAction(state, { type: "queueSimulationJob", reactorId: "reactor-1" });
    expect(summarizeReactorSimulation(state, "reactor-1").slots[0]).toEqual(
      expect.objectContaining({ status: "queued", advancesRemaining: 3 })
    );

    state = applyFleetBoardAction(state, { type: "tickDay" });
    expect(summarizeReactorSimulation(state, "reactor-1").slots[0]).toEqual(
      expect.objectContaining({ status: "running", advancesRemaining: 2 })
    );
    state = applyFleetBoardAction(state, { type: "tickDay" });
    state = applyFleetBoardAction(state, { type: "tickDay" });
    expect(summarizeFleetBoard(state)).toEqual(
      expect.objectContaining({ completedSimulationJobs: 1, insightTokens: 1 })
    );
  });
});
