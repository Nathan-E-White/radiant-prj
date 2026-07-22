const schemaVersion = "radiant.pipeline-policy.v1";

export function verifyPipelinePolicy({ policy, ledger, workflows = {} } = {}) {
  const findings = [];
  if (policy?.schemaVersion !== schemaVersion) findings.push(fail("policy.schema", "schemaVersion", schemaVersion, String(policy?.schemaVersion)));
  if (!Array.isArray(policy?.policies)) return report(findings.concat(fail("policy.schema", "policies", "an array", String(policy?.policies))));
  const capabilities = new Set((ledger?.capabilities ?? []).map(({ id }) => id));
  for (const declaration of policy.policies) {
    const prefix = `pipeline-policy.${declaration?.id ?? "unknown"}`;
    if (!capabilities.has(declaration?.capabilityId)) { findings.push(fail(prefix, "capabilityId", "an existing Capability Ledger record", String(declaration?.capabilityId))); continue; }
    const workflow = workflows[declaration.workflow];
    if (!workflow) { findings.push(fail(prefix, "workflow", "a readable workflow declaration", declaration.workflow)); continue; }
    comparePermissions(prefix, declaration.permissions, workflow.permissions, findings);
    for (const run of declaration.requiredRuns ?? []) if (!workflowRuns(workflow).includes(run)) findings.push(fail(prefix, "requiredRuns", run, "not present"));
    if (declaration.untrustedPullRequest) verifyUntrustedPullRequest(prefix, declaration.untrustedPullRequest, workflow, findings);
    if (declaration.trustedPublication) verifyTrustedPublication(prefix, declaration.trustedPublication, workflow, findings);
  }
  return report(findings);
}

function comparePermissions(prefix, expected = {}, actual = {}, findings) {
  for (const [scope, value] of Object.entries(expected)) if (actual?.[scope] !== value) findings.push(fail(prefix, `permissions.${scope}`, value, String(actual?.[scope])));
  for (const [scope, value] of Object.entries(actual ?? {})) if (value === "write" && expected[scope] !== "write") findings.push(fail(prefix, `permissions.${scope}`, "no undeclared write permission", value));
}
function verifyUntrustedPullRequest(prefix, policy, workflow, findings) {
  if (!hasPullRequestTrigger(workflow)) findings.push(fail(prefix, "untrustedPullRequest", "a pull_request trigger", "missing"));
  for (const [action, condition] of Object.entries(policy.allowActionWhen ?? {})) {
    for (const step of workflowSteps(workflow).filter(({ uses }) => uses === action)) if (step.if !== condition) findings.push(fail(prefix, `untrustedPullRequest.${action}`, condition, String(step.if)));
  }
}
function verifyTrustedPublication(prefix, policy, workflow, findings) {
  const trigger = workflow.on?.[policy.event];
  if (!trigger || !Array.isArray(trigger.branches) || !policy.branches.every((branch) => trigger.branches.includes(branch))) findings.push(fail(prefix, "trustedPublication", `${policy.event} restricted to ${policy.branches.join(", ")}`, JSON.stringify(trigger)));
}
function workflowSteps(workflow) { return Object.values(workflow.jobs ?? {}).flatMap(({ steps = [] }) => steps); }
function workflowRuns(workflow) { return workflowSteps(workflow).map(({ run }) => run).filter(Boolean); }
function hasPullRequestTrigger(workflow) { return Object.prototype.hasOwnProperty.call(workflow.on ?? {}, "pull_request"); }
function fail(policyId, field, expected, observed) { return { status: "fail", policyId, field, expected, observed }; }
function report(findings) { return { schemaVersion, exitCode: findings.length ? 1 : 0, findings: findings.sort((left, right) => `${left.policyId}:${left.field}`.localeCompare(`${right.policyId}:${right.field}`)) }; }
