import {
  Activity,
  Boxes,
  ClipboardCheck,
  Cpu,
  GitBranch,
  ServerCog,
  ShieldCheck,
  TerminalSquare
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";
import { Metric, StatusPill } from "../shared/presentation";

export type AppTabId = "brief" | "workbench" | "simulator-workbench" | "evidence" | "simops";

type Tab = { id: AppTabId; label: string; icon: LucideIcon };

export const appTabItems: Array<Tab> = [
  { id: "brief", label: "Kaleidos Brief", icon: Boxes },
  { id: "workbench", label: "Compute Workbench", icon: TerminalSquare },
  { id: "simulator-workbench", label: "Simulator Workbench", icon: Activity },
  { id: "evidence", label: "Evidence Matrix", icon: ClipboardCheck },
  { id: "simops", label: "SimOps Control", icon: ServerCog }
];

type AppShellProps = {
  activeTab: AppTabId;
  onTabChange: (tabId: AppTabId) => void;
  jobCount: number;
  traceabilityProblemsCount: number;
  deploymentReadiness: number;
  briefTab: ReactNode;
  workbenchTab: ReactNode;
  simulatorWorkbenchTab: ReactNode;
  evidenceTab: ReactNode;
  simopsTab: ReactNode;
};

export function AppShell({
  activeTab,
  onTabChange,
  jobCount,
  traceabilityProblemsCount,
  deploymentReadiness,
  briefTab,
  workbenchTab,
  simulatorWorkbenchTab,
  evidenceTab,
  simopsTab
}: AppShellProps) {
  return (
    <main className="app-shell">
      <section className="top-bar" aria-label="Program summary">
        <div>
          <p className="eyebrow">Public-safe engineering demo</p>
          <h1>Kaleidos Compute Readiness Console</h1>
          <p className="deck">
            Source-linked product facts, synthetic transport jobs, HPC failure triage, and controlled evidence
            records in one compact screen-share.
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

      {activeTab === "brief" && <>{briefTab}</>}
      {activeTab === "workbench" && <>{workbenchTab}</>}
      {activeTab === "simulator-workbench" && <>{simulatorWorkbenchTab}</>}
      {activeTab === "evidence" && <>{evidenceTab}</>}
      {activeTab === "simops" && <>{simopsTab}</>}
    </main>
  );
}
