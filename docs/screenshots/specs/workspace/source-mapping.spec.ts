// The Source Mapping flow (user/screens/flows/source-mapping): CI's source
// configurations, the mapping of hc:PharmacyClaim, and the dialog that pastes
// a mapping, cancelled.
import { expect, test } from "@playwright/test";
import { openFlowMenu, screenshotsFor, settle, signIn, waitForTable } from "../../lib/shots";

const { shot } = screenshotsFor("user/screens/flows/source-mapping");

test("Source Mapping flow", async ({ page }) => {
  await signIn(page);
  await openFlowMenu(page, "Source Mapping");
  const configs = await waitForTable(page);
  await shot(page, "select-source-config");

  await configs.locator("tbody tr").filter({ hasText: "hc:PharmacyClaim" }).getByRole("checkbox").check();
  await page.getByRole("button", { name: "Next", exact: true }).click();
  const mapping = await waitForTable(page);
  await expect(mapping.locator("tbody tr").filter({ hasText: "hc:Claim_ID" })).toBeVisible();
  await shot(page, "file-mapping");

  await page.getByRole("button", { name: "Paste File Mapping" }).click();
  const dialog = page.locator("dialog[open]");
  await expect(dialog).toBeVisible();
  await settle(page);
  await shot(dialog, "paste-mapping-dialog");
  await dialog.getByRole("button", { name: "Cancel" }).click();
  await expect(dialog).toHaveCount(0);
});
