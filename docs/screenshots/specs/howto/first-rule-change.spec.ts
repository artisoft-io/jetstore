// user/tutorials/first-rule-change: edit a rule in the Workspace IDE, compile
// it, and run its rules unit test.
//
// The edit is made in the stack's copy of jets_ws, a `git archive` export in a
// temporary directory, and saved as an override in the stack's database; the
// tutorial reverts it at the end.
//
// Not performed: Compile Workspace. A compile rewrites jets_ws's row on the
// Workspaces screen, which another page shows. The test therefore runs against
// the rules compiled when the stack started. The change keeps the test passing,
// so its result is the one the compiled change gives.
//
// The test run this spec starts is deleted after it, through the Query Tool,
// so the rules unit test pages show only their own.
import { expect, test, type Page } from "@playwright/test";
import { openScreen, screenshotsFor, settle, signIn, waitForTable } from "../../lib/shots";
import { FIRST_RUN_KEY, changeLine, deleteTestRunsSql, dropWorkspaceChangeSql, hideColumn, hideRunValues, openWorkspaceFile, saveFile, selectRow, undo } from "./support";

const { shot } = screenshotsFor("user/tutorials/first-rule-change");
const FOLDERS = ["unit_tests", "salience_shared_consequent"];
const RULE_FILE = "jet_rules/unit_tests/salience_shared_consequent/salience_shared_consequent.jr";
const TEST_FILE = "jet_rules/unit_tests/salience_shared_consequent/salience_shared_consequent.tst.json";
const BEFORE = "[UT_SSAL_20, s=5]:";
const AFTER = "[UT_SSAL_20, s=1]:";

test("Your first rule change", async ({ page }) => {
  await signIn(page);

  // 1. Open the rule file.
  await openWorkspaceFile(page, FOLDERS, "salience_shared_consequent.jr", "end");
  await expect(page.locator(".cm-line", { hasText: BEFORE })).toBeVisible();
  await shot(page, "rule-file");

  // 2. Change UT_SSAL_20's salience from 5 to 1, and save.
  await changeLine(page, BEFORE, AFTER);
  await expect(page.getByText("Modified", { exact: true })).toBeVisible();
  await shot(page, "rule-edited");
  await saveFile(page);

  // 3. Compile: Workspaces, jets_ws, Compile Workspace (not pressed; see above).
  await openScreen(page, "Workspaces");
  await waitForTable(page);
  await selectRow(page, "jets_ws");
  await expect(page.getByRole("button", { name: "Compile Workspace" })).toBeEnabled();
  await hideRunValues(page);
  await shot(page, "workspaces");

  // 4. Run the file's unit test.
  await page.getByRole("button", { name: "Rules Unit Tests" }).click();
  await expect(page.getByRole("heading", { name: "Rules Unit Test Runs" })).toBeVisible();
  await page.getByRole("button", { name: "+ Add" }).click();
  await expect(page.getByRole("heading", { name: "Select Rules Unit Tests" })).toBeVisible();
  const cases = await waitForTable(page);
  await selectRow(page, TEST_FILE);
  await hideColumn(cases, "Comment");
  await shot(page, "select-test");
  await page.getByRole("button", { name: "Start Test(s)" }).click();
  await expect(page.getByRole("heading", { name: "Rules Unit Test Runs" })).toBeVisible();
  const runs = page.locator(".jets-datatable").first();
  const run = runs.locator("tbody tr").filter({ hasText: String(FIRST_RUN_KEY) });
  await expect(async () => {
    await page.getByRole("button", { name: "Refresh" }).click();
    await expect(run).toContainText("pass", { timeout: 2_000 });
  }).toPass({ timeout: 60_000 });
  await settle(page);
  await selectRow(runs, String(FIRST_RUN_KEY));
  for (const column of ["Duration (ms)", "Load (ms)"]) await hideColumn(runs, column, String(FIRST_RUN_KEY));
  await hideRunValues(page);
  await shot(page, "test-runs");
  await page.getByRole("button", { name: "View Result" }).click();
  await expect(page.getByRole("heading", { name: "Checks of the Selected Run" })).toBeVisible();
  await waitForTable(page);
  await shot(page, "test-result");

  // 5. Put the file back: revert the change in the Workspace IDE.
  await openScreen(page, "Workspace IDE");
  await expect(page.getByRole("heading", { name: "Workspace Changes" })).toBeVisible();
  await waitForTable(page);
  await selectRow(page, RULE_FILE);
  await hideRunValues(page);
  await shot(page, "workspace-changes");
  await revert(page);
});

// Not part of the page, and run whether the steps passed or not: delete the
// test run, and the saved change if a failed step left it.
test.afterEach(async ({ page }) => {
  await undo(page, [...deleteTestRunsSql(), dropWorkspaceChangeSql(RULE_FILE)]);
});

/**
 * Delete/Revert Changes: the file is restored from the stack's pristine copy
 * of the workspace (the stash an installation takes at start-up, which the
 * stack takes in up.sh), and the change is deleted.
 */
async function revert(page: Page): Promise<void> {
  await openScreen(page, "Workspace IDE");
  await expect(page.getByRole("heading", { name: "Workspace Changes" })).toBeVisible();
  const changes = await waitForTable(page);
  await selectRow(page, RULE_FILE);
  await page.getByRole("button", { name: "Delete/Revert Changes" }).click();
  await page.getByRole("button", { name: "OK", exact: true }).click();
  await expect(changes.locator("tbody")).toContainText("No rows.");
  // The file is as it was before the change.
  await openWorkspaceFile(page, FOLDERS, "salience_shared_consequent.jr", "end");
  await expect(page.locator(".cm-line", { hasText: BEFORE })).toHaveCount(1);
}
