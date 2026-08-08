import { useMemo, useState } from "react";
import {
  buildBoardSceneWorkbench,
  resolveBoardSceneWorkbenchView,
  type BoardSceneControlState
} from "../../domain/fleet-board";
import type { WorkbenchProjection } from "../../domain/simulator-workbench";
import { BoardSceneControls } from "./BoardSceneControls";
import { FleetBoardCanvas } from "./FleetBoardCanvas";

export function BoardSceneWorkbench({ projection }: { projection: WorkbenchProjection }) {
  const workbench = useMemo(() => buildBoardSceneWorkbench(projection), [projection]);
  const [controls, setControls] = useState<BoardSceneControlState>(workbench.controls);
  const view = useMemo(() => resolveBoardSceneWorkbenchView(projection, controls), [projection, controls]);

  return (
    <section
      className={`board-scene-workbench board-scene-density-${view.controls.density}`}
      aria-label="Board Scene Workbench"
      data-testid="board-scene-workbench"
    >
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
            <article
              key={scenario.id}
              className={scenario.id === view.scenario.id ? "active" : ""}
            >
              <span>{scenario.name}</span>
              <small>{scenario.expectedVisibleOutcome}</small>
            </article>
          ))}
        </aside>

        <div className="board-scene-main">
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

          <div className="board-scene-summary" data-testid="board-scene-summary">
            <strong>{view.scenario.name}</strong>
            <span>{view.scenario.purpose}</span>
            <span>
              {view.summary.complete ? "complete" : view.summary.removed ? "removed" : "active"} ·{" "}
              {view.summary.queuedSimulationJobs} queued · {view.summary.runningSimulationJobs} running ·{" "}
              {view.summary.completedSimulationJobs} completed
            </span>
          </div>

          <FleetBoardCanvas
            scene={view.scene}
            onPlaceFacility={() => undefined}
            onSelectReactor={() => undefined}
            testId="board-scene-canvas"
          />
        </div>

        <aside className="board-scene-side">
          <section className="board-scene-panel" aria-label="Board Navigator">
            <h4>Board Navigator</h4>
            <div className="board-navigator-list">
              <h5>Facilities</h5>
              {view.navigator.facilities.map((facility) => (
                <article key={facility.id}>
                  <strong>{facility.label}</strong>
                  <span>{facility.kind} · {facility.status} · {facility.location}</span>
                </article>
              ))}
              <h5>Routes</h5>
              {view.navigator.routes.map((route) => (
                <article key={`${route.from}-${route.to}`}>
                  <strong>Route</strong>
                  <span>{route.label}</span>
                </article>
              ))}
              <h5>Reactor Slot Rails</h5>
              {view.navigator.reactorSlotRails.map((rail) => (
                <article key={rail.reactorId}>
                  <strong>Reactor Slot Rail</strong>
                  <span>{rail.reactorId}: {rail.slots.join(" / ")}</span>
                </article>
              ))}
              <h5>Pawns</h5>
              {view.navigator.pawns.map((pawn) => (
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
