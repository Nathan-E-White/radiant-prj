const byteUnits = {
  B: 1,
  kB: 1000,
  MB: 1000 ** 2,
  GB: 1000 ** 3,
  KiB: 1024,
  MiB: 1024 ** 2,
  GiB: 1024 ** 3
};

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

function over(violations, label, actual, limit) {
  if (!Number.isFinite(actual)) {
    violations.push(`${label} measurement is unavailable`);
  } else if (actual > limit) {
    violations.push(`${label} is ${actual} bytes; budget is ${limit} bytes`);
  }
}
