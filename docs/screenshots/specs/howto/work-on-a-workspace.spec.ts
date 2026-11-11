// user/how-to/work-on-a-workspace: edit a file, compile, commit, pull, and
// revert a change.
//
// The edit is made in the stack's copy of jets_ws, a `git archive` export in a
// temporary directory, and saved as an override in the stack's database; the
// page's last step reverts it.
//
// Not performed:
//  - Compile Workspace: the apiserver's compile runs update_db from
//    /usr/local/bin rather than from the stack's binaries, and a compile
//    rewrites jets_ws's row on the Workspaces screen, which another page shows;
//  - Commit & Push: git access is off on the stack, so the dialog is filled and
//    cancelled;
//  - Pull Workspace: the stack's jets_ws has no git repository to pull from.
import { expect, test, type Page } from "@playwright/test";
import { openScreen, screenshotsFor, signIn, waitForTable } from "../../lib/shots";
import { changeLine, dropWorkspaceChangeSql, hideRunValues, openWorkspaceFile, saveFile, selectRow, undo } from "./support";

const { shot } = screenshotsFor("user/how-to/work-on-a-workspace");
const FILE = "jet_rules/hc_model_rules.jr";
const BEFORE = "# Values comming from input record that might get overriden";
const AFTER = "# Values coming from input record that might get overridden";

test("Work on a workspace", async ({ page }) => {
  await signIn(page);

  // Edit and save a file.
  await openWorkspaceFile(page, [], "hc_model_rules.jr");
  await changeLine(page, BEFORE, AFTER);
  await expect(page.getByText("Modified", { exact: true })).toBeVisible();
  await shot(page, "edit-file");
  await saveFile(page);

  // The saved file is a change of the workspace.
  await page.getByRole("button", { name: `Close ${FILE}` }).click();
  await expect(page.getByRole("heading", { name: "Workspace Changes" })).toBeVisible();
  await waitForTable(page);
  await selectRow(page, FILE);
  await hideRunValues(page);
  await shot(page, "workspace-changes");

  // Compile, then commit: both from the Workspaces screen.
  await openScreen(page, "Workspaces");
  await waitForTable(page);
  await selectRow(page, "jets_ws");
  await expect(page.getByRole("button", { name: "Compile Workspace" })).toBeEnabled();
  await page.getByRole("button", { name: "Commit & Push Workspace" }).click();
  const dialog = page.getByRole("dialog", { name: "Commit and Push Workspace Changes to Repository" });
  await expect(dialog).toBeVisible();
  await dialog.getByRole("textbox", { name: "Commit Message" }).fill("Correct the spelling of a comment in hc_model_rules.jr");
  await hideRunValues(page);
  await shot(page, "commit-dialog");
  await dialog.getByRole("button", { name: "Cancel" }).click();
  await expect(dialog).toBeHidden();
  await expect(page.getByRole("button", { name: "Pull Workspace" })).toBeEnabled();

  // Revert the change.
  await revert(page);
});

// Not part of the page: the saved change, if a failed step left it.
test.afterEach(async ({ page }) => {
  await undo(page, [dropWorkspaceChangeSql(FILE)]);
});

/**
 * Delete/Revert Changes. The stack keeps no pristine copy of the workspace
 * (the stash an installation takes at start-up), so on the stack a revert
 * deletes the change and leaves the file as saved; the spec first saves the
 * original line back, so the file ends as it began.
 */
async function revert(page: Page): Promise<void> {
  await openWorkspaceFile(page, [], "hc_model_rules.jr");
  await changeLine(page, AFTER, BEFORE);
  await saveFile(page);
  await page.getByRole("button", { name: `Close ${FILE}` }).click();
  const changes = await waitForTable(page);
  await selectRow(page, FILE);
  await page.getByRole("button", { name: "Delete/Revert Changes" }).click();
  await page.getByRole("button", { name: "OK", exact: true }).click();
  await expect(changes.locator("tbody")).toContainText("No rows.");
}
