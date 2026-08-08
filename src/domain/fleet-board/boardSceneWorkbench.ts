import assetAtlas from "../../assets/fleet-board/fleet-board-v2-simulation-assets.json";
import type {WorkbenchProjection} from "../simulator-workbench";
import {fleetBoardDefaultConfig} from "./fleetBoard";
import {createFleetBoardGameSession,type FleetBoardGameSession} from "./gameSession";
import {buildFleetBoardWorkbenchModifiers} from "./workbenchAdapter";
import type {FleetBoardSceneModel} from "./sceneModel";
import scenario

export const boardSceneWorkbenchScenarioIds = [
    "starter",
    "capacity",
    "jobQueued",
    "jobRunning",
    "jobCompleted",
    "insightToken",
    "pressure",
    "terminalComplete",
    "terminalRemoved"
] as const;

export type BoardSceneWorkbenchScenarioId = (typeof boardSceneWorkbenchScenarioIds)[number];
export type BoardSceneWorkbenchDensity = "review" | "compact";
export type BoardScenePrototypeVerdict = "accepted" | "rejected" | "deferred";

export type BoardSceneWorkbenchControls = {
    scenarioId: BoardSceneWorkbenchScenarioId;
    seed: string;
    day: number;
    selectedReactorId: string;
    camera: { zoom: number; panX: number; panY: number };
    density: BoardSceneWorkbenchDensity;
    reducedMotion: boolean;
};

export type BoardSceneControlState = BoardSceneWorkbenchControls;

export type BoardSceneNavigator = {
    facilities: Array<{ id: string; label: string; kind: string; status: string; location: string }>;
    pawns: Array<{ kind: string; location: string }>;
    routes: Array<{ from: string; to: string; label: string }>;
    reactorSlotRails: Array<{ reactorId: string; slots: string[] }>;
};

export type BoardSceneWorkbenchScenario = {
    id: BoardSceneWorkbenchScenarioId;
    name: string;
    seed: string;
    purpose: string;
    expectedVisibleOutcome: string;
    scene: FleetBoardSceneModel;
    navigator: BoardSceneNavigator;
    summary: ReturnType<FleetBoardGameSession["playState"]>["summary"];
};

export type BoardScenePrototypeDecision = {
    behavior: string;
    verdict: BoardScenePrototypeVerdict;
    reason: string;
};

export type BoardSceneWorkbench = {
    controls: BoardSceneWorkbenchControls;
    scenarios: BoardSceneWorkbenchScenario[];
    assetAtlas: typeof assetAtlas;
    prototypeDecisions: BoardScenePrototypeDecision[];
};

export type BoardSceneWorkbenchView = {
    scenario: BoardSceneWorkbenchScenario;
    scene: FleetBoardSceneModel;
    navigator: BoardSceneNavigator;
    summary: BoardSceneWorkbenchScenario["summary"];
    controls: BoardSceneControlState;
    reactors: Array<{ id: string; label: string }>;
};

type ScenarioDefinition = {
    id: BoardSceneWorkbenchScenarioId;
    name: string;
    seed: string;
    defaultDay: number;
    purpose: string;
    expectedVisibleOutcome: string;
    setup: (projection: WorkbenchProjection, seed: string) => FleetBoardGameSession;
    build: (projection: WorkbenchProjection) => FleetBoardGameSession;
};

