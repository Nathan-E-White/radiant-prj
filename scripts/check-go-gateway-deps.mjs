import { spawnSync } from "node:child_process";

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
  "github.com/docker/docker",
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

console.log("internal/gateway default dependency graph excludes Iceberg/Arrow, Kafka, pgx, Docker SDK, and client-go adapters.");
