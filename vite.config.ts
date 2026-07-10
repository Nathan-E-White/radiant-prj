import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": {
        target: "http://127.0.0.1:8080",
        changeOrigin: true,
        secure: false
      }
    }
  },
  test: {
    exclude: ["node_modules/**", "dist/**", "tests/e2e/**"],
    environment: "node",
    globals: true,
    exclude: ["**/node_modules/**", "**/dist/**", "**/.worktrees/**"]
  }
});
