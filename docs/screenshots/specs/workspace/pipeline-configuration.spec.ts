// The Pipeline Configuration flow (user/screens/flows/pipeline-configuration):
// CI's pipeline configurations, the add page, and each page of the edit of
// Patient_Profile_Template up to its summary. Nothing is saved: the summary is
// left with Cancel.
import { expect, test } from "@playwright/test";
import { openFlowMenu, screenshotsFor, settle, signIn, waitForTable } from "../../lib/shots";

const { shot } = screenshotsFor("user/screens/flows/pipeline-configuration");

test("Pipeline Configuration flow", async ({ page }) => {
  await signIn(page);
  await openFlowMenu(page, "Pipeline Configuration");
  const configs = await waitForTable(page);
  await shot(page, "select-pipeline-config");

  await page.getByRole("button", { name: "+ Add" }).click();
  await expect(page.getByLabel("Process")).toBeVisible();
  await settle(page);
  await shot(page, "add-pipeline-config");
  await page.getByRole("button", { name: "Cancel", exact: true }).click();
  await waitForTable(page);

  await configs.locator("tbody tr").filter({ hasText: "Patient_Profile_Template" }).getByRole("checkbox").check();
  await page.getByRole("button", { name: "Edit", exact: true }).click();
  await waitForTable(page);
  await shot(page, "main-process-input");

  const next = page.getByRole("button", { name: "Next", exact: true });
  await next.click();
  await expect(page.getByRole("button", { name: "Add Data Source to Merge" })).toBeVisible();
  await settle(page);
  await shot(page, "merged-inputs");

  await next.click();
  await expect(page.getByRole("button", { name: "Add Data Source for Historical Data" })).toBeVisible();
  await settle(page);
  await shot(page, "injected-inputs");

  await next.click();
  await expect(page.getByLabel("Execution Frequency")).toBeVisible();
  await settle(page);
  await shot(page, "automation");

  await next.click();
  await expect(page.getByRole("button", { name: "Save", exact: true })).toBeVisible();
  await settle(page);
  await shot(page, "summary");

  await page.getByRole("button", { name: "Cancel", exact: true }).click();
  await waitForTable(page);
});
