import type { KeyboardEvent, ReactNode } from "react";
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
        <svg
          aria-label="Kaleidos Unit twin topology overlay"
          className="simwb-overlay"
          role="img"
          viewBox="0 0 100 64"
        >
          <title>Kaleidos Unit public-safe digital twin schematic</title>
          <SchematicPlate />
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

function SchematicPlate() {
  return (
    <g aria-hidden="true" className="simwb-schematic">
      <defs>
        <linearGradient id="simwb-schematic-vessel-gradient" x1="0" x2="1" y1="0" y2="1">
          <stop offset="0%" stopColor="#d8f5f0" stopOpacity="0.24" />
          <stop offset="52%" stopColor="#7fc8d8" stopOpacity="0.1" />
          <stop offset="100%" stopColor="#d8f5f0" stopOpacity="0.2" />
        </linearGradient>
        <linearGradient id="simwb-schematic-core-gradient" x1="0" x2="0" y1="0" y2="1">
          <stop offset="0%" stopColor="#f3c058" stopOpacity="0.16" />
          <stop offset="48%" stopColor="#f08548" stopOpacity="0.44" />
          <stop offset="100%" stopColor="#55dda0" stopOpacity="0.18" />
        </linearGradient>
      </defs>

      <rect className="simwb-schematic-bg" height="64" width="100" />
      <path className="simwb-schematic-grid" d="M0 8H100 M0 16H100 M0 24H100 M0 32H100 M0 40H100 M0 48H100 M0 56H100" />
      <path className="simwb-schematic-grid" d="M10 0V64 M20 0V64 M30 0V64 M40 0V64 M50 0V64 M60 0V64 M70 0V64 M80 0V64 M90 0V64" />
      <path className="simwb-schematic-grid diagonal" d="M-8 64L32 24 M8 64L48 24 M24 64L64 24 M40 64L80 24 M56 64L96 24 M72 64L112 24" />

      <rect className="simwb-schematic-boundary" height="58" rx="8" width="78" x="11" y="3" />
      <rect className="simwb-schematic-shield" height="52" rx="18" width="46" x="27" y="6" />
      <path className="simwb-schematic-loop cool" d="M22 32C22 13 78 13 78 32C78 51 22 51 22 32Z" />
      <path className="simwb-schematic-loop hot" d="M18 35C29 53 70 53 82 35" />
      <rect className="simwb-schematic-vessel" height="42" rx="16" width="34" x="33" y="10" />
      <ellipse className="simwb-schematic-core" cx="50" cy="31" rx="9" ry="14" />
      <path className="simwb-schematic-core-lines" d="M42 25H58 M41 31H59 M42 37H58 M47 18V44 M53 18V44" />

      <path className="simwb-schematic-drums" d="M35 20L41 16L43 45L36 43Z M59 16L65 20L64 43L57 45Z" />
      <circle className="simwb-schematic-circulator" cx="24" cy="43" r="6" />
      <path className="simwb-schematic-circulator-blade" d="M24 37V49 M18 43H30 M20 39L28 47 M28 39L20 47" />

      <rect className="simwb-schematic-module power" height="12" rx="2" width="16" x="5" y="26" />
      <path className="simwb-schematic-module-lines" d="M9 29H17 M9 32H17 M9 35H17" />
      <rect className="simwb-schematic-module exchanger" height="18" rx="2" width="14" x="75" y="22" />
      <path className="simwb-schematic-module-lines" d="M78 25H86 M78 29H86 M78 33H86 M78 37H86" />
      <path className="simwb-schematic-secondary" d="M86 32H98" />

      <path className="simwb-schematic-flow measured" d="M21 32C21 18 38 13 50 13C62 13 79 18 79 32" />
      <path className="simwb-schematic-flow imputed" d="M50 45C40 43 36 37 36 31C36 24 41 18 50 17C59 18 64 24 64 31C64 37 60 43 50 45Z" />
      <path className="simwb-schematic-flow simulated" d="M17 34H24 M76 34H90" />
    </g>
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
