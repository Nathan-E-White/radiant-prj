import type { Meta, StoryObj } from "@storybook/react";
import { ExperienceScenarioStudio, experienceScenarios } from "./ExperienceScenarioStudio";

const meta: Meta<typeof ExperienceScenarioStudio> = { title: "Experience Scenario Studio", component: ExperienceScenarioStudio };
export default meta;
type Story = StoryObj<typeof ExperienceScenarioStudio>;
export const Scenarios: Record<string, Story> = Object.fromEntries(experienceScenarios.map((scenario) => [scenario.name.replaceAll(/[^A-Za-z0-9]/g, ""), { args: { scenario } }]));
