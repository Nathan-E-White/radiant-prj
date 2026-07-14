import { describe, expect, it } from "vitest";
import {
  applyFleetBoardAction,
  createInitialFleetBoardState,
  fleetBoardDefaultConfig,
  summarizeReactorSimulation,
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
      expect.objectContaining({
        simulationBudget: 2,
        simulationContainerTokens: 2,
        simulationContainerTokensByReactorId: { "reactor-1": 2 },
        simulationContainerTokenCost: 2,
        simulationContainerTokenCapPerReactor: 2
      })
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

  it("queues work on an idle reactor token, starts next tick, and awards an Insight Token after three advances", () => {
    let state = createInitialFleetBoardState({ seed: "simulation-job" });
    state = applyFleetBoardAction(state, {
      type: "placeFacility",
      facilityId: "reactor-1",
      facilityKind: "reactor",
      position: { x: 5, y: 2 }
    });

    const withoutCapacity = applyFleetBoardAction(state, { type: "queueSimulationJob", reactorId: "reactor-1" });
    expect(withoutCapacity.events.at(-1)).toEqual(
      expect.objectContaining({
        kind: "simulationJobQueueBlocked",
        detail: expect.stringContaining("no idle Simulation Container Token")
      })
    );

    state = applyFleetBoardAction(state, { type: "buySimulationContainerToken", reactorId: "reactor-1" });
    state = applyFleetBoardAction(state, { type: "queueSimulationJob", reactorId: "reactor-1" });

    expect(summarizeFleetBoard(state)).toEqual(
      expect.objectContaining({ queuedSimulationJobs: 1, runningSimulationJobs: 0, completedSimulationJobs: 0 })
    );
    expect(summarizeReactorSimulation(state, "reactor-1")).toEqual({
      slots: [
        expect.objectContaining({
          tokenId: "simulation-container-token-1",
          status: "queued",
          advancesRemaining: 3
        })
      ],
      insightTokens: 0
    });

    const busy = applyFleetBoardAction(state, { type: "queueSimulationJob", reactorId: "reactor-1" });
    expect(busy.events.at(-1)).toEqual(
      expect.objectContaining({
        kind: "simulationJobQueueBlocked",
        detail: expect.stringContaining("no idle Simulation Container Token")
      })
    );

    state = applyFleetBoardAction(state, { type: "tickDay" });
    expect(summarizeReactorSimulation(state, "reactor-1").slots[0]).toEqual(
      expect.objectContaining({ status: "running", advancesRemaining: 2 })
    );
    expect(state.events).toEqual(
      expect.arrayContaining([expect.objectContaining({ kind: "simulationJobStarted", facilityId: "reactor-1" })])
    );

    state = applyFleetBoardAction(state, { type: "tickDay" });
    expect(summarizeReactorSimulation(state, "reactor-1").slots[0]).toEqual(
      expect.objectContaining({ status: "running", advancesRemaining: 1 })
    );

    state = applyFleetBoardAction(state, { type: "tickDay" });
    expect(summarizeFleetBoard(state)).toEqual(
      expect.objectContaining({ queuedSimulationJobs: 0, runningSimulationJobs: 0, completedSimulationJobs: 1 })
    );
    expect(summarizeReactorSimulation(state, "reactor-1")).toEqual({
      slots: [expect.objectContaining({ status: "idle" })],
      insightTokens: 1
    });
    expect(state.events).toEqual(
      expect.arrayContaining([expect.objectContaining({ kind: "simulationJobCompleted", facilityId: "reactor-1" })])
    );
  });

  it("spends one reactor Insight Token to absorb Inspector pressure", () => {
    let state = createInitialFleetBoardState({ seed: "inspector-insight", fuelBlocks: 100 });
    state = applyFleetBoardAction(state, {
      type: "placeFacility",
      facilityId: "reactor-1",
      facilityKind: "reactor",
      position: { x: 5, y: 2 }
    });
    state = {
      ...state,
      day: 5,
      simulation: { ...state.simulation, insightTokensByReactorId: { "reactor-1": 1 } }
    };

    state = applyFleetBoardAction(state, { type: "tickDay" });

    expect(state.facilities["reactor-1"]?.status).toBe("active");
    expect(state.simulation.insightTokensByReactorId["reactor-1"]).toBe(0);
    expect(state.events).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          kind: "insightTokenSpent",
          facilityId: "reactor-1",
          detail: expect.stringContaining("Inspector")
        })
      ])
    );
    expect(state.events.some((event) => event.day === 6 && event.kind === "inspectorHold")).toBe(false);
  });

  it("spends one reactor Insight Token to absorb Trouble pressure", () => {
    let state = createInitialFleetBoardState({ seed: "trouble-insight", fuelBlocks: 100 });
    state = applyFleetBoardAction(state, {
      type: "placeFacility",
      facilityId: "reactor-1",
      facilityKind: "reactor",
      position: { x: 5, y: 2 }
    });
    state = {
      ...state,
      day: 4,
      simulation: { ...state.simulation, insightTokensByReactorId: { "reactor-1": 1 } }
    };

    state = applyFleetBoardAction(state, { type: "tickDay" });

    expect(state.facilities["reactor-1"]?.status).toBe("active");
    expect(state.simulation.insightTokensByReactorId["reactor-1"]).toBe(0);
    expect(state.events.at(-1)).toEqual(
      expect.objectContaining({
        kind: "insightTokenSpent",
        facilityId: "reactor-1",
        detail: expect.stringContaining("Trouble")
      })
    );
  });

  it("does not spend Insight Tokens on a fuel-driven refueling outage", () => {
    let state = createInitialFleetBoardState({ seed: "refueling-insight", fuelBlocks: 0 });
    state = applyFleetBoardAction(state, {
      type: "placeFacility",
      facilityId: "reactor-1",
      facilityKind: "reactor",
      position: { x: 5, y: 2 }
    });
    state = {
      ...state,
      simulation: { ...state.simulation, insightTokensByReactorId: { "reactor-1": 1 } }
    };

    state = applyFleetBoardAction(state, { type: "tickDay" });

    expect(state.facilities["reactor-1"]?.status).toBe("refueling");
    expect(state.simulation.insightTokensByReactorId["reactor-1"]).toBe(1);
    expect(state.events).toEqual(
      expect.arrayContaining([expect.objectContaining({ kind: "refuelingNeeded", facilityId: "reactor-1" })])
    );
    expect(state.events.some((event) => event.kind === "insightTokenSpent")).toBe(false);
  });
});
