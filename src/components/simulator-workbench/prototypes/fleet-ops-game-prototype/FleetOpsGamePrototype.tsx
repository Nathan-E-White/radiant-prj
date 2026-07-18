import {
  Activity,
  AlertTriangle,
  ChevronRight,
  Clock,
  Cpu,
  Flame,
  Gauge,
  Layers,
  Map as MapIcon,
  Play,
  Radio,
  RotateCcw,
  Sparkles,
  Target,
  Zap
} from "lucide-react";
import { useCallback, useMemo, useState } from "react";
import type { CSSProperties } from "react";
import type { WorkbenchProjection, ProjectedFleetUnit, ProjectedWorkbenchValue } from "../../../../domain/simulator-workbench";
import type { ComputeJob } from "../../../../domain/types";
import {
  backendFeedCards,
  directorModes,
  fleetEventDeck,
  fleetOpsVariantMeta,
  gameMechanicLoops,
  replayFrames,
  unitLayout,
  type DirectorModeId,
  type FleetEventId,
  type FleetOpsPrototypeVariant
} from "./fleetOpsPrototypeModel";
import {
  PrototypeVariantSwitcher,
  readPrototypeVariant,
  writePrototypeVariant
} from "./PrototypeVariantSwitcher";

// PROTOTYPE - Three variants of the Status Workbench twin pane, switchable via ?prototype=fleet-ops&variant=.
export function FleetOpsGamePrototype({
  projection,
  onSelectUnit,
  onSelectValue,
  selectedJob,
  scenario,
  scenarioJobs,
  bundleState
}: {
  projection: WorkbenchProjection;
  onSelectUnit: (unitId: string) => void;
  onSelectValue: (valueId: string) => void;
  selectedJob: ComputeJob;
  scenario: string;
  scenarioJobs: ComputeJob[];
  bundleState: string;
}) {
  const [variant, setVariant] = useState<FleetOpsPrototypeVariant>(() => readPrototypeVariant());
  const [directorModeId, setDirectorModeId] = useState<DirectorModeId>("balanced");
  const [activeEventId, setActiveEventId] = useState<FleetEventId>("heat-spike");
  const [frameIndex, setFrameIndex] = useState(2);
  const [spotlightValueId, setSpotlightValueId] = useState(projection.selectedValue?.valueId ?? "");

  const director = directorModes.find((mode) => mode.id === directorModeId) ?? directorModes[0];
  const activeEvent = fleetEventDeck.find((event) => event.id === activeEventId) ?? fleetEventDeck[0];
  const values = useMemo(
    () => [
      ...projection.groups.measured.values,
      ...projection.groups.imputed.values,
      ...projection.groups.simulated.values
    ],
    [projection.groups.imputed.values, projection.groups.measured.values, projection.groups.simulated.values]
  );
  const spotlightValue = values.find((value) => value.valueId === spotlightValueId) ?? projection.selectedValue;
  const replayFrame = replayFrames[frameIndex] ?? replayFrames[0];

  const changeVariant = useCallback((nextVariant: FleetOpsPrototypeVariant) => {
    setVariant(nextVariant);
    writePrototypeVariant(nextVariant);
  }, []);

  function selectUnit(unit: ProjectedFleetUnit) {
    onSelectUnit(unit.unitId);
  }

  function selectValue(value: ProjectedWorkbenchValue) {
    setSpotlightValueId(value.valueId);
    onSelectValue(value.valueId);
  }

  return (
    <section className="fleet-proto-shell" aria-label="PROTOTYPE fleet ops game twin">
      <FleetPrototypeHeader
        activeEventTitle={activeEvent.title}
        directorLabel={director.label}
        projection={projection}
        variant={variant}
      />

      {variant === "A" && (
        <FleetBoardVariant
          activeEventId={activeEventId}
          directorModeId={directorModeId}
          onDirectorChange={setDirectorModeId}
          onEventChange={setActiveEventId}
          onSelectUnit={selectUnit}
          projection={projection}
          replayFrameIndex={frameIndex}
          units={projection.fleetUnits}
        />
      )}

      {variant === "B" && (
        <UnitCutawayVariant
          activeEventId={activeEventId}
          directorModeId={directorModeId}
          onDirectorChange={setDirectorModeId}
          onEventChange={setActiveEventId}
          onSelectValue={selectValue}
          projection={projection}
          spotlightValue={spotlightValue}
          values={values}
        />
      )}

      {variant === "C" && (
        <ReplayTriageVariant
          activeEventId={activeEventId}
          frameIndex={frameIndex}
          onEventChange={setActiveEventId}
          onFrameChange={setFrameIndex}
          onSelectUnit={selectUnit}
          projection={projection}
          units={projection.fleetUnits}
        />
      )}

      {variant === "D" && (
        <OverseerBoardVariant
          activeEventId={activeEventId}
          directorModeId={directorModeId}
          onDirectorChange={setDirectorModeId}
          onEventChange={setActiveEventId}
          onSelectUnit={selectUnit}
          onSelectValue={selectValue}
          projection={projection}
          replayFrame={replayFrame}
          spotlightValue={spotlightValue}
          units={projection.fleetUnits}
          values={values}
        />
      )}

      <GameMechanicsAndState
        activeEvent={activeEvent}
        bundleState={bundleState}
        director={director}
        frameIndex={frameIndex}
        projection={projection}
        replayFrame={replayFrame}
        scenario={scenario}
        scenarioJobs={scenarioJobs.length}
        selectedJobId={selectedJob.id}
        spotlightValue={spotlightValue}
        variant={variant}
      />

      <PrototypeVariantSwitcher current={variant} onChange={changeVariant} />
    </section>
  );
}

