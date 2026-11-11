// The Rules Configuration screen (user/screens/rules-configuration), from the
// JetStore Workspace menu: the rule configurations of 20_workspace.sql, and
// the dialog that adds or updates one, opened on a selected row and cancelled.
import { expect, test } from "@playwright/test";
import { openFlowMenu, screenshotsFor, settle, signIn, waitForTable } from "../../lib/shots";

const { shot } = screenshotsFor("user/screens/rules-configuration");

test("Rules Configuration", async ({ page }) => {
  await signIn(page);
  await openFlowMenu(page, "Rules Configuration");
  const table = await waitForTable(page);
  await shot(page, "rule-configurations");

  await table.locator("tbody tr").filter({ hasText: "Patient_Profile_Template" }).getByRole("checkbox").check();
  await page.getByRole("button", { name: "Add/Update Rule Configuration" }).click();
  const dialog = page.locator("dialog[open]");
  await expect(dialog).toBeVisible();
  await settle(page);
  await shot(dialog, "rule-configuration-dialog");
  await dialog.getByRole("button", { name: "Cancel" }).click();
  await expect(dialog).toHaveCount(0);
});
