import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": {
        target: "http://127.0.0.1:8080",
        changeOrigin: true,
        secure: false,
        configure(proxy) {
          proxy.on("error", (_error, _request, response) => {
            if ("writeHead" in response && !response.headersSent) {
              response.writeHead(503, { "Content-Type": "application/json" });
              response.end(JSON.stringify({ error: "Local Workbench gateway unavailable", code: "workbench_unavailable" }));
            }
          });
        }
      }
    }
  },
  test: {
    exclude: ["**/node_modules/**", "**/dist/**", "**/.worktrees/**", "tests/e2e/**"],
    environment: "node",
    globals: true
  }
});
