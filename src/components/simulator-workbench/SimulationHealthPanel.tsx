import { AlertTriangle, CloudCog, Cpu, RefreshCw } from "lucide-react";
import type { ReactNode } from "react";

export type SimulationHealthSeverity = "healthy" | "degraded" | "critical" | "stale";

export type SimulationHealthCard = {
  title: string;
  summary: string;
  detail: string;
  status: SimulationHealthSeverity;
};

export type SimulationHealthPanelModel = {
  lifecycle: SimulationHealthCard;
  artifact: SimulationHealthCard;
  worker: SimulationHealthCard;
  streamFreshness: SimulationHealthCard;
};

export type SimulationHealthPanelProps = {
  model: SimulationHealthPanelModel;
};

const iconByCard: Record<keyof SimulationHealthPanelModel, ReactNode> = {
  lifecycle: <CloudCog size={15} />,
  artifact: <AlertTriangle size={15} />,
  worker: <Cpu size={15} />,
  streamFreshness: <RefreshCw size={15} />
};

export function SimulationHealthPanel({ model }: SimulationHealthPanelProps) {
  return (
    <section className="simwb-card" aria-label="Simulation health cards">
      <div className="simwb-card-heading">
        <div>
          <p className="eyebrow">Status Workbench</p>
          <h3>HPC Status Summary (4-card fixture)</h3>
        </div>
        <span className="simwb-count complete">4 cards</span>
      </div>

      <div className="simwb-health-grid">
        {(Object.entries(model) as Array<[keyof SimulationHealthPanelModel, SimulationHealthCard]>).map(
          ([cardType, card]) => (
            <HealthCard
              icon={iconByCard[cardType]}
              card={card}
              key={cardType}
            />
          )
        )}
      </div>
    </section>
  );
}

function HealthCard({ icon, card }: { icon: ReactNode; card: SimulationHealthCard }) {
  return (
    <span className="simwb-summary-metric">
      {icon}
      <small>{card.title}</small>
      <strong>{card.summary}</strong>
      <span className={`status-pill ${statusClass(card.status)}`}>{card.status}</span>
      <small>{card.detail}</small>
    </span>
  );
}

function statusClass(status: SimulationHealthSeverity) {
  if (status === "healthy") return "complete";
  if (status === "stale") return "queued";
  if (status === "degraded") return "degraded";
  return "failed";
}
