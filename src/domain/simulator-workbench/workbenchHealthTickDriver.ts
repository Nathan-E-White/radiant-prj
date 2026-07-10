import type { SimulationHealthPanelModel } from "../../components/simulator-workbench/SimulationHealthPanel";
import { projectHealthCards, type WorkbenchStateView } from "./workbenchHealthPanelProjection";

export type HealthTickDriver = {
  stop: () => void;
};

type HealthTickInput = {
  intervalMs: number;
  fixtures: Array<WorkbenchStateView>;
  onTick: (model: SimulationHealthPanelModel) => void;
  initialNow?: Date;
};

const DEFAULT_TICK_INTERVAL_MS = 1000;

export function createHealthTickDriver({
  intervalMs,
  fixtures,
  onTick,
  initialNow = new Date()
}: HealthTickInput): HealthTickDriver {
  const safeFixtures = Array.isArray(fixtures) && fixtures.length > 0 ? fixtures : [
    {
      generatedAt: new Date(0).toISOString(),
      activeSimulationRuns: []
    }
  ];
  const safeInterval = Number.isFinite(intervalMs) && intervalMs > 0 ? intervalMs : DEFAULT_TICK_INTERVAL_MS;
  let currentTime = initialNow instanceof Date && !Number.isNaN(initialNow.getTime()) ? initialNow : new Date();
  let index = 0;

  const emit = (frameIndex: number, observedAt: Date) => {
    onTick(projectHealthCards(safeFixtures[frameIndex]!, observedAt));
  };

  emit(index, currentTime);

  const timer = setInterval(() => {
    index = (index + 1) % safeFixtures.length;
    currentTime = new Date(currentTime.getTime() + safeInterval);
    emit(index, currentTime);
  }, safeInterval);

  return {
    stop: () => clearInterval(timer)
  };
};
