import { affectedCapabilities } from "../capability-ledger/ledger.mjs";

const actions = new Set(["preserve", "retire", "supersede"]);

export function evaluateCapabilityRemovalPolicy({ previousLedger, currentLedger, currentManifest, changes = [], evidence = {} } = {}) {
  const declarations = evidence.declarations ?? [];
  const currentById = new Map((currentLedger?.capabilities ?? []).map((capability) => [capability.id, capability]));
  const claimIds = new Set((currentManifest?.claims ?? []).map(({ id }) => id));
  const paths = changes.flatMap((change) => [change.path, change.oldPath].filter(Boolean));
  const affected = affectedCapabilities(previousLedger, paths);
  const removals = [];

  for (const capability of previousLedger?.capabilities ?? []) {
    if (capability.lifecycle !== "active") continue;
    for (const surface of capability.surfaces ?? []) {
      if (surface.kind !== "artifact") continue;
      if (changes.some((change) => (change.status === "D" && change.path === surface.path) || (change.status === "R" && change.oldPath === surface.path))) {
        removals.push({ capabilityId: capability.id, kind: "artifact", path: surface.path });
      }
    }
    if (!claimIds.has(capability.verificationClaim)) {
      removals.push({ capabilityId: capability.id, kind: "verification-claim", id: capability.verificationClaim });
    }
  }

  const findings = removals.map((removal) => {
    const declaration = declarations.find((candidate) => matches(candidate, removal));
    if (!declaration) return fail(removal, "declare preserve, retire, or supersede evidence for this removed contract");
    if (!actions.has(declaration.action)) return fail(removal, `unsupported lifecycle action: ${declaration.action}`);
    const current = currentById.get(removal.capabilityId);
    if (!current) return fail(removal, "retain the capability record and record its lifecycle disposition");
    if (declaration.action === "preserve" && current.lifecycle === "active") return pass(removal, declaration.action);
    if (declaration.action === "retire" && current.lifecycle === "intentionally-retired") return pass(removal, declaration.action);
    if (declaration.action === "supersede" && current.lifecycle === "superseded" && current.successorId) {
      const successor = currentById.get(current.successorId);
      if (successor?.lifecycle === "active" && claimIds.has(successor.verificationClaim)) return pass(removal, declaration.action);
      return fail(removal, "supersession requires an active successor with a current verification claim");
    }
    return fail(removal, `${declaration.action} evidence does not match the current lifecycle record`);
  });

  return { schemaVersion: "radiant.capability-removal-policy.v1", affectedCapabilities: affected, findings: findings.sort(compareFindings), exitCode: findings.some(({ status }) => status === "fail") ? 1 : 0 };
}

function matches(declaration, removal) {
  return declaration?.capabilityId === removal.capabilityId
    && declaration?.removed?.kind === removal.kind
    && (removal.kind === "artifact" ? declaration.removed.path === removal.path : declaration.removed.id === removal.id);
}
function pass(removal, action) { return { status: "pass", ...removal, action, required: "lifecycle evidence matches the current Ledger record" }; }
function fail(removal, required) { return { status: "fail", ...removal, required }; }
function compareFindings(left, right) { return `${left.capabilityId}:${left.kind}:${left.path ?? left.id}`.localeCompare(`${right.capabilityId}:${right.kind}:${right.path ?? right.id}`); }