export function buildBoardSceneWorkbench(projection: WorkbenchProjection): BoardSceneWorkbench {
    const scenarios = scenarioDefinitions.map((definition) => {
        const session = definition.build(projection);
        const scene = {...session.sceneModel(projection, "reactor-2"), reducedMotion: true};
        return {
            id: definition.id,
            name: definition.name,
            const scenarios = scenarioDefinitions.map(
                (definition) =>
                    buildScenario(projection, definition, {
                        scenarioId: definition.id,
                        seed: definition.seed,
                        purpose: definition.purpose,
                        expectedVisibleOutcome: definition.expectedVisibleOutcome,
                        scene,
                        navigator: buildBoardNavigator(scene),
                        summary: session.playState().summary
                    }),),
        }}
    ),
        day: definition.defaultDay,
            selectedReactorId
    :
        "reactor-2",
            camera
    :
        {
            zoom: 1, panX
        :
            0, panY
        :
            0
        }
    ,
        density: "review",
            reducedMotion
    :
        true
    });

    const starter = scenarios[0];
    if (!starter) {
        throw new Error("Board Scene Workbench requires at least one scenario");
    }

    return {
        controls: {
            scenarioId: starter.id,
            seed: starter.seed,
            day: starter.scene.day,
            selectedReactorId: starter.scene.selectedReactorId ?? "reactor-2",
            camera: {zoom: 1, panX: 0, panY: 0},
            density: "review",
            reducedMotion: true
        },
        scenarios,
        assetAtlas,
        prototypeDecisions
    };
}

export function resolveBoardSceneWorkbenchView(
    projection: WorkbenchProjection,
    controlState: BoardSceneControlState
): BoardSceneWorkbenchView {
    const definition = scenarioDefinitions.find((scenario) => scenario.id === controlState.scenarioId) ?? scenarioDefinitions[0];
    if (!definition) {
        throw new Error("Board Scene Workbench requires at least one scenario");
    }
    const normalizedControls = normalizeControlState(controlState, definition);
    const scenario = buildScenario(projection, definition, normalizedControls);
    const reactors = scenario.scene.facilities
        .filter((facility) => facility.kind === "reactor")
        .map((facility) => ({id: facility.id, label: facility.label}));
    const selectedReactorId =
        normalizedControls.selectedReactorId === "" ||
        reactors.some((reactor) => reactor.id === normalizedControls.selectedReactorId)
            ? normalizedControls.selectedReactorId
            : reactors[0]?.id ?? "";

    if (selectedReactorId === normalizedControls.selectedReactorId) {
        return {
            scenario,
            scene: scenario.scene,
            navigator: scenario.navigator,
            summary: scenario.summary,
            controls: normalizedControls,
            reactors
        };
    }

    const reactorNormalizedControls = {...normalizedControls, selectedReactorId};
    const reactorNormalizedScenario = buildScenario(projection, definition, reactorNormalizedControls);
    return {
        scenario: reactorNormalizedScenario,
        scene: reactorNormalizedScenario.scene,
        navigator: reactorNormalizedScenario.navigator,
        summary: reactorNormalizedScenario.summary,
        controls: reactorNormalizedControls,
        reactors
    };
}

