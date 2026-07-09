import type { FleetBoardWorkbenchModifiers } from "./workbenchAdapter";

export type FleetBoardFacilityKind = "trisoFactory" | "reactor" | "desalPlant" | "armyBase" | "battery";
export type FleetBoardFacilityStatus = "active" | "outage" | "refueling";
export type FleetBoardPawnKind = "inspector" | "trouble";

export type FleetBoardPosition = {
  x: number;
  y: number;
};

export type FleetBoardFacility = {
  id: string;
  kind: FleetBoardFacilityKind;
  label: string;
  position: FleetBoardPosition;
  status: FleetBoardFacilityStatus;
  outageDaysRemaining: number;
};

export type FleetBoardPawn = {
  kind: FleetBoardPawnKind;
  position: FleetBoardPosition;
  routeIndex: number;
};

export type FleetBoardResources = {
  cash: number;
  fuelBlocks: number;
  electricMwe: number;
  thermalMwt: number;
  waterCredits: number;
  resilienceCredits: number;
};

export type FleetBoardEventKind =
  | "facilityPlaced"
  | "fuelProduced"
  | "reactorGenerated"
  | "refuelingNeeded"
  | "refueled"
  | "inspectorHold"
  | "trouble"
  | "debtWarning"
  | "debtRemoval"
  | "scenarioComplete";

export type FleetBoardEvent = {
  id: string;
  day: number;
  kind: FleetBoardEventKind;
  detail: string;
  facilityId?: string;
};

export type FleetBoardScore = {
  water: number;
  resilience: number;
  cash: number;
  continuity: number;
  total: number;
};

export type FleetBoardConfig = {
  scenarioDays: number;
  startingCash: number;
  startingFuelBlocks: number;
  debtLimit: number;
  debtGraceDays: number;
  routeRange: number;
  facilityCosts: Record<FleetBoardFacilityKind, number>;
  fuelFactoryRate: number;
  reactorFuelBurnPerDay: number;
  reactorElectricMwe: number;
  reactorThermalMwt: number;
  desalWaterPerThermalMwt: number;
  baseResiliencePerElectricMwe: number;
  electricRevenuePerMwe: number;
  waterRevenuePerCredit: number;
  resilienceRevenuePerCredit: number;
  dailyOperationsCost: number;
};

export type FleetBoardState = {
  seed: string;
  rngState: number;
  day: number;
  resources: FleetBoardResources;
  facilities: Record<string, FleetBoardFacility>;
  pawns: Record<FleetBoardPawnKind, FleetBoardPawn>;
  events: FleetBoardEvent[];
  score: FleetBoardScore;
  debtDays: number;
  removed: boolean;
  complete: boolean;
  config: FleetBoardConfig;
  modifiers: FleetBoardWorkbenchModifiers;
};

export type FleetBoardAction =
  | {
      type: "placeFacility";
      facilityId: string;
      facilityKind: FleetBoardFacilityKind;
      position: FleetBoardPosition;
    }
  | { type: "tickDay" }
  | { type: "refuelFacility"; facilityId: string };

export const fleetBoardDefaultConfig: FleetBoardConfig = {
  scenarioDays: 30,
  startingCash: 1200,
  startingFuelBlocks: 4,
  debtLimit: -250,
  debtGraceDays: 3,
  routeRange: 4,
  facilityCosts: {
    trisoFactory: 180,
    reactor: 360,
    desalPlant: 220,
    armyBase: 180,
    battery: 120
  },
  fuelFactoryRate: 3,
  reactorFuelBurnPerDay: 1,
  reactorElectricMwe: 0.94,
  reactorThermalMwt: 2.6,
  desalWaterPerThermalMwt: 1.4,
  baseResiliencePerElectricMwe: 1.8,
  electricRevenuePerMwe: 12,
  waterRevenuePerCredit: 8,
  resilienceRevenuePerCredit: 10,
  dailyOperationsCost: 18
};

export const fleetBoardNeutralModifiers: FleetBoardWorkbenchModifiers = {
  selectedUnitId: "fixture",
  freshnessRisk: 0,
  inspectorPressure: 0,
  confidenceMultiplier: 1,
  simulatedResultPressure: 0,
  valueBasisCounts: { measured: 0, imputed: 0, simulated: 0 }
};

const facilityLabels: Record<FleetBoardFacilityKind, string> = {
  trisoFactory: "TRISO Supply",
  reactor: "Reactor",
  desalPlant: "Desal Plant",
  armyBase: "Base Load",
  battery: "Battery Sink"
};

