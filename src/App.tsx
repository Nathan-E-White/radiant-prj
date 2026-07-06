import {
  Activity,
  AlertTriangle,
  Boxes,
  CheckCircle2,
  ClipboardCheck,
  Cpu,
  ExternalLink,
  FileText,
  GitBranch,
  HardDrive,
  Network,
  RefreshCcw,
  Play,
  ServerCog,
  ShieldCheck,
  Square,
  TerminalSquare
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import {
  createSimopsRun as createSimopsRunRequest,
  getSimopsRun,
  listSimopsEvents,
  stopSimopsRun as stopSimopsRunRequest
} from "./api/simops";
import type {
  SimopsEvent,
  SimopsLifecycle,
  SimopsRunRequest,
  SimopsRunResponse,
  SimopsWorkerKind
} from "./api/simops";
import { SimulatorWorkbenchSurface } from "./components/simulator-workbench";
import {
  deploymentScore,
  diagnoseJob,
  fixtures,
  requirementCoverage,
  runFleetTelemetryToy,
  runPassiveThermalToy,
  runTransportToy,
  validateTraceability
} from "./domain/readiness";
import type {
  ControlledEvidenceRecord,
  ComputeJob,
  DeploymentCheck,
  EvidencePack,
  PublicFact,
  Requirement,
  SchedulerState
} from "./domain/types";
import { buildWorkbenchProjection, loadFixtureWorkbenchData } from "./domain/simulator-workbench";

type StatusPillState = SchedulerState | BundleState | SimopsLifecycle | "complete";
type TabId = "brief" | "workbench" | "simulator-workbench" | "evidence" | "simops";
type BundleState = "ready" | "running" | "complete";

const tabItems: Array<{ id: TabId; label: string; icon: LucideIcon }> = [
  { id: "brief", label: "Kaleidos Brief", icon: Boxes },
  { id: "workbench", label: "Compute Workbench", icon: TerminalSquare },
  { id: "simulator-workbench", label: "Simulator Workbench", icon: Activity },
  { id: "evidence", label: "Evidence Matrix", icon: ClipboardCheck },
  { id: "simops", label: "SimOps Control", icon: ServerCog }
];

const scenarioOptions = [
  "DOME synthetic full-power readiness bundle",
  "Buckley synthetic remote-site readiness"
];
const simopsScenarios = ["nominal", "scheduler-drift", "checkpoint-pressure", "cloud-burst", "fabric-warning"];
const simopsWorkerOptions: SimopsWorkerKind[] = ["scheduler", "storage", "burst", "fabric"];
const simopsLaunchModes: Array<SimopsRunRequest["launch_mode"]> = ["resident", "spawn", "auto"];

export default function App() {
  const [activeTab, setActiveTab] = useState<TabId>("brief");
  const [scenario, setScenario] = useState(scenarioOptions[0]);
  const [bundleState, setBundleState] = useState<BundleState>("ready");
  const [selectedJobId, setSelectedJobId] = useState("JOB-HPC-404");
  const [simopsScenario, setSimopsScenario] = useState(simopsScenarios[0]);
  const [simopsSource, setSimopsSource] = useState("frontend");
  const [simopsWorkerKinds, setSimopsWorkerKinds] = useState<SimopsWorkerKind[]>(["scheduler", "storage"]);
  const [simopsLaunchMode, setSimopsLaunchMode] = useState<SimopsRunRequest["launch_mode"]>("auto");
  const [simopsRuntimeLimit, setSimopsRuntimeLimit] = useState(120);
  const [simopsIdempotency, setSimopsIdempotency] = useState("");
  const [simopsRun, setSimopsRun] = useState<SimopsRunResponse | null>(null);
  const [simopsEvents, setSimopsEvents] = useState<SimopsEvent[]>([]);
  const [simopsMessage, setSimopsMessage] = useState("No run selected.");
  const [simopsError, setSimopsError] = useState("");
  const [isSubmittingSimops, setIsSubmittingSimops] = useState(false);
  const [selectedWorkbenchValueId, setSelectedWorkbenchValueId] = useState<string | undefined>();

  useEffect(() => {
    if (bundleState !== "running") {
      return;
    }

    const timer = window.setTimeout(() => setBundleState("complete"), 720);
    return () => window.clearTimeout(timer);
  }, [bundleState]);

  const displayJobs = useMemo(
    () =>
      fixtures.computeJobs.map((job) => ({
        ...job,
        state: displayedState(job, bundleState)
      })),
    [bundleState]
  );
  const scenarioJobs = displayJobs.filter((job) => job.scenario === scenario);
  const selectedJob =
    displayJobs.find((job) => job.id === selectedJobId) ?? displayJobs[displayJobs.length - 1];
  const traceabilityProblems = validateTraceability();
  const coverage = requirementCoverage();
  const deploymentReadiness = deploymentScore();
  const transportResult = runTransportToy({
    cells: 12,
    sourceStrength: 1,
    absorption: 0.16,
    scatter: 0.62
  });
  const thermalResult = runPassiveThermalToy({
    heatKw: 950,
    ambientC: 42,
    thermalResistanceCPerKw: 0.18,
    limitC: 260
  });
  const fleetResult = runFleetTelemetryToy({
    values: [100, 101, 99, 100, 102, 98, 120, 101, 100],
    zLimit: 2.2,
    packetLossPct: 0.7,
    missingPacketLimitPct: 1.5
  });
  const simulatorWorkbenchData = useMemo(() => loadFixtureWorkbenchData(), []);
  const simulatorWorkbenchProjection = useMemo(
    () => buildWorkbenchProjection(simulatorWorkbenchData, selectedWorkbenchValueId),
    [selectedWorkbenchValueId, simulatorWorkbenchData]
  );
  useEffect(() => {
    if (!simopsRun || !isSimopsRunActive(simopsRun.lifecycle)) {
      return;
    }
    const poll = window.setInterval(() => {
      void refreshSimopsRun(simopsRun.run_id);
    }, 2500);
    return () => {
      window.clearInterval(poll);
    };
  }, [simopsRun]);

  function runBundle() {
    setBundleState("running");
    setActiveTab("workbench");
    setSelectedJobId("JOB-HPC-404");
  }

  async function refreshSimopsRun(runID: string) {
    if (!runID) {
      return;
    }
    try {
      const next = await getSimopsRun(runID);
      setSimopsRun(next);
      await refreshSimopsEvents(runID);
      setSimopsError("");
    } catch (error) {
      setSimopsError(`simops refresh failed: ${(error as Error).message}`);
    }
  }

  async function refreshSimopsEvents(runID: string) {
    const response = await listSimopsEvents(runID);
    setSimopsEvents(response.events);
  }

  async function createSimopsRun() {
    setIsSubmittingSimops(true);
    setSimopsError("");
    setSimopsMessage("Launching simops run...");

    if (simopsWorkerKinds.length === 0) {
      setSimopsError("Choose at least one worker kind.");
      setIsSubmittingSimops(false);
      return;
    }

    const payload: SimopsRunRequest = {
      scenario_id: simopsScenario,
      source: simopsSource,
      work_script: simopsScenario,
      launch_mode: simopsLaunchMode,
      worker_kinds: simopsWorkerKinds,
      runtime_limit_sec: simopsRuntimeLimit
    };
    const idempotencyKey = simopsIdempotency.trim();
    if (idempotencyKey) {
      payload.idempotency_key = idempotencyKey;
    }

    try {
      const run = await createSimopsRunRequest(payload);
      setSimopsRun(run);
      setSimopsMessage("Run launched. Polling for worker status...");
      await refreshSimopsEvents(run.run_id);
      void refreshSimopsRun(run.run_id);
    } catch (error) {
      setSimopsError(`simops create failed: ${(error as Error).message}`);
    } finally {
      setIsSubmittingSimops(false);
    }
  }

  async function stopSimopsRun() {
    if (!simopsRun?.run_id) {
      return;
    }
    setIsSubmittingSimops(true);
    setSimopsError("");
    setSimopsMessage("Issuing stop command...");
    try {
      const run = await stopSimopsRunRequest(simopsRun.run_id);
      setSimopsRun(run);
      await refreshSimopsEvents(run.run_id);
      setSimopsMessage("Stop requested for run.");
    } catch (error) {
      setSimopsError(`simops stop failed: ${(error as Error).message}`);
    } finally {
      setIsSubmittingSimops(false);
    }
  }

  function toggleSimopsWorker(kind: SimopsWorkerKind) {
    setSimopsWorkerKinds((previous) => {
      const existing = previous.includes(kind);
      if (existing) {
        return previous.filter((worker) => worker !== kind);
      }
      return [...previous, kind];
    });
  }

  return (
    <main className="app-shell">
      <section className="top-bar" aria-label="Program summary">
        <div>
          <p className="eyebrow">Public-safe engineering demo</p>
          <h1>Kaleidos Compute Readiness Console</h1>
          <p className="deck">
            Source-linked product facts, synthetic transport jobs, HPC failure triage, and
            controlled evidence records in one compact screen-share.
          </p>
        </div>
        <div className="summary-strip" aria-label="Readiness summary">
          <Metric icon={ShieldCheck} label="Claim boundaries" value="5/5" tone="good" />
          <Metric icon={Cpu} label="Synthetic jobs" value={`${displayJobs.length}`} tone="info" />
          <Metric icon={GitBranch} label="Trace links" value={traceabilityProblems.length ? "hold" : "clean"} tone={traceabilityProblems.length ? "warn" : "good"} />
          <Metric icon={ServerCog} label="Deploy score" value={`${deploymentReadiness}%`} tone="warn" />
        </div>
      </section>

      <nav className="tabs" aria-label="Console sections">
        {tabItems.map((tab) => {
          const Icon = tab.icon;
          return (
            <button
              className={activeTab === tab.id ? "tab active" : "tab"}
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              type="button"
            >
              <Icon size={16} />
              {tab.label}
            </button>
          );
        })}
      </nav>

      {activeTab === "brief" && (
        <BriefTab
          onRunBundle={runBundle}
          publicFacts={fixtures.publicFacts}
          transportPeak={transportResult.peakScalarFlux}
          thermalMargin={thermalResult.marginC}
          fleetFlags={fleetResult.channelsFlagged}
        />
      )}

      {activeTab === "workbench" && (
        <WorkbenchTab
          bundleState={bundleState}
          jobs={displayJobs}
          scenario={scenario}
          scenarioJobs={scenarioJobs}
          selectedJob={selectedJob}
          setScenario={setScenario}
          setSelectedJobId={setSelectedJobId}
          onRunBundle={runBundle}
        />
      )}

      {activeTab === "simulator-workbench" && (
        <SimulatorWorkbenchSurface
          projection={simulatorWorkbenchProjection}
          onSelectValue={setSelectedWorkbenchValueId}
        />
      )}

      {activeTab === "evidence" && (
        <EvidenceTab
          requirements={fixtures.requirements}
          evidencePacks={fixtures.evidencePacks}
          controlledEvidence={fixtures.controlledEvidence}
          deploymentChecks={fixtures.deploymentChecks}
          coverage={coverage}
          traceabilityProblems={traceabilityProblems}
        />
      )}

      {activeTab === "simops" && (
        <SimOpsControlPanel
          message={simopsMessage}
          error={simopsError}
          scenario={simopsScenario}
          source={simopsSource}
          launchMode={simopsLaunchMode}
          runtimeLimit={simopsRuntimeLimit}
          workerKinds={simopsWorkerKinds}
          idempotencyKey={simopsIdempotency}
          activeRun={simopsRun}
          events={simopsEvents}
          submitting={isSubmittingSimops}
          onScenarioChange={setSimopsScenario}
          onSourceChange={setSimopsSource}
          onLaunchModeChange={setSimopsLaunchMode}
          onRuntimeLimitChange={setSimopsRuntimeLimit}
          onIdempotencyChange={setSimopsIdempotency}
          onWorkerToggle={toggleSimopsWorker}
          onCreate={createSimopsRun}
          onStop={stopSimopsRun}
          onRefresh={() => {
            if (simopsRun) {
              void refreshSimopsRun(simopsRun.run_id);
            } else {
              setSimopsError("No run selected to refresh.");
            }
          }}
        />
      )}
    </main>
  );
}

function SimOpsControlPanel({
  message,
  error,
  scenario,
  source,
  launchMode,
  runtimeLimit,
  workerKinds,
  idempotencyKey,
  activeRun,
  events,
  submitting,
  onScenarioChange,
  onSourceChange,
  onLaunchModeChange,
  onRuntimeLimitChange,
  onIdempotencyChange,
  onWorkerToggle,
  onCreate,
  onStop,
  onRefresh
}: {
  message: string;
  error: string;
  scenario: string;
  source: string;
  launchMode: SimopsRunRequest["launch_mode"];
  runtimeLimit: number;
  workerKinds: SimopsWorkerKind[];
  idempotencyKey: string;
  activeRun: SimopsRunResponse | null;
  events: SimopsEvent[];
  submitting: boolean;
  onScenarioChange: (scenario: string) => void;
  onSourceChange: (source: string) => void;
  onLaunchModeChange: (mode: SimopsRunRequest["launch_mode"]) => void;
  onRuntimeLimitChange: (seconds: number) => void;
  onIdempotencyChange: (next: string) => void;
  onWorkerToggle: (kind: SimopsWorkerKind) => void;
  onCreate: () => Promise<void>;
  onStop: () => Promise<void>;
  onRefresh: () => void;
}) {
  const isRunActive = activeRun !== null && isSimopsRunActive(activeRun.lifecycle);
  const isRunComplete = activeRun !== null && isSimopsRunTerminal(activeRun.lifecycle);

  return (
    <section className="content-grid simops-grid">
      <div className="panel simops-control">
        <div className="panel-heading">
          <div>
            <p className="eyebrow">SimOps control-plane</p>
            <h2>Containerized worker orchestration</h2>
          </div>
          <button className="icon-command" onClick={onRefresh} type="button" title="Refresh run">
            <RefreshCcw size={18} />
          </button>
        </div>
        <div className="simops-field">
          <label htmlFor="simops-scenario">Scenario</label>
          <select
            id="simops-scenario"
            value={scenario}
            onChange={(event) => onScenarioChange(event.target.value)}
          >
            {simopsScenarios.map((option) => (
              <option value={option} key={option}>
                {option}
              </option>
            ))}
          </select>
        </div>
        <div className="simops-field">
          <label htmlFor="simops-source">Source</label>
          <select id="simops-source" value={source} onChange={(event) => onSourceChange(event.target.value)}>
            <option value="frontend">frontend</option>
            <option value="work-script">work-script</option>
          </select>
        </div>
        <div className="simops-field">
          <label>Workers</label>
          <div className="simops-checkbox-group">
            {simopsWorkerOptions.map((worker) => (
              <label className="simops-checkbox" key={worker}>
                <input
                  checked={workerKinds.includes(worker)}
                  onChange={() => onWorkerToggle(worker)}
                  type="checkbox"
                />
                {worker}
              </label>
            ))}
          </div>
        </div>
        <div className="simops-field">
          <label>Launch mode</label>
          <div className="simops-radio-group">
            {simopsLaunchModes.map((mode) => (
              <label className="simops-radio" key={mode}>
                <input
                  checked={mode === launchMode}
                  onChange={() => onLaunchModeChange(mode)}
                  name="simops-launch-mode"
                  type="radio"
                />
                {mode}
              </label>
            ))}
          </div>
        </div>
        <div className="simops-field">
          <label htmlFor="simops-runtime-limit">Runtime limit (sec)</label>
          <input
            id="simops-runtime-limit"
            max={3600}
            min={1}
            onChange={(event) => onRuntimeLimitChange(Number(event.target.value) || 120)}
            type="number"
            value={runtimeLimit}
          />
        </div>
        <div className="simops-field">
          <label htmlFor="simops-idempotency">Idempotency key (optional)</label>
          <input
            id="simops-idempotency"
            onChange={(event) => onIdempotencyChange(event.target.value)}
            placeholder="click once, prevent accidental repeats"
            value={idempotencyKey}
          />
        </div>
        <div className="simops-actions">
          <button disabled={submitting || isRunActive} className="primary-command" onClick={onCreate} type="button">
            <Play size={16} />
            {isRunActive ? "Run in progress" : activeRun ? "Launch replacement" : "Launch run"}
          </button>
          <button
            className="secondary-command"
            disabled={!activeRun || isRunComplete || submitting}
            onClick={onStop}
            type="button"
          >
            <Square size={16} />
            Stop run
          </button>
        </div>
        <p className="simops-message">{message}</p>
        {error && <p className="simops-error">{error}</p>}
      </div>

      <div className="panel simops-details">
        <div className="panel-heading">
          <div>
            <p className="eyebrow">Run state</p>
            <h2>Lifecycle and worker status</h2>
          </div>
        </div>
        {!activeRun ? (
          <p className="simops-placeholder">No active SimOps run yet.</p>
        ) : (
          <>
            <div className="simops-run-header">
              <span>
                <strong>run:</strong> {activeRun.run_id}
              </span>
              <StatusPill label={activeRun.lifecycle} state={activeRun.lifecycle} />
            </div>
            <div className="simops-meta">
              <span>scenario: {activeRun.scenario_id}</span>
              <span>source: {activeRun.source}</span>
              <span>mode: {activeRun.launch_mode}</span>
              <span>created: {new Date(activeRun.created_at).toLocaleString()}</span>
              <span>updated: {new Date(activeRun.updated_at).toLocaleString()}</span>
            </div>
            <div className="simops-subsection">
              <h3>Workers</h3>
              <div className="simops-worker-grid">
                {activeRun.workers.map((worker) => (
                  <article className="simops-worker-card" key={`${activeRun.run_id}-${worker.worker_id}`}>
                    <div className="simops-worker-title">
                      <span className="record-id">{worker.worker_id}</span>
                      <StatusPill label={worker.lifecycle} state={worker.lifecycle} />
                    </div>
                    <p>
                      kind: {worker.worker_kind} • frames: {worker.frames} • mode: {worker.launch_mode}
                    </p>
                    <p>endpoint: {worker.endpoint || "n/a"}</p>
                    <p>updated: {new Date(worker.updated_at).toLocaleString()}</p>
                  </article>
                ))}
              </div>
            </div>
            <div className="simops-subsection">
              <h3>Latest spool commands</h3>
              <div className="simops-log">
                {activeRun.spool_commands.map((command) => (
                  <p key={command.command_id}>
                    {command.state} {command.worker_id}: {command.message}
                  </p>
                ))}
              </div>
            </div>
            <div className="simops-subsection">
              <h3>Artifacts</h3>
              <div className="simops-log">
                {activeRun.artifacts.map((artifact) => (
                  <p key={artifact.artifact_id}>
                    {artifact.status} {artifact.kind} @ {artifact.location}
                  </p>
                ))}
              </div>
            </div>
            <div className="simops-subsection">
              <h3>Events</h3>
              <div className="simops-log">
                {events.length === 0 ? (
                  <p>No persisted events yet.</p>
                ) : (
                  events.slice(-8).map((event, index) => (
                    <p key={`${event.run_id}-${event.event_type}-${event.occurred_at}-${index}`}>
                      {event.event_type}
                      {event.worker_id ? ` ${event.worker_id}` : ""}
                      {event.lifecycle ? ` ${event.lifecycle}` : ""} @ {new Date(event.occurred_at).toLocaleTimeString()}
                    </p>
                  ))
                )}
              </div>
            </div>
            <div className="simops-subsection">
              <h3>MoQ subscription</h3>
              <div className="simops-log">
                <p>protocol: {activeRun.moq_subscription.protocol}</p>
                <p>endpoint: {activeRun.moq_subscription.endpoint}</p>
                <p>namespace: {activeRun.moq_subscription.namespace}</p>
                <p>token: {activeRun.moq_subscription.token}</p>
              </div>
            </div>
          </>
        )}
      </div>
    </section>
  );
}

function BriefTab({
  publicFacts,
  transportPeak,
  thermalMargin,
  fleetFlags,
  onRunBundle
}: {
  publicFacts: PublicFact[];
  transportPeak: number;
  thermalMargin: number;
  fleetFlags: number;
  onRunBundle: () => void;
}) {
  return (
    <section className="content-grid brief-grid">
      <div className="panel cutaway-panel">
        <div className="panel-heading">
          <div>
            <p className="eyebrow">Kaleidos public architecture</p>
            <h2>Transportable HTGR briefing surface</h2>
          </div>
          <button className="icon-command" onClick={onRunBundle} type="button" title="Run synthetic readiness bundle">
            <Play size={18} />
          </button>
        </div>
        <KaleidosCutaway />
      </div>

      <div className="panel facts-panel">
        <div className="panel-heading">
          <div>
            <p className="eyebrow">Controlled facts</p>
            <h2>Public-source claim set</h2>
          </div>
          <StatusPill label="public-safe" state="completed" />
        </div>
        <div className="fact-list">
          {publicFacts.map((fact) => (
            <article className="fact-item" key={fact.id}>
              <div>
                <span className="record-id">{fact.id}</span>
                <h3>{fact.topic}</h3>
              </div>
              <p>{fact.claim}</p>
              <div className="fact-footer">
                <span>{fact.boundary}</span>
                <a href={fact.sourceUrl} target="_blank" rel="noreferrer">
                  {fact.sourceTitle}
                  <ExternalLink size={13} />
                </a>
              </div>
            </article>
          ))}
        </div>
      </div>

      <div className="panel timeline-panel">
        <div className="panel-heading">
          <div>
            <p className="eyebrow">Readiness path</p>
            <h2>Milestones mirrored in the demo</h2>
          </div>
        </div>
        <div className="timeline">
          {fixtures.milestones.map((milestone) => (
            <article className="timeline-item" key={milestone.id}>
              <span className="timeline-dot" />
              <div>
                <span className="record-id">{milestone.id}</span>
                <h3>{milestone.title}</h3>
                <p>{milestone.note}</p>
              </div>
              <span className="phase-chip">{milestone.phase}</span>
            </article>
          ))}
        </div>
      </div>

      <div className="panel mini-results">
        <div className="panel-heading">
          <div>
            <p className="eyebrow">Synthetic outputs</p>
            <h2>Demo calculation thread</h2>
          </div>
        </div>
        <Metric icon={Activity} label="Transport peak flux proxy" value={transportPeak.toFixed(2)} tone="info" />
        <Metric icon={CheckCircle2} label="Thermal toy margin" value={`${thermalMargin} C`} tone="good" />
        <Metric icon={AlertTriangle} label="Fleet channels flagged" value={`${fleetFlags}`} tone="warn" />
      </div>
    </section>
  );
}

function WorkbenchTab({
  bundleState,
  jobs,
  scenario,
  scenarioJobs,
  selectedJob,
  setScenario,
  setSelectedJobId,
  onRunBundle
}: {
  bundleState: BundleState;
  jobs: ComputeJob[];
  scenario: string;
  scenarioJobs: ComputeJob[];
  selectedJob: ComputeJob;
  setScenario: (scenario: string) => void;
  setSelectedJobId: (jobId: string) => void;
  onRunBundle: () => void;
}) {
  const diagnosis = diagnoseJob(selectedJob);

  return (
    <section className="content-grid workbench-grid">
      <div className="panel queue-panel">
        <div className="panel-heading">
          <div>
            <p className="eyebrow">Scheduler emulator</p>
            <h2>Scientific compute queue</h2>
          </div>
          <button className="primary-command" onClick={onRunBundle} type="button">
            <Play size={16} />
            Run Bundle
          </button>
        </div>

        <div className="scenario-row" aria-label="Scenario selector">
          {scenarioOptions.map((option) => (
            <button
              className={option === scenario ? "scenario active" : "scenario"}
              key={option}
              onClick={() => setScenario(option)}
              type="button"
            >
              {option.replace(" synthetic ", " ")}
            </button>
          ))}
        </div>

        <div className="queue-table" role="table" aria-label="Compute jobs">
          <div className="queue-row queue-head" role="row">
            <span>Job</span>
            <span>Discipline</span>
            <span>Resources</span>
            <span>State</span>
          </div>
          {jobs.map((job) => (
            <button
              className={selectedJob.id === job.id ? "queue-row selected" : "queue-row"}
              key={job.id}
              onClick={() => setSelectedJobId(job.id)}
              type="button"
              role="row"
            >
              <span>
                <strong>{job.id}</strong>
                <small>{job.title}</small>
              </span>
              <span>{job.discipline}</span>
              <span>
                {job.resources.nodes}n / {job.resources.ranks}r / {job.resources.storageGb}GB
              </span>
              <StatusPill label={job.state} state={job.state} />
            </button>
          ))}
        </div>
      </div>

      <div className="panel job-panel">
        <div className="panel-heading">
          <div>
            <p className="eyebrow">{selectedJob.stakeholder}</p>
            <h2>{selectedJob.title}</h2>
          </div>
          <StatusPill label={selectedJob.state} state={selectedJob.state} />
        </div>
        <div className="job-metrics">
          <Metric icon={Cpu} label="Ranks" value={`${selectedJob.resources.ranks}`} tone="info" />
          <Metric icon={HardDrive} label="Storage" value={`${selectedJob.resources.storageGb} GB`} tone="warn" />
          <Metric icon={Network} label="Walltime" value={`${selectedJob.resources.walltimeMin} min`} tone="good" />
        </div>
        <div className="module-strip">
          {selectedJob.resources.modules.map((moduleName) => (
            <span key={moduleName}>{moduleName}</span>
          ))}
        </div>
        <LogBlock logs={selectedJob.logs} />
      </div>

      <div className="panel diagnosis-panel">
        <div className="panel-heading">
          <div>
            <p className="eyebrow">Failure analysis</p>
            <h2>Root cause and control</h2>
          </div>
        </div>
        <Finding label="Root cause" value={diagnosis.rootCause} />
        <Finding label="Next action" value={diagnosis.nextAction} />
        <Finding label="Preventative control" value={diagnosis.preventativeControl} />
      </div>

      <div className="panel scenario-panel">
        <div className="panel-heading">
          <div>
            <p className="eyebrow">Bundle state</p>
            <h2>{scenario}</h2>
          </div>
          <StatusPill label={bundleState} state={bundleState === "complete" ? "completed" : bundleState === "running" ? "running" : "queued"} />
        </div>
        <div className="scenario-stack">
          {scenarioJobs.map((job) => (
            <div className="scenario-card" key={job.id}>
              <span className={`status-led ${job.state}`} />
              <div>
                <strong>{job.id}</strong>
                <p>{job.title}</p>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

function EvidenceTab({
  requirements,
  evidencePacks,
  controlledEvidence,
  deploymentChecks,
  coverage,
  traceabilityProblems
}: {
  requirements: Requirement[];
  evidencePacks: EvidencePack[];
  controlledEvidence: ControlledEvidenceRecord[];
  deploymentChecks: DeploymentCheck[];
  coverage: ReturnType<typeof requirementCoverage>;
  traceabilityProblems: string[];
}) {
  return (
    <section className="content-grid evidence-grid">
      <div className="panel requirements-panel">
        <div className="panel-heading">
          <div>
            <p className="eyebrow">Controlled requirements</p>
            <h2>Traceability matrix</h2>
          </div>
          <StatusPill label={traceabilityProblems.length ? "hold" : "clean"} state={traceabilityProblems.length ? "held" : "completed"} />
        </div>
        <div className="requirements-table" role="table" aria-label="Requirements matrix">
          <div className="requirements-row requirements-head" role="row">
            <span>ID</span>
            <span>Requirement</span>
            <span>Verification</span>
            <span>Links</span>
          </div>
          {requirements.map((requirement) => {
            const coverageRecord = coverage.find((record) => record.id === requirement.id);
            return (
              <div className="requirements-row" key={requirement.id} role="row">
                <span className="record-id">{requirement.id}</span>
                <span>{requirement.text}</span>
                <span>{requirement.verificationMethod}</span>
                <span>
                  {coverageRecord?.linkedJobs ?? 0} jobs / {coverageRecord?.linkedEvidence ?? 0} packs
                </span>
              </div>
            );
          })}
        </div>
      </div>

      <div className="panel evidence-pack-panel">
        <div className="panel-heading">
          <div>
            <p className="eyebrow">Objective evidence</p>
            <h2>Artifact and control index</h2>
          </div>
          <Metric icon={FileText} label="Controlled" value={`${controlledEvidence.length}`} tone="info" />
        </div>
        <div className="evidence-list">
          {evidencePacks.map((pack) => (
            <article className="evidence-card" key={pack.id}>
              <div>
                <span className="record-id">{pack.id}</span>
                <h3>{pack.title}</h3>
              </div>
              <p>{pack.summary}</p>
              <small>{pack.limitations}</small>
              <div className="hash-grid">
                {Object.entries(pack.artifactHashes).map(([artifact, hash]) => (
                  <span key={artifact}>
                    {artifact}
                    <strong>{hash}</strong>
                  </span>
                ))}
              </div>
            </article>
          ))}
          {controlledEvidence.map((record) => (
            <article className="evidence-card controlled-evidence-card" key={record.id}>
              <div>
                <span className="record-id">{record.id}</span>
                <h3>{record.title}</h3>
              </div>
              <p>{record.summary}</p>
              <small>{record.limitations}</small>
              <div className="hash-grid">
                {record.artifacts.map((artifact) => (
                  <span key={artifact}>
                    {artifact}
                    <strong>{record.category}</strong>
                  </span>
                ))}
              </div>
            </article>
          ))}
        </div>
      </div>

      <div className="panel deployment-panel">
        <div className="panel-heading">
          <div>
            <p className="eyebrow">Dry-run deployment controls</p>
            <h2>Linux/HPC baseline checks</h2>
          </div>
          <Metric icon={ServerCog} label="Readiness" value={`${deploymentScore()}%`} tone="warn" />
        </div>
        <div className="deployment-grid">
          {deploymentChecks.map((check) => (
            <DeploymentCard check={check} key={check.id} />
          ))}
        </div>
      </div>
    </section>
  );
}

function KaleidosCutaway() {
  return (
    <div className="cutaway-wrap">
      <svg className="cutaway" viewBox="0 0 920 520" role="img" aria-labelledby="cutaway-title">
        <title id="cutaway-title">Stylized Kaleidos public-safe cutaway</title>
        <defs>
          <linearGradient id="containerGradient" x1="0" x2="1" y1="0" y2="1">
            <stop offset="0%" stopColor="#e7ecea" />
            <stop offset="100%" stopColor="#9aa6a2" />
          </linearGradient>
          <linearGradient id="coreGradient" x1="0" x2="1" y1="0" y2="1">
            <stop offset="0%" stopColor="#f8d66d" />
            <stop offset="100%" stopColor="#cf4b36" />
          </linearGradient>
        </defs>
        <rect x="70" y="120" width="780" height="265" rx="12" fill="url(#containerGradient)" stroke="#283633" strokeWidth="3" />
        <rect x="98" y="147" width="724" height="210" rx="8" fill="#f6f3ea" stroke="#60736d" strokeWidth="1.5" />
        <rect x="130" y="175" width="146" height="154" rx="10" fill="#26332f" />
        <rect x="158" y="196" width="90" height="112" rx="45" fill="#d8dedb" stroke="#70847e" strokeWidth="6" />
        <circle cx="203" cy="252" r="34" fill="url(#coreGradient)" stroke="#7c2b22" strokeWidth="4" />
        <path d="M185 226 L221 226 L236 252 L221 278 L185 278 L170 252 Z" fill="#3b4f49" opacity="0.42" />
        <rect x="310" y="181" width="118" height="62" rx="10" fill="#a5c7c9" stroke="#31565d" strokeWidth="3" />
        <rect x="310" y="263" width="118" height="62" rx="10" fill="#a5c7c9" stroke="#31565d" strokeWidth="3" />
        <path d="M248 238 C274 206 287 202 310 212" fill="none" stroke="#cf4b36" strokeWidth="9" strokeLinecap="round" />
        <path d="M248 272 C274 304 287 306 310 294" fill="none" stroke="#2f7c8c" strokeWidth="9" strokeLinecap="round" />
        <rect x="470" y="174" width="112" height="158" rx="14" fill="#dde7e4" stroke="#536b65" strokeWidth="3" />
        <circle cx="526" cy="220" r="27" fill="#2f7c8c" />
        <circle cx="526" cy="286" r="27" fill="#2f7c8c" />
        <path d="M512 220 h28 M526 206 v28 M512 286 h28 M526 272 v28" stroke="#f9fbf7" strokeWidth="5" strokeLinecap="round" />
        <rect x="626" y="183" width="146" height="134" rx="12" fill="#2d3942" stroke="#151c22" strokeWidth="3" />
        <path d="M650 282 C684 224 710 224 748 282" fill="none" stroke="#f8d66d" strokeWidth="10" strokeLinecap="round" />
        <rect x="792" y="170" width="26" height="174" rx="6" fill="#677b74" />
        <path d="M120 397 h690" stroke="#42514d" strokeWidth="5" strokeLinecap="round" strokeDasharray="12 16" />
        <CutawayLabel x={203} y={82} text="TRISO fuel / prismatic graphite core" targetX={203} targetY={252} />
        <CutawayLabel x={370} y={83} text="Primary heat exchangers" targetX={370} targetY={211} />
        <CutawayLabel x={526} y={436} text="Helium circulators" targetX={526} targetY={286} />
        <CutawayLabel x={700} y={84} text="Power conversion / cooling train" targetX={700} targetY={238} />
        <CutawayLabel x={460} y={434} text="Containerized transport envelope" targetX={460} targetY={385} />
      </svg>
    </div>
  );
}

function CutawayLabel({
  x,
  y,
  text,
  targetX,
  targetY
}: {
  x: number;
  y: number;
  text: string;
  targetX: number;
  targetY: number;
}) {
  return (
    <g>
      <path d={`M${x} ${y + 16} L${targetX} ${targetY}`} stroke="#4d625c" strokeWidth="1.5" strokeDasharray="4 5" />
      <rect x={x - 92} y={y - 8} width="184" height="42" rx="7" fill="#ffffff" stroke="#c5d0ca" />
      <text x={x} y={y + 17} textAnchor="middle" fontSize="13" fill="#26332f" fontWeight="700">
        {text}
      </text>
    </g>
  );
}

function Metric({
  icon: Icon,
  label,
  value,
  tone
}: {
  icon: LucideIcon;
  label: string;
  value: string;
  tone: "good" | "warn" | "info";
}) {
  return (
    <div className={`metric ${tone}`}>
      <Icon size={18} />
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function StatusPill({ label, state }: { label: string; state: StatusPillState }) {
  return <span className={`status-pill ${state}`}>{label}</span>;
}

function Finding({ label, value }: { label: string; value: string }) {
  return (
    <article className="finding">
      <span>{label}</span>
      <p>{value}</p>
    </article>
  );
}

function LogBlock({ logs }: { logs: string[] }) {
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

function DeploymentCard({ check }: { check: DeploymentCheck }) {
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

function Check({ label, value }: { label: string; value: "pass" | "warn" | "fail" }) {
  return (
    <span className={`check ${value}`}>
      {label}
      <strong>{value}</strong>
    </span>
  );
}

function displayedState(job: ComputeJob, bundleState: BundleState): SchedulerState {
  if (bundleState === "ready") {
    return job.id === "JOB-HPC-404" ? "held" : "queued";
  }

  if (bundleState === "running") {
    return job.id === "JOB-HPC-404" ? "running" : "completed";
  }

  return job.state;
}

function isSimopsRunActive(lifecycle: SimopsLifecycle): boolean {
  return lifecycle === "created" || lifecycle === "starting" || lifecycle === "streaming" || lifecycle === "degraded";
}

function isSimopsRunTerminal(lifecycle: SimopsLifecycle): boolean {
  return lifecycle === "complete" || lifecycle === "failed" || lifecycle === "stopped";
}
