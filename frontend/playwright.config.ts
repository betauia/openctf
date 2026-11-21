import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "tests",
  globalSetup: "./tests/login.ts",
  fullyParallel: true,
  reporter: "list",
  use: {
    baseURL: process.env.OCTF_URL ?? "http://localhost:4000",
    storageState: "tests/.auth.json",
  },
});
