import { Camera, Gauge, Grid3X3, Layers, Route, SlidersHorizontal } from "lucide-react";
import type {
  BoardSceneControlState,
  BoardSceneWorkbenchDensity,
  BoardSceneWorkbenchScenario
} from "../../domain/fleet-board";

type BoardSceneControlPatch = Partial<Omit<BoardSceneControlState, "camera">> & {
  camera?: Partial<BoardSceneControlState["camera"]>;
};

export function BoardSceneControls({
  controls,
  scenarios,
  reactors,
  onChange
}: {
  controls: BoardSceneControlState;
  scenarios: BoardSceneWorkbenchScenario[];
  reactors: Array<{ id: string; label: string }>;
  onChange: (patch: BoardSceneControlPatch) => void;
}) {
  const selectedScenario = scenarios.find((scenario) => scenario.id === controls.scenarioId) ?? scenarios[0];

  return (
    <div className="board-scene-panel board-scene-controls" aria-label="Board scene controls">
      <div className="board-scene-scenario-buttons" role="group" aria-label="Scenario">
        {scenarios.map((scenario) => (
          <button
            key={scenario.id}
            type="button"
            aria-pressed={scenario.id === controls.scenarioId}
            className={scenario.id === controls.scenarioId ? "active" : ""}
            onClick={() => selectScenario(scenario, onChange)}
          >
            {scenario.name}
          </button>
        ))}
      </div>

      <label className="board-scene-control-field">
        <SlidersHorizontal size={16} />
        <span>Seed</span>
        <input
          data-testid="board-scene-seed"
          type="text"
          value={controls.seed}
          onChange={(event) => onChange({ seed: event.target.value })}
        />
      </label>

      <label className="board-scene-control-field">
        <Gauge size={16} />
        <span>Day</span>
        <input
          data-testid="board-scene-day"
          type="number"
          min={0}
          max={30}
          value={controls.day}
          onChange={(event) => onChange({ day: event.target.valueAsNumber })}
        />
      </label>

      <label className="board-scene-control-field">
        <Grid3X3 size={16} />
        <span>Selected reactor</span>
        <select
          data-testid="board-scene-selected-reactor"
          value={controls.selectedReactorId}
          onChange={(event) => onChange({ selectedReactorId: event.target.value })}
        >
          <option value="">None</option>
          {reactors.map((reactor) => (
            <option key={reactor.id} value={reactor.id}>
              {reactor.label}
            </option>
          ))}
        </select>
      </label>

      <fieldset className="board-scene-control-field board-scene-camera-controls">
        <legend>
          <Camera size={16} />
          <span>Camera</span>
        </legend>
        <label>
          <span>Zoom</span>
          <input
            aria-label="Camera zoom"
            type="number"
            min={0.82}
            max={1.25}
            step={0.01}
            value={controls.camera.zoom}
            onChange={(event) => onChange({ camera: { zoom: event.target.valueAsNumber } })}
          />
        </label>
        <label>
          <span>Pan X</span>
          <input
            aria-label="Camera pan X"
            type="number"
            min={0}
            max={600}
            value={controls.camera.panX}
            onChange={(event) => onChange({ camera: { panX: event.target.valueAsNumber } })}
          />
        </label>
        <label>
          <span>Pan Y</span>
          <input
            aria-label="Camera pan Y"
            type="number"
            min={0}
            max={420}
            value={controls.camera.panY}
            onChange={(event) => onChange({ camera: { panY: event.target.valueAsNumber } })}
          />
        </label>
      </fieldset>

      <div className="board-scene-control-field">
        <Layers size={16} />
        <span>Density</span>
        <div className="board-scene-segmented" data-testid="board-scene-density">
          {(["review", "compact"] as BoardSceneWorkbenchDensity[]).map((density) => (
            <button
              key={density}
              type="button"
              aria-pressed={controls.density === density}
              onClick={() => onChange({ density })}
            >
              {density}
            </button>
          ))}
        </div>
      </div>

      <label className="board-scene-control-field board-scene-toggle">
        <Route size={16} />
        <span>Reduced motion</span>
        <input
          data-testid="board-scene-reduced-motion"
          type="checkbox"
          checked={controls.reducedMotion}
          onChange={(event) => onChange({ reducedMotion: event.target.checked })}
        />
      </label>

      <output className="board-scene-effective-state" aria-live="polite">
        <span>{selectedScenario?.name ?? "Scenario"}</span>
        <strong>
          Day {controls.day} · {controls.selectedReactorId || "no reactor"} · {controls.camera.zoom.toFixed(2)}x
        </strong>
      </output>
    </div>
  );
}

function selectScenario(
  scenario: BoardSceneWorkbenchScenario,
  onChange: (patch: BoardSceneControlPatch) => void
) {
  onChange({
    scenarioId: scenario.id,
    seed: scenario.seed,
    day: scenario.scene.day,
    selectedReactorId: scenario.scene.selectedReactorId ?? "reactor-2"
  });
}
