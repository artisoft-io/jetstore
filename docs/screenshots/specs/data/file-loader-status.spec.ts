// File Loader Status: user/screens/file-loader-status.
// Reached from the Source Mapping flow's Loader Status button, its only link.
import { expect, test } from "@playwright/test";
import { openFlowMenu, screenshotsFor, signIn, waitForTable } from "../../lib/shots";
import { press, selectRow } from "./steps";

const { shot } = screenshotsFor("user/screens/file-loader-status");

test("File Loader Status, from Source Mapping, then a file's loaded rows", async ({ page }) => {
  await signIn(page);
  await openFlowMenu(page, "Source Mapping");
  await waitForTable(page);
  await press(page, "Loader Status");

  const loads = await waitForTable(page);
  await expect(page.getByText("File Loader Status", { exact: true })).toBeVisible();
  await expect(loads.locator("tbody tr")).toHaveCount(3);
  await selectRow(loads, "hc:MedicalClaim");
  await shot(page, "file-loader-status");

  // View Loaded Data opens the staging table at the file's session.
  await press(page, "View Loaded Data");
  await waitForTable(page);
  await expect(page.getByText("Table CI_D1_hc:MedicalClaim")).toBeVisible();
});
