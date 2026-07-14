import { readFileSync } from "node:fs";

const implementation = [
  "backend/slurm-gateway/internal/gateway/configured_data_flush.go",
  "backend/slurm-gateway/internal/gateway/configured_data_flush_postgres.go",
  "backend/slurm-gateway/cmd/configured-data-flush/main.go"
].map((file) => readFileSync(file, "utf8")).join("\n");
const operations = readFileSync("docs/operations/configured-data-flush.md", "utf8");
const packageJson = readFileSync("package.json", "utf8");

const requiredImplementation = [
  "workbench_snapshot_generation",
  "sql.LevelSerializable",
  "ErrConfiguredDataFlushStalePlan",
  "workbench-resident-sources",
  "consumer-recovery-cursors",
  "required-redpanda-topics"
];
const forbiddenImplementation = [
  /TRUNCATE/i,
  /DROP\s+(TABLE|SCHEMA|DATABASE)/i,
  /DELETE\s+FROM\s+simops_runs/i,
  /DELETE\s+FROM\s+workbench_resident_/i,
  /docker\s+compose\s+down/i,
  /volume\s+prune/i
];
const problems = [];

for (const token of requiredImplementation) {
  if (!implementation.includes(token)) problems.push(`Configured Data Flush implementation missing ${token}`);
}
for (const pattern of forbiddenImplementation) {
  if (pattern.test(implementation)) problems.push(`Configured Data Flush implementation contains forbidden operation ${pattern}`);
}
for (const token of ["Dry-run plan", "Protected resources", "Recovery and verification", "--apply-plan"]) {
  if (!operations.includes(token)) problems.push(`Configured Data Flush operations guide missing ${token}`);
}
if (!packageJson.includes('"configured-data-flush"')) problems.push("package.json missing configured-data-flush command");

if (problems.length > 0) {
  for (const problem of problems) console.error(`- ${problem}`);
  process.exit(1);
}

console.log("Configured Data Flush contract checks passed.");
