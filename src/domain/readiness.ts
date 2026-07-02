import fixturesJson from "../data/readiness-fixtures.json";
import type {
  ControlledEvidenceRecord,
  ComputeJob,
  DeploymentCheck,
  EvidencePack,
  ReadinessFixtures,
  Requirement
} from "./types";

export const fixtures = fixturesJson as unknown as ReadinessFixtures;

export type TransportInput = {
  cells: number;
  sourceStrength: number;
  absorption: number;
  scatter: number;
};

export type TransportResult = {
  scalarFlux: number[];
  peakScalarFlux: number;
  edgeLeakageProxy: number;
  balanceResidual: number;
  iterations: number;
};

export type ThermalInput = {
  heatKw: number;
  ambientC: number;
  thermalResistanceCPerKw: number;
  limitC: number;
};

export type ThermalResult = {
  peakTemperatureC: number;
  marginC: number;
  loadFollowSettlingMin: number;
  status: "margin-positive" | "margin-negative";
};

export type FleetTelemetryInput = {
  values: number[];
  zLimit: number;
  packetLossPct: number;
  missingPacketLimitPct: number;
};

export type FleetTelemetryResult = {
  channelsFlagged: number;
  maxZScore: number;
  packetLossPct: number;
  status: "nominal" | "review-required";
};

export type Diagnosis = {
  rootCause: string;
  nextAction: string;
  preventativeControl: string;
};

export function runTransportToy(input: TransportInput): TransportResult {
  const cells = Math.max(2, Math.floor(input.cells));
  const dx = 1 / cells;
  const removal = Math.max(0.02, input.absorption + 0.25 * input.scatter);
  let angularPositive = 0;
  let angularNegative = 0;
  const source = Array.from({ length: cells }, (_, index) => {
    const center = (index + 0.5) / cells;
    return input.sourceStrength * (0.72 + 0.28 * Math.cos(Math.PI * (center - 0.5)));
  });
  const scalarFlux = source.map((cellSource) => cellSource / removal);

  for (let iteration = 0; iteration < 28; iteration += 1) {
    for (let index = 0; index < cells; index += 1) {
      const localSource = source[index] + input.scatter * scalarFlux[index] * 0.48;
      angularPositive = (angularPositive + localSource * dx) / (1 + removal * dx);
      scalarFlux[index] = 0.55 * scalarFlux[index] + 0.45 * (angularPositive + localSource);
    }

    for (let index = cells - 1; index >= 0; index -= 1) {
      const localSource = source[index] + input.scatter * scalarFlux[index] * 0.48;
      angularNegative = (angularNegative + localSource * dx) / (1 + removal * dx);
      scalarFlux[index] = 0.5 * scalarFlux[index] + 0.5 * (angularNegative + localSource);
    }
  }

  const peakScalarFlux = round(Math.max(...scalarFlux), 3);
  const edgeLeakageProxy = round(
    (scalarFlux[0] + scalarFlux[cells - 1]) / (2 * scalarFlux.reduce((sum, value) => sum + value, 0)),
    4
  );
  const balanceResidual = round(edgeLeakageProxy / 90 + 0.0001, 4);

  return {
    scalarFlux: scalarFlux.map((value) => round(value, 3)),
    peakScalarFlux,
    edgeLeakageProxy,
    balanceResidual,
    iterations: 28
  };
}

export function runPassiveThermalToy(input: ThermalInput): ThermalResult {
  const peakTemperatureC = round(
    input.ambientC + input.heatKw * input.thermalResistanceCPerKw,
    1
  );
  const marginC = round(input.limitC - peakTemperatureC, 1);
  const loadFollowSettlingMin = Math.max(4, Math.round(input.heatKw / 52));

  return {
    peakTemperatureC,
    marginC,
    loadFollowSettlingMin,
    status: marginC >= 0 ? "margin-positive" : "margin-negative"
  };
}

export function runFleetTelemetryToy(input: FleetTelemetryInput): FleetTelemetryResult {
  const mean = input.values.reduce((sum, value) => sum + value, 0) / input.values.length;
  const variance =
    input.values.reduce((sum, value) => sum + (value - mean) ** 2, 0) / input.values.length;
  const sigma = Math.sqrt(variance) || 1;
  const zScores = input.values.map((value) => Math.abs((value - mean) / sigma));
  const maxZScore = round(Math.max(...zScores), 2);
  const channelsFlagged = zScores.filter((zScore) => zScore >= input.zLimit).length;

  return {
    channelsFlagged,
    maxZScore,
    packetLossPct: input.packetLossPct,
    status:
      channelsFlagged > 0 || input.packetLossPct > input.missingPacketLimitPct
        ? "review-required"
        : "nominal"
  };
}

export function diagnoseJob(job: ComputeJob): Diagnosis {
  const logText = job.logs.join("\n").toLowerCase();

  if (job.diagnosis) {
    return job.diagnosis;
  }

  if (logText.includes("module") && logText.includes("not found")) {
    return {
      rootCause: "Required software module is missing from at least one worker image.",
      nextAction: "Drain the affected worker, rerun baseline configuration, and resubmit from the last verified artifact.",
      preventativeControl: "Gate worker deployments on module inventory comparison against the scheduler baseline."
    };
  }

  if (logText.includes("quota")) {
    return {
      rootCause: "Artifact storage quota blocked output manifest creation.",
      nextAction: "Move stale artifacts to retention storage and rerun the failed step.",
      preventativeControl: "Add artifact quota checks before scheduler release."
    };
  }

  return {
    rootCause: "No deterministic failure signature matched the current log bundle.",
    nextAction: "Escalate to engineering review with logs, inputs, and scheduler allocation record attached.",
    preventativeControl: "Add a new diagnosis rule after root cause is confirmed."
  };
}

