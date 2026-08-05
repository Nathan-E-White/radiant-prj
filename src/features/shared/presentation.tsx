import { Box, GitBranch, Waves } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import type { WorkbenchValueBasis } from "../../api/simulatorWorkbench";
import { visualGrammarSpec } from "../../domain/visualGrammar";
import type { DeploymentCheck } from "../../domain/types";

type MetricTone = "good" | "warn" | "info";

export function Metric({
  icon: Icon,
  label,
  value,
  tone
}: {
  icon: LucideIcon;
  label: string;
  value: string;
  tone: MetricTone;
}) {
  return (
    <div className={`metric ${tone}`}>
      <Icon size={18} />
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

export function StatusPill({ label, state }: { label: string; state: string }) {
  return <span className={`status-pill ${state}`}>{label}</span>;
}

export function ValueBasisMarker({ basis }: { basis: WorkbenchValueBasis }) {
  const spec = visualGrammarSpec.valueBasisSpecs[basis];
  const Icon = basis === "measured" ? Waves : basis === "imputed" ? GitBranch : Box;

  return (
    <span className={`value-basis-marker ${basis}`} aria-label={spec.label}>
      <Icon size={14} />
      <span>{spec.label}</span>
      <small>{spec.ruleStyle}</small>
    </span>
  );
}

export function VisualGrammarSpecimen() {
  return (
    <article className="grammar-specimen" aria-label="Radiant visual grammar specimen">
      <div className="grammar-specimen-heading">
        <div>
          <p className="eyebrow">Living specimen</p>
          <h3>Radiant experience grammar</h3>
        </div>
        <StatusPill label={`${visualGrammarSpec.density} density`} state="completed" />
      </div>

      <div className="grammar-specimen-section">
        <h4>Semantic Color</h4>
        <div className="grammar-swatch-grid">
          {visualGrammarSpec.semanticColorTokens.map((token) => (
            <span className="grammar-token" key={token.id}>
              <i className={token.className} aria-hidden="true" />
              <strong>{token.label}</strong>
              <small>{token.usage}</small>
            </span>
          ))}
        </div>
      </div>

      <div className="grammar-specimen-section">
        <h4>Type And Density</h4>
        <div className="grammar-type-grid">
          {visualGrammarSpec.typeScaleTokens.map((token) => (
            <span className={`grammar-type-token ${token.className}`} key={token.id}>
              <strong>{token.label}</strong>
              <small>{token.usage}</small>
            </span>
          ))}
        </div>
      </div>

      <div className="grammar-specimen-section">
        <h4>Rules And Targets</h4>
        <div className="grammar-rule-grid">
          {visualGrammarSpec.ruleStyleTokens.map((token) => (
            <span className={`grammar-rule-token ${token.className}`} key={token.id}>
              <strong>{token.label}</strong>
              <small>{token.usage}</small>
            </span>
          ))}
          {visualGrammarSpec.targetSizeTokens.map((token) => (
            <span className={`grammar-rule-token ${token.className}`} key={token.id}>
              <strong>{token.label}</strong>
              <small>{token.usage}</small>
            </span>
          ))}
        </div>
      </div>

      <div className="grammar-specimen-section">
        <h4>Interaction States</h4>
        <div className="grammar-state-grid">
          {visualGrammarSpec.interactionStateSpecs.map((state) => (
            <span className={`grammar-state-token ${state.className}`} key={state.id}>
              <strong>{state.label}</strong>
              <small>{state.usage}</small>
            </span>
          ))}
        </div>
      </div>

      <div className="grammar-specimen-section">
        <h4>Value Basis</h4>
        <div className="grammar-basis-grid">
          {Object.values(visualGrammarSpec.valueBasisSpecs).map((spec) => (
            <span className={`grammar-basis-card ${spec.id}`} key={spec.id}>
              <ValueBasisMarker basis={spec.id} />
              <strong>{spec.iconLabel}</strong>
              <small>{spec.texture}</small>
              <em>{spec.usage}</em>
            </span>
          ))}
        </div>
      </div>
    </article>
  );
}

export function Finding({ label, value }: { label: string; value: string }) {
  return (
    <article className="finding">
      <span>{label}</span>
      <p>{value}</p>
    </article>
  );
}

export function LogBlock({ logs }: { logs: string[] }) {
  return (
    <pre className="log-block">
      {logs.map((line, index) => (
        <code key={line}>
          {String(index + 1).padStart(2, "0")}  {line}
          {"\n"}
        </code>
      ))}
    </pre>
  );
}

export function Check({ label, value }: { label: string; value: "pass" | "warn" | "fail" }) {
  return (
    <span className={`check ${value}`}>
      {label}
      <strong>{value}</strong>
    </span>
  );
}

export function DeploymentCard({ check }: { check: DeploymentCheck }) {
  return (
    <article className="deployment-card">
      <div>
        <span className="record-id">{check.id}</span>
        <h3>{check.hostRole}</h3>
      </div>
      <p>{check.finding}</p>
      <div className="check-row">
        <Check label="config" value={check.configStatus} />
        <Check label="service" value={check.serviceStatus} />
        <Check label="net/storage" value={check.networkStorage} />
      </div>
      <small>{check.linkedRequirement}</small>
    </article>
  );
}
