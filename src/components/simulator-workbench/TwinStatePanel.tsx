import type { DigitalTwinState } from "../../api/simulatorWorkbench";

export function TwinStatePanel({ twin }: { twin: DigitalTwinState }) {
  return (
    <section aria-label="Digital twin state scaffold">
      <h2>Imputed</h2>
      <p>{twin.twinId}</p>
    </section>
  );
}
