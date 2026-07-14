import { BatteryCharging, CalendarClock, Cpu, Droplets, Factory, Fuel, Gauge, Shield, Zap } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import type { WorkbenchProjection } from "../../domain/simulator-workbench";
import {
  applyFleetBoardAction,
  buildFleetBoardSceneModel,
  buildFleetBoardWorkbenchModifiers,
  createInitialFleetBoardState,
  summarizeFleetBoard,
  type FleetBoardFacilityKind,
  type FleetBoardState
} from "../../domain/fleet-board";
import { FleetBoardCanvas } from "./FleetBoardCanvas";

const starterFacilities: Array<{ kind: FleetBoardFacilityKind; x: number; y: number }> = [
  { kind: "trisoFactory", x: 2, y: 2 },
  { kind: "reactor", x: 5, y: 2 },
  { kind: "desalPlant", x: 8, y: 2 },
  { kind: "armyBase", x: 5, y: 5 }
];

const buildOptions: Array<{
  kind: FleetBoardFacilityKind;
  label: string;
  icon: typeof Factory;
  position: { x: number; y: number };
}> = [
  { kind: "trisoFactory", label: "TRISO Supply", icon: Factory, position: { x: 2, y: 2 } },
  { kind: "reactor", label: "Reactor", icon: Gauge, position: { x: 5, y: 2 } },
  { kind: "desalPlant", label: "Desal Plant", icon: Droplets, position: { x: 8, y: 2 } },
  { kind: "armyBase", label: "Base Load", icon: Shield, position: { x: 5, y: 5 } },
  { kind: "battery", label: "Battery Sink", icon: BatteryCharging, position: { x: 8, y: 5 } }
];

