// The Clients & Vendors flow (user/screens/flows/clients-and-vendors): the
// client list, the dialog that adds a client, and CI's vendors. Both dialogs
// are cancelled.
import { expect, test } from "@playwright/test";
import { openFlowMenu, screenshotsFor, settle, signIn, waitForTable } from "../../lib/shots";

const { shot } = screenshotsFor("user/screens/flows/clients-and-vendors");

test("Clients & Vendors flow", async ({ page }) => {
  await signIn(page);
  await openFlowMenu(page, "Clients & Vendors");
  const clients = await waitForTable(page);
  await shot(page, "select-client");

  await page.getByRole("button", { name: "+ Add" }).click();
  let dialog = page.locator("dialog[open]");
  await expect(dialog).toBeVisible();
  await settle(page);
  await shot(dialog, "add-client-dialog");
  await dialog.getByRole("button", { name: "Cancel" }).click();
  await expect(dialog).toHaveCount(0);

  await clients.locator("tbody tr").filter({ hasText: "Clinical Intel" }).getByRole("checkbox").check();
  await page.getByRole("button", { name: "Next", exact: true }).click();
  const orgs = await waitForTable(page);
  await expect(orgs.locator("tbody tr").filter({ hasText: "D1" })).toBeVisible();
  await shot(page, "vendors");

  await page.getByRole("button", { name: "+ Add" }).click();
  dialog = page.locator("dialog[open]");
  await expect(dialog).toBeVisible();
  await settle(page);
  await shot(dialog, "add-vendor-dialog");
  await dialog.getByRole("button", { name: "Cancel" }).click();
  await expect(dialog).toHaveCount(0);
});
