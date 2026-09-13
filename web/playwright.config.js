import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./tests",
  use: { baseURL: "http://127.0.0.1:8081", headless: true },
  webServer: {
    command: "go run . -addr 127.0.0.1:8081 -demo -memory",
    cwd: "..",
    url: "http://127.0.0.1:8081/api/queues",
    reuseExistingServer: false,
  },
});
