import assert from "node:assert/strict";
import { mkdtemp, mkdir, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";

import { verifyBrowserDelivery } from "./browser-delivery-verifier.mjs";

test("browser delivery verification runs the public TypeScript, test, and production-build commands once and budgets build outputs", async (t) => {
  const root = await mkdtemp(path.join(os.tmpdir(), "radiant-browser-delivery-"));
  t.after(async () => (await import("node:fs/promises")).rm(root, { recursive: true, force: true }));
  await mkdir(path.join(root, "dist", "assets"), { recursive: true });
  await writeFile(path.join(root, "dist", "index.html"), '<script type="module" src="/assets/main.js"></script>');
  await writeFile(path.join(root, "dist", "assets", "main.js"), "main");
  await writeFile(path.join(root, "dist", "assets", "lazy.js"), "lazy");
  await writeFile(path.join(root, "dist", "assets", "background.png"), "png");

  const calls = [];
  const report = await verifyBrowserDelivery({
    root,
    budgets: { maxEntryBytes: 4, maxLazyChunkBytes: 4, maxRasterBytes: 3, maxTotalBytes: 100 },
    run: async (command, args) => {
      calls.push([command, ...args]);
      return { status: 0, stdout: `${args.at(-1)} passed`, stderr: "" };
    },
  });

  assert.equal(report.exitCode, 0);
  assert.deepEqual(calls, [["bun", "run", "typecheck"], ["bun", "run", "test"], ["bun", "run", "build:production"]]);
  assert.deepEqual(report.outputs, { entryBytes: 4, lazyChunkBytes: 4, rasterBytes: 3, totalBytes: 64 });
});

test("browser delivery verification reports command and output-budget failures", async (t) => {
  const root = await mkdtemp(path.join(os.tmpdir(), "radiant-browser-delivery-"));
  t.after(async () => (await import("node:fs/promises")).rm(root, { recursive: true, force: true }));
  await mkdir(path.join(root, "dist", "assets"), { recursive: true });
  await writeFile(path.join(root, "dist", "index.html"), '<script type="module" src="/assets/main.js"></script>');
  await writeFile(path.join(root, "dist", "assets", "main.js"), "main-too-large");

  const report = await verifyBrowserDelivery({
    root,
    budgets: { maxEntryBytes: 1, maxLazyChunkBytes: 1, maxRasterBytes: 1, maxTotalBytes: 1 },
    run: async (_command, args) => ({ status: args.at(-1) === "test" ? 1 : 0, stdout: "", stderr: "failed" }),
  });

  assert.equal(report.exitCode, 1);
  assert.match(report.failures.join("\n"), /test exited 1/);
  assert.match(report.failures.join("\n"), /entry asset/);
  assert.match(report.failures.join("\n"), /total production output/);
});

test("browser delivery verification does not budget stale output after a failed production build", async (t) => {
  const root = await mkdtemp(path.join(os.tmpdir(), "radiant-browser-delivery-"));
  t.after(async () => (await import("node:fs/promises")).rm(root, { recursive: true, force: true }));
  await mkdir(path.join(root, "dist", "assets"), { recursive: true });
  await writeFile(path.join(root, "dist", "index.html"), '<script type="module" src="/assets/main.js"></script>');
  await writeFile(path.join(root, "dist", "assets", "main.js"), "stale-output");

  const report = await verifyBrowserDelivery({
    root,
    budgets: { maxEntryBytes: 1, maxLazyChunkBytes: 1, maxRasterBytes: 1, maxTotalBytes: 1 },
    run: async (_command, args) => ({
      status: args.at(-1) === "build:production" ? 1 : 0,
      stdout: "domain failure marker",
      stderr: "",
    }),
  });

  assert.equal(report.outputs, undefined);
  assert.match(report.failures.join("\n"), /domain failure marker/);
  assert.match(report.failures.join("\n"), /output budgets were not evaluated/);
});
