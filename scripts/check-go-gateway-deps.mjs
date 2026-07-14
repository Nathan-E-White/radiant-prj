import { spawnSync } from "node:child_process";
import { readFileSync } from "node:fs";

import {
  assertLocalModuleReplacement,
  assertMinimumModuleVersion,
  assertModuleNotRequired,
} from "./go-module-version.mjs";

const legacyDockerModule = "github.com/docker/docker";
const mobyApiModule = "github.com/moby/moby/api";
const mobyClientModule = "github.com/moby/moby/client";
const dockerCompatReplacement = "./third_party/docker-compat";
const goMod = readFileSync("backend/slurm-gateway/go.mod", "utf8");

try {
  assertModuleNotRequired(goMod, legacyDockerModule);
  assertMinimumModuleVersion(goMod, mobyApiModule, "1.55.0");
  assertMinimumModuleVersion(goMod, mobyClientModule, "0.5.0");
} catch (error) {
  console.error(error.message);
  process.exit(1);
}

const moduleCommand = spawnSync("go", ["list", "-m", "-json", legacyDockerModule], {
  cwd: "backend/slurm-gateway",
  encoding: "utf8",
  env: {
    ...process.env,
    GOCACHE: process.env.GOCACHE || "/tmp/radiant-go-cache",
    GOMODCACHE: process.env.GOMODCACHE || "/tmp/radiant-go-mod-cache",
    GOTMPDIR: process.env.GOTMPDIR || "/tmp"
  }
});

if (moduleCommand.error || moduleCommand.status !== 0) {
  process.stderr.write(moduleCommand.stderr || moduleCommand.error?.message || "go list -m failed\n");
  process.exit(moduleCommand.status ?? 1);
}

try {
  assertLocalModuleReplacement(
    JSON.parse(moduleCommand.stdout),
    legacyDockerModule,
    dockerCompatReplacement,
  );
} catch (error) {
  console.error(error.message);
  process.exit(1);
}

const command = spawnSync("go", ["list", "-deps", "-test", "./internal/gateway"], {
  cwd: "backend/slurm-gateway",
  encoding: "utf8",
  env: {
    ...process.env,
    GOCACHE: process.env.GOCACHE || "/tmp/radiant-go-cache",
    GOMODCACHE: process.env.GOMODCACHE || "/tmp/radiant-go-mod-cache",
    GOTMPDIR: process.env.GOTMPDIR || "/tmp"
  }
});

if (command.error) {
  console.error(`go list failed: ${command.error.message}`);
  process.exit(1);
}
if (command.status !== 0) {
  process.stderr.write(command.stderr);
  process.exit(command.status ?? 1);
}

const deps = command.stdout.split(/\r?\n/).filter(Boolean);
const forbidden = [
  "github.com/apache/arrow-go",
  "github.com/apache/iceberg-go",
  "github.com/jackc/pgx",
  "github.com/segmentio/kafka-go",
  legacyDockerModule,
  mobyApiModule,
  mobyClientModule,
  "k8s.io/client-go"
];
const hits = deps.filter((dep) => forbidden.some((prefix) => dep.startsWith(prefix)));

if (hits.length > 0) {
  console.error("internal/gateway should not depend on heavyweight concrete runtime adapters by default:");
  for (const hit of hits) {
    console.error(`- ${hit}`);
  }
  process.exit(1);
}

console.log(
  "internal/gateway default dependency graph excludes Iceberg/Arrow, Kafka, pgx, Docker SDK, and client-go adapters; the transitive legacy Docker edge resolves to a local test-only shim and split Moby API/client modules meet their security floors.",
);
