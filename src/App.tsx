import { useState } from "react";
import { AppShell, type AppTabId } from "./features/app-shell/AppShell";
import {
  BriefTab,
  ComputeQueuePanel,
  EvidenceTab,
  useReadinessConsoleState
} from "./features/readiness-console/readinessConsole";
import { SimOpsControlPanel, useSimOpsController } from "./features/simops/simopsFeature";
import { StatusWorkbenchTab, useSimulatorWorkbenchFeature } from "./features/simulator-workbench/simulatorWorkbenchFeature";

export default function App() {
  const [activeTab, setActiveTab] = useState<AppTabId>("welcome");

  const readiness = useReadinessConsoleState({
    onRunBundleStarted: () => {
      setActiveTab("status");
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
      welcomeTab={
        <BriefTab
          publicFacts={readiness.publicFacts}
          transportPeak={readiness.transportPeak}
          thermalMargin={readiness.thermalMargin}
          fleetFlags={readiness.fleetFlags}
          onRunBundle={readiness.runBundle}
        />
      }
      statusWorkbenchTab={
        <StatusWorkbenchTab
          projection={workbench.projection}
          onSelectUnit={workbench.selectUnit}
          onSelectValue={workbench.selectValue}
          computeQueue={
            <ComputeQueuePanel
              jobs={readiness.displayJobs}
              scenario={readiness.selectedScenario}
              selectedJob={readiness.selectedJob}
              setScenario={readiness.setSelectedScenario}
              setSelectedJobId={readiness.setSelectedJobId}
              onRunBundle={readiness.runBundle}
            />
          }
          selectedJob={readiness.selectedJob}
          scenario={readiness.selectedScenario}
          scenarioJobs={readiness.scenarioJobs}
          bundleState={readiness.bundleState}
          orchestrationPanel={
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
    />
  );
}
