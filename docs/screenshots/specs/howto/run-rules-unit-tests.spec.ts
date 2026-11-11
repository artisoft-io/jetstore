// user/how-to/run-rules-unit-tests: start a workspace's rules unit tests and
// read their results.
//
// The tests run against the rules compiled when the stack started. The runs
// this spec starts are deleted after it, through the Query Tool, so the
// rules unit test pages show only their own.
import { expect, test } from "@playwright/test";
import { openScreen, screenshotsFor, settle, signIn, waitForTable } from "../../lib/shots";
import { FIRST_RUN_KEY, deleteTestRunsSql, hideColumn, hideRunValues, selectRow, undo } from "./support";

const { shot } = screenshotsFor("user/how-to/run-rules-unit-tests");
const TESTS = [
  "jet_rules/unit_tests/retraction_defect1/retraction_defect1.tst.json",
  "jet_rules/unit_tests/salience_shared_consequent/salience_shared_consequent.tst.json",
];

test("Run rules unit tests", async ({ page }) => {
  await signIn(page);

  await openScreen(page, "Workspaces");
  await waitForTable(page);
  await selectRow(page, "jets_ws");
  await page.getByRole("button", { name: "Rules Unit Tests" }).click();
  await expect(page.getByRole("heading", { name: "Rules Unit Test Runs" })).toBeVisible();

  // Select the tests.
  await page.getByRole("button", { name: "+ Add" }).click();
  await expect(page.getByRole("heading", { name: "Select Rules Unit Tests" })).toBeVisible();
  const cases = await waitForTable(page);
  for (const file of TESTS) await selectRow(page, file);
  await hideColumn(cases, "Comment");
  await shot(page, "select-tests");

  // Start them, and wait for both to finish.
  await page.getByRole("button", { name: "Start Test(s)" }).click();
  await expect(page.getByRole("heading", { name: "Rules Unit Test Runs" })).toBeVisible();
  const runs = page.locator(".jets-datatable").first();
  const keys = [FIRST_RUN_KEY, FIRST_RUN_KEY + 1].map(String);
  await expect(async () => {
    await page.getByRole("button", { name: "Refresh" }).click();
    for (const key of keys) {
      await expect(runs.locator("tbody tr").filter({ hasText: key })).toContainText(/pass|fail|error/, { timeout: 2_000 });
    }
  }).toPass({ timeout: 60_000 });
  await settle(page);
  for (const key of keys) {
    for (const column of ["Duration (ms)", "Load (ms)"]) await hideColumn(runs, column, key);
  }
  await hideRunValues(page);
  await shot(page, "test-runs");

  // One run's checks, and its log.
  const retraction = runs
    .locator("tbody tr")
    .filter({ hasText: "retraction defect 1" })
    .filter({ hasText: new RegExp(keys.join("|")) });
  await retraction.getByRole("checkbox").check();
  await settle(page);
  await page.getByRole("button", { name: "View Result" }).click();
  await expect(page.getByRole("heading", { name: "Checks of the Selected Run" })).toBeVisible();
  await waitForTable(page);
  await shot(page, "test-result");
  await page.getByRole("button", { name: "Back" }).click();
  await expect(page.getByRole("heading", { name: "Rules Unit Test Runs" })).toBeVisible();
  await waitForTable(page);
  await retraction.getByRole("checkbox").check();
  await settle(page);
  await page.getByRole("button", { name: "View Log" }).click();
  const log = page.getByRole("dialog", { name: "Rules Unit Test Log" });
  await expect(log.getByRole("textbox", { name: "Log" })).toHaveValue(/: pass/);
  await log.getByRole("button", { name: "Close" }).click();
  await expect(log).toBeHidden();
});

// Not part of the page, and run whether the steps passed or not.
test.afterEach(async ({ page }) => {
  await undo(page, deleteTestRunsSql());
});