function OverseerBoardVariant({
  activeEventId,
  directorModeId,
  onDirectorChange,
  onEventChange,
  onSelectUnit,
  onSelectValue,
  projection,
  replayFrame,
  spotlightValue,
  units,
  values
}: {
  activeEventId: FleetEventId;
  directorModeId: DirectorModeId;
  onDirectorChange: (mode: DirectorModeId) => void;
  onEventChange: (event: FleetEventId) => void;
  onSelectUnit: (unit: ProjectedFleetUnit) => void;
  onSelectValue: (value: ProjectedWorkbenchValue) => void;
  projection: WorkbenchProjection;
  replayFrame: (typeof replayFrames)[number];
  spotlightValue: ProjectedWorkbenchValue | null;
  units: ProjectedFleetUnit[];
  values: ProjectedWorkbenchValue[];
}) {
  const activeEvent = fleetEventDeck.find((event) => event.id === activeEventId) ?? fleetEventDeck[0];
  const director = directorModes.find((mode) => mode.id === directorModeId) ?? directorModes[0];
  const selectedValues = values.slice(0, 5);

  return (
    <div className="fleet-proto-overseer">
      <section className="fleet-proto-command-stage">
        <div className="fleet-proto-map-toolbar">
          <span><MapIcon size={16} /> Pharaoh / FTL fleet board</span>
          <span><Target size={16} /> {director.shortLabel}</span>
          <span><AlertTriangle size={16} /> {activeEvent.title}</span>
        </div>

        <div className="fleet-proto-command-grid">
          <div className="fleet-proto-command-map" aria-label="Overseer fleet map">
            <div className="fleet-proto-map compact">
              <svg className="fleet-proto-network" viewBox="0 0 100 72" aria-hidden="true">
                <path d="M18 32 C28 16 39 13 48 18 S64 33 77 70" />
                <path d="M18 32 C30 54 42 65 62 34" />
                <path d="M38 62 C48 48 56 38 62 34" />
                <path d="M48 18 C48 34 49 50 38 62" />
              </svg>
              {units.map((unit) => (
                <FleetMapNode
                  key={unit.unitId}
                  unit={unit}
                  selected={unit.unitId === projection.selectedUnit.unitId}
                  onSelect={() => onSelectUnit(unit)}
                />
              ))}
            </div>
            <div className="fleet-proto-overseer-readout">
              <strong>{projection.selectedUnit.displayName}</strong>
              <span>{projection.selectedUnit.phaseLine}</span>
              <span>{projection.selectedUnit.outputLine}</span>
              <span>{projection.selectedUnit.accruedDisplayLabel}</span>
            </div>
          </div>

          <div className="fleet-proto-command-cutaway">
            <UnitDiorama spotlightValue={spotlightValue} density="compact" />
          </div>

          <aside className="fleet-proto-command-rail">
            <SectionHeading eyebrow="Event deck" title="Drama button row" icon={AlertTriangle} />
            <EventDeck activeEventId={activeEventId} onEventChange={onEventChange} />
          </aside>
        </div>
      </section>

      <section className="fleet-proto-overseer-console">
        <div className="fleet-proto-panel">
          <SectionHeading eyebrow="Director controls" title={director.label} icon={Gauge} />
          <p className="fleet-proto-copy">{director.copy}</p>
          <div className="fleet-proto-mode-grid compact">
            {directorModes.map((mode) => (
              <button
                className={mode.id === directorModeId ? "fleet-proto-mode selected" : "fleet-proto-mode"}
                key={mode.id}
                onClick={() => onDirectorChange(mode.id)}
                type="button"
              >
                <strong>{mode.shortLabel}</strong>
              </button>
            ))}
          </div>
        </div>

        <div className="fleet-proto-panel">
          <SectionHeading eyebrow="React truth rail" title="Inspectable value layers" icon={Layers} />
          <div className="fleet-proto-value-stack compact">
            {selectedValues.map((value) => (
              <button
                className={spotlightValue?.valueId === value.valueId ? `fleet-proto-value selected ${value.valueBasis}` : `fleet-proto-value ${value.valueBasis}`}
                key={value.valueId}
                onClick={() => onSelectValue(value)}
                type="button"
              >
                <span>{value.valueBasis}</span>
                <strong>{value.label}</strong>
                <small>{value.displayValue} {value.unit} - {value.confidencePct}%</small>
              </button>
            ))}
          </div>
        </div>

        <div className="fleet-proto-panel">
          <SectionHeading eyebrow="Backend makes it real" title="Feeds into scene" icon={Radio} />
          <BackendFeedList />
        </div>

        <div className="fleet-proto-panel">
          <SectionHeading eyebrow="Replay pulse" title={replayFrame.headline} icon={Clock} />
          <div className="fleet-proto-score-row">
            <Meter label="confidence" value={replayFrame.confidence} />
            <Meter label="service pressure" value={activeEvent.id === "heat-spike" ? 92 : 68} />
            <Meter label="attention load" value={spotlightValue ? 74 : 45} />
          </div>
        </div>
      </section>
    </div>
  );
}

