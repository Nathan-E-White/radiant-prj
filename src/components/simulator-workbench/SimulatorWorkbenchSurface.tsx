import { FleetStrip } from "./FleetStrip";
import { LineagePanel } from "./LineagePanel";
import { MeasuredStatePanel } from "./MeasuredStatePanel";
import { SimulationResultsPanel } from "./SimulationResultsPanel";
import { TwinStatePanel } from "./TwinStatePanel";
import { TwinViewport } from "./TwinViewport";
import type { CSSProperties, ReactNode } from "react";
import { diagnoseJob } from "../../domain/readiness";
import type { ComputeJob } from "../../domain/types";
import type { WorkbenchProjection } from "../../domain/simulator-workbench";

export function SimulatorWorkbenchSurface({
  projection,
  onSelectUnit,
  onSelectValue,
  computeQueue,
  selectedJob,
  scenario,
  scenarioJobs,
  bundleState,
  orchestrationPanel
}: {
  projection: WorkbenchProjection;
  onSelectUnit: (unitId: string, commercialBasisId: string) => void;
  onSelectValue: (valueId: string) => void;
  computeQueue: ReactNode;
  selectedJob: ComputeJob;
  scenario: string;
  scenarioJobs: ComputeJob[];
  bundleState: string;
  orchestrationPanel: ReactNode;
}) {
  const selectedValueId = projection.selectedValue?.valueId ?? "";
  const selectedUnitValues = [
    ...projection.groups.measured.values,
    ...projection.groups.imputed.values,
    ...projection.groups.simulated.values
  ];
  const diagnosis = diagnoseJob(selectedJob);
  const runContext = `${projection.scenarioId} / ${projection.healthSummary.label}`;

  return (
    <section className="simwb-shell" aria-label="Status Workbench">
      <div className="simwb-head">
        <div>
          <p className="eyebrow">Status Workbench</p>
          <h2>{projection.selectedUnit.displayName}</h2>
        </div>
        <div className="simwb-context">
          <span>{projection.selectedUnit.phaseLine}</span>
          <span>Twin: {projection.twinId}</span>
          <span>Generated: {formatTime(projection.generatedAt)}</span>
        </div>
      </div>

      <FleetStrip units={projection.fleetUnits} onSelectUnit={onSelectUnit} />

      <div className="simwb-grid">
        <aside className="simwb-stack">
          <MeasuredStatePanel
            group={projection.groups.measured}
            selectedValueId={selectedValueId}
            onSelectValue={onSelectValue}
          />
        </aside>

        <TwinViewport
          model={projection.viewport}
          selectedValue={projection.selectedValue}
          values={selectedUnitValues}
          onSelectValue={onSelectValue}
        />

        <aside className="simwb-stack">
          <TwinStatePanel
            group={projection.groups.imputed}
            selectedValueId={selectedValueId}
            onSelectValue={onSelectValue}
          />
          <SimulationResultsPanel
            group={projection.groups.simulated}
            healthSummary={projection.healthSummary}
            selectedValueId={selectedValueId}
            onSelectValue={onSelectValue}
          />
        </aside>
      </div>

      <LineagePanel explanation={projection.explanation} />

      <StatusMessageTerminal
        generatedAt={projection.generatedAt}
        selectedJob={selectedJob}
        selectedValueName={projection.selectedValue?.label ?? "No value selected"}
        runContext={runContext}
      />

      <section className="status-ops-section" aria-label="Container orchestration">
        <div className="status-section-heading">
          <div>
            <p className="eyebrow">Container orchestration</p>
            <h3>Run-scoped simulation workers</h3>
          </div>
          <span className="simwb-count simulated">operational telemetry</span>
        </div>
        {orchestrationPanel}
      </section>

      <section className="status-hpc-section" aria-label="HPC status panel">
        <div className="status-section-heading">
          <div>
            <p className="eyebrow">HPC Status Panel</p>
            <h3>Queue-driven synthetic simulation-ops status</h3>
          </div>
          <span className="simwb-count pending">{bundleState}</span>
        </div>

        <div className="status-hpc-grid">
          <div className="status-queue-rail">{computeQueue}</div>
          <HpcStatusBay
            diagnosis={diagnosis}
            scenario={scenario}
            scenarioJobs={scenarioJobs}
            selectedJob={selectedJob}
          />
        </div>

        <OpsMessageTerminal
          diagnosis={diagnosis}
          selectedJob={selectedJob}
          runContext={runContext}
        />
      </section>
    </section>
  );
}

