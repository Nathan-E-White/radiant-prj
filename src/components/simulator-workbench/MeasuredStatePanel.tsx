import { Waves } from "lucide-react";
import type { WorkbenchBasisGroup } from "../../domain/simulator-workbench";
import { WorkbenchValueList } from "./ValueList";

export function MeasuredStatePanel({
  group,
  selectedValueId,
  onSelectValue
}: {
  group: WorkbenchBasisGroup;
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
          <Waves size={17} />
          {group.count}
        </span>
      </div>
      <WorkbenchValueList selectedValueId={selectedValueId} values={group.values} onSelectValue={onSelectValue} />
    </section>
  );
}
