// The Workspace IDE (user/screens/workspace-ide): reached from the top bar,
// with jets_ws picked. Its changes table, a file open in the editor and edited,
// and the compiled view of the data model. The edit is never saved: the tab is
// closed and the change discarded.
import { expect, test } from "@playwright/test";
import { openScreen, screenshotsFor, settle, signIn } from "../../lib/shots";

const { shot } = screenshotsFor("user/screens/workspace-ide");

test("Workspace IDE", async ({ page }) => {
  await signIn(page);
  await openScreen(page, "Workspace IDE");
  await expect(page.locator(".ws-picker select")).toHaveValue("jets_ws");
  await settle(page);
  await shot(page, "ide-changes");

  // A file opens in a tab of the editor.
  const tree = page.locator("aside.sidebar");
  await tree.getByTitle("hc_model.jr", { exact: true }).click();
  await expect(page.locator(".cm-content")).toBeVisible();
  await settle(page);
  await shot(page, "ide-file");

  // An edit marks the tab as modified and enables Save.
  await page.locator(".cm-content").click();
  await page.keyboard.press("Control+End");
  await page.keyboard.type("\n# a comment");
  await expect(page.getByRole("button", { name: "Save", exact: true })).toBeEnabled();
  await settle(page);
  await shot(page.locator("footer.statusbar"), "ide-status-modified");

  // Closing the tab asks first; the edit is discarded.
  await page.getByRole("button", { name: "Close data_model/hc_model.jr" }).click();
  const confirm = page.locator("dialog[open]");
  await expect(confirm).toBeVisible();
  await shot(confirm, "ide-close-unsaved");
  await confirm.getByRole("button", { name: "OK" }).click();
  await expect(confirm).toHaveCount(0);

  // A section heading opens its compiled view.
  await tree.getByRole("button", { name: "Data Model", exact: true }).click();
  await settle(page);
  await shot(page, "ide-compiled-data-model");
});
