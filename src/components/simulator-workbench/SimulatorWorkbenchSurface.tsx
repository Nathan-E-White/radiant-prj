import {
  Activity,
  Box,
  CheckCircle2,
  Database,
  FileText,
  GitBranch,
  Layers3,
  Radio,
  ShieldCheck,
  Waves
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import twinConceptUrl from "../../../docs/design/simulator-workbench-visuals/digital-twin-concept-v1.png";
import type {
  ProjectedWorkbenchValue,
  WorkbenchBasisGroup,
  WorkbenchHealthSummary,
  WorkbenchProjection
} from "../../domain/simulator-workbench";
import type { WorkbenchValueBasis } from "../../api/simulatorWorkbench";

export function SimulatorWorkbenchSurface({
  projection,
  onSelectValue
}: {
  projection: WorkbenchProjection;
  onSelectValue: (valueId: string) => void;
}) {
  return (
    <section className="simwb-shell" aria-label="Simulator Workbench">
      <div className="simwb-head">
        <div>
          <p className="eyebrow">Simulator Workbench</p>
          <h2>{projection.scenarioId}</h2>
        </div>
        <div className="simwb-context">
          <span>Asset: Core Test Asset A</span>
          <span>Twin: {projection.twinId}</span>
          <span>Generated: {formatTime(projection.generatedAt)}</span>
        </div>
      </div>

      <div className="simwb-grid">
        <aside className="simwb-stack">
          <BasisPanel
            group={projection.groups.measured}
            icon={Waves}
            selectedValueId={projection.selectedValue?.valueId ?? ""}
            onSelectValue={onSelectValue}
          />
          <HealthSummaryCard summary={projection.healthSummary} />
        </aside>

        <section className="simwb-viewport-panel" aria-label="Digital twin viewport">
          <div className="simwb-tool-strip" aria-label="Workbench layer summary">
            <LayerChip basis="measured" count={projection.valueBasisSummary.measured} />
            <LayerChip basis="imputed" count={projection.valueBasisSummary.imputed} />
            <LayerChip basis="simulated" count={projection.valueBasisSummary.simulated} />
          </div>
          <div className="simwb-viewport">
            <img src={twinConceptUrl} alt="Public-safe simulator workbench digital twin concept" />
            {projection.selectedValue && (
              <div className={`simwb-selected-callout ${projection.selectedValue.valueBasis}`}>
                <span>{projection.selectedValue.entityName}</span>
                <strong>{projection.selectedValue.label}</strong>
                <small>
                  {projection.selectedValue.displayValue} {projection.selectedValue.unit}
                </small>
              </div>
            )}
          </div>
        </section>

        <aside className="simwb-stack">
          <BasisPanel
            group={projection.groups.imputed}
            icon={Activity}
            selectedValueId={projection.selectedValue?.valueId ?? ""}
            onSelectValue={onSelectValue}
          />
          <BasisPanel
            group={projection.groups.simulated}
            icon={Box}
            selectedValueId={projection.selectedValue?.valueId ?? ""}
            onSelectValue={onSelectValue}
          />
        </aside>
      </div>

      <LineageRail projection={projection} />
    </section>
  );
}

function BasisPanel({
  group,
  icon: Icon,
  selectedValueId,
  onSelectValue
}: {
  group: WorkbenchBasisGroup;
  icon: LucideIcon;
  selectedValueId: string;
  onSelectValue: (valueId: string) => void;
}) {
  return (
    <section className={`simwb-card basis-${group.basis}`} aria-label={group.panelTitle}>
      <div className="simwb-card-heading">
        <div>
          <p className="eyebrow">{group.label}</p>
          <h3>{group.panelTitle}</h3>
        </div>
        <span className={`simwb-count ${group.basis}`}>
          <Icon size={17} />
          {group.count}
        </span>
      </div>
      <div className="simwb-value-list">
        {group.values.map((value) => (
          <ValueButton
            key={value.valueId}
            selected={value.valueId === selectedValueId}
            value={value}
            onSelectValue={onSelectValue}
          />
        ))}
      </div>
    </section>
  );
}

function ValueButton({
  value,
  selected,
  onSelectValue
}: {
  value: ProjectedWorkbenchValue;
  selected: boolean;
  onSelectValue: (valueId: string) => void;
}) {
  return (
    <button
      aria-pressed={selected}
      className={selected ? `simwb-value selected ${value.valueBasis}` : `simwb-value ${value.valueBasis}`}
      onClick={() => onSelectValue(value.valueId)}
      type="button"
    >
      <span className="simwb-value-main">
        <strong>{value.label}</strong>
        <small>{value.entityName}</small>
      </span>
      <span className="simwb-value-reading">
        <strong>{value.displayValue}</strong>
        <small>{value.unit}</small>
      </span>
      <span className="simwb-value-meta">
        <small>{value.freshnessLabel}</small>
        <small>{value.confidencePct}%</small>
        <small>{value.sourceQuality}</small>
      </span>
    </button>
  );
}

function HealthSummaryCard({ summary }: { summary: WorkbenchHealthSummary }) {
  return (
    <section className={`simwb-card simwb-health-summary ${summary.status}`} aria-label="Simulation health summary">
      <div className="simwb-card-heading">
        <div>
          <p className="eyebrow">Simulation Summary</p>
          <h3>{summary.label}</h3>
        </div>
        <span className={`simwb-count ${summary.status}`}>
          <ShieldCheck size={17} />
          {summary.runCount}
        </span>
      </div>
      <div className="simwb-health-grid">
        <SummaryMetric icon={CheckCircle2} label="Runs" value={summary.detail} />
        <SummaryMetric icon={Database} label="Artifacts" value={summary.artifactStatuses.join(", ") || "none"} />
        <SummaryMetric icon={Radio} label="Scope" value={summary.scope} />
      </div>
    </section>
  );
}

function SummaryMetric({
  icon: Icon,
  label,
  value
}: {
  icon: LucideIcon;
  label: string;
  value: string;
}) {
  return (
    <span className="simwb-summary-metric">
      <Icon size={15} />
      <small>{label}</small>
      <strong>{value}</strong>
    </span>
  );
}

function LayerChip({ basis, count }: { basis: WorkbenchValueBasis; count: number }) {
  return (
    <span className={`simwb-layer ${basis}`}>
      <Layers3 size={15} />
      {basis}
      <strong>{count}</strong>
    </span>
  );
}

function LineageRail({ projection }: { projection: WorkbenchProjection }) {
  return (
    <section className="simwb-lineage" aria-label="Selected value lineage">
      <div className="simwb-lineage-heading">
        <div>
          <p className="eyebrow">Lineage</p>
          <h3>{projection.selectedValue?.label ?? "No value selected"}</h3>
        </div>
        {projection.selectedValue && (
          <span className={`simwb-count ${projection.selectedValue.valueBasis}`}>
            <GitBranch size={17} />
            {projection.selectedValue.valueBasis}
          </span>
        )}
      </div>
      {projection.selectedLineageMissing ? (
        <div className="simwb-lineage-missing">
          <FileText size={17} />
          <span>Lineage pending for {projection.selectedValue?.valueId}</span>
        </div>
      ) : (
        <div className="simwb-lineage-steps">
          {projection.lineageSteps.map((step) => (
            <article className={`simwb-lineage-step ${step.basis ?? "neutral"}`} key={step.id}>
              <span>{step.label}</span>
              <strong>{step.detail}</strong>
            </article>
          ))}
        </div>
      )}
    </section>
  );
}

function formatTime(value: string): string {
  return new Date(value).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}