function FleetPrototypeHeader({
  activeEventTitle,
  directorLabel,
  projection,
  variant
}: {
  activeEventTitle: string;
  directorLabel: string;
  projection: WorkbenchProjection;
  variant: FleetOpsPrototypeVariant;
}) {
  const meta = fleetOpsVariantMeta[variant];
  return (
    <header className="fleet-proto-header">
      <div>
        <p className="eyebrow">PROTOTYPE - Fleet Ops Game Twin</p>
        <h2>{variant} - {meta.name}</h2>
        <p>{meta.intent}</p>
      </div>
      <div className="fleet-proto-status-strip" aria-label="Prototype state summary">
        <PrototypeMetric icon={Sparkles} label="Enjoyment bet" value={meta.fun} />
        <PrototypeMetric icon={Target} label="Director" value={directorLabel} />
        <PrototypeMetric icon={AlertTriangle} label="Event" value={activeEventTitle} />
        <PrototypeMetric icon={Radio} label="Twin feed" value={projection.twinId} />
      </div>
    </header>
  );
}

function FleetBoardVariant({
  activeEventId,
  directorModeId,
  onDirectorChange,
  onEventChange,
  onSelectUnit,
  projection,
  replayFrameIndex,
  units
}: {
  activeEventId: FleetEventId;
  directorModeId: DirectorModeId;
  onDirectorChange: (mode: DirectorModeId) => void;
  onEventChange: (event: FleetEventId) => void;
  onSelectUnit: (unit: ProjectedFleetUnit) => void;
  projection: WorkbenchProjection;
  replayFrameIndex: number;
  units: ProjectedFleetUnit[];
}) {
  return (
    <div className="fleet-proto-layout fleet-proto-layout-board">
      <aside className="fleet-proto-panel">
        <SectionHeading eyebrow="Director layer" title="Pick the fleet personality" icon={Gauge} />
        <div className="fleet-proto-mode-grid">
          {directorModes.map((mode) => (
            <button
              className={mode.id === directorModeId ? "fleet-proto-mode selected" : "fleet-proto-mode"}
              key={mode.id}
              onClick={() => onDirectorChange(mode.id)}
              type="button"
            >
              <strong>{mode.shortLabel}</strong>
              <span>{mode.tension}</span>
            </button>
          ))}
        </div>
      </aside>

      <section className="fleet-proto-map-stage" aria-label="Playable fleet board prototype">
        <div className="fleet-proto-map-toolbar">
          <span><MapIcon size={16} /> Kaleidos Fleet Board</span>
          <span><Clock size={16} /> replay frame {replayFrameIndex + 1}/5</span>
          <span><Layers size={16} /> measured + imputed + simulated</span>
        </div>
        <div className="fleet-proto-map">
          <svg className="fleet-proto-network" viewBox="0 0 100 72" aria-hidden="true">
            <path d="M18 32 C28 16 39 13 48 18 S64 33 77 70" />
            <path d="M18 32 C30 54 42 65 62 34" />
            <path d="M38 62 C48 48 56 38 62 34" />
            <path d="M48 18 C48 34 49 50 38 62" />
          </svg>
          {units.map((unit) => (
            <FleetMapNode
              key={unit.unitId}
              unit={unit}
              selected={unit.unitId === projection.selectedUnit.unitId}
              onSelect={() => onSelectUnit(unit)}
            />
          ))}
          <div className="fleet-proto-map-caption">
            <strong>{projection.selectedUnit.displayName}</strong>
            <span>{projection.selectedUnit.phaseLine}</span>
            <span>{projection.selectedUnit.outputLine}</span>
          </div>
        </div>
      </section>

      <aside className="fleet-proto-panel">
        <SectionHeading eyebrow="Event deck" title="Pressure, not procedure" icon={AlertTriangle} />
        <EventDeck activeEventId={activeEventId} onEventChange={onEventChange} />
      </aside>
    </div>
  );
}

