import type { WorkbenchProjection } from "../simulator-workbench";
import {
  applyFleetBoardAction,
  createInitialFleetBoardState,
  fleetBoardDefaultConfig,
  fleetBoardNeutralModifiers,
  summarizeFleetBoard,
  summarizeReactorSimulation,
  type FleetBoardConfig,
  type FleetBoardEvent,
  type FleetBoardFacility,
  type FleetBoardFacilityKind,
  type FleetBoardPosition,
  type FleetBoardState
} from "./fleetBoard";
import { buildFleetBoardSceneModel, type FleetBoardSceneModel } from "./sceneModel";
import type { FleetBoardWorkbenchModifiers } from "./workbenchAdapter";

export type FleetBoardPlayState = Readonly<{
  summary: ReturnType<typeof summarizeFleetBoard>;
  facilities: readonly FleetBoardFacility[];
  events: readonly FleetBoardEvent[];
  modifiers: FleetBoardWorkbenchModifiers;
  scenarioDays: number;
  reactorSimulation: (reactorId: string) => ReturnType<typeof summarizeReactorSimulation>;
}>;

export type CreateFleetBoardGameSessionOptions = {
  seed: string;
  cash?: number;
  fuelBlocks?: number;
  modifiers?: FleetBoardWorkbenchModifiers;
  config?: FleetBoardConfig;
};

declare const fleetBoardGameSessionIdBrand: unique symbol;
export type FleetBoardGameSessionId = string & { readonly [fleetBoardGameSessionIdBrand]: true };

export class FleetBoardGameSession {
  readonly id: FleetBoardGameSessionId;
  #state: FleetBoardState;

  private constructor(id: FleetBoardGameSessionId, state: FleetBoardState) {
    this.id = id;
    this.#state = state;
  }

  static create(options: CreateFleetBoardGameSessionOptions): FleetBoardGameSession {
    return new FleetBoardGameSession(
      createSessionId(),
      createInitialFleetBoardState({
        ...options,
        config: cloneConfig(options.config ?? fleetBoardDefaultConfig),
        modifiers: cloneModifiers(options.modifiers ?? fleetBoardNeutralModifiers)
      })
    );
  }

  acceptModifiers(modifiers: FleetBoardWorkbenchModifiers): FleetBoardGameSession {
    return this.withState({ ...this.#state, modifiers: cloneModifiers(modifiers) });
  }

  placeFacility(kind: FleetBoardFacilityKind, position: FleetBoardPosition): FleetBoardGameSession {
    return this.transition({
      type: "placeFacility",
      facilityId: `${kind}-${Object.keys(this.#state.facilities).length + 1}`,
      facilityKind: kind,
      position: { ...position }
    });
  }

  advanceDay(): FleetBoardGameSession {
    return this.transition({ type: "tickDay" });
  }

  refuelReactor(reactorId: string): FleetBoardGameSession {
    return this.transition({ type: "refuelFacility", facilityId: reactorId });
  }

  buySimulationContainerToken(reactorId: string): FleetBoardGameSession {
    return this.transition({ type: "buySimulationContainerToken", reactorId });
  }

  queueSimulationJob(reactorId: string): FleetBoardGameSession {
    return this.transition({ type: "queueSimulationJob", reactorId });
  }

  playState(): FleetBoardPlayState {
    const state = this.#state;
    const summary = summarizeFleetBoard(state);
    return {
      summary: {
        ...summary,
        simulationContainerTokensByReactorId: { ...summary.simulationContainerTokensByReactorId }
      },
      facilities: Object.values(state.facilities).map((facility) => ({
        ...facility,
        position: { ...facility.position }
      })),
      events: state.events.map((event) => ({ ...event })),
      modifiers: {
        ...state.modifiers,
        valueBasisCounts: { ...state.modifiers.valueBasisCounts }
      },
      scenarioDays: state.config.scenarioDays,
      reactorSimulation: (reactorId) => {
        const simulation = summarizeReactorSimulation(state, reactorId);
        return { slots: simulation.slots.map((slot) => ({ ...slot })), insightTokens: simulation.insightTokens };
      }
    };
  }

  sceneModel(projection: WorkbenchProjection, selectedReactorId: string | null = null): FleetBoardSceneModel {
    return buildFleetBoardSceneModel(projection, this.#state, selectedReactorId);
  }

  private transition(action: Parameters<typeof applyFleetBoardAction>[1]): FleetBoardGameSession {
    return this.withState(applyFleetBoardAction(this.#state, action));
  }

  private withState(state: FleetBoardState): FleetBoardGameSession {
    return state === this.#state ? this : new FleetBoardGameSession(this.id, state);
  }
}

export function createFleetBoardGameSession(
  options: CreateFleetBoardGameSessionOptions
): FleetBoardGameSession {
  return FleetBoardGameSession.create(options);
}

function createSessionId(): FleetBoardGameSessionId {
  return `fleet-board-session-${globalThis.crypto.randomUUID()}` as FleetBoardGameSessionId;
}

function cloneConfig(config: FleetBoardConfig): FleetBoardConfig {
  return { ...config, facilityCosts: { ...config.facilityCosts } };
}

function cloneModifiers(modifiers: FleetBoardWorkbenchModifiers): FleetBoardWorkbenchModifiers {
  return { ...modifiers, valueBasisCounts: { ...modifiers.valueBasisCounts } };
}
