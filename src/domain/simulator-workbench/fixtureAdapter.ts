import type {
  CommercialDisplayBasis,
  DigitalTwinState,
  KaleidosUnitSummary,
  MeasuredTelemetryFrame,
  SimulatorWorkbenchState,
  WorkbenchLineage
} from "../../api/simulatorWorkbench";
import commercialBasisFixture from "../../../examples/simulator-workbench/commercial-display-basis.mixed.json";
import coreDistributionLineageFixture from "../../../examples/digital-twin/value-lineage.core-distribution.json";
import cooldownHeatLineageFixture from "../../../examples/digital-twin/value-lineage.cooldown-heat.json";
import fleetUnitsFixture from "../../../examples/simulator-workbench/fleet-units.mixed.json";
import lineageFixture from "../../../examples/digital-twin/value-lineage.core-margin.json";
import twinFixture from "../../../examples/digital-twin/twin-state.mixed.json";
import measuredTelemetryFixture from "../../../examples/scada/telemetry.mixed.ndjson?raw";
import workbenchFixture from "../../../examples/simulator-workbench/workbench-state.mixed.json";
import type { WorkbenchDataAdapter, WorkbenchProjectionInput } from "./projection";

export function loadFixtureWorkbenchData(): WorkbenchProjectionInput {
  return {
    state: workbenchFixture as SimulatorWorkbenchState,
    measured: parseMeasuredTelemetryNdjson(measuredTelemetryFixture),
    twin: twinFixture as DigitalTwinState,
    lineages: [
      coreDistributionLineageFixture as WorkbenchLineage,
      lineageFixture as WorkbenchLineage,
      cooldownHeatLineageFixture as WorkbenchLineage
    ],
    fleetUnits: fleetUnitsFixture.units as KaleidosUnitSummary[],
    commercialDisplayBasis: commercialBasisFixture.basis as CommercialDisplayBasis[]
  };
}

export const fixtureWorkbenchDataAdapter: WorkbenchDataAdapter = {
  async load() {
    return loadFixtureWorkbenchData();
  }
};

export function parseMeasuredTelemetryNdjson(raw: string): MeasuredTelemetryFrame[] {
  return raw
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => JSON.parse(line) as MeasuredTelemetryFrame);
}
