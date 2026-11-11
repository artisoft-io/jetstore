// The User Administration screen (user/screens/user-administration), from the
// JetStore Workspace menu: the accounts of 20_workspace.sql, and the dialog
// that activates an account and sets its roles, opened on the inactive account.
// The account is then activated as ops_user; no image shows it afterwards.
import { expect, test } from "@playwright/test";
import { openFlowMenu, screenshotsFor, settle, signIn, waitForTable } from "../../lib/shots";

const { shot } = screenshotsFor("user/screens/user-administration");

test("User Administration", async ({ page }) => {
  await signIn(page);
  await openFlowMenu(page, "User Administration");
  const table = await waitForTable(page);
  await shot(page, "users");

  await table.locator("tbody tr").filter({ hasText: "lee.marsh@localhost.test" }).getByRole("checkbox").check();
  await page.getByRole("button", { name: "Update User Profile" }).click();
  const dialog = page.locator("dialog[open]");
  await expect(dialog).toBeVisible();
  await settle(page);
  await shot(dialog, "update-user-dialog");
  await dialog.getByRole("button", { name: "Cancel" }).click();
  await expect(dialog).toHaveCount(0);

  // Activate the account with one role.
  const lee = table.locator("tbody tr").filter({ hasText: "lee.marsh@localhost.test" });
  await page.getByRole("button", { name: "Update User Profile" }).click();
  await expect(dialog).toBeVisible();
  await dialog.getByLabel("Active User?").selectOption("1");
  await dialog.locator("tbody tr").filter({ hasText: "ops_user" }).getByRole("checkbox").check();
  await dialog.getByRole("button", { name: "Submit" }).click();
  await expect(dialog).toHaveCount(0);
  await expect(lee.locator("td").nth(3)).toHaveText("1");
});
