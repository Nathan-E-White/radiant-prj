import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "tests/e2e",
  timeout: 45_000,
  expect: {
    timeout: 10_000
  },
  use: {
    baseURL: "http://127.0.0.1:5174",
    viewport: { width: 1440, height: 980 },
    trace: "on-first-retry",
    screenshot: "only-on-failure"
  },
  webServer: {
    command: "bun run dev -- --port 5174",
    url: "http://127.0.0.1:5174",
    reuseExistingServer: false,
    timeout: 120_000
  }
});