export function FleetBoardSurface({ projection }: { projection: WorkbenchProjection }) {
  const modifiers = useMemo(() => buildFleetBoardWorkbenchModifiers(projection), [projection]);
  const [gameState, setGameState] = useState(() => createStarterState(modifiers));
  const [selectedReactorId, setSelectedReactorId] = useState<string | null>(null);

  useEffect(() => {
    setGameState((current) => ({ ...current, modifiers }));
  }, [modifiers]);

  const scene = useMemo(
    () => buildFleetBoardSceneModel(projection, gameState, selectedReactorId),
    [gameState, projection, selectedReactorId]
  );
  const summary = summarizeFleetBoard(gameState);

  const placeFacility = useCallback((facilityKind: FleetBoardFacilityKind, x: number, y: number) => {
    setGameState((current) =>
      applyFleetBoardAction(current, {
        type: "placeFacility",
        facilityId: `${facilityKind}-${Object.keys(current.facilities).length + 1}`,
        facilityKind,
        position: { x, y }
      })
    );
  }, []);

  const tickDay = useCallback(() => {
    setGameState((current) => applyFleetBoardAction(current, { type: "tickDay" }));
  }, []);

  const refuelFirstReactor = useCallback(() => {
    setGameState((current) => {
      const reactor = Object.values(current.facilities).find((facility) => facility.kind === "reactor");
      return reactor ? applyFleetBoardAction(current, { type: "refuelFacility", facilityId: reactor.id }) : current;
    });
  }, []);

  const buySimulationContainerToken = useCallback(() => {
    if (!selectedReactorId) {
      return;
    }
    setGameState((current) =>
      applyFleetBoardAction(current, { type: "buySimulationContainerToken", reactorId: selectedReactorId })
    );
  }, [selectedReactorId]);

  return (
    <section className="fleet-board-shell" aria-label="Fleet Board">
      <div className="fleet-board-head">
        <div>
          <p className="eyebrow">Fleet Board</p>
          <h3>30-day contract sprint</h3>
        </div>
        <div className="fleet-board-score" aria-label="Fleet Board score">
          <span>
            <CalendarClock size={16} /> Day {summary.day}/30
          </span>
          <strong>{summary.score} pts</strong>
        </div>
      </div>

      <div className="fleet-board-readout" aria-label="Fleet Board resource readout">
        <span>
          <Fuel size={15} /> {summary.fuelBlocks} fuel blocks
        </span>
        <span>
          <Zap size={15} /> {summary.electricMwe} MWe
        </span>
        <span>{summary.thermalMwt} MWt</span>
        <span>
          <Droplets size={15} /> {summary.waterCredits} water
        </span>
        <span>
          <Shield size={15} /> {summary.resilienceCredits} resilience
        </span>
        <span className={summary.cash < 0 ? "fleet-board-negative" : ""}>${summary.cash}</span>
        <span data-testid="fleet-board-facility-count">{Object.keys(gameState.facilities).length} facilities</span>
        <span data-testid="fleet-board-simulation-budget" aria-live="polite">
          <Cpu size={15} /> {summary.simulationBudget} Simulation Budget
        </span>
        <span data-testid="fleet-board-simulation-container-tokens" aria-live="polite">
          {summary.simulationContainerTokens} Simulation Container Tokens
        </span>
      </div>

      <div className="fleet-board-layout">
        <aside className="fleet-board-controls" aria-label="Fleet Board controls">
          <div className="fleet-board-control-group">
            <h4>Build Palette</h4>
            {buildOptions.map((option) => {
              const Icon = option.icon;
              return (
                <button
                  key={option.kind}
                  type="button"
                  onClick={() => placeFacility(option.kind, option.position.x, option.position.y)}
                >
                  <Icon size={16} />
                  {option.label}
                </button>
              );
            })}
          </div>
          <div className="fleet-board-control-group">
            <h4>Run</h4>
            <button type="button" onClick={tickDay}>
              <CalendarClock size={16} />
              Tick Day
            </button>
            <button type="button" onClick={refuelFirstReactor}>
              <Fuel size={16} />
              Refuel Reactor
            </button>
          </div>
          <div className="fleet-board-control-group">
            <h4>Local Simulation Capacity</h4>
            <p data-testid="fleet-board-selected-reactor">
              {selectedReactorId ? `Selected ${selectedReactorId}` : "No reactor selected"}
            </p>
            <p>Local game state only — does not submit backend work.</p>
            <button type="button" onClick={buySimulationContainerToken} disabled={!selectedReactorId}>
              <Cpu size={16} />
              Buy Simulation Container Token (2 budget)
            </button>
          </div>
          <div className="fleet-board-modifiers">
            <span>freshness risk {Math.round(modifiers.freshnessRisk * 100)}%</span>
            <span>confidence x{modifiers.confidenceMultiplier.toFixed(2)}</span>
            <span>inspector pressure {Math.round(modifiers.inspectorPressure * 100)}%</span>
          </div>
        </aside>

        <FleetBoardCanvas scene={scene} onPlaceFacility={placeFacility} onSelectReactor={setSelectedReactorId} />

        <aside className="fleet-board-events" aria-label="Fleet Board event log">
          <h4>Event Log</h4>
          {gameState.events.slice(-8).map((event) => (
            <article key={event.id}>
              <span>Day {event.day}</span>
              <strong>{event.kind}</strong>
              <p>{event.detail}</p>
            </article>
          ))}
        </aside>
      </div>
    </section>
  );
}

function createStarterState(modifiers: ReturnType<typeof buildFleetBoardWorkbenchModifiers>): FleetBoardState {
  let state = createInitialFleetBoardState({ seed: `fleet-board-${modifiers.selectedUnitId}`, modifiers });
  for (const facility of starterFacilities) {
    state = applyFleetBoardAction(state, {
      type: "placeFacility",
      facilityId: `${facility.kind}-${Object.keys(state.facilities).length + 1}`,
      facilityKind: facility.kind,
      position: { x: facility.x, y: facility.y }
    });
  }
  return state;
}