const inspectorRoute: FleetBoardPosition[] = [
  { x: 1, y: 1 },
  { x: 9, y: 1 },
  { x: 9, y: 7 },
  { x: 1, y: 7 }
];

const troubleRoute: FleetBoardPosition[] = [
  { x: 10, y: 6 },
  { x: 6, y: 8 },
  { x: 3, y: 5 },
  { x: 7, y: 2 }
];

export function createInitialFleetBoardState({
  seed,
  cash = fleetBoardDefaultConfig.startingCash,
  fuelBlocks = fleetBoardDefaultConfig.startingFuelBlocks,
  modifiers = fleetBoardNeutralModifiers,
  config = fleetBoardDefaultConfig
}: {
  seed: string;
  cash?: number;
  fuelBlocks?: number;
  modifiers?: FleetBoardWorkbenchModifiers;
  config?: FleetBoardConfig;
}): FleetBoardState {
  return recalculateScore({
    seed,
    rngState: hashSeed(seed),
    day: 0,
    resources: {
      cash,
      fuelBlocks,
      electricMwe: 0,
      thermalMwt: 0,
      waterCredits: 0,
      resilienceCredits: 0
    },
    facilities: {},
    pawns: {
      inspector: { kind: "inspector", position: inspectorRoute[0], routeIndex: 0 },
      trouble: { kind: "trouble", position: troubleRoute[0], routeIndex: 0 }
    },
    events: [],
    score: { water: 0, resilience: 0, cash: 0, continuity: 0, total: 0 },
    debtDays: cash < config.debtLimit ? 1 : 0,
    removed: false,
    complete: false,
    config,
    modifiers
  });
}

export function applyFleetBoardAction(state: FleetBoardState, action: FleetBoardAction): FleetBoardState {
  if (state.removed || state.complete) {
    return state;
  }

  if (action.type === "placeFacility") {
    return placeFacility(state, action);
  }

  if (action.type === "refuelFacility") {
    return refuelFacility(state, action.facilityId);
  }

  return tickDay(state);
}

export function summarizeFleetBoard(state: FleetBoardState) {
  return {
    day: state.day,
    cash: Math.round(state.resources.cash),
    fuelBlocks: state.resources.fuelBlocks,
    electricMwe: round(state.resources.electricMwe),
    thermalMwt: round(state.resources.thermalMwt),
    waterCredits: round(state.resources.waterCredits),
    resilienceCredits: round(state.resources.resilienceCredits),
    score: state.score.total,
    removed: state.removed,
    complete: state.complete
  };
}

function placeFacility(
  state: FleetBoardState,
  action: Extract<FleetBoardAction, { type: "placeFacility" }>
): FleetBoardState {
  if (state.facilities[action.facilityId]) {
    return state;
  }

  const cost = state.config.facilityCosts[action.facilityKind];
  const next: FleetBoardState = {
    ...state,
    resources: { ...state.resources, cash: state.resources.cash - cost },
    facilities: {
      ...state.facilities,
      [action.facilityId]: {
        id: action.facilityId,
        kind: action.facilityKind,
        label: facilityLabels[action.facilityKind],
        position: action.position,
        status: "active",
        outageDaysRemaining: 0
      }
    }
  };
  return recalculateScore(addEvent(next, "facilityPlaced", `${facilityLabels[action.facilityKind]} placed`, action.facilityId));
}

function refuelFacility(state: FleetBoardState, facilityId: string): FleetBoardState {
  const facility = state.facilities[facilityId];
  if (!facility || facility.kind !== "reactor") {
    return state;
  }

  const nextFacility: FleetBoardFacility = {
    ...facility,
    status: "active",
    outageDaysRemaining: 0
  };
  const next: FleetBoardState = {
    ...state,
    resources: {
      ...state.resources,
      fuelBlocks: state.resources.fuelBlocks - state.config.reactorFuelBurnPerDay
    },
    facilities: { ...state.facilities, [facilityId]: nextFacility }
  };
  return recalculateScore(addEvent(next, "refueled", "Reactor refueled from abstract TRISO supply", facilityId));
}

