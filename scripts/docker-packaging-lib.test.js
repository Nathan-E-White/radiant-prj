import { describe, expect, test } from "vitest";
import { writeFileSync } from "node:fs";
import { mkdtemp } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import {
  buildInputIdentity,
  evaluatePackagingEvidence,
  parseBuildContextBytes,
  parseByteSize,
  summarizeBuildxBake
} from "./docker-packaging-lib.mjs";

describe("Docker packaging budgets", () => {
  test("parses Docker and BuildKit byte units", () => {
    expect(parseByteSize("2B")).toBe(2);
    expect(parseByteSize("1.5kB")).toBe(1500);
    expect(parseByteSize(" 1.25MB ")).toBe(1250000);
    expect(parseByteSize("1.5MiB")).toBe(1572864);
    expect(() => parseByteSize("12 blocks")).toThrow("Unsupported byte size");
    expect(() => parseByteSize("prefix 1MB")).toThrow("Unsupported byte size");
    expect(() => parseByteSize("1MB suffix")).toThrow("Unsupported byte size");
    expect(() => parseByteSize("1.25.5MB")).toThrow("Unsupported byte size");
  });

  test("uses the largest transferred context progress value", () => {
    expect(parseBuildContextBytes("transferring context: 4MB\ntransferring context:   14.55MB done")).toBe(14550000);
    expect(parseBuildContextBytes("transferring context: 4MB done")).toBe(4000000);
    expect(() => parseBuildContextBytes("context cached")).toThrow("did not report");
  });

  test("reports every exceeded or missing budget with useful labels", () => {
    const budgets = {
      buildContext: { maxBytes: 20 },
      builderCache: { maxGrowthBytes: 20, maxAggregateBytes: 30, maxReclaimableBytes: 10 },
      browserAssets: {
        maxTotalRawBytes: 20,
        maxTotalGzipBytes: 10,
        maxJavaScriptChunkBytes: 8,
        maxSingleAssetBytes: 12
      },
      images: { console: { maxBytes: 10 }, worker: { maxBytes: 10 } }
    };
    const evidence = {
      buildContextBytes: 21,
      builderCache: { growthBytes: 21, aggregateBytes: 31, reclaimableBytes: 11 },
      browserAssets: {
        totalRawBytes: 21,
        totalGzipBytes: 11,
        maxJavaScriptChunkBytes: 9,
        maxSingleAssetBytes: 13
      },
      images: { console: { sizeBytes: 11 } }
    };
    expect(evaluatePackagingEvidence(evidence, budgets)).toEqual([
      "Docker build context is 21 bytes; budget is 20 bytes",
      "Builder cache growth is 21 bytes; budget is 20 bytes",
      "Builder cache aggregate is 31 bytes; budget is 30 bytes",
      "Builder cache reclaimable is 11 bytes; budget is 10 bytes",
      "Browser assets raw total is 21 bytes; budget is 20 bytes",
      "Browser assets gzip total is 11 bytes; budget is 10 bytes",
      "Largest JavaScript chunk is 9 bytes; budget is 8 bytes",
      "Largest browser asset is 13 bytes; budget is 12 bytes",
      "Final image console is 11 bytes; budget is 10 bytes",
      "Missing final image evidence for worker"
    ]);
  });

  test("accepts measurements exactly at each limit", () => {
    const budgets = {
      buildContext: { maxBytes: 20 },
      builderCache: { maxGrowthBytes: 20, maxAggregateBytes: 30, maxReclaimableBytes: 10 },
      browserAssets: {
        maxTotalRawBytes: 20,
        maxTotalGzipBytes: 10,
        maxJavaScriptChunkBytes: 8,
        maxSingleAssetBytes: 12
      },
      images: { console: { maxBytes: 10 } }
    };
    const evidence = {
      buildContextBytes: 20,
      builderCache: { growthBytes: 20, aggregateBytes: 30, reclaimableBytes: 10 },
      browserAssets: {
        totalRawBytes: 20,
        totalGzipBytes: 10,
        maxJavaScriptChunkBytes: 8,
        maxSingleAssetBytes: 12
      },
      images: { console: { sizeBytes: 10 } }
    };
    expect(evaluatePackagingEvidence(evidence, budgets)).toEqual([]);
  });

  test("uses a tight architecture-specific image budget when one is configured", () => {
    const budgets = {
      buildContext: { maxBytes: 20 },
      builderCache: { maxGrowthBytes: 20, maxAggregateBytes: 30, maxReclaimableBytes: 10 },
      browserAssets: {
        maxTotalRawBytes: 20,
        maxTotalGzipBytes: 10,
        maxJavaScriptChunkBytes: 8,
        maxSingleAssetBytes: 12
      },
      images: {
        console: { maxBytes: 10, maxBytesByArchitecture: { amd64: 20 } }
      }
    };
    const evidence = {
      buildContextBytes: 20,
      builderCache: { growthBytes: 20, aggregateBytes: 30, reclaimableBytes: 10 },
      browserAssets: {
        totalRawBytes: 20,
        totalGzipBytes: 10,
        maxJavaScriptChunkBytes: 8,
        maxSingleAssetBytes: 12
      },
      images: { console: { architecture: "amd64", sizeBytes: 20 } }
    };

    expect(evaluatePackagingEvidence(evidence, budgets)).toEqual([]);
    evidence.images.console.sizeBytes = 21;
    expect(evaluatePackagingEvidence(evidence, budgets)).toEqual([
      "Final image console (amd64) is 21 bytes; budget is 20 bytes"
    ]);
  });

  test("rejects unavailable numeric measurements", () => {
    const budgets = {
      buildContext: { maxBytes: 20 },
      builderCache: { maxGrowthBytes: 20, maxAggregateBytes: 30, maxReclaimableBytes: 10 },
      browserAssets: {
        maxTotalRawBytes: 20,
        maxTotalGzipBytes: 10,
        maxJavaScriptChunkBytes: 8,
        maxSingleAssetBytes: 12
      },
      images: { console: { maxBytes: 10 } }
    };
    const evidence = {
      buildContextBytes: Number.NaN,
      builderCache: { growthBytes: 20, aggregateBytes: 30, reclaimableBytes: 10 },
      browserAssets: {
        totalRawBytes: 20,
        totalGzipBytes: 10,
        maxJavaScriptChunkBytes: 8,
        maxSingleAssetBytes: 12
      },
      images: { console: { sizeBytes: 10 } }
    };
    expect(evaluatePackagingEvidence(evidence, budgets)).toEqual(["Docker build context measurement is unavailable"]);
  });

  test("summarizes repeated Buildx Bake cache and Go build signals", () => {
    const summary = summarizeBuildxBake(`
      #10 CACHED
      #11 DONE 1.0s
      #12 DONE 2.0s
      importing cache manifest from gha
      cache hit from gha
      exporting cache
      preparing build cache for export
      failed to export cache
      RUN go build -p 2 ./cmd/server
      RUN go build -p 2 ./cmd/twin-projector
      RUN go test -p 2 ./...
    `);

    expect(summary).toEqual({
      cachedStepCount: 1,
      completedStepCount: 2,
      cacheImportCount: 2,
      cacheExportCount: 2,
      cacheExportErrorCount: 1,
      goBuildCommandCount: 2,
      goTestCommandCount: 1
    });
  });

  test("derives a stable completed-image identity from declared build inputs only", async () => {
    const root = await mkdtemp(join(tmpdir(), "radiant-packaging-key-"));
    writeFileSync(join(root, "Dockerfile"), "FROM base\nCOPY app.js app.js\n");
    writeFileSync(join(root, "package.json"), "{\"name\":\"fixture\"}\n");
    writeFileSync(join(root, "unrelated.md"), "first\n");
    const manifest = {
      registry: { host: "ghcr.io", owner: "example", repository: "repo", packagePrefix: "radiant-packaging" },
      images: [
        {
          role: "console",
          bakeTarget: "console",
          dockerfile: "Dockerfile",
          buildArgs: { BASE_IMAGE: "base@sha256:1234" },
          platforms: ["linux/amd64"],
          baseImages: [{ name: "base", ref: "base@sha256:1234" }],
          inputs: [{ path: "Dockerfile" }, { path: "package.json" }]
        }
      ]
    };

    const initial = buildInputIdentity(manifest, "console", { root, platform: "linux/amd64" });
    writeFileSync(join(root, "unrelated.md"), "second\n");
    const unrelated = buildInputIdentity(manifest, "console", { root, platform: "linux/amd64" });
    writeFileSync(join(root, "package.json"), "{\"name\":\"fixture\",\"version\":\"2\"}\n");
    const changedInput = buildInputIdentity(manifest, "console", { root, platform: "linux/amd64" });

    expect(unrelated.key).toBe(initial.key);
    expect(changedInput.key).not.toBe(initial.key);
    expect(initial.tag).toMatch(/^input-[a-f0-9]{64}$/);
    expect(initial.registryRef).toBe(`ghcr.io/example/repo/radiant-packaging-console:${initial.tag}`);
    expect(initial.files.map((file) => file.path)).toEqual(["Dockerfile", "package.json"]);
  });

  test("completed-image identity changes for platform, target, build args, and base image digest", async () => {
    const root = await mkdtemp(join(tmpdir(), "radiant-packaging-key-"));
    writeFileSync(join(root, "Dockerfile"), "FROM base\n");
    const manifest = {
      registry: { host: "ghcr.io", owner: "example", repository: "repo", packagePrefix: "radiant-packaging" },
      images: [
        {
          role: "gateway",
          bakeTarget: "gateway",
          dockerfile: "Dockerfile",
          target: "gateway-runtime",
          buildArgs: { BASE_IMAGE: "base@sha256:1234" },
          platforms: ["linux/amd64"],
          baseImages: [{ name: "base", ref: "base@sha256:1234" }],
          inputs: [{ path: "Dockerfile" }]
        }
      ]
    };
    const baseline = buildInputIdentity(manifest, "gateway", { root, platform: "linux/amd64" }).key;

    expect(buildInputIdentity(manifest, "gateway", { root, platform: "linux/arm64" }).key).not.toBe(baseline);
    manifest.images[0].target = "other-runtime";
    expect(buildInputIdentity(manifest, "gateway", { root, platform: "linux/amd64" }).key).not.toBe(baseline);
    manifest.images[0].target = "gateway-runtime";
    manifest.images[0].buildArgs.BASE_IMAGE = "base@sha256:5678";
    expect(buildInputIdentity(manifest, "gateway", { root, platform: "linux/amd64" }).key).not.toBe(baseline);
    manifest.images[0].buildArgs.BASE_IMAGE = "base@sha256:1234";
    manifest.images[0].baseImages[0].ref = "base@sha256:5678";
    expect(buildInputIdentity(manifest, "gateway", { root, platform: "linux/amd64" }).key).not.toBe(baseline);
  });
});
