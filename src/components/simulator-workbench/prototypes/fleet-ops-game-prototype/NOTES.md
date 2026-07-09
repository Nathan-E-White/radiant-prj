# PROTOTYPE - Fleet Ops Game Twin

Question: could the empty twin pane become a fun Phaser-style fleet ops game surface while still using the backend workbench data we already built?

Run command:

```sh
bun run dev
```

Open:

```text
http://127.0.0.1:5173/?prototype=fleet-ops&variant=A
http://127.0.0.1:5173/?prototype=fleet-ops&variant=B
http://127.0.0.1:5173/?prototype=fleet-ops&variant=C
http://127.0.0.1:5173/?prototype=fleet-ops&variant=D
```

This is throwaway. Delete it or fold the winning idea into real workbench code after review.

## Variants

### A - Fleet Board

The twin pane becomes a playable board of Kaleidos Units. Each unit is a clickable node with output, freshness, phase, and lane identity. A director panel changes what the player is paying attention to: balanced fleet, electric push, thermal service, resilience posture, or confidence chase.

What is enjoyable:

- fast unit switching;
- visible fleet motion without needing real-time telemetry;
- obvious tradeoffs between output, service mode, freshness, and confidence;
- board-game clarity instead of dashboard oatmeal.

Backend fit:

- `fleetUnits` drives unit nodes;
- freshness and output labels come from the Workbench projection;
- selected unit still drives the whole workbench context;
- Phaser could later own node motion, particles, pulsing, and hit targets.

### B - Unit Cutaway

The selected unit opens into a reactor-diorama style cutaway. The player can click measured, imputed, and simulated value layers. The cutaway responds with glow, service-flow animation, and a callout.

What is enjoyable:

- the unit feels like an object instead of a spreadsheet;
- the player gets immediate sensory feedback for abstract values;
- measured/imputed/simulated becomes a visual toy vocabulary;
- the event deck makes the cutaway react.

Backend fit:

- twin state values drive cutaway highlights;
- value basis controls layer color;
- lineage remains in React panels;
- simulated results can create temporary overlays.

### C - Replay Triage

The twin pane becomes a timeline scrubber. The player moves through a synthetic fleet hour and watches freshness, output, confidence, and review pressure shift.

What is enjoyable:

- time is a control, not just a timestamp;
- cause and effect become visible;
- the backend history path becomes a mechanic;
- the player can compare recovery versus drift.

Backend fit:

- measured frames provide source freshness over time;
- twin state snapshots provide imputed confidence over time;
- simulated results provide scenario beats;
- Iceberg/history readback could feed replay frames later.

### D - Overseer Board

This is the recommended "Pharaoh-overseer / FTL-style" composite. It puts map mode, selected-unit cutaway, event deck, director controls, backend-feed explanation, and inspectable value layers on one screen.

What is enjoyable:

- the player can scan the whole fleet and dive into one unit without route or tab hopping;
- event cards create immediate pressure, so the board has drama instead of passive metrics;
- director controls make the same backend state feel strategically different;
- the cutaway gives tactile feedback while React keeps the values, lineage, and feeds legible;
- the screen has a cockpit rhythm: map, inspect, react, compare, repeat.

Backend fit:

- the Workbench projection is the scene model;
- `fleetUnits` drives map nodes and selected-unit readout;
- twin state values drive the cutaway and value rail;
- simulated results and workbench health drive event consequences;
- measured frame freshness can become ring effects, pulsing, dimming, and review pressure;
- the same selected value still feeds lineage/detail panels when the prototype is folded into production.

## Game Mechanics

Core loop:

1. Scan the fleet board.
2. Pick a director posture.
3. Draw or select an event.
4. Inspect a unit or value layer.
5. Scrub replay to see whether the fleet recovered.

Systems:

- Fleet units: clickable objects with phase, output, freshness, service mode, and display value.
- Director posture: a toy priority filter that changes the player's attention economy.
- Event deck: scripted pressure cards such as heat demand spike, freshness drift, cooldown tail, and worker slowdown.
- Unit cutaway: diorama view for selected measured, imputed, or simulated values.
- Replay timeline: history scrubber for fleet-level state changes.
- Visible state dump: every variant renders current prototype state so the mechanics can be judged directly.

Scoring idea for a later pass:

- Service score: how well the chosen director posture matches the active event.
- Confidence score: how fresh and well-supported the selected unit values are.
- Continuity score: how much useful fleet context remains visible during outage/cooldown phases.
- Attention score: whether the player investigated the right unit or value after an event.

## Why This Would Use the Technical Backend Well

The fun version still wants the serious backend. Otherwise the game surface is just a disconnected toy with neon eyeliner.

- Workbench projection becomes the clean scene model.
- Measured frames drive freshness rings and data-quality pressure.
- Twin projected state drives cutaway glow, confidence, and residual heat.
- Simulated results drive event consequences and scenario overlays.
- Lineage explains why an animated value is believable.
- Historical frames make replay mode possible.

## Phaser Direction

If this wins, Phaser should only own the animated scene:

- fleet node motion;
- particles and service flows;
- cutaway glow;
- event impact animation;
- drag/scrub feedback;
- board hit testing.

React should keep:

- data loading;
- selection state;
- lineages and detail panels;
- accessible controls;
- normal workbench layout;
- backend API contracts.

Do not let Phaser become the data client from hell. Feed it a small scene model and make it dance.
