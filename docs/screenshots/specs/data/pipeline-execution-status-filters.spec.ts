// The Pipeline Execution Status Filters flow (homeFiltersUF):
// user/screens/flows/pipeline-execution-status-filters.
//
// The filters live in the browser's memory, so setting them here changes no
// data another spec reads.
import { expect, test } from "@playwright/test";
import { openScreen, screenshotsFor, settle, signIn, waitForTable } from "../../lib/shots";
import { press, selectRow } from "./steps";

const { shot } = screenshotsFor("user/screens/flows/pipeline-execution-status-filters");

test("Filter Home's runs to the failed runs of one process", async ({ page }) => {
  await signIn(page);
  await openScreen(page, "Home");
  await waitForTable(page);
  await press(page, "Set Filters");
  await expect(page).toHaveURL(/\/flow\/homeFiltersUF/);

  // 1. Process.
  const processes = await waitForTable(page);
  await selectRow(processes, "Patient_Profile_Template");
  await press(page, "Next");

  // 2. Status.
  const statuses = await waitForTable(page);
  await selectRow(statuses, "Failed");
  await shot(page, "status");
  await press(page, "Next");

  // 3. File key: no filter.
  const types = await waitForTable(page);
  await selectRow(types, "None");
  await press(page, "Next");

  // 4. Time window: runs that started on or after 2026-01-01.
  await page.getByLabel("Start Time").fill("2026-01-01T00:00:00Z");
  await shot(page, "time-window");
  await press(page, "Next");

  // 5. The runs the filters select.
  const preview = await waitForTable(page);
  await expect(preview.locator("tbody tr")).toHaveCount(1);
  await press(page, "Save & Done");

  // Back on Home, filtered, with Clear Filters enabled.
  await expect(page).toHaveURL(/\/home/);
  const runs = await waitForTable(page);
  await expect(runs.locator("tbody tr")).toHaveCount(1);
  await expect(page.getByRole("button", { name: "Clear Filters", exact: true })).toBeEnabled();
  await shot(page, "filtered-home");

  await press(page, "Clear Filters");
  await settle(page);
  await expect(runs.locator("tbody tr")).toHaveCount(9);
});