function UnitCutawayVariant({
  activeEventId,
  directorModeId,
  onDirectorChange,
  onEventChange,
  onSelectValue,
  projection,
  spotlightValue,
  values
}: {
  activeEventId: FleetEventId;
  directorModeId: DirectorModeId;
  onDirectorChange: (mode: DirectorModeId) => void;
  onEventChange: (event: FleetEventId) => void;
  onSelectValue: (value: ProjectedWorkbenchValue) => void;
  projection: WorkbenchProjection;
  spotlightValue: ProjectedWorkbenchValue | null;
  values: ProjectedWorkbenchValue[];
}) {
  return (
    <div className="fleet-proto-layout fleet-proto-layout-cutaway">
      <section className="fleet-proto-cutaway-stage">
        <div className="fleet-proto-map-toolbar">
          <span><Flame size={16} /> Animated unit diorama</span>
          <span><Activity size={16} /> {projection.selectedUnit.displayName}</span>
          <span><Zap size={16} /> {projection.selectedUnit.outputLine}</span>
        </div>
        <UnitDiorama spotlightValue={spotlightValue} />
      </section>

      <aside className="fleet-proto-panel">
        <SectionHeading eyebrow="Game knobs" title="Scenario posture" icon={Target} />
        <p className="fleet-proto-copy">
          These are toy scenario levers: they steer presentation pressure and scoring, not plant behavior.
        </p>
        <div className="fleet-proto-mode-grid compact">
          {directorModes.map((mode) => (
            <button
              className={mode.id === directorModeId ? "fleet-proto-mode selected" : "fleet-proto-mode"}
              key={mode.id}
              onClick={() => onDirectorChange(mode.id)}
              type="button"
            >
              <strong>{mode.shortLabel}</strong>
            </button>
          ))}
        </div>

        <SectionHeading eyebrow="Inspectable values" title="Click a layer" icon={Layers} />
        <div className="fleet-proto-value-stack">
          {values.slice(0, 7).map((value) => (
            <button
              className={spotlightValue?.valueId === value.valueId ? `fleet-proto-value selected ${value.valueBasis}` : `fleet-proto-value ${value.valueBasis}`}
              key={value.valueId}
              onClick={() => onSelectValue(value)}
              type="button"
            >
              <span>{value.valueBasis}</span>
              <strong>{value.label}</strong>
              <small>{value.displayValue} {value.unit} - {value.confidencePct}%</small>
            </button>
          ))}
        </div>
      </aside>

      <aside className="fleet-proto-panel">
        <SectionHeading eyebrow="Event deck" title="Make the unit react" icon={AlertTriangle} />
        <EventDeck activeEventId={activeEventId} onEventChange={onEventChange} />
      </aside>
    </div>
  );
}

