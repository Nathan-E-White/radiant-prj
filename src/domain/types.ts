export type SourceType = "public-source" | "synthetic-model" | "controlled-record";

export type VerificationMethod =
  | "analysis"
  | "inspection"
  | "test"
  | "demonstration"
  | "configuration-audit";

export type SchedulerState = "queued" | "running" | "completed" | "failed" | "held";

export type Discipline =
  | "transport"
  | "thermal"
  | "fleet"
  | "deployment"
  | "infrastructure";

export type Requirement = {
  id: string;
  kind: "system" | "software" | "deployment";
  text: string;
  rationale: string;
  source: string;
  verificationMethod: VerificationMethod;
  linkedJobs: string[];
  linkedArtifacts: string[];
  owner: string;
  status: "draft" | "baselined" | "verified";
};

export type PublicFact = {
  id: string;
  topic: string;
  claim: string;
  sourceTitle: string;
  sourceUrl: string;
  sourceType: SourceType;
  confidence: "high" | "medium";
  boundary: string;
};

export type Milestone = {
  id: string;
  title: string;
  phase: string;
  status: "public" | "planned" | "in-progress";
  target: string;
  sourceUrl: string;
  dependencies: string[];
  note: string;
};

export type ComputeResources = {
  nodes: number;
  ranks: number;
  walltimeMin: number;
  storageGb: number;
  modules: string[];
};

export type ComputeJob = {
  id: string;
  title: string;
  discipline: Discipline;
  state: SchedulerState;
  scenario: string;
  priority: "normal" | "elevated" | "mission";
  stakeholder: string;
  resources: ComputeResources;
  inputs: Record<string, number | string | boolean>;
  logs: string[];
  outputs: Record<string, number | string | boolean>;
  diagnosis?: {
    rootCause: string;
    nextAction: string;
    preventativeControl: string;
  };
  linkedRequirements: string[];
  artifacts: string[];
};

export type EvidencePack = {
  id: string;
  runId: string;
  title: string;
  requirementIds: string[];
  artifactHashes: Record<string, string>;
  summary: string;
  limitations: string;
  approver: string;
  generatedAt: string;
};

export type ControlledEvidenceRecord = {
  id: string;
  title: string;
  category: "documentation-baseline" | "release-tooling" | "backend-gateway";
  requirementIds: string[];
  artifacts: string[];
  summary: string;
  limitations: string;
  approver: string;
  generatedAt: string;
};

export type DeploymentCheck = {
  id: string;
  hostRole: "scheduler" | "worker" | "artifact-store" | "monitor";
  configStatus: "pass" | "warn" | "fail";
  serviceStatus: "pass" | "warn" | "fail";
  networkStorage: "pass" | "warn" | "fail";
  finding: string;
  linkedRequirement: string;
};

export type ReadinessFixtures = {
  publicFacts: PublicFact[];
  milestones: Milestone[];
  requirements: Requirement[];
  computeJobs: ComputeJob[];
  evidencePacks: EvidencePack[];
  controlledEvidence: ControlledEvidenceRecord[];
  deploymentChecks: DeploymentCheck[];
};