function tickDay(state: FleetBoardState): FleetBoardState {
  let next: FleetBoardState = {
    ...state,
    day: state.day + 1,
    resources: {
      ...state.resources,
      cash: state.resources.cash - state.config.dailyOperationsCost
    },
    facilities: tickOutages(state.facilities)
  };

  next = produceFuel(next);
  next = runReactors(next);
  next = movePawns(next);
  next = maybeTriggerInspector(next);
  next = maybeTriggerTrouble(next);
  next = applyDebtRules(next);

  if (!next.removed && next.day >= next.config.scenarioDays) {
    next = addEvent({ ...next, complete: true }, "scenarioComplete", "30-day Fleet Board contract sprint complete");
  }

  return recalculateScore(next);
}

function tickOutages(facilities: Record<string, FleetBoardFacility>): Record<string, FleetBoardFacility> {
  return Object.fromEntries(
    Object.entries(facilities).map(([id, facility]) => {
      if (facility.status === "active" || facility.status === "refueling") {
        return [id, facility];
      }
      const outageDaysRemaining = Math.max(0, facility.outageDaysRemaining - 1);
      return [id, { ...facility, status: outageDaysRemaining === 0 ? "active" : "outage", outageDaysRemaining }];
    })
  );
}

function produceFuel(state: FleetBoardState): FleetBoardState {
  const factoryCount = activeFacilities(state, "trisoFactory").length;
  if (factoryCount === 0) {
    return state;
  }

  const produced = factoryCount * state.config.fuelFactoryRate;
  const next = {
    ...state,
    resources: { ...state.resources, fuelBlocks: state.resources.fuelBlocks + produced }
  };
  return addEvent(next, "fuelProduced", `${produced} abstract TRISO fuel blocks produced`);
}

function runReactors(state: FleetBoardState): FleetBoardState {
  let next = state;
  for (const reactor of activeFacilities(state, "reactor")) {
    if (next.resources.fuelBlocks < next.config.reactorFuelBurnPerDay) {
      next = setFacilityStatus(next, reactor.id, "refueling", 1);
      next = addEvent(next, "refuelingNeeded", "Reactor entered refueling outage", reactor.id);
      continue;
    }

    const electricMwe = next.config.reactorElectricMwe * next.modifiers.confidenceMultiplier;
    const thermalMwt = next.config.reactorThermalMwt * next.modifiers.confidenceMultiplier;
    const waterCredits = connectedFacilityCount(next, reactor, "desalPlant") * thermalMwt * next.config.desalWaterPerThermalMwt;
    const resilienceCredits =
      connectedFacilityCount(next, reactor, "armyBase") * electricMwe * next.config.baseResiliencePerElectricMwe +
      connectedFacilityCount(next, reactor, "battery") * electricMwe * 0.8;
    const revenue =
      electricMwe * next.config.electricRevenuePerMwe +
      waterCredits * next.config.waterRevenuePerCredit +
      resilienceCredits * next.config.resilienceRevenuePerCredit;

    next = {
      ...next,
      resources: {
        ...next.resources,
        fuelBlocks: next.resources.fuelBlocks - next.config.reactorFuelBurnPerDay,
        electricMwe: next.resources.electricMwe + electricMwe,
        thermalMwt: next.resources.thermalMwt + thermalMwt,
        waterCredits: next.resources.waterCredits + waterCredits,
        resilienceCredits: next.resources.resilienceCredits + resilienceCredits,
        cash: next.resources.cash + revenue
      }
    };
    next = addEvent(next, "reactorGenerated", `${round(electricMwe)} MWe and ${round(thermalMwt)} MWt routed`, reactor.id);
  }
  return next;
}

function movePawns(state: FleetBoardState): FleetBoardState {
  return {
    ...state,
    pawns: {
      inspector: movePawn(state.pawns.inspector, inspectorRoute),
      trouble: movePawn(state.pawns.trouble, troubleRoute)
    }
  };
}

function movePawn(pawn: FleetBoardPawn, route: FleetBoardPosition[]): FleetBoardPawn {
  const routeIndex = (pawn.routeIndex + 1) % route.length;
  return { ...pawn, routeIndex, position: route[routeIndex] };
}

function maybeTriggerInspector(state: FleetBoardState): FleetBoardState {
  if (state.day % 6 !== 0) {
    return state;
  }
  const reactor = nearestFacility(state, state.pawns.inspector.position, "reactor");
  if (!reactor || reactor.status !== "active") {
    return state;
  }
  const holdDays = state.modifiers.inspectorPressure > 0.35 ? 2 : 1;
  const next = setFacilityStatus(state, reactor.id, "outage", holdDays);
  return addEvent(next, "inspectorHold", `Inspector hold applied for ${holdDays} day(s)`, reactor.id);
}

