// The Workspace IDE (user/screens/workspace-ide): reached from the top bar,
// with jets_ws picked. Its changes table, the New file prompt, a file open in
// the editor and edited,
// the compiled views of the data model and of the rules, the Documentation
// section with the guide's landing page open, and the page of the workspace
// guide that Workspace guide opens for a data-model file. The edit is never
// saved: the tab is closed and the change discarded.
import { expect, test } from "@playwright/test";
import { openScreen, openTab, screenshotsFor, settle, signIn, waitForTable } from "../../lib/shots";

const { shot } = screenshotsFor("user/screens/workspace-ide");

test("Workspace IDE", async ({ page }) => {
  await signIn(page);
  await openScreen(page, "Workspace IDE");
  await expect(page.locator(".ws-picker select")).toHaveValue("jets_ws");
  await settle(page);
  await shot(page, "ide-changes");

  // New file asks for the path of the file to add; cancelled, nothing is added.
  await page.getByRole("button", { name: "New file", exact: true }).click();
  const prompt = page.locator("dialog[open]");
  await expect(prompt).toBeVisible();
  await prompt.getByRole("textbox").fill("runbooks/my_runbook.rb.json");
  await shot(prompt, "ide-new-file");
  await prompt.getByRole("button", { name: "Cancel" }).click();
  await expect(prompt).toHaveCount(0);

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

  // The rules' compiled view: each rule with the main rule file it is compiled
  // under and the file it is written in, the page's last two columns.
  await tree.getByRole("button", { name: "Jets Rules", exact: true }).click();
  await openTab(page, "Jet Rules");
  const rules = await waitForTable(page);
  await expect(rules.getByRole("columnheader", { name: "Main Rule File", exact: true })).toBeInViewport();
  await expect(rules.getByRole("columnheader", { name: "File", exact: true })).toBeInViewport();
  await shot(page, "ide-compiled-jet-rules");

  // The Files Relationship tab: each main rule file with the files it includes.
  await openTab(page, "Files Relationship");
  const files = await waitForTable(page, page.locator(".jets-datatable").filter({ visible: true }).first());
  await expect(files.getByRole("heading", { name: "Files Relationship", exact: true })).toBeVisible();
  await shot(page, "ide-compiled-files-relationship");

  // The Documentation section lists the workspace's docs/ folder; a page opens
  // in a tab like any other file.
  const expand = tree.getByRole("button", { name: "Expand Documentation", exact: true });
  if (await expand.isVisible()) await expand.click();
  await tree.getByTitle("index.md", { exact: true }).first().click();
  await expect(page.getByRole("button", { name: "Close docs/index.md" })).toBeVisible();
  await expect(page.locator(".cm-content")).toBeVisible();
  await settle(page);
  await shot(page, "ide-documentation");

  // Workspace guide, beside Save, opens the guide's page for the file open in
  // the editor, in the documentation's tab: for a data-model file, its page of
  // the domain. The stack's documentation site is built with jets_ws's guide,
  // as an installation's is (stack/up.sh).
  await tree.getByTitle("hc_model.jr", { exact: true }).click();
  await expect(page.getByRole("tab", { name: /hc_model\.jr/ })).toBeVisible();
  const guideTab = page.waitForEvent("popup");
  await page.getByRole("button", { name: "Workspace guide", exact: true }).click();
  const guide = await guideTab;
  await guide.waitForURL(/\/docs\/workspace\/reference\/domain\/hc_model\/$/);
  await guide.waitForLoadState("networkidle");
  await expect(guide.getByRole("heading", { level: 1 })).toBeVisible();
  await shot(guide, "workspace-guide-page");
  await guide.close();
});
