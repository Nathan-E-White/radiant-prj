import type { WorkbenchLineage } from "../../api/simulatorWorkbench";

export function LineagePanel({ lineage }: { lineage: WorkbenchLineage }) {
  return (
    <section aria-label="Lineage scaffold">
      <h2>Lineage</h2>
      <p>
        {lineage.valueId} {lineage.valueBasis}
      </p>
    </section>
  );
}
