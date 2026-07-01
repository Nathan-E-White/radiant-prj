import { readFileSync } from "node:fs";

const fixtures = JSON.parse(readFileSync(new URL("../src/data/readiness-fixtures.json", import.meta.url), "utf8"));

console.log("mock-worker: starting dry-run scheduler drain");

for (const job of fixtures.computeJobs) {
  const modules = job.resources.modules.join(",");
  console.log(
    `mock-worker: ${job.id} state=${job.state} nodes=${job.resources.nodes} ranks=${job.resources.ranks} modules=${modules}`
  );

  if (job.state === "failed") {
    console.log(`mock-worker: ${job.id} diagnosis=${job.diagnosis?.rootCause ?? "manual review required"}`);
  }
}

console.log("mock-worker: dry-run complete; no external infrastructure contacted");