function ReplayTriageVariant({
  activeEventId,
  frameIndex,
  onEventChange,
  onFrameChange,
  onSelectUnit,
  projection,
  units
}: {
  activeEventId: FleetEventId;
  frameIndex: number;
  onEventChange: (event: FleetEventId) => void;
  onFrameChange: (frameIndex: number) => void;
  onSelectUnit: (unit: ProjectedFleetUnit) => void;
  projection: WorkbenchProjection;
  units: ProjectedFleetUnit[];
}) {
  const frame = replayFrames[frameIndex] ?? replayFrames[0];
  return (
    <div className="fleet-proto-layout fleet-proto-layout-replay">
      <section className="fleet-proto-timeline-stage">
        <div className="fleet-proto-map-toolbar">
          <span><Play size={16} /> Replay triage</span>
          <span><Clock size={16} /> {frame.label}</span>
          <span><Cpu size={16} /> {frame.headline}</span>
        </div>

        <div className="fleet-proto-replay-board">
          <div className="fleet-proto-replay-track">
            {replayFrames.map((candidate, index) => (
              <button
                className={index === frameIndex ? "fleet-proto-frame selected" : "fleet-proto-frame"}
                key={candidate.label}
                onClick={() => onFrameChange(index)}
                type="button"
              >
                <strong>{candidate.label}</strong>
                <span>{candidate.headline}</span>
              </button>
            ))}
          </div>
          <label className="fleet-proto-slider">
            <span>Scrub simulated fleet hour</span>
            <input
              max={replayFrames.length - 1}
              min={0}
              onChange={(event) => onFrameChange(Number(event.target.value))}
              type="range"
              value={frameIndex}
            />
          </label>
          <div className="fleet-proto-triage-grid">
            {units.map((unit, index) => (
              <button
                className={unit.selected ? "fleet-proto-triage-unit selected" : "fleet-proto-triage-unit"}
                key={unit.unitId}
                onClick={() => onSelectUnit(unit)}
                type="button"
              >
                <span>{unit.unitId}</span>
                <strong>{unit.availabilityPhase}</strong>
                <Meter label="freshness" value={freshnessScore(unit.freshnessWarningLabel, frameIndex, index)} />
                <Meter label="output" value={outputScore(unit, frameIndex)} />
                <Meter label="confidence" value={Math.max(28, frame.confidence - index * 5)} />
              </button>
            ))}
          </div>
        </div>
      </section>

      <aside className="fleet-proto-panel">
        <SectionHeading eyebrow="Review pressure" title="Choose a current event" icon={AlertTriangle} />
        <EventDeck activeEventId={activeEventId} onEventChange={onEventChange} />
      </aside>

      <aside className="fleet-proto-panel">
        <SectionHeading eyebrow="Backend feeds" title="What makes replay real" icon={Radio} />
        <BackendFeedList />
        <p className="fleet-proto-copy">
          Selected context: {projection.selectedUnit.displayName}. Replay is the place where Iceberg/history stops
          sounding like infrastructure trivia and becomes a damn game mechanic.
        </p>
      </aside>
    </div>
  );
}

function FleetMapNode({
  onSelect,
  selected,
  unit
}: {
  onSelect: () => void;
  selected: boolean;
  unit: ProjectedFleetUnit;
}) {
  const fallback = { x: 50, y: 50, lane: "mixed" };
  const position = unitLayout[unit.unitId] ?? fallback;
  const style = {
    "--x": `${position.x}%`,
    "--y": `${position.y}%`
  } as CSSProperties;
  return (
    <button
      className={selected ? `fleet-proto-unit-node selected ${position.lane}` : `fleet-proto-unit-node ${position.lane}`}
      onClick={onSelect}
      style={style}
      type="button"
    >
      <span>{unit.unitId}</span>
      <strong>{unit.electricOutputMwe.toFixed(2)} MWe</strong>
      <small>{unit.freshnessWarningLabel ?? "fresh"}</small>
    </button>
  );
}

