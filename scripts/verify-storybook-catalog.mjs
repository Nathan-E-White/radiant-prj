import { readFile, rm } from "node:fs/promises";
import path from "node:path";
import { spawnSync } from "node:child_process";

const outputDirectory = "storybook-static";
const requiredVariants = ["Nominal", "LifecycleRunningWithStaleStream", "ArtifactPipelineDegraded", "CriticalWorkerAndArtifacts"];

export function inspectStorybookCatalog({ indexHtml, index }) {
  if (!indexHtml.includes('<div id="root">')) {
    return "the production catalog does not contain Storybook's application root";
  }
  const stories = Object.values(index.entries ?? {}).filter(({ type }) => type === "story");
  if (stories.length === 0) return "the production catalog contains no compiled stories";
  const titles = stories.map(({ title }) => title).filter(Boolean);
  if (!titles.some((title) => title.includes("Simulation Health"))) {
    return "the production catalog does not represent Simulation Health";
  }
  if (!titles.some((title) => title.includes("Simulator Workbench"))) {
    return "the production catalog does not represent Simulator Workbench";
  }
  const exports = new Set(stories.map(({ exportName }) => exportName));
  const missingVariants = requiredVariants.filter((variant) => !exports.has(variant));
  if (missingVariants.length) return `the production catalog is missing presentation variants: ${missingVariants.join(", ")}`;
  return { storyCount: stories.length, titles: [...new Set(titles)].sort() };
}

async function main() {
  const root = process.cwd();
  const output = path.join(root, outputDirectory);
  await rm(output, { recursive: true, force: true });
  const build = spawnSync("bun", ["run", "build-storybook"], { cwd: root, encoding: "utf8" });
  if (build.error) throw build.error;
  if (build.status !== 0) throw new Error(`Storybook production build exited ${build.status}: ${(build.stderr || build.stdout).trim()}`);

  const [indexHtml, indexText] = await Promise.all([
    readFile(path.join(output, "index.html"), "utf8"),
    readFile(path.join(output, "index.json"), "utf8"),
  ]);
  const observation = inspectStorybookCatalog({ indexHtml, index: JSON.parse(indexText) });
  if (typeof observation === "string") throw new Error(observation);
  console.log(`Storybook catalog: ${observation.storyCount} compiled stories; ${observation.titles.join(", ")}`);
}

if (process.argv[1] && path.resolve(process.argv[1]) === new URL(import.meta.url).pathname) {
  await main();
}
