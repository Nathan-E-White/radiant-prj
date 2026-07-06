import type { WorkbenchValue, WorkbenchValueBasis } from "../../api/simulatorWorkbench";

export const workbenchValueBasisOrder: WorkbenchValueBasis[] = ["measured", "imputed", "simulated"];

export function valueBasisLabel(basis: WorkbenchValueBasis): string {
  switch (basis) {
    case "measured":
      return "Measured";
    case "imputed":
      return "Imputed";
    case "simulated":
      return "Simulated";
  }
}

export function summarizeValueBasis(values: WorkbenchValue[]): Record<WorkbenchValueBasis, number> {
  return values.reduce<Record<WorkbenchValueBasis, number>>(
    (summary, value) => {
      summary[value.valueBasis] += 1;
      return summary;
    },
    { measured: 0, imputed: 0, simulated: 0 }
  );
}

export function flattenWorkbenchValues(
  entities: Array<{ values: WorkbenchValue[] }>
): WorkbenchValue[] {
  return entities.flatMap((entity) => entity.values);
}