function UnitDiorama({
  density = "standard",
  spotlightValue
}: {
  density?: "standard" | "compact";
  spotlightValue: ProjectedWorkbenchValue | null;
}) {
  return (
    <div className={density === "compact" ? "fleet-proto-diorama compact" : "fleet-proto-diorama"}>
      <svg viewBox="0 0 120 78" role="img" aria-label="Prototype Kaleidos Unit cutaway diorama">
        <defs>
          <radialGradient id="fleet-proto-core" cx="50%" cy="50%" r="55%">
            <stop offset="0%" stopColor="#fff3a5" />
            <stop offset="48%" stopColor="#f0a63a" />
            <stop offset="100%" stopColor="#3dd69b" stopOpacity="0.42" />
          </radialGradient>
        </defs>
        <rect className="fleet-proto-diorama-bg" height="78" width="120" />
        <path className="fleet-proto-flow cool" d="M24 38C24 13 96 13 96 38C96 63 24 63 24 38Z" />
        <path className="fleet-proto-flow hot" d="M19 42C34 67 85 67 101 42" />
        <rect className="fleet-proto-vessel" height="52" rx="18" width="42" x="39" y="12" />
        <ellipse className="fleet-proto-core" cx="60" cy="39" rx="12" ry="18" />
        <path className="fleet-proto-core-lines" d="M50 32H70 M49 39H71 M50 46H70 M56 23V55 M64 23V55" />
        <path className="fleet-proto-drums" d="M42 23L49 19L51 58L43 55Z M71 19L78 23L77 55L69 58Z" />
        <circle className="fleet-proto-circulator" cx="25" cy="52" r="8" />
        <path className="fleet-proto-blades" d="M25 44V60 M17 52H33 M19 46L31 58 M31 46L19 58" />
        <rect className="fleet-proto-module power" height="16" rx="3" width="21" x="3" y="32" />
        <rect className="fleet-proto-module heat" height="24" rx="3" width="18" x="96" y="27" />
        <path className="fleet-proto-service-flow" d="M114 39H120" />
      </svg>
      <div className={spotlightValue ? `fleet-proto-diorama-callout ${spotlightValue.valueBasis}` : "fleet-proto-diorama-callout"}>
        <span>{spotlightValue?.entityName ?? "No value selected"}</span>
        <strong>{spotlightValue?.label ?? "Select a value layer"}</strong>
        <small>
          {spotlightValue ? `${spotlightValue.displayValue} ${spotlightValue.unit} - ${spotlightValue.valueBasis}` : "Measured, imputed, and simulated layers are the toy colors."}
        </small>
      </div>
    </div>
  );
}

function EventDeck({
  activeEventId,
  onEventChange
}: {
  activeEventId: FleetEventId;
  onEventChange: (event: FleetEventId) => void;
}) {
  return (
    <div className="fleet-proto-event-deck">
      {fleetEventDeck.map((event) => (
        <button
          className={event.id === activeEventId ? "fleet-proto-event selected" : "fleet-proto-event"}
          key={event.id}
          onClick={() => onEventChange(event.id)}
          type="button"
        >
          <span>{event.pressure}</span>
          <strong>{event.title}</strong>
          <small>{event.description}</small>
        </button>
      ))}
    </div>
  );
}

function BackendFeedList() {
  return (
    <div className="fleet-proto-feed-list">
      {backendFeedCards.map((feed) => (
        <div className="fleet-proto-feed" key={feed.source}>
          <span>{feed.label}</span>
          <strong>{feed.source}</strong>
          <small>{feed.use}</small>
        </div>
      ))}
    </div>
  );
}

