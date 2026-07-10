import {
  Activity,
  Boxes,
  ClipboardCheck,
  Cpu,
  GitBranch,
  ServerCog,
  ShieldCheck
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";
import { Metric, StatusPill } from "../shared/presentation";

export type AppTabId = "welcome" | "status" | "evidence";

type Tab = { id: AppTabId; label: string; icon: LucideIcon };

export const appTabItems: Array<Tab> = [
  { id: "welcome", label: "Welcome", icon: Boxes },
  { id: "status", label: "Status Workbench", icon: Activity },
  { id: "evidence", label: "Evidence", icon: ClipboardCheck }
];

type AppShellProps = {
  activeTab: AppTabId;
  onTabChange: (tabId: AppTabId) => void;
  jobCount: number;
  traceabilityProblemsCount: number;
  deploymentReadiness: number;
  welcomeTab: ReactNode;
  statusWorkbenchTab: ReactNode;
  evidenceTab: ReactNode;
};

export function AppShell({
  activeTab,
  onTabChange,
  jobCount,
  traceabilityProblemsCount,
  deploymentReadiness,
  welcomeTab,
  statusWorkbenchTab,
  evidenceTab
}: AppShellProps) {
  return (
    <main className="app-shell">
      <section className="top-bar" aria-label="Program summary">
        <div>
          <p className="eyebrow">Public-safe engineering demo</p>
          <h1>Kaleidos Compute Readiness Console</h1>
          <p className="deck">
            Source-linked product facts, Status Workbench simulation state, HPC orchestration, and controlled
            evidence records in one compact screen-share.
          </p>
        </div>
        <div className="summary-strip" aria-label="Readiness summary">
          <Metric icon={ShieldCheck} label="Claim boundaries" value="5/5" tone="good" />
          <Metric icon={Cpu} label="Synthetic jobs" value={`${jobCount}`} tone="info" />
          <Metric
            icon={GitBranch}
            label="Trace links"
            value={traceabilityProblemsCount ? "hold" : "clean"}
            tone={traceabilityProblemsCount ? "warn" : "good"}
          />
          <Metric icon={ServerCog} label="Deploy score" value={`${deploymentReadiness}%`} tone="warn" />
        </div>
      </section>

      <nav className="tabs" aria-label="Console sections">
        {appTabItems.map((tab) => {
          const Icon = tab.icon;
          return (
            <button
              className={activeTab === tab.id ? "tab active" : "tab"}
              key={tab.id}
              onClick={() => onTabChange(tab.id)}
              type="button"
            >
              <Icon size={16} />
              {tab.label}
            </button>
          );
        })}
      </nav>

      {activeTab === "welcome" && <>{welcomeTab}</>}
      {activeTab === "status" && <>{statusWorkbenchTab}</>}
      {activeTab === "evidence" && <>{evidenceTab}</>}
    </main>
  );
}