function formatTime(value: string): string {
  return new Date(value).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

function StatusMessageTerminal({
  generatedAt,
  selectedJob,
  selectedValueName,
  runContext
}: {
  generatedAt: string;
  selectedJob: ComputeJob;
  selectedValueName: string;
  runContext: string;
}) {
  return (
    <section className="status-terminal" aria-label="Status Workbench message terminal">
      <div className="status-terminal-heading">
        <span>Message terminal</span>
        <strong>{formatTime(generatedAt)}</strong>
      </div>
      <p>
        twin accepted selected value <strong>{selectedValueName}</strong> with value-basis lineage preserved.
      </p>
      <p>
        compute context <strong>{selectedJob.id}</strong> linked to run scope <strong>{runContext}</strong>.
      </p>
    </section>
  );
}

function HpcStatusBay({
  diagnosis,
  scenario,
  scenarioJobs,
  selectedJob
}: {
  diagnosis: ReturnType<typeof diagnoseJob>;
  scenario: string;
  scenarioJobs: ComputeJob[];
  selectedJob: ComputeJob;
}) {
  const activeJobs = scenarioJobs.filter((job) => job.state === "running" || job.state === "held").length;
  return (
    <div className="status-hpc-panels">
      <HpcPanel title="Panel 1: Multiphysics Co-scheduler" tag="non-safety">
        <MetricStrip
          metrics={[
            ["Slurm allocation", `${selectedJob.resources.ranks} ranks`, `${selectedJob.resources.nodes} nodes selected`],
            ["Internode sync", "1.04 ms", "p95 MPI barrier wait"],
            ["Queue pressure", `${activeJobs} active`, scenario],
            ["Synthetic alert", selectedJob.id, diagnosis.rootCause]
          ]}
        />
        <MiniGantt jobs={scenarioJobs} />
        <BoundaryNote>Scheduler and MPI telemetry are synthetic infrastructure stress signals.</BoundaryNote>
      </HpcPanel>

      <HpcPanel title="Panel 2: I/O Checkpoint Burst Buffer" tag="checkpoint active">
        <MetricStrip
          metrics={[
            ["Parallel FS IOPS", "1.8M", "synthetic aggregate"],
            ["Burst throughput", "12.4 GB/s", "64KB block writer path"],
            ["Cache saturation", "82%", "warn threshold crossed"],
            ["Checkpoint ETA", "02:47", `${selectedJob.resources.storageGb}GB job storage`]
          ]}
        />
        <div className="status-pipeline">
          <span>MEM BUFFER</span>
          <span>NVMe-oF CACHE</span>
          <span>PARALLEL FS</span>
        </div>
        <BoundaryNote>Dropped display samples are allowed; evidence degradation must be counted.</BoundaryNote>
      </HpcPanel>

      <HpcPanel title="Panel 3: Core Thermal Mesh Cloud Burst" tag="cloud burst active">
        <MetricStrip
          metrics={[
            ["ParallelCluster", "scaling", "128 local -> 512 cloud nodes"],
            ["AWS EFA drops", "0.18%", "warning threshold: 0.15%"],
            ["Spot estimate", "$41.20/hr", "synthetic burst estimate"],
            ["Hot-spot trigger", "cell C7", "click target, not physics claim"]
          ]}
        />
        <TopologyNodes labels={["LOCAL", "BURST", "EFA", "CLOUD"]} />
        <BoundaryNote>Thermal mesh is a workload visual, not a reactor analysis display.</BoundaryNote>
      </HpcPanel>

      <HpcPanel title="Panel 4: Fabric Topology and MPI Profiler" tag="fabric sample 2 Hz">
        <MetricStrip
          metrics={[
            ["Port errors", "3", "IB port counter deltas"],
            ["P95 link utilization", "84%", "synthetic fabric warning"],
            ["Non-blocking overhead", "7.8%", "MPI_Isend/Irecv proxy"],
            ["Hot link temp", "68 C", "color-coded topology"]
          ]}
        />
        <TopologyNodes labels={["SW-01", "N-12", "N-17", "N-22"]} />
        <BoundaryNote>Profiler values are synthetic HPC diagnostics, not plant instrumentation.</BoundaryNote>
      </HpcPanel>
    </div>
  );
}

function HpcPanel({ children, tag, title }: { children: ReactNode; tag: string; title: string }) {
  return (
    <article className="status-hpc-card">
      <div className="status-hpc-card-head">
        <h4>{title}</h4>
        <span>{tag}</span>
      </div>
      {children}
    </article>
  );
}

function MetricStrip({ metrics }: { metrics: Array<[string, string, string]> }) {
  return (
    <div className="status-metric-strip">
      {metrics.map(([label, value, detail]) => (
        <span key={label}>
          <small>{label}</small>
          <strong>{value}</strong>
          <em>{detail}</em>
        </span>
      ))}
    </div>
  );
}

function MiniGantt({ jobs }: { jobs: ComputeJob[] }) {
  return (
    <div className="status-mini-gantt" aria-label="Active Slurm queue Gantt">
      {jobs.slice(0, 5).map((job, index) => (
        <span className={job.state} key={job.id} style={{ "--offset": `${index * 12}px` } as CSSProperties}>
          <strong>{job.id}</strong>
          <em>{job.discipline}</em>
        </span>
      ))}
    </div>
  );
}

function TopologyNodes({ labels }: { labels: string[] }) {
  return (
    <div className="status-topology" aria-label="Synthetic topology">
      {labels.map((label) => (
        <span key={label}>{label}</span>
      ))}
    </div>
  );
}

function BoundaryNote({ children }: { children: ReactNode }) {
  return <p className="status-boundary-note">{children}</p>;
}

function OpsMessageTerminal({
  diagnosis,
  runContext,
  selectedJob
}: {
  diagnosis: ReturnType<typeof diagnoseJob>;
  runContext: string;
  selectedJob: ComputeJob;
}) {
  return (
    <section className="status-terminal ops" aria-label="Ops message terminal">
      <div className="status-terminal-heading">
        <span>Ops message terminal</span>
        <strong>{selectedJob.id}</strong>
      </div>
      <p>
        scheduler: <strong>{selectedJob.id}</strong> state {selectedJob.state}; next action {diagnosis.nextAction}
      </p>
      <p>
        artifact: synthetic status bay linked to <strong>{runContext}</strong>; evidence handoff remains in Evidence.
      </p>
    </section>
  );
}
