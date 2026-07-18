import { describe, expect, it } from "vitest";
import { buildWorkbenchProjection, loadFixtureWorkbenchData } from "../simulator-workbench";
import { fleetBoardDefaultConfig, fleetBoardNeutralModifiers } from "./fleetBoard";
import { createFleetBoardGameSession } from "./gameSession";

describe("Fleet Board game session", () => {
  it("creates deterministic games with opaque session and stable facility identities", () => {
    const create = () =>
      createFleetBoardGameSession({ seed: "stable-session" })
        .placeFacility("trisoFactory", { x: 2, y: 2 })
        .placeFacility("reactor", { x: 5, y: 2 });

    const first = create();
    const second = create();

    expect(first.id).not.toBe(second.id);
    expect(first.placeFacility("battery", { x: 8, y: 5 }).id).toBe(first.id);
    expect({ ...first.playState(), reactorSimulation: undefined }).toEqual({
      ...second.playState(),
      reactorSimulation: undefined
    });
    expect(first.playState().facilities.map((facility) => facility.id)).toEqual([
      "trisoFactory-1",
      "reactor-2"
    ]);
  });

  it("accepts modifier changes without teaching callers the reducer state shape", () => {
    const session = createFleetBoardGameSession({ seed: "modifiers" });
    const updated = session.acceptModifiers({
      selectedUnitId: "KAL-04",
      freshnessRisk: 0.25,
      inspectorPressure: 0.4,
      confidenceMultiplier: 0.85,
      simulatedResultPressure: 0.2,
      valueBasisCounts: { measured: 2, imputed: 1, simulated: 3 }
    });

    expect(updated.id).toBe(session.id);
    expect(updated.playState().modifiers).toEqual(
      expect.objectContaining({ selectedUnitId: "KAL-04", confidenceMultiplier: 0.85 })
    );
    expect(session.playState().modifiers.selectedUnitId).toBe("fixture");
  });

  it("copies mutable configuration, modifier, and position inputs at the session boundary", () => {
    const config = {
      ...fleetBoardDefaultConfig,
      facilityCosts: { ...fleetBoardDefaultConfig.facilityCosts }
    };
    const modifiers = {
      selectedUnitId: "KAL-01",
      freshnessRisk: 0.1,
      inspectorPressure: 0.2,
      confidenceMultiplier: 0.9,
      simulatedResultPressure: 0.3,
      valueBasisCounts: { measured: 1, imputed: 2, simulated: 3 }
    };
    const position = { x: 5, y: 2 };
    const session = createFleetBoardGameSession({ seed: "copied-inputs", config, modifiers }).placeFacility(
      "reactor",
      position
    );

    config.scenarioDays = 1;
    config.facilityCosts.reactor = 999;
    modifiers.selectedUnitId = "mutated";
    modifiers.valueBasisCounts.measured = 999;
    position.x = 99;

    expect(session.playState()).toEqual(
      expect.objectContaining({
        scenarioDays: 30,
        modifiers: expect.objectContaining({
          selectedUnitId: "KAL-01",
          valueBasisCounts: { measured: 1, imputed: 2, simulated: 3 }
        }),
        facilities: [expect.objectContaining({ position: { x: 5, y: 2 } })]
      })
    );
    expect(session.playState().summary.cash).toBe(840);
  });

  it("copies the effective default configuration and modifiers at creation", () => {
    const session = createFleetBoardGameSession({ seed: "copied-defaults" });
    const scenarioDays = fleetBoardDefaultConfig.scenarioDays;
    const selectedUnitId = fleetBoardNeutralModifiers.selectedUnitId;

    try {
      fleetBoardDefaultConfig.scenarioDays = 1;
      fleetBoardNeutralModifiers.selectedUnitId = "mutated-default";

      expect(session.playState().scenarioDays).toBe(scenarioDays);
      expect(session.playState().modifiers.selectedUnitId).toBe(selectedUnitId);
    } finally {
      fleetBoardDefaultConfig.scenarioDays = scenarioDays;
      fleetBoardNeutralModifiers.selectedUnitId = selectedUnitId;
    }
  });

  it("reaches debt removal, pressure outage, and refueling through supported commands", () => {
    let debt = createFleetBoardGameSession({ seed: "debt", cash: -320 });
    for (let day = 0; day < fleetBoardDefaultConfig.debtGraceDays + 1; day += 1) {
      debt = debt.advanceDay();
    }

    expect(debt.playState().summary.removed).toBe(true);
    expect(debt.playState().events.at(-1)?.kind).toBe("debtRemoval");

    let outage = createFleetBoardGameSession({ seed: "outage", fuelBlocks: 100 }).placeFacility(
      "reactor",
      { x: 5, y: 2 }
    );
    for (let day = 0; day < 6; day += 1) {
      outage = outage.advanceDay();
    }

    expect(outage.playState().facilities[0]).toEqual(
      expect.objectContaining({ id: "reactor-1", status: "outage" })
    );
    expect(outage.playState().events).toEqual(
      expect.arrayContaining([expect.objectContaining({ kind: "inspectorHold", facilityId: "reactor-1" })])
    );
  });

  it("runs a local Simulation Job and earns and spends an Insight Token without direct mutation", () => {
    let session = createFleetBoardGameSession({ seed: "insight-session", fuelBlocks: 100 }).placeFacility(
      "reactor",
      { x: 5, y: 2 }
    );
    session = session.buySimulationContainerToken("reactor-1").queueSimulationJob("reactor-1");

    expect(session.playState().reactorSimulation("reactor-1").slots[0]).toEqual(
      expect.objectContaining({ status: "queued", advancesRemaining: 3 })
    );

    session = session.advanceDay().advanceDay().advanceDay();

    expect(session.playState().summary.completedSimulationJobs).toBe(1);
    expect(session.playState().reactorSimulation("reactor-1").insightTokens).toBe(1);

    session = session.advanceDay().advanceDay().advanceDay();

    expect(session.playState().reactorSimulation("reactor-1").insightTokens).toBe(0);
    expect(session.playState().events).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ kind: "insightTokenSpent", facilityId: "reactor-1" })
      ])
    );
  });

  it("keeps Simulation Budget bounded and reactor-scoped through session commands", () => {
    let session = createFleetBoardGameSession({ seed: "capacity-session" })
      .placeFacility("reactor", { x: 2, y: 3 })
      .placeFacility("reactor", { x: 6, y: 3 });

    session = session
      .buySimulationContainerToken("reactor-1")
      .buySimulationContainerToken("reactor-1")
      .buySimulationContainerToken("reactor-1")
      .buySimulationContainerToken("reactor-2");

    expect(session.playState().summary).toEqual(
      expect.objectContaining({
        simulationBudget: 0,
        simulationContainerTokens: 3,
        simulationContainerTokensByReactorId: { "reactor-1": 2, "reactor-2": 1 }
      })
    );
    expect(session.playState().events).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          kind: "simulationPurchaseBlocked",
          facilityId: "reactor-1"
        })
      ])
    );
  });

  it("does not spend an earned Insight Token on fuel-driven refueling", () => {
    let session = createFleetBoardGameSession({ seed: "refueling-session", fuelBlocks: 0 })
      .placeFacility("reactor", { x: 5, y: 2 })
      .buySimulationContainerToken("reactor-1")
      .queueSimulationJob("reactor-1")
      .advanceDay()
      .advanceDay()
      .advanceDay();

    expect(session.playState().facilities[0]?.status).toBe("refueling");
    expect(session.playState().reactorSimulation("reactor-1").insightTokens).toBe(1);
    expect(session.playState().events.some((event) => event.kind === "insightTokenSpent")).toBe(false);
  });

  it("projects a render-ready scene without exposing mutable session resources", () => {
    const projection = buildWorkbenchProjection(loadFixtureWorkbenchData());
    const session = createFleetBoardGameSession({ seed: "scene-session" }).placeFacility("reactor", {
      x: 5,
      y: 2
    });
    const scene = session.sceneModel(projection, "reactor-1");
    const originalCash = session.playState().summary.cash;
    const originalMeasuredCount = projection.valueBasisSummary.measured;

    scene.resources.cash = -999;
    scene.valueBasisCounts.measured = -999;

    expect(scene.selectedReactorId).toBe("reactor-1");
    expect(scene.facilities[0]).toEqual(expect.objectContaining({ id: "reactor-1", spriteKey: "reactor" }));
    expect(session.playState().summary.cash).toBe(originalCash);
    expect(projection.valueBasisSummary.measured).toBe(originalMeasuredCount);
  });
});
