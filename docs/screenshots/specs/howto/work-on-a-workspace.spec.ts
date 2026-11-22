// user/how-to/work-on-a-workspace: edit a file, add a folder, compile, commit,
// pull, and revert a change.
//
// The edit is made in the stack's copy of jets_ws, a `git archive` export in a
// temporary directory, and saved as an override in the stack's database; the
// page's last step reverts it.
//
// The folder added, a runbook and its SQL, is uploaded with Upload folder and
// reverted at the end of its step, which deletes its files.
//
// Not performed:
//  - Compile Workspace: a compile rewrites jets_ws's row on the Workspaces
//    screen, which another page shows;
//  - Commit & Push: git access is off on the stack, so the dialog is filled and
//    cancelled;
//  - Pull Workspace: the stack's jets_ws has no git repository to pull from.
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { expect, test, type Page } from "@playwright/test";
import { openScreen, screenshotsFor, signIn, waitForTable } from "../../lib/shots";
import {
  changeLine,
  dismissNotice,
  dropWorkspaceChangeSql,
  hideRunValues,
  openWorkspaceFile,
  saveFile,
  selectRow,
  undo,
} from "./support";

const { shot } = screenshotsFor("user/how-to/work-on-a-workspace");
const FILE = "jet_rules/hc_model_rules.jr";
const BEFORE = "# Values comming from input record that might get overriden";
const AFTER = "# Values coming from input record that might get overridden";
const ADDED = ["runbooks/table_counts.rb.json", "runbooks/table_counts/counts.sql"];
const RUNBOOK = {
  format: 1,
  runbook: "table_counts",
  description: "Counts the workspaces this installation knows",
  steps: [{ id: "counts", type: "query", description: "Count the workspaces", returns: { workspaces: "integer" } }],
};

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

  // Add a folder: a runbook and its folder of SQL, uploaded to the workspace's
  // root from a folder named runbooks, so they keep their paths under it.
  const folder = test.info().outputPath("runbooks");
  mkdirSync(join(folder, "table_counts"), { recursive: true });
  writeFileSync(join(folder, "table_counts.rb.json"), JSON.stringify(RUNBOOK, null, 2) + "\n");
  writeFileSync(
    join(folder, "table_counts", "counts.sql"),
    "SELECT count(*) AS workspaces FROM jetsapi.workspace_registry\n",
  );
  await openScreen(page, "Workspace IDE");
  await expect(page.getByRole("heading", { name: "Workspace Changes" })).toBeVisible();
  await page.getByLabel("Folder to upload").setInputFiles(folder);
  const where = page.locator("dialog[open]");
  await expect(where).toBeVisible();
  await where.getByRole("textbox").fill("");
  await where.getByRole("button", { name: "OK" }).click();
  const confirm = page.getByRole("dialog", { name: "Upload 2 files to jets_ws?" });
  await expect(confirm).toBeVisible();
  await shot(confirm, "upload-confirm");
  await confirm.getByRole("button", { name: "OK" }).click();
  await expect(page.getByRole("status")).toContainText("Uploaded 2 files to jets_ws.");
  await dismissNotice(page);
  await page.getByRole("searchbox", { name: "Filter files" }).fill("table_counts");
  const added = await waitForTable(page);
  for (const file of ADDED) await expect(added.locator("tbody")).toContainText(file);
  await hideRunValues(page);
  await shot(page, "uploaded-files");
  await revertAdded(page);

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
  await undo(page, [FILE, ...ADDED].map(dropWorkspaceChangeSql));
});

/**
 * Delete/Revert Changes on the files the folder added: a file the workspace
 * did not have at start-up is deleted, and its change with it. Leaves the
 * changes table as it was before the upload.
 */
async function revertAdded(page: Page): Promise<void> {
  await page.getByRole("searchbox", { name: "Filter files" }).fill("");
  const changes = await waitForTable(page);
  for (const file of ADDED) await selectRow(page, file);
  await page.getByRole("button", { name: "Delete/Revert Changes" }).click();
  await page.getByRole("button", { name: "OK", exact: true }).click();
  for (const file of ADDED) await expect(changes.locator("tbody")).not.toContainText(file);
  await expect(page.locator("aside.sidebar")).not.toContainText("table_counts");
}

/**
 * Delete/Revert Changes: the file is restored from the stack's pristine copy
 * of the workspace (the stash an installation takes at start-up, which the
 * stack takes in up.sh), and the change is deleted.
 */
async function revert(page: Page): Promise<void> {
  await openScreen(page, "Workspace IDE");
  await expect(page.getByRole("heading", { name: "Workspace Changes" })).toBeVisible();
  const changes = await waitForTable(page);
  await selectRow(page, FILE);
  await page.getByRole("button", { name: "Delete/Revert Changes" }).click();
  await page.getByRole("button", { name: "OK", exact: true }).click();
  await expect(changes.locator("tbody")).toContainText("No rows.");
  // The file is as it was before the change.
  await openWorkspaceFile(page, [], "hc_model_rules.jr");
  await expect(page.locator(".cm-line", { hasText: BEFORE })).toHaveCount(1);
}
