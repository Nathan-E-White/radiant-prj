import {describe, expect, it} from "vitest";
import {buildWorkbenchProjection, loadFixtureWorkbenchData} from "../simulator-workbench";
import {
    buildBoardSceneWorkbench,
    boardSceneWorkbenchScenarioIds,
    type BoardSceneWorkbenchScenarioId, resolveBoardSceneWorkbenchView
} from "./boardSceneWorkbench";

describe("Board Scene Workbench", () => {
    it("catalogs deterministic board scenes required for contributor review", () => {
        const projection = buildWorkbenchProjection(loadFixtureWorkbenchData(), {selectedUnitId: "KAL-03"});
        const workbench = buildBoardSceneWorkbench(projection);

        expect(workbench.scenarios.map((scenario) => scenario.id)).toEqual([
            "starter",
            "capacity",
            "jobQueued",
            "jobRunning",
            "jobCompleted",
            "insightToken",
            "pressure",
            "terminalComplete",
            "terminalRemoved"
        ]);
        expect(new Set(workbench.scenarios.map((scenario) => scenario.seed)).size).toBe(workbench.scenarios.length);
        expect(workbench.scenarios.every((scenario) => scenario.purpose && scenario.expectedVisibleOutcome)).toBe(true);
        expect(workbench.scenarios.map((scenario) => scenario.scene.day)).toEqual([0, 0, 0, 1, 3, 6, 6, 30, 3]);
        expect(workbench.controls).toMatchObject({
            scenarioId: "starter",
            seed: "starter",
            day: 0,
            selectedReactorId: "reactor-2",
            camera: {zoom: 1, panX: 0, panY: 0},
            density: "review",
            reducedMotion: true
        });
    });

    it("keeps each required scene inspectable through the scene model and Board Navigator", () => {
        const projection = buildWorkbenchProjection(loadFixtureWorkbenchData(), {selectedUnitId: "KAL-03"});
        const workbench = buildBoardSceneWorkbench(projection);
        const byId = scenarioMap(workbench.scenarios);

        expect(byId.capacity.scene.reactorSlotRails[0]?.slots.map((slot) => slot.status)).toEqual(["idle", "empty"]);
        expect(byId.jobQueued.scene.reactorSlotRails[0]?.slots.map((slot) => slot.status)).toEqual(["queued", "empty"]);
        expect(byId.jobRunning.scene.reactorSlotRails[0]?.slots.map((slot) => slot.status)).toEqual(["running", "empty"]);
        expect(byId.jobCompleted.scene.insightTokenBadges).toEqual([
            expect.objectContaining({reactorId: "reactor-2", count: 1})
        ]);
        expect(byId.insightToken.scene.facilities.find((facility) => facility.id === "reactor-2")?.status).toBe("active");
        expect(byId.pressure.scene.facilities.find((facility) => facility.id === "reactor-2")?.status).toBe("outage");
        expect(byId.terminalComplete.summary.complete).toBe(true);
        expect(byId.terminalRemoved.summary.removed).toBe(true);

        for (const scenarioId of boardSceneWorkbenchScenarioIds) {
            const scenario = byId[scenarioId];
            expect(scenario.navigator.facilities.length).toBe(scenario.scene.facilities.length);
            expect(scenario.navigator.routes.length).toBe(scenario.scene.routes.length);
            expect(scenario.navigator.routes.map((route) => route.label)).toEqual(
                scenario.scene.routes.map((route) => `${route.from.label} -> ${route.to.label}`)
            );
            expect(scenario.navigator.reactorSlotRails.length).toBe(scenario.scene.reactorSlotRails.length);
            expect(scenario.scene.pawns.map((pawn) => pawn.kind)).toEqual(["inspector", "trouble"]);
        }
        expect(byId.starter.navigator.routes).toEqual(
            expect.arrayContaining([expect.objectContaining({label: "Reactor -> TRISO Supply"})])
        );
    });

    it("resolves effective scene controls into a normalized contributor review view", () => {
        const projection = buildWorkbenchProjection(loadFixtureWorkbenchData(), {selectedUnitId: "KAL-03"});
        const workbench = buildBoardSceneWorkbench(projection);
        const view = resolveBoardSceneWorkbenchView(projection, {
            ...workbench.controls,
            scenarioId: "jobQueued",
            seed: "custom-seed-42",
            day: 2,
            selectedReactorId: "",
            camera: {zoom: 2, panX: 24, panY: 12},
            density: "compact",
            reducedMotion: false
        });

        expect(view.controls).toEqual({
            scenarioId: "jobQueued",
            seed: "custom-seed-42",
            day: 2,
            selectedReactorId: "",
            camera: {zoom: 1.25, panX: 24, panY: 12},
            density: "compact",
            reducedMotion: false
        });
        expect(view.scene.selectedReactorId).toBeNull();
        expect(view.scene.day).toBe(2);
        expect(view.scene.camera).toEqual({zoom: 1.25, panX: 24, panY: 12});
        expect(view.scene.reducedMotion).toBe(false);
        expect(view.navigator.routes).toEqual(
            expect.arrayContaining([expect.objectContaining({label: "Reactor -> TRISO Supply"})])
        );
        expect(view.reactors).toEqual([expect.objectContaining({id: "reactor-2", label: "Reactor"})]);

        const clampedView = resolveBoardSceneWorkbenchView(projection, {...workbench.controls, day: 99});
        expect(clampedView.controls.day).toBe(30);
    });

    it("exposes the asset atlas and records prototype decisions with accepted, rejected, or deferred verdicts", () => {
        const projection = buildWorkbenchProjection(loadFixtureWorkbenchData());
        const workbench = buildBoardSceneWorkbench(projection);

        expect(workbench.assetAtlas.assets.map((asset) => asset.semanticKey)).toEqual(
            expect.arrayContaining([
                "simulation-container-token",
                "reactor-slot-rail-empty",
                "reactor-slot-rail-idle",
                "reactor-slot-rail-queued",
                "reactor-slot-rail-running",
                "simulation-job-completed",
                "insight-token"
            ])
        );
        expect(workbench.prototypeDecisions).toEqual(
            expect.arrayContaining([
                expect.objectContaining({behavior: "Fleet Board map mode", verdict: "accepted"}),
                expect.objectContaining({behavior: "Unit Cutaway diorama", verdict: "deferred"}),
                expect.objectContaining({behavior: "Phaser owns data loading", verdict: "rejected"})
            ])
        );
        expect(workbench.prototypeDecisions.every((decision) => decision.reason.length > 12)).toBe(true);
    });
});

function scenarioMap(scenarios: ReturnType<typeof buildBoardSceneWorkbench>["scenarios"]) {
    return Object.fromEntries(scenarios.map((scenario) => [scenario.id, scenario])) as Record<
        BoardSceneWorkbenchScenarioId,
        (typeof scenarios)[number]
    >;
}