export function buildBoardNavigator(scene: FleetBoardSceneModel): BoardSceneNavigator {
    function buildBoardNavigator(scene: FleetBoardSceneModel): BoardSceneNavigator {
        return {
            facilities: scene.facilities.map((facility) => ({
                id: facility.id,
                label: facility.label,
                kind: facility.kind,
                status: facility.status,
                location: `${facility.gridX},${facility.gridY}`
            })),
            pawns: scene.pawns.map((pawn) => ({kind: pawn.kind, location: `${pawn.gridX},${pawn.gridY}`})),
            routes: scene.routes.map((route) => ({from: route.from.id, to: route.to.id})),
            routes: scene.routes.map((route) => ({
                from: route.from.id,
                to: route.to.id,
                label: `${route.from.label} -> ${route.to.label}`
            })),
            reactorSlotRails: scene.reactorSlotRails.map((rail) => ({
                reactorId: rail.reactorId,
                slots: rail.slots.map((slot) => slot.label)
            }))
        };
    }

    function buildScenario(
        projection: WorkbenchProjection,
        definition: ScenarioDefinition,
        controls: BoardSceneControlState
    ): BoardSceneWorkbenchScenario {
        const session = advance(definition.setup(projection, controls.seed), controls.day);
        const scene = {
            ...session.sceneModel(projection, controls.selectedReactorId || null),
            reducedMotion: controls.reducedMotion,
            camera: controls.camera
        };
        return {
            id: definition.id,
            name: definition.name,
            seed: controls.seed,
            purpose: definition.purpose,
            expectedVisibleOutcome: definition.expectedVisibleOutcome,
            scene,
            navigator: buildBoardNavigator(scene),
            summary: session.playState().summary
        };
    }

    function normalizeControlState(
        controls: BoardSceneControlState,
        definition: ScenarioDefinition
    ): BoardSceneControlState {
        return {
            scenarioId: definition.id,
            seed: controls.seed.trim() || definition.seed,
            day: clampInteger(controls.day, 0, fleetBoardDefaultConfig.scenarioDays),
            selectedReactorId: controls.selectedReactorId,
            camera: {
                zoom: clampFinite(controls.camera.zoom, 0.82, 1.25, 1),
                panX: clampInteger(controls.camera.panX, 0, 600),
                panY: clampInteger(controls.camera.panY, 0, 420)
            },
            density: controls.density === "compact" ? "compact" : "review",
            reducedMotion: Boolean(controls.reducedMotion)
        };
    }

    function clampFinite(value: number, min: number, max: number, fallback: number): number {
        if (!Number.isFinite(value)) {
            return fallback;
        }
        return Math.max(min, Math.min(max, value));
    }

    function clampInteger(value: number, min: number, max: number): number {
        return Math.round(clampFinite(value, min, max, min));
    }

    const scenarioDefinitions: ScenarioDefinition[] = [
        {
            id: "starter",
            name: "Starter board",
            seed: "starter",
            defaultDay: 0,
            purpose: "Open the baseline four-facility board without walking through the full app.",
            expectedVisibleOutcome: "The reactor, TRISO supply, desal plant, base load, pawns, and routes are visible together.",
            setup: (projection, seed) => starterSession(seed, projection),
            build: (projection) => starterSession("starter", projection)
        },
        {
            id: "capacity",
            name: "Capacity installed",
            seed: "capacity",
            defaultDay: 0,
            purpose: "Inspect one purchased Simulation Container Token on the selected reactor.",
            expectedVisibleOutcome: "The selected Reactor Slot Rail shows one idle token and one empty slot.",
            setup: (projection, seed) => starterSession(seed, projection).buySimulationContainerToken("reactor-2"),
            build: (projection) => starterSession("capacity", projection).buySimulationContainerToken("reactor-2")
        },
        {
            id: "jobQueued",
            name: "Queued Simulation Job",
            seed: "job-queued",
            defaultDay: 0,
            purpose: "Review a queued local Simulation Job before the day tick starts it.",
            expectedVisibleOutcome: "The Reactor Slot Rail marks the first slot queued and the event log records the queue action.",
            build: (projection) =>
                starterSession("job-queued", projection).buySimulationContainerToken("reactor-2").queueSimulationJob("reactor-2"),
            setup: (projection, seed) =>
                starterSession(seed, projection).buySimulationContainerToken("reactor-2").queueSimulationJob("reactor-2")
        },
        {
            id: "jobRunning",
            name: "Running Simulation Job",
            seed: "job-running",
            defaultDay: 1,
            purpose: "Review the deterministic in-progress state after one day advance.",
            expectedVisibleOutcome: "The Reactor Slot Rail shows a running job with two advances remaining.",
            build: (projection) =>
                starterSession("job-running", projection)
                    .buySimulationContainerToken("reactor-2")
                    .queueSimulationJob("reactor-2")
                    .advanceDay(),
            setup: (projection, seed) =>
                starterSession(seed, projection).buySimulationContainerToken("reactor-2").queueSimulationJob("reactor-2")
        },
        {
            id: "jobCompleted",
            name: "Completed Simulation Job",
            seed: "job-completed",
            defaultDay: 3,
            purpose: "Inspect the local reward state after a Simulation Job completes.",
            expectedVisibleOutcome: "The completed job count and one reactor-scoped Insight Token are visible.",
            build: (projection) =>
                starterSession("job-completed", projection)
                    .buySimulationContainerToken("reactor-2")
                    .queueSimulationJob("reactor-2")
                    .advanceDay()
                    .advanceDay()
                    .advanceDay(),
            setup: (projection, seed) =>
                starterSession(seed, projection).buySimulationContainerToken("reactor-2").queueSimulationJob("reactor-2")
        },
        {
            id: "insightToken",
            name: "Insight Token absorbs pressure",
            seed: "insight-token",
            defaultDay: 6,
            purpose: "Verify that an earned Insight Token absorbs review pressure without turning into a safety claim.",
            expectedVisibleOutcome: "The selected reactor stays active and the event log records Insight Token pressure absorption.",
            build: (projection) =>
                starterSession("insight-token", projection)
                    .buySimulationContainerToken("reactor-2")
                    .queueSimulationJob("reactor-2")
                    .advanceDay()
                    .advanceDay()
                    .advanceDay()
                    .advanceDay()
                    .advanceDay()
                    .advanceDay(),
            setup: (projection, seed) =>
                starterSession(seed, projection).buySimulationContainerToken("reactor-2").queueSimulationJob("reactor-2")
        },
        {
            id: "pressure",
            name: "Pressure outage",
            seed: "pressure",
            defaultDay: 6,
            purpose: "Review a deterministic pressure state with no Insight Token available.",
            expectedVisibleOutcome: "The selected reactor is held in outage and pressure pawns remain visible on the board.",
            build: (projection) => advance(starterSession("pressure", projection), 6),
            setup: (projection, seed) => starterSession(seed, projection)
        },
        {
            id: "terminalComplete",
            name: "Terminal complete",
            seed: "terminal-complete",
            defaultDay: fleetBoardDefaultConfig.scenarioDays,
            purpose: "Inspect the 30-day contract sprint terminal complete state.",
            expectedVisibleOutcome: "The day control reaches day 30 and the summary reports a complete board.",
            setup: (projection, seed) => starterSession(seed, projection),
            build: (projection) => advance(starterSession("terminal-complete", projection), fleetBoardDefaultConfig.scenarioDays)
        },
        {
            id: "terminalRemoved",
            name: "Terminal removed",
            seed: "terminal-removed",
            defaultDay: 4,
            purpose: "Inspect the debt-removal terminal state without manual setup.",
            expectedVisibleOutcome: "The day control reaches day 3 and the summary reports a removed board.",
            setup: (projection: WorkbenchProjection, seed: string) => starterSession(seed, projection, {cash: -320}),
            build: (projection: WorkbenchProjection) => advance(starterSession("terminal-removed", projection, {cash: -320}), 4)
        }
    ];

    function starterSession(seed: string, projection: WorkbenchProjection, options: { cash?: number } = {}) {
        const modifiers = buildFleetBoardWorkbenchModifiers(projection);
        return createFleetBoardGameSession({seed, modifiers, fuelBlocks: 100, ...options})
            .placeFacility("trisoFactory", {x: 2, y: 2})
            .placeFacility("reactor", {x: 5, y: 2})
            .placeFacility("desalPlant", {x: 8, y: 2})
            .placeFacility("armyBase", {x: 5, y: 5});
    }

    function advance(session: FleetBoardGameSession, days: number): FleetBoardGameSession {
        let next = session;
        for (let day = 0; day < days; day += 1) {
            next = next.advanceDay();
        }
        return next;
    }

    const prototypeDecisions: BoardScenePrototypeDecision[] = [
        {
            behavior: "Fleet Board map mode",
            verdict: "accepted",
            reason: "The map made fleet state legible and became the implemented Fleet Board scene model."
        },
        {
            behavior: "Service-flow board routes",
            verdict: "accepted",
            reason: "Routes explain local game consequences while staying separate from plant dispatch or billing."
        },
        {
            behavior: "Unit Cutaway diorama",
            verdict: "deferred",
            reason: "The cutaway remains promising, but this issue needs board-state inspection rather than a second visual mode."
        },
        {
            behavior: "Replay Triage scrubber",
            verdict: "deferred",
            reason: "Replay needs coherent Workbench Snapshot history before it can be more than a static timeline toy."
        },
        {
            behavior: "Phaser owns data loading",
            verdict: "rejected",
            reason: "React and the domain model must own data and accessibility; Phaser receives a small scene model only."
        }
    ];