function GameMechanicsAndState({
  activeEvent,
  bundleState,
  director,
  frameIndex,
  projection,
  replayFrame,
  scenario,
  scenarioJobs,
  selectedJobId,
  spotlightValue,
  variant
}: {
  activeEvent: (typeof fleetEventDeck)[number];
  bundleState: string;
  director: (typeof directorModes)[number];
  frameIndex: number;
  projection: WorkbenchProjection;
  replayFrame: (typeof replayFrames)[number];
  scenario: string;
  scenarioJobs: number;
  selectedJobId: string;
  spotlightValue: ProjectedWorkbenchValue | null;
  variant: FleetOpsPrototypeVariant;
}) {
  const stateDump = {
    prototype: "fleet-ops-game-twin",
    variant: `${variant} - ${fleetOpsVariantMeta[variant].name}`,
    selectedUnit: projection.selectedUnit.unitId,
    directorMode: director.id,
    activeEvent: activeEvent.id,
    replayFrame: replayFrames[frameIndex]?.label,
    selectedValue: spotlightValue?.valueId ?? null,
    valueBasisSummary: projection.valueBasisSummary,
    scenario,
    scenarioJobs,
    selectedJobId,
    bundleState
  };

  return (
    <section className="fleet-proto-mechanics">
      <div className="fleet-proto-panel">
        <SectionHeading eyebrow="Game loop" title="Why this is fun" icon={RotateCcw} />
        <div className="fleet-proto-loop-grid">
          {gameMechanicLoops.map((loop, index) => (
            <div className="fleet-proto-loop" key={loop.label}>
              <span>{index + 1}</span>
              <strong>{loop.label}</strong>
              <small>{loop.detail}</small>
            </div>
          ))}
        </div>
      </div>

      <div className="fleet-proto-panel">
        <SectionHeading eyebrow="Current event" title={activeEvent.title} icon={AlertTriangle} />
        <p className="fleet-proto-copy">{activeEvent.playerChoice}</p>
        <p className="fleet-proto-copy">Backend hook: {activeEvent.backendFeed}</p>
      </div>

      <div className="fleet-proto-panel">
        <SectionHeading eyebrow="Replay frame" title={replayFrame.headline} icon={Clock} />
        <div className="fleet-proto-score-row">
          <Meter label="confidence" value={replayFrame.confidence} />
          <Meter label="measured" value={projection.valueBasisSummary.measured * 18} />
          <Meter label="imputed" value={projection.valueBasisSummary.imputed * 18} />
        </div>
      </div>

      <div className="fleet-proto-state-dump">
        <strong>Visible prototype state</strong>
        <pre>{JSON.stringify(stateDump, null, 2)}</pre>
      </div>
    </section>
  );
}

function SectionHeading({
  eyebrow,
  icon: Icon,
  title
}: {
  eyebrow: string;
  icon: typeof Activity;
  title: string;
}) {
  return (
    <div className="fleet-proto-section-heading">
      <div>
        <p className="eyebrow">{eyebrow}</p>
        <h3>{title}</h3>
      </div>
      <Icon size={18} />
    </div>
  );
}

function PrototypeMetric({
  icon: Icon,
  label,
  value
}: {
  icon: typeof Activity;
  label: string;
  value: string;
}) {
  return (
    <span className="fleet-proto-metric">
      <Icon size={16} />
      <small>{label}</small>
      <strong>{value}</strong>
    </span>
  );
}

function Meter({ label, value }: { label: string; value: number }) {
  const clampedValue = Math.max(0, Math.min(100, Math.round(value)));
  const style = { "--value": `${clampedValue}%` } as CSSProperties;
  return (
    <span className="fleet-proto-meter" style={style}>
      <small>{label}</small>
      <strong>{clampedValue}%</strong>
      <i />
    </span>
  );
}

function freshnessScore(warning: string | null, frameIndex: number, unitIndex: number): number {
  if (warning?.includes("stale")) return Math.max(18, 44 - frameIndex * 4);
  if (warning?.includes("late")) return Math.max(28, 68 - frameIndex * 5);
  return Math.max(40, 94 - unitIndex * 3 - frameIndex * 2);
}

function outputScore(unit: ProjectedFleetUnit, frameIndex: number): number {
  const base = unit.electricOutputMwe * 58 + unit.usefulThermalOutputMwt * 14 + (unit.residualHeatMwth ?? 0) * 80;
  return Math.min(100, Math.max(12, base + frameIndex * 3));
}
