import assert from "node:assert/strict";
import { mkdtemp, mkdir, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import test from "node:test";

import {
  formatVerificationReport,
  verifyRepository,
} from "./repository-verification/verifier.mjs";

test("structured JSON evidence is parsed and checked by path", async (t) => {
  const root = await temporaryRepository(t);
  await writeJson(root, "evidence.json", { services: { gateway: { mode: "mock" } } });

  const report = await verifyRepository({
    root,
    manifest: manifest([
      claim("compose.gateway-mode", "json", "evidence.json", [
        { path: "services.gateway.mode", equals: "mock" },
      ]),
    ]),
  });

  assert.equal(report.exitCode, 0);
  assert.equal(report.results[0].observed, 'services.gateway.mode = "mock"');
});

test("malformed and missing structured evidence report the claim contract", async (t) => {
  const root = await temporaryRepository(t);
  await writeFile(path.join(root, "broken.json"), "{ nope", "utf8");

  const report = await verifyRepository({
    root,
    manifest: manifest([
      claim("evidence.malformed", "json", "broken.json", [{ path: "ok", equals: true }]),
      claim("evidence.missing", "json", "missing.json", [{ path: "ok", equals: true }]),
    ]),
  });

  assert.equal(report.exitCode, 1);
  assert.deepEqual(report.results.map(({ claimId }) => claimId), ["evidence.malformed", "evidence.missing"]);
  assert.match(report.results[0].observed, /invalid JSON/);
  assert.match(report.results[1].observed, /input is missing/);
  assert.match(formatVerificationReport(report), /evidence: broken\.json/);
  assert.match(formatVerificationReport(report), /expected: ok equals true/);
});

test("document evidence stays textual when wording is the contract", async (t) => {
  const root = await temporaryRepository(t);
  await writeFile(path.join(root, "operations.md"), "# Flush\n\nDry-run plan\n", "utf8");

  const report = await verifyRepository({
    root,
    manifest: manifest([{
      id: "docs.flush-wording",
      title: "The operator guide names the dry-run plan",
      evidence: { adapter: "document", source: "operations.md" },
      expected: "required wording: Dry-run plan, Protected resources",
      requiredText: ["Dry-run plan", "Protected resources"],
    }]),
  });

  assert.equal(report.exitCode, 1);
  assert.equal(report.results[0].observed, 'missing required wording: "Protected resources"');
});

test("YAML and required-file evidence are verified structurally", async (t) => {
  const root = await temporaryRepository(t);
  await mkdir(path.join(root, "deploy"));
  await writeFile(path.join(root, "workflow.yml"), "jobs:\n  verify:\n    steps:\n      - run: bun run repository:verify\n", "utf8");
  await writeFile(path.join(root, "deploy", "compose.yml"), "services: {}\n", "utf8");

  const report = await verifyRepository({
    root,
    manifest: manifest([
      {
        id: "yaml.workflow",
        title: "workflow invokes repository verification",
        evidence: { adapter: "yaml", source: "workflow.yml" },
        expected: "jobs.verify.steps contains a step that runs repository verification",
        assertions: [{ path: "jobs.verify.steps", some: { run: "bun run repository:verify" } }],
      },
      {
        id: "files.infrastructure",
        title: "required infrastructure artifacts exist",
        evidence: { adapter: "files", source: "infrastructure artifact manifest", sources: ["deploy/compose.yml", "deploy/missing.yml"] },
        expected: "every declared infrastructure artifact exists",
      },
    ]),
  });

  assert.equal(report.exitCode, 1);
  assert.equal(report.results[0].observed, "missing inputs: deploy/missing.yml");
  assert.equal(report.results[1].status, "pass");
});

test("source-set contracts survive behavior-preserving token moves between files", async (t) => {
  const root = await temporaryRepository(t);
  await mkdir(path.join(root, "module"));
  await writeFile(path.join(root, "module", "first.go"), "package module\nconst guard = \"serializable\"\n", "utf8");
  await writeFile(path.join(root, "module", "second.go"), "package module\n", "utf8");
  await writeFile(path.join(root, "module", "safety_test.go"), "package module\nconst fixture = \"TRUNCATE TABLE\"\n", "utf8");
  const sourceClaim = {
    id: "module.safety",
    title: "module owns its safety guard",
    evidence: { adapter: "source-set", source: "module", extensions: [".go"], excludeSuffixes: ["_test.go"] },
    expected: "module contains serializable and no destructive operation",
    requiredText: ["serializable"],
    forbiddenPatterns: ["TRUNCATE\\s+TABLE"],
  };

  const before = await verifyRepository({ root, manifest: manifest([sourceClaim]) });
  await writeFile(path.join(root, "module", "first.go"), "package module\n", "utf8");
  await writeFile(path.join(root, "module", "second.go"), "package module\nconst guard = \"serializable\"\n", "utf8");
  const after = await verifyRepository({ root, manifest: manifest([sourceClaim]) });

  assert.equal(before.exitCode, 0);
  assert.equal(after.exitCode, 0);
  assert.equal(after.results[0].observed, "2 files satisfy 1 required and 1 forbidden text invariants");
});

test("command adapters report missing tools and observable output", async () => {
  const calls = [];
  const run = async (command, args, options) => {
    calls.push({ command, args, options });
    if (command === "missing-tool") {
      return { error: Object.assign(new Error("spawn missing-tool ENOENT"), { code: "ENOENT" }) };
    }
    return { status: 0, stdout: "contract passed\n", stderr: "" };
  };

  const report = await verifyRepository({
    root: "/repo",
    run,
    manifest: manifest([
      { ...commandClaim("behavior.ok", ["go", "test", "./..."], "command succeeds"), evidence: { adapter: "command", source: "go test ./...", command: ["go", "test", "./..."], cwd: "backend", env: { GOCACHE: "/tmp/go-cache" } } },
      commandClaim("behavior.tool", ["missing-tool", "--check"], "tool is available"),
    ]),
  });

  assert.equal(report.exitCode, 1);
  assert.equal(report.results[0].observed, "exit 0: contract passed");
  assert.equal(report.results[1].observed, "tool not found: missing-tool");
  assert.deepEqual(calls[0].args, ["test", "./..."]);
  assert.equal(calls[0].options.cwd, "/repo/backend");
  assert.equal(calls[0].options.env.GOCACHE, "/tmp/go-cache");
  assert.equal(calls[0].options.env.PATH, process.env.PATH);
});

test("environment-gated executable evidence is explicit when unavailable", async () => {
  const report = await verifyRepository({
    root: "/repo",
    manifest: manifest([{
      ...commandClaim("postgres.behavior", ["go", "test", "-tags", "postgresintegration", "./..."], "Postgres behavior passes"),
      evidence: {
        adapter: "command",
        source: "Postgres integration tests",
        command: ["go", "test", "-tags", "postgresintegration", "./..."],
        whenEnvironment: "RADIANT_VERIFIER_TEST_DSN_THAT_IS_NOT_SET",
      },
    }]),
  });

  assert.equal(report.exitCode, 0);
  assert.equal(report.results[0].status, "skip");
  assert.match(formatVerificationReport(report), /Repository verification: 0 passed, 1 skipped, 0 failed\./);
});

test("Compose and OpenTofu adapters parse command output structurally", async () => {
  const calls = [];
  const run = async (command, args) => {
    calls.push([command, ...args]);
    if (command === "docker") return { status: 0, stdout: JSON.stringify({ services: { gateway: { build: { target: "gateway-runtime" }, security_opt: ["no-new-privileges:true"] } } }), stderr: "" };
    if (args.includes("fmt")) return { status: 0, stdout: "", stderr: "" };
    return { status: 0, stdout: JSON.stringify({ valid: true, diagnostics: [] }), stderr: "" };
  };

  const report = await verifyRepository({
    root: "/repo",
    run,
    manifest: manifest([
      {
        id: "compose.target",
        title: "Compose selects the narrow gateway image",
        evidence: { adapter: "compose", source: "deploy/app.compose.yml" },
        expected: "services.gateway.build.target equals gateway-runtime",
        assertions: [
          { path: "services.gateway.build.target", equals: "gateway-runtime" },
          { path: "services.gateway.security_opt", equals: ["no-new-privileges:true"] },
        ],
      },
      {
        id: "tofu.valid",
        title: "OpenTofu configuration is valid",
        evidence: { adapter: "opentofu", source: "infra/opentofu" },
        expected: "OpenTofu validate reports valid=true",
      },
      {
        id: "tofu.syntax",
        title: "OpenTofu configuration parses",
        evidence: { adapter: "opentofu", source: "infra/simops", mode: "format" },
        expected: "OpenTofu fmt accepts every configuration file",
      },
    ]),
  });

  assert.equal(report.exitCode, 0);
  assert.deepEqual(report.results.map(({ status }) => status), ["pass", "pass", "pass"]);
  assert.ok(calls.some((call) => call.join(" ") === "tofu -chdir=infra/simops fmt -check -recursive"));
});

test("aggregate reporting is deterministic regardless of manifest order", async (t) => {
  const root = await temporaryRepository(t);
  await writeJson(root, "value.json", { ok: true });
  const claims = [
    claim("z.last", "json", "value.json", [{ path: "ok", equals: false }]),
    claim("a.first", "json", "value.json", [{ path: "ok", equals: true }]),
  ];

  const first = await verifyRepository({ root, manifest: manifest(claims) });
  const second = await verifyRepository({ root, manifest: manifest([...claims].reverse()) });

  assert.equal(formatVerificationReport(first), formatVerificationReport(second));
  assert.deepEqual(first.results.map(({ claimId }) => claimId), ["a.first", "z.last"]);
});

test("focused verification fails when a requested claim does not exist", async () => {
  const report = await verifyRepository({
    root: "/repo",
    claimIds: ["claim.missing"],
    manifest: manifest([commandClaim("claim.present", ["node", "--version"], "command succeeds")]),
  });

  assert.equal(report.exitCode, 1);
  assert.equal(report.results[0].claimId, "claim.missing");
  assert.equal(report.results[0].observed, "claim is not present in the manifest");
});

test("CLI has stable exit behavior and supports focused claim selection", async (t) => {
  const root = await temporaryRepository(t);
  await writeJson(root, "value.json", { ok: true });
  await writeJson(root, "claims.json", manifest([
    claim("claim.pass", "json", "value.json", [{ path: "ok", equals: true }]),
    claim("claim.fail", "json", "value.json", [{ path: "ok", equals: false }]),
  ]));
  const cli = path.resolve("scripts/repository-verification/cli.mjs");

  const focused = spawnSync(process.execPath, [cli, "--root", root, "--manifest", "claims.json", "--claim", "claim.pass"], { encoding: "utf8" });
  const aggregate = spawnSync(process.execPath, [cli, "--root", root, "--manifest", "claims.json"], { encoding: "utf8" });

  assert.equal(focused.status, 0);
  assert.match(focused.stdout, /Repository verification: 1 passed, 0 skipped, 0 failed\./);
  assert.equal(aggregate.status, 1);
  assert.match(aggregate.stderr, /\[FAIL\] claim\.fail/);
  assert.match(aggregate.stderr, /evidence: value\.json/);
  assert.match(aggregate.stderr, /expected: ok equals false/);
  assert.match(aggregate.stderr, /observed: ok = true/);
});

function manifest(claims) {
  return { schemaVersion: "radiant.repository-verification.v1", claims };
}

function claim(id, adapter, source, assertions) {
  return {
    id,
    title: id,
    evidence: { adapter, source },
    expected: assertions.map(({ path: assertionPath, equals }) => `${assertionPath} equals ${JSON.stringify(equals)}`).join("; "),
    assertions,
  };
}

function commandClaim(id, command, expected) {
  return {
    id,
    title: id,
    evidence: { adapter: "command", source: command.join(" "), command },
    expected,
  };
}

async function temporaryRepository(t) {
  const root = await mkdtemp(path.join(os.tmpdir(), "radiant-verification-"));
  t.after(async () => {
    const { rm } = await import("node:fs/promises");
    await rm(root, { recursive: true, force: true });
  });
  return root;
}

async function writeJson(root, relativePath, value) {
  const destination = path.join(root, relativePath);
  await mkdir(path.dirname(destination), { recursive: true });
  await writeFile(destination, JSON.stringify(value), "utf8");
}
