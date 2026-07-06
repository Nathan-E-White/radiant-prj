import { existsSync, readFileSync } from "node:fs";

const requiredDocs = [
  "docs/quality/quality-plan.md",
  "docs/quality/document-control.md",
  "docs/quality/configuration-management.md",
  "docs/quality/software-lifecycle.md",
  "docs/quality/verification-and-validation.md",
  "docs/quality/corrective-action.md",
  "docs/quality/records-management.md",
  "docs/quality/tool-control.md",
  "docs/quality/supplier-control.md",
  "docs/quality/release-readiness.md",
  "docs/quality/document-index.md",
  "docs/design/software-design-description.md",
  "docs/design/interface-control.md",
  "docs/design/simulator-workbench-backend-dataflow-slice.md",
  "docs/verification/verification-plan.md",
  "docs/verification/test-procedure.md",
  "docs/verification/test-report-template.md",
  "docs/release/release-checklist.md",
  "docs/release/baseline-record.md",
  "docs/release/approval-record.md",
  "docs/release/review-minutes-template.md",
  "docs/release/version-history.md",
  "docs/requirements/system-requirements.md",
  "docs/requirements/software-requirements.md",
  "docs/requirements/verification-matrix.md",
  "docs/requirements/objective-evidence-index.md",
  "docs/requirements/change-log.md"
];

const requiredTokens = ["Document ID", "Revision", "Status", "Owner"];
const problems = [];

for (const path of requiredDocs) {
  if (!existsSync(path)) {
    problems.push(`Missing controlled document: ${path}`);
    continue;
  }

  const text = readFileSync(path, "utf8");
  for (const token of requiredTokens) {
    if (!text.includes(token)) {
      problems.push(`${path} missing metadata token: ${token}`);
    }
  }

  if (!/^# .+/m.test(text)) {
    problems.push(`${path} missing top-level title`);
  }
}

requireContent("docs/quality/document-index.md", [
  "QP-001",
  "QP-010",
  "Software Design Description",
  "Simulator Workbench Backend Dataflow Slice",
  "Release Checklist"
]);
requireContent("docs/requirements/verification-matrix.md", [
  "SR-005",
  "SW-006",
  "SLURM-GATEWAY-001",
  "SW-009",
  "SR-008",
  "SW-012",
  "SIMOPS-BACKEND-001",
  "bun run quality:check",
  "bun run backend:test",
  "bun run simops:contract:check",
  "REL-TOOL-001",
  "SW-013",
  "SW-014",
  "SW-015",
  "WORKBENCH-DATAFLOW-001",
  "bun run simulator-workbench:dataflow:smoke"
]);
requireContent("docs/release/release-checklist.md", [
  "checkpoint-wip.sh",
  "fold-branch.sh",
  "checkpoint-version.sh"
]);
requireContent("README.md", [
  "Version 3.0",
  "quality:check",
  "backend:test",
  "simops:contract:check",
  "simulator-workbench:dataflow:smoke",
  "slurm-gateway",
  "simops-moq-gateway",
  "checkpoint-version.sh"
]);
requireContent("package.json", [
  "\"backend:test\"",
  "\"certs:local\"",
  "\"checkpoint:version\"",
  "\"fold:branch\"",
  "\"cleanup:version\"",
  "\"quality:check\"",
  "\"simulator-workbench:dataflow:smoke\"",
  "\"checkpoint:v2\"",
  "\"fold:v2\""
]);

if (problems.length) {
  console.error("Quality documentation check failed:");
  for (const problem of problems) {
    console.error(`- ${problem}`);
  }
  process.exit(1);
}

console.log(`Quality documentation check passed: ${requiredDocs.length} controlled documents verified.`);

function requireContent(path, tokens) {
  if (!existsSync(path)) {
    problems.push(`Missing file for content check: ${path}`);
    return;
  }

  const text = readFileSync(path, "utf8");
  for (const token of tokens) {
    if (!text.includes(token)) {
      problems.push(`${path} missing required content: ${token}`);
    }
  }
}
