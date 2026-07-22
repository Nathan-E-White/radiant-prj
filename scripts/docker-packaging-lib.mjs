import { createHash } from "node:crypto";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative, resolve } from "node:path";

const byteUnits = {
  B: 1,
  kB: 1000,
  MB: 1000 ** 2,
  GB: 1000 ** 3,
  KiB: 1024,
  MiB: 1024 ** 2,
  GiB: 1024 ** 3
};

export function buildInputIdentity(manifest, role, { root = process.cwd(), platform } = {}) {
  const image = manifest.images?.find((candidate) => candidate.role === role);
  if (!image) throw new Error(`Unknown Docker packaging image role: ${role}`);
  const selectedPlatform = platform || image.platforms?.[0] || manifest.defaults?.platforms?.[0] || "linux/amd64";
  const files = collectBuildInputFiles(root, image.inputs ?? []);
  const canonical = {
    schemaVersion: manifest.schemaVersion || "radiant.docker-packaging-inputs.v1",
    role: image.role,
    bakeTarget: image.bakeTarget,
    dockerfile: image.dockerfile,
    target: image.target || null,
    platform: selectedPlatform,
    buildArgs: sortObject(image.buildArgs ?? {}),
    baseImages: selectedBaseImages(manifest, image.baseImages ?? [], selectedPlatform),
    files
  };
  const key = sha256Hex(JSON.stringify(canonical));
  const tag = `input-${key}`;
  return {
    role: image.role,
    bakeTarget: image.bakeTarget,
    key,
    tag,
    registryRef: registryRef(manifest, image, tag),
    platform: selectedPlatform,
    files,
    canonical
  };
}

export function parseByteSize(value) {
  const match = String(value).trim().match(/^(\d+(?:\.\d+)?)\s*(B|kB|MB|GB|KiB|MiB|GiB)$/);
  if (!match) throw new Error(`Unsupported byte size: ${value}`);
  return Math.round(Number(match[1]) * byteUnits[match[2]]);
}

export function parseBuildContextBytes(log) {
  const values = [...String(log).matchAll(/transferring context:\s+(\d+(?:\.\d+)?(?:B|kB|MB|GB|KiB|MiB|GiB))/g)]
    .map((match) => parseByteSize(match[1]));
  if (values.length === 0) throw new Error("BuildKit output did not report transferred build context bytes");
  return Math.max(...values);
}

export function evaluatePackagingEvidence(evidence, budgets) {
  const violations = [];
  over(violations, "Docker build context", evidence.buildContextBytes, budgets.buildContext.maxBytes);
  over(violations, "Builder cache growth", evidence.builderCache.growthBytes, budgets.builderCache.maxGrowthBytes);
  over(violations, "Builder cache aggregate", evidence.builderCache.aggregateBytes, budgets.builderCache.maxAggregateBytes);
  over(violations, "Builder cache reclaimable", evidence.builderCache.reclaimableBytes, budgets.builderCache.maxReclaimableBytes);
  over(violations, "Browser assets raw total", evidence.browserAssets.totalRawBytes, budgets.browserAssets.maxTotalRawBytes);
  over(violations, "Browser assets gzip total", evidence.browserAssets.totalGzipBytes, budgets.browserAssets.maxTotalGzipBytes);
  over(violations, "Largest JavaScript chunk", evidence.browserAssets.maxJavaScriptChunkBytes, budgets.browserAssets.maxJavaScriptChunkBytes);
  over(violations, "Largest browser asset", evidence.browserAssets.maxSingleAssetBytes, budgets.browserAssets.maxSingleAssetBytes);

  for (const [role, budget] of Object.entries(budgets.images)) {
    const image = evidence.images[role];
    const actual = image?.sizeBytes;
    if (actual === undefined) {
      violations.push(`Missing final image evidence for ${role}`);
    } else {
      const architecture = image.architecture;
      const architectureLimit = budget.maxBytesByArchitecture?.[architecture];
      const limit = architectureLimit ?? budget.maxBytes;
      const label = architectureLimit === undefined ? `Final image ${role}` : `Final image ${role} (${architecture})`;
      over(violations, label, actual, limit);
    }
  }
  return violations;
}

