import type { MeasuredTelemetryFrame } from "../../api/simulatorWorkbench";

export function MeasuredStatePanel({ frames }: { frames: MeasuredTelemetryFrame[] }) {
  return (
    <section aria-label="Measured state scaffold">
      <h2>Measured</h2>
      <ul>
        {frames.map((frame) => (
          <li key={`${frame.sourceId}-${frame.tagId}-${frame.sequence}`}>
            {frame.tagId} {frame.quality}
          </li>
        ))}
      </ul>
    </section>
  );
}