export function hashArtifact(value: unknown): string {
  const text = stableStringify(value);
  let hash = 0x811c9dc5;

  for (let index = 0; index < text.length; index += 1) {
    hash ^= text.charCodeAt(index);
    hash = Math.imul(hash, 0x01000193);
  }

  return `fnv1a-${(hash >>> 0).toString(16).padStart(8, "0")}`;
}

export function buildEvidencePack(job: ComputeJob): EvidencePack {
  return {
    id: `EP-${job.id}`,
    runId: job.id,
    title: `${job.title} generated evidence`,
    requirementIds: job.linkedRequirements,
    artifactHashes: Object.fromEntries(
      job.artifacts.map((artifact) => [
        artifact,
        hashArtifact({ artifact, jobId: job.id, outputs: job.outputs })
      ])
    ),
    summary:
      job.state === "failed"
        ? `Run ${job.id} failed and has a captured diagnostic trail.`
        : `Run ${job.id} completed with deterministic synthetic outputs.`,
    limitations: "Generated from synthetic fixture data for interview demonstration only.",
    approver: "Automated evidence generator",
    generatedAt: new Date("2026-06-30T19:30:00-05:00").toISOString()
  };
}

export function validateTraceability(data: ReadinessFixtures = fixtures): string[] {
  const problems: string[] = [];
  const requirementIds = new Set(data.requirements.map((requirement) => requirement.id));
  const jobIds = new Set(data.computeJobs.map((job) => job.id));
  const artifactIds = new Set([
    ...data.publicFacts.map((fact) => fact.id),
    ...data.evidencePacks.map((pack) => pack.id),
    ...data.controlledEvidence.map((record) => record.id),
    ...data.deploymentChecks.map((check) => check.id),
    ...data.computeJobs.flatMap((job) => job.artifacts)
  ]);

  for (const requirement of data.requirements) {
    for (const jobId of requirement.linkedJobs) {
      if (!jobIds.has(jobId)) {
        problems.push(`${requirement.id} links missing job ${jobId}`);
      }
    }

    for (const artifactId of requirement.linkedArtifacts) {
      if (!artifactIds.has(artifactId)) {
        problems.push(`${requirement.id} links missing artifact ${artifactId}`);
      }
    }
  }

  for (const job of data.computeJobs) {
    for (const requirementId of job.linkedRequirements) {
      if (!requirementIds.has(requirementId)) {
        problems.push(`${job.id} links missing requirement ${requirementId}`);
      }
    }
  }

  for (const pack of data.evidencePacks) {
    if (!jobIds.has(pack.runId)) {
      problems.push(`${pack.id} links missing run ${pack.runId}`);
    }

    for (const requirementId of pack.requirementIds) {
      if (!requirementIds.has(requirementId)) {
        problems.push(`${pack.id} links missing requirement ${requirementId}`);
      }
    }
  }

  for (const record of data.controlledEvidence) {
    for (const requirementId of record.requirementIds) {
      if (!requirementIds.has(requirementId)) {
        problems.push(`${record.id} links missing requirement ${requirementId}`);
      }
    }
  }

  for (const check of data.deploymentChecks) {
    if (!requirementIds.has(check.linkedRequirement)) {
      problems.push(`${check.id} links missing requirement ${check.linkedRequirement}`);
    }
  }

  return problems;
}

export function requirementCoverage(
  requirements: Requirement[] = fixtures.requirements,
  jobs: ComputeJob[] = fixtures.computeJobs,
  evidencePacks: EvidencePack[] = fixtures.evidencePacks,
  controlledEvidence: ControlledEvidenceRecord[] = fixtures.controlledEvidence
) {
  return requirements.map((requirement) => ({
    id: requirement.id,
    status: requirement.status,
    verificationMethod: requirement.verificationMethod,
    linkedJobs: requirement.linkedJobs.length,
    linkedEvidence:
      evidencePacks.filter((pack) => pack.requirementIds.includes(requirement.id)).length +
      controlledEvidence.filter((record) => record.requirementIds.includes(requirement.id)).length,
    jobStates: jobs
      .filter((job) => job.linkedRequirements.includes(requirement.id))
      .map((job) => job.state)
  }));
}

export function deploymentScore(checks: DeploymentCheck[] = fixtures.deploymentChecks) {
  const weight = { pass: 2, warn: 1, fail: 0 };
  const possible = checks.length * 6;
  const actual = checks.reduce(
    (sum, check) =>
      sum + weight[check.configStatus] + weight[check.serviceStatus] + weight[check.networkStorage],
    0
  );

  return Math.round((actual / possible) * 100);
}

function stableStringify(value: unknown): string {
  if (Array.isArray(value)) {
    return `[${value.map(stableStringify).join(",")}]`;
  }

  if (value && typeof value === "object") {
    return `{${Object.entries(value as Record<string, unknown>)
      .sort(([left], [right]) => left.localeCompare(right))
      .map(([key, child]) => `${JSON.stringify(key)}:${stableStringify(child)}`)
      .join(",")}}`;
  }

  return JSON.stringify(value);
}

function round(value: number, places: number) {
  const scale = 10 ** places;
  return Math.round(value * scale) / scale;
}
