import type { KeyboardEvent, ReactNode } from "react";
import twinConceptUrl from "../../../docs/design/simulator-workbench-visuals/digital-twin-concept-v1.png";
import type { TwinViewportEntity } from "../../api/simulatorWorkbench";
import type {
  ProjectedWorkbenchValue,
  TwinViewportLayer,
  TwinViewportModel
} from "../../domain/simulator-workbench";

type EntityShape = {
  label: string;
  render: (interactive: boolean) => ReactNode;
};

export function TwinViewport({
  model,
  selectedValue,
  values,
  onSelectValue
}: {
  model: TwinViewportModel;
  selectedValue: ProjectedWorkbenchValue | null;
  values: ProjectedWorkbenchValue[];
  onSelectValue: (valueId: string) => void;
}) {
  const layerByEntity = firstLayerByEntity(model.layers);
  const valueByEntity = firstValueByEntity(values);

  function selectEntity(entityId: TwinViewportEntity) {
    const value = valueByEntity.get(entityId);
    if (value) {
      onSelectValue(value.valueId);
    }
  }

  function handleEntityKey(event: KeyboardEvent<SVGGElement>, entityId: TwinViewportEntity) {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      selectEntity(entityId);
    }
  }

  return (
    <section className="simwb-viewport-panel" aria-label="Selected Kaleidos Unit Twin Viewport">
      <div className="simwb-tool-strip" aria-label="Workbench layer summary">
        <span className="simwb-layer measured">measured</span>
        <span className="simwb-layer imputed">imputed</span>
        <span className="simwb-layer simulated">simulated</span>
      </div>
      <div className="simwb-viewport">
        <img src={twinConceptUrl} alt="Public-safe simulator workbench digital twin concept" />
        <svg
          aria-label="Kaleidos Unit twin topology overlay"
          className="simwb-overlay"
          role="img"
          viewBox="0 0 100 64"
        >
          {model.entityIds.map((entityId) => {
            const layer = layerByEntity.get(entityId);
            const interactive = Boolean(layer);
            const shape = entityShapes[entityId];
            return (
              <g
                aria-label={shape.label}
                className={overlayClass(layer)}
                data-value-basis={layer?.valueBasis ?? "none"}
                id={entityId}
                key={entityId}
                onClick={interactive ? () => selectEntity(entityId) : undefined}
                onKeyDown={interactive ? (event) => handleEntityKey(event, entityId) : undefined}
                role={interactive ? "button" : "presentation"}
                tabIndex={interactive ? 0 : undefined}
              >
                <title>{layer?.label ?? shape.label}</title>
                {shape.render(interactive)}
              </g>
            );
          })}
        </svg>
        {selectedValue && (
          <div className={`simwb-selected-callout ${selectedValue.valueBasis}`}>
            <span>{selectedValue.entityName}</span>
            <strong>{selectedValue.label}</strong>
            <small>
              {selectedValue.displayValue} {selectedValue.unit}
            </small>
          </div>
        )}
      </div>
    </section>
  );
}

function firstLayerByEntity(layers: TwinViewportLayer[]): Map<TwinViewportEntity, TwinViewportLayer> {
  const mapped = new Map<TwinViewportEntity, TwinViewportLayer>();
  for (const layer of layers) {
    const current = mapped.get(layer.entityId);
    if (!current || layer.selected) {
      mapped.set(layer.entityId, layer);
    }
  }
  return mapped;
}

function firstValueByEntity(values: ProjectedWorkbenchValue[]): Map<TwinViewportEntity, ProjectedWorkbenchValue> {
  const mapped = new Map<TwinViewportEntity, ProjectedWorkbenchValue>();
  for (const value of values) {
    if (!mapped.has(value.viewportEntity) || value.valueBasis === "imputed") {
      mapped.set(value.viewportEntity, value);
    }
  }
  return mapped;
}

function overlayClass(layer: TwinViewportLayer | undefined): string {
  if (!layer) {
    return "simwb-overlay-entity idle";
  }
  return layer.selected
    ? `simwb-overlay-entity selected ${layer.valueBasis}`
    : `simwb-overlay-entity ${layer.valueBasis}`;
}

const entityShapes: Record<TwinViewportEntity, EntityShape> = {
  core: {
    label: "core",
    render: () => (
      <>
        <ellipse className="simwb-overlay-visible" cx="50" cy="31" rx="12" ry="16" />
        <ellipse className="simwb-overlay-hit" cx="50" cy="31" rx="18" ry="22" />
      </>
    )
  },
  controlDrums: {
    label: "control drums",
    render: () => (
      <>
        <path className="simwb-overlay-visible" d="M35 20 L41 16 L43 45 L36 43 Z M59 16 L65 20 L64 43 L57 45 Z" />
        <path className="simwb-overlay-hit" d="M31 14 L45 10 L47 50 L32 48 Z M55 10 L69 14 L68 48 L53 50 Z" />
      </>
    )
  },
  primaryLoop: {
    label: "primary helium loop",
    render: () => (
      <>
        <path className="simwb-overlay-visible" d="M23 32 C23 13 77 13 77 32 C77 51 23 51 23 32 Z" />
        <path className="simwb-overlay-hit" d="M17 32 C17 8 83 8 83 32 C83 56 17 56 17 32 Z" />
      </>
    )
  },
  heatExchangers: {
    label: "heat exchangers",
    render: () => (
      <>
        <rect className="simwb-overlay-visible" height="18" rx="2" width="14" x="75" y="22" />
        <rect className="simwb-overlay-hit" height="28" rx="4" width="24" x="70" y="17" />
      </>
    )
  },
  circulators: {
    label: "circulators",
    render: () => (
      <>
        <circle className="simwb-overlay-visible" cx="24" cy="43" r="6" />
        <circle className="simwb-overlay-hit" cx="24" cy="43" r="12" />
      </>
    )
  },
  vessel: {
    label: "vessel",
    render: () => (
      <>
        <rect className="simwb-overlay-visible" height="42" rx="16" width="34" x="33" y="10" />
        <rect className="simwb-overlay-hit" height="50" rx="18" width="42" x="29" y="6" />
      </>
    )
  },
  shielding: {
    label: "shielding",
    render: () => (
      <>
        <rect className="simwb-overlay-visible" height="52" rx="18" width="46" x="27" y="6" />
        <rect className="simwb-overlay-hit" height="58" rx="20" width="54" x="23" y="3" />
      </>
    )
  },
  containerBoundary: {
    label: "container boundary",
    render: () => (
      <>
        <rect className="simwb-overlay-visible" height="58" rx="8" width="78" x="11" y="3" />
        <rect className="simwb-overlay-hit" height="62" rx="10" width="86" x="7" y="1" />
      </>
    )
  },
  secondaryHeatUse: {
    label: "secondary heat use",
    render: () => (
      <>
        <path className="simwb-overlay-visible" d="M86 32 H98" />
        <path className="simwb-overlay-hit" d="M80 24 H100 V40 H80 Z" />
      </>
    )
  },
  powerConversion: {
    label: "power conversion",
    render: () => (
      <>
        <rect className="simwb-overlay-visible" height="12" rx="2" width="16" x="5" y="26" />
        <rect className="simwb-overlay-hit" height="24" rx="4" width="28" x="0" y="20" />
      </>
    )
  }
};
