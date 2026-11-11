// The Source Configuration flow (user/screens/flows/source-configuration):
// CI's source configurations, the add page, and each page of the edit of
// hc:PharmacyClaim up to its summary. Nothing is saved: the summary is left
// with Cancel.
import { expect, test } from "@playwright/test";
import { openFlowMenu, screenshotsFor, settle, signIn, waitForTable } from "../../lib/shots";

const { shot } = screenshotsFor("user/screens/flows/source-configuration");

test("Source Configuration flow", async ({ page }) => {
  await signIn(page);
  await openFlowMenu(page, "Source Configuration");
  const configs = await waitForTable(page);
  await shot(page, "select-source-config");

  // + Add opens the add page; Cancel returns to the list.
  await page.getByRole("button", { name: "+ Add" }).click();
  await expect(page.getByLabel("Object Type")).toBeVisible();
  await settle(page);
  await shot(page, "add-source-config");
  await page.getByRole("button", { name: "Cancel", exact: true }).click();
  await waitForTable(page);

  // Edit hc:PharmacyClaim, page by page.
  await configs.locator("tbody tr").filter({ hasText: "hc:PharmacyClaim" }).getByRole("checkbox").check();
  await page.getByRole("button", { name: "Edit", exact: true }).click();
  await waitForTable(page);
  await shot(page, "file-format");

  const next = page.getByRole("button", { name: "Next", exact: true });
  await next.click();
  await waitForTable(page);
  await shot(page, "single-or-multi-part");

  await next.click();
  await expect(page.getByLabel("Domain Key(s) (json)")).toBeVisible();
  await settle(page);
  await shot(page, "domain-keys");

  await next.click();
  await expect(page.getByLabel("Code Values Mapping (csv or json)")).toBeVisible();
  await settle(page);

  await next.click();
  await expect(page.getByLabel("Schema Provider JSON")).toBeVisible();
  await settle(page);

  await next.click();
  await expect(page.getByRole("button", { name: "Save", exact: true })).toBeVisible();
  await settle(page);
  await shot(page, "summary");

  await page.getByRole("button", { name: "Cancel", exact: true }).click();
  await waitForTable(page);
});
