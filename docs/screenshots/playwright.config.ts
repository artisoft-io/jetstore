// The screenshot specs' configuration. `npm run shots` (run.mjs) starts the
// stack and sets STACK_URL and JETS_ADMIN_PWD before Playwright reads this.
import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./specs",
  testMatch: "**/*.spec.ts",
  // One stack, one admin: the specs run one at a time, in file order, so that
  // a spec that changes data does so before or after another, never during.
  workers: 1,
  fullyParallel: false,
  retries: 0,
  timeout: 120_000,
  expect: { timeout: 15_000 },
  reporter: [["list"]],
  outputDir: "./test-results",
  use: {
    baseURL: process.env.STACK_URL,
    // The installed Google Chrome: nothing is downloaded.
    channel: "chrome",
    headless: true,
    viewport: { width: 1440, height: 900 },
    deviceScaleFactor: 1,
    colorScheme: "light",
    locale: "en-US",
    timezoneId: "UTC",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
});
