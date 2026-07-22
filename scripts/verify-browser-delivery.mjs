#!/usr/bin/env node
import { readFile } from "node:fs/promises";
import path from "node:path";

import { verifyBrowserDelivery } from "./browser-delivery-verifier.mjs";

const root = path.resolve(process.cwd());
const budgets = JSON.parse(await readFile(path.join(root, "config/browser-delivery-budgets.json"), "utf8"));
const report = await verifyBrowserDelivery({ root, budgets });
if (report.exitCode === 0) {
  console.log(`Browser delivery verified: entry=${report.outputs.entryBytes} bytes, lazy=${report.outputs.lazyChunkBytes} bytes, raster=${report.outputs.rasterBytes} bytes, total=${report.outputs.totalBytes} bytes.`);
} else {
  console.error("Browser delivery verification failed:");
  for (const failure of report.failures) console.error(`- ${failure}`);
}
process.exitCode = report.exitCode;
