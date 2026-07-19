import { readFileSync } from "node:fs";

const forge = readFileSync("backend/slurm-gateway/internal/gateway/artifact_forge.go", "utf8");
const intents = readFileSync("backend/slurm-gateway/internal/gateway/fleet_board_intents.go", "utf8");
const workbenchStore = readFileSync("backend/slurm-gateway/internal/gateway/workbench_store.go", "utf8");
const flush = readFileSync("backend/slurm-gateway/internal/gateway/configured_data_flush.go", "utf8");
const problems = [];

for (const token of [
  'ArtifactForgeEligibleArtifactKind = "simulated-result-state"',
  'ArtifactForgeTelemetryIneligible',
  'ArtifactForgeArtifactIncomplete',
  'ArtifactForgeIntegrityFailed',
  'ArtifactForgeLineageMissing',
  'ArtifactForgeOutcomeApplied',
  'SourceKind: "game-session"',
  'SourceKind: "fleet-reactor"',
  'SourceKind: "simulation-recipe"',
  'artifactForgeLineageEligible',
]) {
  if (!forge.includes(token)) problems.push(`Artifact Forge contract missing ${token}`);
}
if (!intents.includes('request.Intent != "requestArtifactForge"')) {
  problems.push("Fleet Board intent handler does not expose requestArtifactForge");
}
if (!workbenchStore.includes("buildArtifactForgeResultArtifact") || !workbenchStore.includes("artifact_forge_result_artifacts")) {
  problems.push("durably projected Simulated Result State does not record verified artifact metadata");
}
if (forge.includes('ArtifactForgeEligibleArtifactKind = "iceberg-table-partition"')) {
  problems.push("operational telemetry artifact was made eligible for a game outcome");
}
if (!flush.includes('Name: "artifact_forge_requests"')) {
  problems.push("Configured Data Flush does not target the Artifact Forge event ledger");
}

if (problems.length > 0) {
  console.error(problems.join("\n"));
  process.exit(1);
}

console.log("Artifact Forge contract checks passed.");
