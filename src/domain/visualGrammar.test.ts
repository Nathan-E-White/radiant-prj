import { describe, expect, it } from "vitest";
import { visualGrammarSpec } from "./visualGrammar";

describe("visual grammar contract", () => {
  it("documents the required interaction states", () => {
    expect(visualGrammarSpec.interactionStateSpecs.map((state) => state.id)).toEqual([
      "hover",
      "focus",
      "selected",
      "disabled",
      "warning",
      "current"
    ]);
  });

  it("keeps every value basis identifiable without color alone", () => {
    const specs = Object.values(visualGrammarSpec.valueBasisSpecs);

    expect(specs).toHaveLength(3);
    expect(new Set(specs.map((spec) => spec.iconLabel)).size).toBe(3);
    expect(new Set(specs.map((spec) => spec.ruleStyle)).size).toBe(3);
    expect(new Set(specs.map((spec) => spec.texture)).size).toBe(3);
    expect(specs.map((spec) => spec.id)).toEqual(["measured", "imputed", "simulated"]);
  });

  it("reserves warning meaning outside the value-basis language", () => {
    const valueBasisLabels = Object.values(visualGrammarSpec.valueBasisSpecs)
      .map((spec) => `${spec.label} ${spec.usage} ${spec.texture}`.toLowerCase())
      .join(" ");

    expect(valueBasisLabels).not.toContain("warning");
    expect(valueBasisLabels).not.toContain("commercial");
  });
});
