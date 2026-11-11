// The Workspaces registry (user/screens/workspaces): the table, a selected
// workspace's actions, and the dialogs they open. Every dialog is cancelled:
// git access is off on the stack, and compiling would change the row the
// images show.
import { expect, test, type Page } from "@playwright/test";
import { openScreen, screenshotsFor, settle, signIn, waitForTable } from "../../lib/shots";

const { shot } = screenshotsFor("user/screens/workspaces");

async function openDialog(page: Page, button: string, image: string, client?: string) {
  await page.getByRole("button", { name: button, exact: true }).click();
  const dialog = page.locator("dialog[open]");
  await expect(dialog).toBeVisible();
  await settle(page);
  if (client) {
    await dialog.getByRole("combobox").selectOption({ label: client });
    await settle(page);
  }
  await shot(dialog, image);
  await dialog.getByRole("button", { name: "Cancel" }).click();
  await expect(dialog).toHaveCount(0);
}

test("Workspaces registry", async ({ page }) => {
  await signIn(page);
  await openScreen(page, "Workspaces");
  const table = await waitForTable(page);
  await shot(page, "registry");

  await table.locator("tbody tr").filter({ hasText: "jets_ws" }).getByRole("checkbox").check();
  await settle(page);
  await shot(page, "registry-selected");

  await openDialog(page, "Add/Update", "add-update-dialog");
  await openDialog(page, "Export Client Config", "export-client-config-dialog", "CI");
  await openDialog(page, "Commit & Push Workspace", "commit-dialog");
  await openDialog(page, "Git Command", "git-command-dialog");
});
