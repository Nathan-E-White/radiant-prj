import { spawnSync } from "node:child_process";
import test from "node:test";
import assert from "node:assert/strict";

const script = new URL("./simops-smoke-json.mjs", import.meta.url);

test("runtime-worker accepts observed Docker worker state and frame evidence", () => {
  const result = runHelper("runtime-worker", ["succeeded", "--frames"], {
    run_id: "RUN-SMOKE-001",
    workers: [
      {
        worker_id: "scheduler-01",
        frames: 2,
        observed_lifecycle: "succeeded",
        runtime: "docker",
        runtime_id: "container-success",
      },
    ],
  });

  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /RUN-SMOKE-001/);
  assert.match(result.stdout, /scheduler-01/);
  assert.match(result.stdout, /succeeded/);
});

test("runtime-worker rejects missing observed runtime state", () => {
  const result = runHelper("runtime-worker", ["succeeded"], {
    run_id: "RUN-SMOKE-002",
    workers: [
      {
        worker_id: "scheduler-01",
        frames: 2,
        lifecycle: "streaming",
      },
    ],
  });

  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /observed_lifecycle/);
});

test("runtime-worker accepts observed Kubernetes worker state", () => {
  const result = runHelper("runtime-worker", ["succeeded", "--frames", "--runtime", "kubernetes"], {
    run_id: "RUN-KIND-001",
    workers: [{
      worker_id: "scheduler-01",
      observed_lifecycle: "succeeded",
      runtime: "kubernetes",
      runtime_id: "radiant-simops/simops-run-kind-001-scheduler-01",
      frames: 2,
    }],
  });
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /runtime=kubernetes/);
});

test("container-proof accepts gateway-ingest-only worker env and redacts tokens", () => {
  const result = runHelper("container-proof", [], [
    {
      Id: "container-123",
      Name: "/simops-RUN-SMOKE-003-scheduler-01",
      Config: {
        Image: "radiant-simops-generator:latest",
        Env: [
          "SIMOPS_RUN_ID=RUN-SMOKE-003",
          "SIMOPS_WORKER_ID=scheduler-01",
          "SIMOPS_INGEST_URL=http://slurm-gateway:8080/internal/simops/runs/RUN-SMOKE-003/ingest",
          "SIMOPS_INGEST_TOKEN=secret-token",
          "SIMOPS_RESULT_INGEST_URL=http://slurm-gateway:8080/internal/simops/runs/RUN-SMOKE-003/results",
          "SIMOPS_RESULT_INGEST_TOKEN=secret-token",
        ],
        Labels: {
          "simops.run_id": "RUN-SMOKE-003",
          "simops.worker_id": "scheduler-01",
          "simops.role": "ordinary-worker",
          "simops.runtime_adapter": "docker-sdk",
        },
      },
    },
  ]);

  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /gateway-ingest-only/);
  assert.match(result.stdout, /SIMOPS_INGEST_TOKEN=<redacted>/);
  assert.doesNotMatch(result.stdout, /secret-token/);
});

test("container-proof rejects direct data-plane credentials on ordinary workers", () => {
  const result = runHelper("container-proof", [], [
    {
      Id: "container-456",
      Config: {
        Image: "radiant-simops-generator:latest",
        Env: [
          "SIMOPS_RUN_ID=RUN-SMOKE-004",
          "SIMOPS_WORKER_ID=scheduler-01",
          "SIMOPS_INGEST_URL=http://slurm-gateway:8080/internal/simops/runs/RUN-SMOKE-004/ingest",
          "SIMOPS_INGEST_TOKEN=secret-token",
          "SIMOPS_RESULT_INGEST_URL=http://slurm-gateway:8080/internal/simops/runs/RUN-SMOKE-004/results",
          "SIMOPS_RESULT_INGEST_TOKEN=secret-token",
          "SIMOPS_REDPANDA_BROKERS=redpanda:9092",
        ],
        Labels: {
          "simops.role": "ordinary-worker",
          "simops.runtime_adapter": "docker-sdk",
        },
      },
    },
  ]);

  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /direct data-plane/);
});

function runHelper(command, args, input) {
  return spawnSync(
    process.execPath,
    [script.pathname, command, ...args],
    {
      input: JSON.stringify(input),
      encoding: "utf8",
    },
  );
}
