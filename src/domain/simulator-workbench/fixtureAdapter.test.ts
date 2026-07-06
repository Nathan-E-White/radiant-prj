import { describe, expect, it } from "vitest";
import { loadFixtureWorkbenchData, parseMeasuredTelemetryNdjson } from "./fixtureAdapter";

describe("simulator workbench fixture adapter", () => {
  it("loads the scaffold examples into projection input", () => {
    const data = loadFixtureWorkbenchData();

    expect(data.state.schemaVersion).toBe("simulator-workbench.state.v1");
    expect(data.twin.schemaVersion).toBe("digital-twin.state.v1");
    expect(data.measured).toHaveLength(6);
    expect(data.lineages).toHaveLength(1);
    expect(data.measured.every((frame) => frame.valueBasis === "measured")).toBe(true);
  });

  it("parses measured telemetry NDJSON without changing value basis", () => {
    const frames = parseMeasuredTelemetryNdjson(
      [
        '{"schemaVersion":"scada.telemetry.v1","sourceId":"SRC-1","tagId":"TAG-1","assetId":"ASSET-1","signalKind":"flux","sampledAt":"2026-07-06T15:00:00Z","observedAt":"2026-07-06T15:00:01Z","sequence":1,"unit":"relative-flux","value":{"scalar":0.7},"quality":"good","valueBasis":"measured","syntheticStatus":"public-safe-standin"}',
        "",
        '{"schemaVersion":"scada.telemetry.v1","sourceId":"SRC-1","tagId":"TAG-2","assetId":"ASSET-2","signalKind":"temperature","sampledAt":"2026-07-06T15:00:00Z","observedAt":"2026-07-06T15:00:01Z","sequence":1,"unit":"degC","value":{"scalar":612.4},"quality":"stale","valueBasis":"measured","syntheticStatus":"public-safe-standin"}'
      ].join("\n")
    );

    expect(frames.map((frame) => frame.tagId)).toEqual(["TAG-1", "TAG-2"]);
    expect(frames.map((frame) => frame.valueBasis)).toEqual(["measured", "measured"]);
  });
});
