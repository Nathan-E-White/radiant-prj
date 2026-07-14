import { readFile, stat } from "node:fs/promises";
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
  const filePath = resolve("src/assets/fleet-board", asset.file);
  const png = await readFile(filePath);
  const signature = png.subarray(0, 8).toString("hex");
  if (signature !== "89504e470d0a1a0a") {
    throw new Error(`${asset.semanticKey} is not a PNG`);
  }
  const width = png.readUInt32BE(16);
  const height = png.readUInt32BE(20);
  const colorType = png[25];
  if (width !== asset.width || height !== asset.height) {
    throw new Error(`${asset.semanticKey} is ${width}x${height}; expected ${asset.width}x${asset.height}`);
  }
  if (colorType !== 6) {
    throw new Error(`${asset.semanticKey} must use RGBA transparency; PNG color type was ${colorType}`);
  }
}

const qaPath = resolve(manifest.qa.file);
const qaPng = await readFile(qaPath);
if (qaPng.readUInt32BE(16) !== 1024 || qaPng.readUInt32BE(20) !== 512) {
  throw new Error("Fleet Board V2 asset QA sheet must be 1024x512");
}
await stat(resolve("src/assets/fleet-board/fleet-board-v2-simulation-atlas.png"));

console.log(`Fleet Board V2 asset pack verified: ${manifest.assets.length} transparent assets and one QA sheet.`);
