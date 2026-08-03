import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { deploymentScore, fixtures, requirementCoverage, validateTraceability } from "../../domain/readiness";
import { EvidenceTab } from "./readinessConsole";

describe("EvidenceTab visual grammar", () => {
  it("renders the living grammar specimen with required semantic states", () => {
    const markup = renderToStaticMarkup(
      <EvidenceTab
        requirements={fixtures.requirements}
        evidencePacks={fixtures.evidencePacks}
        controlledEvidence={fixtures.controlledEvidence}
        deploymentChecks={fixtures.deploymentChecks}
        coverage={requirementCoverage()}
        traceabilityProblems={validateTraceability()}
        deploymentReadiness={deploymentScore()}
      />
    );

    expect(markup).toContain("Radiant experience grammar");
    expect(markup).toContain("Semantic Color");
    expect(markup).toContain("Interaction States");
    expect(markup).toContain("Hover");
    expect(markup).toContain("Focus");
    expect(markup).toContain("Selected");
    expect(markup).toContain("Disabled");
    expect(markup).toContain("Warning");
    expect(markup).toContain("Current");
    expect(markup).toContain("Measured State");
    expect(markup).toContain("Imputed State");
    expect(markup).toContain("Simulated Result State");
    expect(markup).toContain("dashed run track");
  });
});
