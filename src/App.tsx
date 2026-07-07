import { useState } from "react";
import { AppShell } from "./features/app-shell/AppShell";
import {
  BriefTab,
  EvidenceTab,
  WorkbenchTab,
  useReadinessConsoleState
} from "./features/readiness-console/readinessConsole";
import { SimOpsControlPanel, useSimOpsController } from "./features/simops/simopsFeature";
import { SimulatorWorkbenchTab, useSimulatorWorkbenchFeature } from "./features/simulator-workbench/simulatorWorkbenchFeature";

export default function App() {
  const [activeTab, setActiveTab] = useState<"brief" | "workbench" | "simulator-workbench" | "evidence" | "simops">(
    "brief"
  );

  const readiness = useReadinessConsoleState({
    onRunBundleStarted: () => {
      setActiveTab("workbench");
    }
  });

  const workbench = useSimulatorWorkbenchFeature();
  const simops = useSimOpsController();

  return (
    <AppShell
      activeTab={activeTab}
      onTabChange={setActiveTab}
      jobCount={readiness.displayJobs.length}
      traceabilityProblemsCount={readiness.traceabilityProblems.length}
      deploymentReadiness={readiness.deploymentReadiness}
      briefTab={
        <BriefTab
          publicFacts={readiness.publicFacts}
          transportPeak={readiness.transportPeak}
          thermalMargin={readiness.thermalMargin}
          fleetFlags={readiness.fleetFlags}
          onRunBundle={readiness.runBundle}
        />
      }
      workbenchTab={
        <WorkbenchTab
          bundleState={readiness.bundleState}
          jobs={readiness.displayJobs}
          scenario={readiness.selectedScenario}
          scenarioJobs={readiness.scenarioJobs}
          selectedJob={readiness.selectedJob}
          setScenario={readiness.setSelectedScenario}
          setSelectedJobId={readiness.setSelectedJobId}
          onRunBundle={readiness.runBundle}
        />
      }
      simulatorWorkbenchTab={
        <SimulatorWorkbenchTab
          projection={workbench.projection}
          healthPanelModel={workbench.healthPanelModel}
          onSelectUnit={workbench.selectUnit}
          onSelectValue={workbench.selectValue}
        />
      }
      evidenceTab={
        <EvidenceTab
          requirements={readiness.requirements}
          evidencePacks={readiness.evidencePacks}
          controlledEvidence={readiness.controlledEvidence}
          deploymentChecks={readiness.deploymentChecks}
          coverage={readiness.coverage}
          traceabilityProblems={readiness.traceabilityProblems}
          deploymentReadiness={readiness.deploymentReadiness}
        />
      }
      simopsTab={
        <SimOpsControlPanel
          message={simops.message}
          error={simops.error}
          scenario={simops.scenario}
          source={simops.source}
          launchMode={simops.launchMode}
          runtimeLimit={simops.runtimeLimit}
          workerKinds={simops.workerKinds}
          idempotencyKey={simops.idempotencyKey}
          activeRun={simops.activeRun}
          events={simops.events}
          submitting={simops.submitting}
          onScenarioChange={simops.setScenario}
          onSourceChange={simops.setSource}
          onLaunchModeChange={simops.setLaunchMode}
          onRuntimeLimitChange={simops.setRuntimeLimit}
          onIdempotencyChange={simops.setIdempotencyKey}
          onWorkerToggle={simops.toggleWorker}
          onCreate={simops.createRun}
          onStop={simops.stopRun}
          onRefresh={simops.refresh}
        />
      }
    />
  );
}
