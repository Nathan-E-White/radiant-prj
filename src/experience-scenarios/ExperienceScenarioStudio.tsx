export type ExperienceScenario = { name: string; group: string; purpose: string; outcome: string };

export const experienceScenarios: ExperienceScenario[] = [
  ["Live Workbench Snapshot — Simulation Health Summary", "Workbench Snapshot", "Review accepted live conditions.", "Live Simulation Health Summary is visible."],
  ["Fixture Workbench Snapshot", "Workbench Snapshot", "Review known fixture data.", "Fixture provenance is visible."],
  ["Stale Workbench Snapshot", "Workbench Snapshot", "Review retained data during refresh.", "Stale status and last accepted values are visible."],
  ["Recovering Workbench Snapshot", "Workbench Snapshot", "Review recovery from unavailability.", "Recovery status is visible."],
  ["Unavailable Workbench Snapshot", "Workbench Snapshot", "Review an unavailable read.", "Unavailable guidance is visible."],
  ["Error Workbench Snapshot", "Workbench Snapshot", "Review a failed read.", "Error guidance is visible."],
  ["Measured State with Complete Lineage", "Value Basis and Lineage", "Review measured evidence.", "Measured State and complete Lineage are visible."],
  ["Imputed State with Missing Lineage", "Value Basis and Lineage", "Review qualified estimate evidence.", "Imputed State and missing Lineage are visible."],
  ["Simulated Result State", "Value Basis and Lineage", "Review synthetic result evidence.", "Simulated Result State is visible."],
  ["Commercial Display Basis", "Value Basis and Lineage", "Review commercial framing.", "Commercial Display Basis is visible."],
  ["Fleet Board Capacity and Simulation Job", "Fleet Board", "Review available simulation capacity.", "Capacity and Simulation Job are visible."],
  ["Fleet Board Insight Token Under Pressure", "Fleet Board", "Review scarce Insight Token decisions.", "Pressure and Insight Token are visible."],
  ["Fleet Board Terminal State", "Fleet Board", "Review completed terminal outcomes.", "Terminal scenario state is visible."],
  ["Review Density Desktop Keyboard Focus", "Presentation", "Review detailed desktop interaction.", "Review density and keyboard focus are visible."],
  ["Compact Density Narrow Reduced Motion", "Presentation", "Review constrained, motion-safe presentation.", "Compact density, narrow viewport, and reduced-motion behavior are visible."],
].map(([name, group, purpose, outcome]) => ({ name, group, purpose, outcome }));

export function ExperienceScenarioStudio({ scenario }: { scenario: ExperienceScenario }) {
  return <section aria-label="Experience Scenario Studio"><p>{scenario.group}</p><h1>{scenario.name}</h1><p><strong>Purpose:</strong> {scenario.purpose}</p><p><strong>Expected visible outcome:</strong> {scenario.outcome}</p></section>;
}
