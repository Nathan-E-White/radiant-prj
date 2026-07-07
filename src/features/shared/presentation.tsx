import type { LucideIcon } from "lucide-react";
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
