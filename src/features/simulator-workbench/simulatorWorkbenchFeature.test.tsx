import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import { fixtures } from "../../domain/readiness";
import { StatusWorkbenchTab } from "./simulatorWorkbenchFeature";

describe("Status Workbench session presentation", () => {
  it("keeps a typed terminal initial error visible and retryable", () => {
    const markup = renderToStaticMarkup(
      <StatusWorkbenchTab
        projection={null}
        readState={{
          phase: "error",
          model: null,
          errorKind: "auth",
          message: "Workbench authorization failed (401)."
        }}
        onRefresh={vi.fn()}
        onSelectUnit={vi.fn()}
        onSelectValue={vi.fn()}
        computeQueue={<div>Scientific compute queue</div>}
        selectedJob={fixtures.computeJobs[0]}
        scenario="scheduler-drift"
        scenarioJobs={fixtures.computeJobs}
        bundleState="ready"
        orchestrationPanel={<div>Container orchestration</div>}
      />
    );

    expect(markup).toContain("Live Snapshot error");
    expect(markup).toContain("Workbench authorization failed (401).");
    expect(markup).toContain("Retry live Snapshot");
    expect(markup).not.toContain("/api/simulator-workbench");
  });
});
