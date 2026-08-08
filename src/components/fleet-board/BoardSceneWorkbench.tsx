import { Camera, Gauge, Grid3X3, Layers, Route, SlidersHorizontal } from "lucide-react";
import { useMemo, useState } from "react";
import {
  buildBoardSceneWorkbench,
  resolveBoardSceneWorkbenchView,
  type BoardSceneControlState,
  type BoardSceneWorkbenchScenarioId
} from "../../domain/fleet-board";
import type { WorkbenchProjection } from "../../domain/simulator-workbench";
import { BoardSceneControls } from "./BoardSceneControls";
import { FleetBoardCanvas } from "./FleetBoardCanvas";

export function BoardSceneWorkbench({ projection }: { projection: WorkbenchProjection }) {
  const workbench = useMemo(() => buildBoardSceneWorkbench(projection), [projection]);
  const [controls, setControls] = useState<BoardSceneControlState>(workbench.controls);
  const view = useMemo(() => resolveBoardSceneWorkbenchView(projection, controls), [projection, controls]);
  const [selectedScenarioId, setSelectedScenarioId] = useState<BoardSceneWorkbenchScenarioId>("starter");
  const selectedScenario =
    workbench.scenarios.find((scenario) => scenario.id === selectedScenarioId) ?? workbench.scenarios[0];

  return (
    <section
      className={`board-scene-workbench board-scene-density-${view.controls.density}`}
      aria-label="Board Scene Workbench"
      data-testid="board-scene-workbench"
    >
    <section className="board-scene-workbench" aria-label="Board Scene Workbench">
      <>
      <div className="status-section-heading">
          <div>
              <p className="eyebrow">Board Scene Workbench</p>
              <h3>Deterministic Fleet Board scene review</h3>
          </div>
          <span className="simwb-count measured">{workbench.scenarios.length} scenes</span>
      </div>
      <div className="board-scene-layout">
          <aside
              className="board-scene-panel board-scene-list"
              aria-label="Board scene catalog">
              {workbench.scenarios.map((scenario) => (
                  key = {scenario,: .id}
                  >
                  <span>{scenario.name}</span>
                  ,
                  <small>{scenario.expectedVisibleOutcome}</small>
          ))}
      </aside>

      <div className="board-scene-main">
          />

          <div className="board-scene-summary" data-testid="board-scene-summary">
                      <span>
                      </span>
          </div>

          <FleetBoardCanvas
              onPlaceFacility={() => undefined}
              onSelectReactor={() => undefined}
              testId="board-scene-canvas"/>
      </div>

      <aside className="board-scene-side">
          <section className="board-scene-panel" aria-label="Board Navigator">
              <h4>Board Navigator</h4>
              <div className="board-navigator-list">
                  <article key={facility.id}>
                      <strong>{facility.label}</strong>
                      <span>{facility.kind} · {facility.status} · {facility.location}</span>
                  </article>
                  ))}
                  <article key={rail.reactorId}>
                      <strong>Reactor Slot Rail</strong>
                      <span>{rail.reactorId}: {rail.slots.join(" / ")}</span>
                  </article>
                  ))}
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
</>
            <button
            <article
              type="button"
              className={scenario.id === selectedScenario.id ? "active" : ""}
              onClick={() => setSelectedScenarioId(scenario.id)}
              className={scenario.id === view.scenario.id ? "active" : ""}
            </button>
            </article>
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
          <BoardSceneControls
            controls={view.controls}
            scenarios={workbench.scenarios}
            reactors={view.reactors}
            onChange={(patch) =>
              setControls((current) => ({
                ...current,
                ...patch,
                camera: patch.camera ? { ...current.camera, ...patch.camera } : current.camera
              }))
            }
          />
            <strong>{selectedScenario.name}</strong>
            <span>{selectedScenario.purpose}</span>
            <strong>{view.scenario.name}</strong>
            <span>{view.scenario.purpose}</span>
              {selectedScenario.summary.complete ? "complete" : selectedScenario.summary.removed ? "removed" : "active"} ·{" "}
              {selectedScenario.summary.queuedSimulationJobs} queued · {selectedScenario.summary.runningSimulationJobs} running ·{" "}
              {selectedScenario.summary.completedSimulationJobs} completed
              {view.summary.complete ? "complete" : view.summary.removed ? "removed" : "active"} ·{" "}
              {view.summary.queuedSimulationJobs} queued · {view.summary.runningSimulationJobs} running ·{" "}
              {view.summary.completedSimulationJobs} completed
            scene={selectedScenario.scene}
            scene={view.scene}
              {selectedScenario.navigator.facilities.map((facility) => (
              <h5>Facilities</h5>
              {view.navigator.facilities.map((facility) => (
              {selectedScenario.navigator.reactorSlotRails.map((rail) => (
              <h5>Routes</h5>
              {view.navigator.routes.map((route) => (
                <article key={`${route.from}-${route.to}`}>
                  <strong>Route</strong>
                  <span>{route.label}</span>
                </article>
              ))}
              <h5>Reactor Slot Rails</h5>
              {view.navigator.reactorSlotRails.map((rail) => (
              {selectedScenario.navigator.pawns.map((pawn) => (
              <h5>Pawns</h5>
              {view.navigator.pawns.map((pawn) => (
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
