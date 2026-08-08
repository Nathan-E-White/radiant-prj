import { Camera, Gauge, Grid3X3, Layers, Route, SlidersHorizontal } from "lucide-react";
import { useMemo, useState } from "react";
import {
  buildBoardSceneWorkbench,
  type BoardSceneWorkbenchScenarioId
} from "../../domain/fleet-board";
import type { WorkbenchProjection } from "../../domain/simulator-workbench";
import { FleetBoardCanvas } from "./FleetBoardCanvas";

export function BoardSceneWorkbench({ projection }: { projection: WorkbenchProjection }) {
  const workbench = useMemo(() => buildBoardSceneWorkbench(projection), [projection]);
  const [selectedScenarioId, setSelectedScenarioId] = useState<BoardSceneWorkbenchScenarioId>("starter");
  const selectedScenario =
    workbench.scenarios.find((scenario) => scenario.id === selectedScenarioId) ?? workbench.scenarios[0];

  return (
    <section className="board-scene-workbench" aria-label="Board Scene Workbench">
      <div className="status-section-heading">
        <div>
          <p className="eyebrow">Board Scene Workbench</p>
          <h3>Deterministic Fleet Board scene review</h3>
        </div>
        <span className="simwb-count measured">{workbench.scenarios.length} scenes</span>
      </div>

      <div className="board-scene-layout">
        <aside className="board-scene-panel board-scene-list" aria-label="Board scene catalog">
          {workbench.scenarios.map((scenario) => (
            <button
              key={scenario.id}
              type="button"
              className={scenario.id === selectedScenario.id ? "active" : ""}
              onClick={() => setSelectedScenarioId(scenario.id)}
            >
              <span>{scenario.name}</span>
              <small>{scenario.expectedVisibleOutcome}</small>
            </button>
          ))}
        </aside>

        <div className="board-scene-main">
          <div className="board-scene-panel board-scene-controls" aria-label="Board scene controls">
            <ControlMetric icon={SlidersHorizontal} label="Seed" value={selectedScenario.seed} testId="board-scene-seed" />
            <ControlMetric icon={Gauge} label="Day" value={`Day ${selectedScenario.scene.day}`} testId="board-scene-day" />
            <ControlMetric
              icon={Grid3X3}
              label="Selected reactor"
              value={selectedScenario.scene.selectedReactorId ?? "none"}
              testId="board-scene-selected-reactor"
            />
            <ControlMetric
              icon={Camera}
              label="Camera"
              value={`${workbench.controls.camera.zoom.toFixed(2)}x / ${workbench.controls.camera.panX},${workbench.controls.camera.panY}`}
            />
            <ControlMetric icon={Layers} label="Density" value={workbench.controls.density} />
            <ControlMetric icon={Route} label="Reduced motion" value={workbench.controls.reducedMotion ? "on" : "off"} />
          </div>

          <div className="board-scene-summary" data-testid="board-scene-summary">
            <strong>{selectedScenario.name}</strong>
            <span>{selectedScenario.purpose}</span>
            <span>
              {selectedScenario.summary.complete ? "complete" : selectedScenario.summary.removed ? "removed" : "active"} ·{" "}
              {selectedScenario.summary.queuedSimulationJobs} queued · {selectedScenario.summary.runningSimulationJobs} running ·{" "}
              {selectedScenario.summary.completedSimulationJobs} completed
            </span>
          </div>

          <FleetBoardCanvas
            scene={selectedScenario.scene}
            onPlaceFacility={() => undefined}
            onSelectReactor={() => undefined}
            testId="board-scene-canvas"
          />
        </div>

        <aside className="board-scene-side">
          <section className="board-scene-panel" aria-label="Board Navigator">
            <h4>Board Navigator</h4>
            <div className="board-navigator-list">
              {selectedScenario.navigator.facilities.map((facility) => (
                <article key={facility.id}>
                  <strong>{facility.label}</strong>
                  <span>{facility.kind} · {facility.status} · {facility.location}</span>
                </article>
              ))}
              {selectedScenario.navigator.reactorSlotRails.map((rail) => (
                <article key={rail.reactorId}>
                  <strong>Reactor Slot Rail</strong>
                  <span>{rail.reactorId}: {rail.slots.join(" / ")}</span>
                </article>
              ))}
              {selectedScenario.navigator.pawns.map((pawn) => (
                <article key={pawn.kind}>
                  <strong>{pawn.kind}</strong>
                  <span>{pawn.location}</span>
                </article>
              ))}
            </div>
          </section>

          <section className="board-scene-panel" aria-label="Asset atlas">
            <h4>Asset atlas</h4>
            <div className="asset-atlas-list">
              {workbench.assetAtlas.assets.map((asset) => (
                <span key={asset.semanticKey}>{asset.semanticKey}</span>
              ))}
            </div>
          </section>

          <section className="board-scene-panel" aria-label="Prototype decisions">
            <h4>Prototype decisions</h4>
            <div className="prototype-decision-list">
              {workbench.prototypeDecisions.map((decision) => (
                <article key={decision.behavior}>
                  <strong>{decision.behavior}</strong>
                  <span className={`status-pill ${decision.verdict}`}>{decision.verdict}</span>
                  <p>{decision.reason}</p>
                </article>
              ))}
            </div>
          </section>
        </aside>
      </div>
    </section>
  );
}

function ControlMetric({
  icon: Icon,
  label,
  value,
  testId
}: {
  icon: typeof Camera;
  label: string;
  value: string;
  testId?: string;
}) {
  return (
    <div className="board-scene-control">
      <Icon size={16} />
      <span>{label}</span>
      <strong data-testid={testId}>{value}</strong>
    </div>
  );
}
