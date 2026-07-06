import { describe, expect, it } from "vitest";
import { summarizeValueBasis, valueBasisLabel } from "./valueBasis";
import type { WorkbenchValue } from "../../api/simulatorWorkbench";

describe("simulator workbench value-basis helpers", () => {
  it("keeps measured, imputed, and simulated values separate", () => {
    const values = [
      sampleValue("VAL-MEASURED", "measured"),
      sampleValue("VAL-IMPUTED", "imputed"),
      sampleValue("VAL-SIMULATED", "simulated"),
      sampleValue("VAL-MEASURED-2", "measured")
    ];

    expect(summarizeValueBasis(values)).toEqual({ measured: 2, imputed: 1, simulated: 1 });
    expect(valueBasisLabel("imputed")).toBe("Imputed");
  });
});

function sampleValue(valueId: string, valueBasis: WorkbenchValue["valueBasis"]): WorkbenchValue {
  return {
    valueId,
    label: valueId,
    valueBasis,
    unit: "value",
    value: { scalar: 1 },
    confidence: 1,
    freshness: { ageSec: 0, status: "fresh" },
    lineageId: `LIN-${valueId}`,
    sourceIds: [`SRC-${valueId}`]
  };
}
