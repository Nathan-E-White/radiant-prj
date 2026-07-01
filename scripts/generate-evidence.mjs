import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { createHash } from "node:crypto";

const fixtures = JSON.parse(readFileSync(new URL("../src/data/readiness-fixtures.json", import.meta.url), "utf8"));
const generatedAt = new Date("2026-06-30T19:30:00-05:00").toISOString();

const generated = {
  generatedAt,
  source: "src/data/readiness-fixtures.json",
  limitation: "Synthetic interview demonstration; not reactor design, safety, licensing, cybersecurity, or infrastructure evidence.",
  packs: fixtures.computeJobs.map((job) => ({
    id: `GEN-${job.id}`,
    runId: job.id,
    requirementIds: job.linkedRequirements,
    artifactHashes: Object.fromEntries(
      job.artifacts.map((artifact) => [
        artifact,
        sha256({ artifact, outputs: job.outputs, jobId: job.id }).slice(0, 16)
      ])
    ),
    schedulerState: job.state,
    diagnosis: job.diagnosis?.rootCause ?? "No failure diagnosis required."
  }))
};

mkdirSync(new URL("../generated", import.meta.url), { recursive: true });
writeFileSync(
  new URL("../generated/evidence-index.json", import.meta.url),
  `${JSON.stringify(generated, null, 2)}\n`
);

console.log(`Generated ${generated.packs.length} evidence pack summaries in generated/evidence-index.json.`);

function sha256(value) {
  return createHash("sha256").update(stableStringify(value)).digest("hex");
}

function stableStringify(value) {
  if (Array.isArray(value)) {
    return `[${value.map(stableStringify).join(",")}]`;
  }

  if (value && typeof value === "object") {
    return `{${Object.entries(value)
      .sort(([left], [right]) => left.localeCompare(right))
      .map(([key, child]) => `${JSON.stringify(key)}:${stableStringify(child)}`)
      .join(",")}}`;
  }

  return JSON.stringify(value);
}