function maybeTriggerTrouble(state: FleetBoardState): FleetBoardState {
  if (state.day % 5 !== 0) {
    return state;
  }
  const [roll, rngState] = nextRandom(state.rngState);
  const kinds: FleetBoardFacilityKind[] = ["trisoFactory", "reactor", "desalPlant", "armyBase", "battery"];
  const targetKind = kinds[Math.floor(roll * kinds.length)] ?? "reactor";
  const target = nearestFacility(state, state.pawns.trouble.position, targetKind) ?? nearestFacility(state, state.pawns.trouble.position);
  const next = { ...state, rngState };
  if (!target) {
    return addEvent(next, "trouble", "Trouble pawn crossed an empty board lane");
  }
  if (target.status === "active" && target.kind !== "trisoFactory") {
    return addEvent(setFacilityStatus(next, target.id, "outage", 1), "trouble", "Trouble pawn forced a one-day outage", target.id);
  }
  return addEvent(next, "trouble", "Trouble pawn raised routing pressure", target.id);
}

function applyDebtRules(state: FleetBoardState): FleetBoardState {
  if (state.resources.cash >= state.config.debtLimit) {
    return { ...state, debtDays: 0 };
  }

  const debtDays = state.debtDays + 1;
  if (debtDays > state.config.debtGraceDays) {
    return addEvent({ ...state, debtDays, removed: true }, "debtRemoval", "Removed after sustained debt breach");
  }
  return addEvent({ ...state, debtDays }, "debtWarning", `Debt breach day ${debtDays}`);
}

function activeFacilities(state: FleetBoardState, kind: FleetBoardFacilityKind): FleetBoardFacility[] {
  return Object.values(state.facilities).filter((facility) => facility.kind === kind && facility.status === "active");
}

function connectedFacilityCount(state: FleetBoardState, source: FleetBoardFacility, kind: FleetBoardFacilityKind): number {
  return activeFacilities(state, kind).filter((facility) => distance(source.position, facility.position) <= state.config.routeRange).length;
}

function nearestFacility(
  state: FleetBoardState,
  position: FleetBoardPosition,
  kind?: FleetBoardFacilityKind
): FleetBoardFacility | null {
  const candidates = Object.values(state.facilities).filter((facility) => !kind || facility.kind === kind);
  return (
    candidates.sort((left, right) => distance(position, left.position) - distance(position, right.position))[0] ?? null
  );
}

function setFacilityStatus(
  state: FleetBoardState,
  facilityId: string,
  status: FleetBoardFacilityStatus,
  outageDaysRemaining: number
): FleetBoardState {
  const facility = state.facilities[facilityId];
  if (!facility) {
    return state;
  }
  return {
    ...state,
    facilities: {
      ...state.facilities,
      [facilityId]: { ...facility, status, outageDaysRemaining }
    }
  };
}

function addEvent(
  state: FleetBoardState,
  kind: FleetBoardEventKind,
  detail: string,
  facilityId?: string
): FleetBoardState {
  const event: FleetBoardEvent = {
    id: `FB-${state.day}-${state.events.length + 1}`,
    day: state.day,
    kind,
    detail,
    facilityId
  };
  return { ...state, events: [...state.events, event].slice(-80) };
}

function recalculateScore(state: FleetBoardState): FleetBoardState {
  const water = Math.round(state.resources.waterCredits * 8);
  const resilience = Math.round(state.resources.resilienceCredits * 10);
  const cash = Math.max(0, Math.round(state.resources.cash / 10));
  const activeReactors = activeFacilities(state, "reactor").length;
  const continuity = activeReactors * Math.max(0, state.config.scenarioDays - state.day);
  return { ...state, score: { water, resilience, cash, continuity, total: water + resilience + cash + continuity } };
}

function distance(left: FleetBoardPosition, right: FleetBoardPosition): number {
  return Math.abs(left.x - right.x) + Math.abs(left.y - right.y);
}

function round(value: number): number {
  return Math.round(value * 100) / 100;
}

function hashSeed(seed: string): number {
  let hash = 2166136261;
  for (let index = 0; index < seed.length; index += 1) {
    hash ^= seed.charCodeAt(index);
    hash = Math.imul(hash, 16777619);
  }
  return hash >>> 0;
}

function nextRandom(current: number): [number, number] {
  const next = (Math.imul(current, 1664525) + 1013904223) >>> 0;
  return [next / 0x100000000, next];
}
