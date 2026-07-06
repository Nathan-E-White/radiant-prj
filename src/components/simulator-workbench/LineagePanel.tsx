import { FileText, GitBranch } from "lucide-react";
import type { WorkbenchExplanation } from "../../domain/simulator-workbench";

export function LineagePanel({ explanation }: { explanation: WorkbenchExplanation }) {
  return (
    <section className="simwb-lineage" aria-label="Bottom Explanation Rail">
      <div className="simwb-lineage-heading">
        <div>
          <p className="eyebrow">{explanation.kind === "commercial" ? "Commercial Display Basis" : "Engineering Lineage"}</p>
          <h3>{explanation.title}</h3>
        </div>
        <span className={`simwb-count ${explanation.kind === "commercial" ? "commercial" : explanation.basisLabel}`}>
          <GitBranch size={17} />
          {explanation.basisLabel}
        </span>
      </div>
      <div className="simwb-explanation-grid">
        {explanation.items.map((item) => (
          <article className="simwb-explanation-item" key={`${item.label}-${item.value}`}>
            <span>{item.label}</span>
            <strong>{item.value}</strong>
          </article>
        ))}
      </div>
      {explanation.exclusions.length > 0 && (
        <div className="simwb-exclusions" aria-label="Display basis exclusions">
          {explanation.exclusions.map((exclusion) => (
            <span key={exclusion}>{exclusion}</span>
          ))}
        </div>
      )}
      {explanation.steps.length > 0 ? (
        <div className="simwb-lineage-steps">
          {explanation.steps.map((step) => (
            <article className={`simwb-lineage-step ${step.basis ?? "neutral"}`} key={step.id}>
              <span>{step.label}</span>
              <strong>{step.detail}</strong>
            </article>
          ))}
        </div>
      ) : (
        explanation.kind === "engineering" && (
          <div className="simwb-lineage-missing">
            <FileText size={17} />
            <span>{explanation.subtitle}</span>
          </div>
        )
      )}
    </section>
  );
}