export function summarizeBuildxBake(log) {
  return {
    cachedStepCount: countMatches(log, /\bCACHED\b/g),
    completedStepCount: countMatches(log, /\bDONE\b/g),
    cacheImportCount: countMatches(log, /importing cache|cache hit from/g),
    cacheExportCount: countMatches(log, /exporting cache|preparing build cache for export/g),
    cacheExportErrorCount: countMatches(log, /failed to export cache|error writing cache|cache export failed/gi),
    goBuildCommandCount: countMatches(log, /go build -p 2/g),
    goTestCommandCount: countMatches(log, /go test -p 2/g)
  };
}

function collectBuildInputFiles(root, inputs) {
  const files = [];
  for (const input of inputs) {
    const inputPath = input.path;
    if (!inputPath) throw new Error("Docker packaging input is missing path");
    const absolute = resolve(root, inputPath);
    const stat = statSync(absolute);
    if (stat.isDirectory()) {
      for (const file of walkFiles(absolute)) files.push(fileRecord(root, file));
    } else {
      files.push(fileRecord(root, absolute));
    }
  }
  return files.sort((left, right) => left.path.localeCompare(right.path));
}

function selectedBaseImages(manifest, baseImages, platform) {
  return baseImages.map((baseImageRef) => {
    const baseImage = typeof baseImageRef === "string" ? manifest.baseImages?.[baseImageRef] : baseImageRef;
    if (!baseImage) throw new Error(`Unknown Docker packaging base image: ${baseImageRef}`);
    const name = typeof baseImageRef === "string" ? baseImageRef : baseImage.name;
    return sortObject({
      name,
      ref: baseImage.digestByPlatform?.[platform] ? `${baseImage.ref}@${baseImage.digestByPlatform[platform]}` : baseImage.ref
    });
  }).sort(compareJson);
}

function walkFiles(root) {
  return readdirSync(root, { withFileTypes: true }).flatMap((entry) => {
    const absolute = join(root, entry.name);
    if (entry.isDirectory()) return walkFiles(absolute);
    if (entry.isFile()) return [absolute];
    return [];
  });
}

function fileRecord(root, absolute) {
  const bytes = readFileSync(absolute);
  return {
    path: relative(root, absolute).replaceAll("\\", "/"),
    sha256: sha256Hex(bytes),
    bytes: bytes.length
  };
}

function registryRef(manifest, image, tag) {
  if (image.registryImage) return `${image.registryImage}:${tag}`;
  const registry = manifest.registry ?? {};
  const host = registry.host || "ghcr.io";
  const owner = registry.owner;
  const repository = registry.repository;
  const packagePrefix = registry.packagePrefix || "radiant-packaging";
  if (!owner || !repository) throw new Error("Docker packaging registry owner and repository are required");
  return `${host}/${owner}/${repository}/${packagePrefix}-${image.role}:${tag}`;
}

function sortObject(value) {
  if (Array.isArray(value)) return value.map(sortObject);
  if (value && typeof value === "object") {
    return Object.fromEntries(Object.entries(value).sort(([left], [right]) => left.localeCompare(right)).map(([key, item]) => [key, sortObject(item)]));
  }
  return value;
}

function compareJson(left, right) {
  return JSON.stringify(left).localeCompare(JSON.stringify(right));
}

function sha256Hex(value) {
  return createHash("sha256").update(value).digest("hex");
}

function over(violations, label, actual, limit) {
  if (!Number.isFinite(actual)) {
    violations.push(`${label} measurement is unavailable`);
  } else if (actual > limit) {
    violations.push(`${label} is ${actual} bytes; budget is ${limit} bytes`);
  }
}

function countMatches(value, pattern) {
  return (String(value).match(pattern) || []).length;
}
