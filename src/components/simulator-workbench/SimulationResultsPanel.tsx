import { Box, CheckCircle2, Database, Radio } from "lucide-react";
import type { ReactNode } from "react";
import type { WorkbenchBasisGroup, WorkbenchHealthSummary } from "../../domain/simulator-workbench";
import { WorkbenchValueList } from "./ValueList";

export function SimulationResultsPanel({
  group,
  healthSummary,
  selectedValueId,
  onSelectValue
}: {
  group: WorkbenchBasisGroup;
  healthSummary: WorkbenchHealthSummary;
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
          <Box size={17} />
          {group.count}
        </span>
      </div>
      <div className="simwb-health-grid">
        <SummaryMetric icon={<CheckCircle2 size={15} />} label="Results" value={healthSummary.detail} />
        <SummaryMetric icon={<Database size={15} />} label="Artifacts" value={healthSummary.artifactStatuses.join(", ") || "none"} />
        <SummaryMetric icon={<Radio size={15} />} label="Scope" value={healthSummary.scope} />
      </div>
      <WorkbenchValueList selectedValueId={selectedValueId} values={group.values} onSelectValue={onSelectValue} />
    </section>
  );
}

function SummaryMetric({ icon, label, value }: { icon: ReactNode; label: string; value: string }) {
  return (
    <span className="simwb-summary-metric">
      {icon}
      <small>{label}</small>
      <strong>{value}</strong>
    </span>
  );
}
