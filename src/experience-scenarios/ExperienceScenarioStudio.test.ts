import { describe, expect, it } from "vitest";
import { experienceScenarios } from "./ExperienceScenarioStudio";

describe("Experience Scenario Studio", () => it("names the required review material in Radiant domain language", () => {
  expect(experienceScenarios.map(({ name }) => name).join(" ")).toContain("Simulation Health Summary");
  expect(experienceScenarios).toHaveLength(15);
}));
