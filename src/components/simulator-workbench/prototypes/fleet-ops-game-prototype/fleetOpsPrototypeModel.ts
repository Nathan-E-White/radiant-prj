export type FleetOpsPrototypeVariant = "A" | "B" | "C" | "D";

export type DirectorModeId =
  | "balanced"
  | "electric"
  | "thermal"
  | "resilience"
  | "confidence";

export type FleetEventId =
  | "heat-spike"
  | "freshness-drift"
  | "cooldown-tail"
  | "worker-slowdown";

export const fleetOpsVariantMeta: Record<
  FleetOpsPrototypeVariant,
  {
    name: string;
    intent: string;
    fun: string;
  }
> = {
  A: {
    name: "Fleet Board",
    intent: "Make the twin pane feel like a playable fleet map driven by live workbench state.",
    fun: "Fast selection, bright state changes, and a constant sense that the fleet is doing something."
  },
  B: {
    name: "Unit Cutaway",
    intent: "Make one selected Kaleidos Unit feel tangible, animated, and inspectable.",
    fun: "The player gets satisfying reactor-diorama feedback without needing a physics sermon."
  },
  C: {
    name: "Replay Triage",
    intent: "Make backend history, freshness, and simulation results feel like a time-control game.",
    fun: "Scrubbing time and watching the fleet reorganize gives the boring data pipe some teeth."
  },
  D: {
    name: "Overseer Board",
    intent: "Combine map mode, unit cutaway, and event deck into one FTL-style fleet surface.",
    fun: "The player can scan, click, react, and inspect without leaving the cockpit. No tab-hopping bullshit."
  }
};

export const directorModes: Array<{
  id: DirectorModeId;
  label: string;
  shortLabel: string;
  copy: string;
  tension: string;
}> = [
  {
    id: "balanced",
    label: "Balanced fleet",
    shortLabel: "Balance",
    copy: "Keep output, heat service, freshness, and simulation confidence moving together.",
    tension: "No single bar screams, but nothing gets to hide either."
  },
  {
    id: "electric",
    label: "Electric push",
    shortLabel: "Electric",
    copy: "Favor units with clean electric output and stable breaker-to-breaker counters.",
    tension: "Thermal service and maintenance context can start feeling neglected."
  },
  {
    id: "thermal",
    label: "Thermal service",
    shortLabel: "Thermal",
    copy: "Favor facility heat, desalination heat, and residual-heat storytelling.",
    tension: "The player watches heat usefulness drift apart from electric flash."
  },
  {
    id: "resilience",
    label: "Resilience posture",
    shortLabel: "Resilience",
    copy: "Favor backup availability, cooldown interpretation, and continuity cues.",
    tension: "Commercial value gets quieter while recovery context gets louder."
  },
  {
    id: "confidence",
    label: "Confidence chase",
    shortLabel: "Confidence",
    copy: "Favor units with fresh measured sources and stronger imputed-state confidence.",
    tension: "High-output units can become less attractive when their sources go stale."
  }
];

export const fleetEventDeck: Array<{
  id: FleetEventId;
  title: string;
  pressure: string;
  description: string;
  playerChoice: string;
  backendFeed: string;
}> = [
  {
    id: "heat-spike",
    title: "Heat demand spike",
    pressure: "thermal demand",
    description: "A facility heat sink wants attention while electric output still looks tasty.",
    playerChoice: "Shift director posture toward thermal service or keep chasing electric output.",
    backendFeed: "simops.results.v1 plus commercial display basis"
  },
  {
    id: "freshness-drift",
    title: "Telemetry freshness drift",
    pressure: "source quality",
    description: "One unit is still productive, but its measured sources are aging like gas-station sushi.",
    playerChoice: "Inspect lineage, de-emphasize the unit, or wait for the next source frame.",
    backendFeed: "scada.measured_frames freshness and quality"
  },
  {
    id: "cooldown-tail",
    title: "Cooldown tail visible",
    pressure: "residual heat",
    description: "A cooldown unit has no commercial output, but the twin still has useful state to show.",
    playerChoice: "Keep it visible as reactor-state context or filter it behind active service units.",
    backendFeed: "digital_twin.state_values with imputed basis"
  },
  {
    id: "worker-slowdown",
    title: "Simulation worker slowdown",
    pressure: "scenario confidence",
    description: "A run-scoped worker is late, so scenario overlays become less decisive.",
    playerChoice: "Scrub replay, compare last known result, or wait for a cleaner simulated frame.",
    backendFeed: "simops telemetry, result ingest, and workbench health projection"
  }
];

export const replayFrames = [
  {
    label: "T+00",
    headline: "Fleet nominal",
    outputBias: "electric",
    confidence: 92,
    freshness: "fresh",
    event: "baseline"
  },
  {
    label: "T+15",
    headline: "Thermal service rises",
    outputBias: "thermal",
    confidence: 88,
    freshness: "fresh",
    event: "heat-spike"
  },
  {
    label: "T+30",
    headline: "Source drift appears",
    outputBias: "mixed",
    confidence: 74,
    freshness: "late",
    event: "freshness-drift"
  },
  {
    label: "T+45",
    headline: "Cooldown context matters",
    outputBias: "resilience",
    confidence: 81,
    freshness: "mixed",
    event: "cooldown-tail"
  },
  {
    label: "T+60",
    headline: "Simulation catches up",
    outputBias: "balanced",
    confidence: 89,
    freshness: "fresh",
    event: "worker-slowdown"
  }
];

export const backendFeedCards = [
  {
    label: "Measured frames",
    source: "scada.measured_frames",
    use: "Freshness rings, source drift, measured badges"
  },
  {
    label: "Twin state",
    source: "digital_twin.state_values",
    use: "Cutaway glow, imputed confidence, residual-heat overlays"
  },
  {
    label: "Simulation results",
    source: "simops.simulated_results",
    use: "Scenario overlays, forecast cards, replay beats"
  },
  {
    label: "Workbench projection",
    source: "/api/simulator-workbench/*",
    use: "Clean scene model for Phaser or React prototype rendering"
  }
];

export const unitLayout: Record<string, { x: number; y: number; lane: string }> = {
  "KAL-01": { x: 18, y: 32, lane: "electric" },
  "KAL-02": { x: 38, y: 62, lane: "thermal" },
  "KAL-03": { x: 62, y: 34, lane: "thermal" },
  "KAL-04": { x: 77, y: 70, lane: "resilience" },
  "KAL-05": { x: 48, y: 18, lane: "maintenance" }
};

export const gameMechanicLoops = [
  {
    label: "Scan",
    detail: "Read the animated fleet board and notice phase, freshness, output, and confidence."
  },
  {
    label: "Prioritize",
    detail: "Pick a director posture that changes which units feel valuable right now."
  },
  {
    label: "Respond",
    detail: "Use the event deck to create pressure and decide where attention goes."
  },
  {
    label: "Inspect",
    detail: "Dive into unit cutaway, value basis, and lineage when the flashy board raises a question."
  },
  {
    label: "Replay",
    detail: "Scrub the last hour and learn whether the fleet recovered, drifted, or just looked good."
  }
];
