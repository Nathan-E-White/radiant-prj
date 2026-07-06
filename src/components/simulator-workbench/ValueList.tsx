import type { ProjectedWorkbenchValue } from "../../domain/simulator-workbench";

export function WorkbenchValueList({
  selectedValueId,
  values,
  onSelectValue
}: {
  selectedValueId: string;
  values: ProjectedWorkbenchValue[];
  onSelectValue: (valueId: string) => void;
}) {
  return (
    <div className="simwb-value-list">
      {values.map((value) => (
        <ValueButton
          key={value.valueId}
          selected={value.valueId === selectedValueId}
          value={value}
          onSelectValue={onSelectValue}
        />
      ))}
    </div>
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
        {value.freshnessLabel && <small>{value.freshnessLabel}</small>}
        <small>{value.confidencePct}%</small>
        <small>{value.sourceQuality}</small>
      </span>
    </button>
  );
}
