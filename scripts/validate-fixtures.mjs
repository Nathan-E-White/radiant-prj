import { readFileSync } from "node:fs";

const fixtures = JSON.parse(readFileSync(new URL("../src/data/readiness-fixtures.json", import.meta.url), "utf8"));
const problems = [];

const ids = {
  facts: new Set(fixtures.publicFacts.map((fact) => fact.id)),
  requirements: new Set(fixtures.requirements.map((requirement) => requirement.id)),
  jobs: new Set(fixtures.computeJobs.map((job) => job.id)),
  evidence: new Set(fixtures.evidencePacks.map((pack) => pack.id)),
  deployments: new Set(fixtures.deploymentChecks.map((check) => check.id))
};

for (const fact of fixtures.publicFacts) {
  requireField(fact, "id", "public fact");
  requireField(fact, "claim", fact.id);
  requireUrl(fact.sourceUrl, fact.id);
  if (!fact.boundary.toLowerCase().includes("not") && !fact.boundary.toLowerCase().includes("only")) {
    problems.push(`${fact.id} should include a clear claim boundary`);
  }
}

for (const requirement of fixtures.requirements) {
  requireField(requirement, "text", requirement.id);
  requireField(requirement, "verificationMethod", requirement.id);

  for (const jobId of requirement.linkedJobs) {
    if (!ids.jobs.has(jobId)) {
      problems.push(`${requirement.id} links missing job ${jobId}`);
    }
  }

  for (const artifactId of requirement.linkedArtifacts) {
    if (
      !ids.facts.has(artifactId) &&
      !ids.evidence.has(artifactId) &&
      !ids.deployments.has(artifactId) &&
      !fixtures.computeJobs.some((job) => job.artifacts.includes(artifactId))
    ) {
      problems.push(`${requirement.id} links missing artifact ${artifactId}`);
    }
  }
}

for (const job of fixtures.computeJobs) {
  requireField(job, "title", job.id);
  requireField(job, "state", job.id);
  if (!job.logs.length) {
    problems.push(`${job.id} must include traceable logs`);
  }

  for (const requirementId of job.linkedRequirements) {
    if (!ids.requirements.has(requirementId)) {
      problems.push(`${job.id} links missing requirement ${requirementId}`);
    }
  }
}

for (const pack of fixtures.evidencePacks) {
  if (!ids.jobs.has(pack.runId)) {
    problems.push(`${pack.id} links missing run ${pack.runId}`);
  }

  if (!pack.limitations.toLowerCase().includes("synthetic")) {
    problems.push(`${pack.id} must state synthetic limitations`);
  }

  for (const requirementId of pack.requirementIds) {
    if (!ids.requirements.has(requirementId)) {
      problems.push(`${pack.id} links missing requirement ${requirementId}`);
    }
  }
}

for (const check of fixtures.deploymentChecks) {
  if (!ids.requirements.has(check.linkedRequirement)) {
    problems.push(`${check.id} links missing requirement ${check.linkedRequirement}`);
  }
}

if (problems.length) {
  console.error("Fixture validation failed:");
  for (const problem of problems) {
    console.error(`- ${problem}`);
  }
  process.exit(1);
}

console.log(`Fixture validation passed: ${fixtures.requirements.length} requirements, ${fixtures.computeJobs.length} jobs, ${fixtures.evidencePacks.length} evidence packs.`);

function requireField(record, field, label) {
  if (record[field] == null || record[field] === "") {
    problems.push(`${label} missing ${field}`);
  }
}

function requireUrl(value, label) {
  try {
    const url = new URL(value);
    if (!["https:"].includes(url.protocol)) {
      problems.push(`${label} source must use https`);
    }
  } catch {
    problems.push(`${label} has invalid source URL`);
  }
}
