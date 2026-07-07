import type { Meta, StoryObj } from "@storybook/react";
import { SimulationHealthPanel, type SimulationHealthPanelProps } from "./SimulationHealthPanel";
import {
  simulationHealthPanelArtifactPipelineDegraded,
  simulationHealthPanelCriticalWorkerAndArtifacts,
  simulationHealthPanelLifecycleRunningWithStaleStream,
  simulationHealthPanelNominal
} from "./fixtures/healthPanels.fixture";

const meta: Meta<typeof SimulationHealthPanel> = {
  title: "Simulator Workbench/Simulation Health Panel",
  component: SimulationHealthPanel
};

export default meta;

type Story = StoryObj<typeof SimulationHealthPanel>;

export const Nominal: Story = {
  args: {
    model: simulationHealthPanelNominal
  } satisfies SimulationHealthPanelProps
};

export const LifecycleRunningWithStaleStream: Story = {
  args: {
    model: simulationHealthPanelLifecycleRunningWithStaleStream
  } satisfies SimulationHealthPanelProps
};

export const ArtifactPipelineDegraded: Story = {
  args: {
    model: simulationHealthPanelArtifactPipelineDegraded
  } satisfies SimulationHealthPanelProps
};

export const CriticalWorkerAndArtifacts: Story = {
  args: {
    model: simulationHealthPanelCriticalWorkerAndArtifacts
  } satisfies SimulationHealthPanelProps
};
