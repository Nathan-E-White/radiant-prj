import { readFile } from "node:fs/promises";
import { resolve } from "node:path";

const manifestPath = resolve("src/assets/fleet-board/fleet-board-v2-simulation-assets.json");
const manifest = JSON.parse(await readFile(manifestPath, "utf8"));
const expectedKeys = [
  "simulation-container-token",
  "reactor-slot-rail-empty",
  "reactor-slot-rail-idle",
  "reactor-slot-rail-queued",
  "reactor-slot-rail-running",
  "simulation-job-completed",
  "insight-token"
];

const actualKeys = manifest.assets.map((asset) => asset.semanticKey);
if (JSON.stringify(actualKeys) !== JSON.stringify(expectedKeys)) {
  throw new Error(`Unexpected V2 simulation asset keys: ${actualKeys.join(", ")}`);
}

for (const asset of manifest.assets) {
  if (asset.semanticKey.includes("artifact-forge") || asset.file.includes("artifact-forge")) {
    throw new Error("Artifact Forge imagery must remain deferred");
  }
  await verifyPng(resolve("src/assets/fleet-board", asset.file), asset.width, asset.height, true, asset.semanticKey);
}

await verifyPng(
  resolve("src/assets/fleet-board", manifest.atlas.file),
  manifest.atlas.width,
  manifest.atlas.height,
  true,
  "V2 source atlas"
);
await verifyPng(
  resolve("src/assets/fleet-board", manifest.preview.file),
  manifest.preview.width,
  manifest.preview.height,
  true,
  "V2 grouped preview"
);
await verifyPng(resolve(manifest.qa.file), manifest.qa.width, manifest.qa.height, false, "V2 transparency QA sheet");
await verifyPng(
  resolve(manifest.qa.boardScale.file),
  manifest.qa.boardScale.width,
  manifest.qa.boardScale.height,
  false,
  "V2 board-scale QA sheet"
);

console.log(`Fleet Board V2 asset pack verified: ${manifest.assets.length} transparent assets and two QA sheets.`);

async function verifyPng(filePath, expectedWidth, expectedHeight, requireAlpha, label) {
  const png = await readFile(filePath);
  const signature = png.subarray(0, 8).toString("hex");
  if (signature !== "89504e470d0a1a0a") {
    throw new Error(`${label} is not a PNG`);
  }
  const width = png.readUInt32BE(16);
  const height = png.readUInt32BE(20);
  const colorType = png[25];
  if (width !== expectedWidth || height !== expectedHeight) {
    throw new Error(`${label} is ${width}x${height}; expected ${expectedWidth}x${expectedHeight}`);
  }
  if (requireAlpha && colorType !== 6) {
    throw new Error(`${label} must use RGBA transparency; PNG color type was ${colorType}`);
  }
}
