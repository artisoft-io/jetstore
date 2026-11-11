// The Rules Unit Tests flow (user/screens/flows/rules-unit-tests), from Rules
// Unit Tests on the Workspaces screen: the runs of 20_workspace.sql, the checks
// of one, its log, and the page that selects the tests to start. No test is
// started. The test cases' Comment column is painted over: it is written for
// the workspace's developers, and quotes material that is not for this guide.
import { expect, test } from "@playwright/test";
import { openScreen, screenshotsFor, settle, signIn, waitForTable } from "../../lib/shots";

const { shot } = screenshotsFor("user/screens/flows/rules-unit-tests");

test("Rules Unit Tests flow", async ({ page }) => {
  await signIn(page);
  await openScreen(page, "Workspaces");
  const registry = await waitForTable(page);
  await registry.locator("tbody tr").filter({ hasText: "jets_ws" }).getByRole("checkbox").check();
  await page.getByRole("button", { name: "Rules Unit Tests", exact: true }).click();
  const runs = await waitForTable(page);
  await expect(runs.locator("tbody tr").filter({ hasText: "20003" })).toBeVisible();
  await shot(page, "runs");

  await runs.locator("tbody tr").filter({ hasText: "20003" }).getByRole("checkbox").check();
  await page.getByRole("button", { name: "View Log" }).click();
  const dialog = page.locator("dialog[open]");
  await expect(dialog).toBeVisible();
  await settle(page);
  await shot(dialog, "log-dialog");
  await dialog.getByRole("button", { name: "Close" }).click();
  await expect(dialog).toHaveCount(0);

  await page.getByRole("button", { name: "View Result" }).click();
  await expect(page.getByRole("button", { name: "Back", exact: true })).toBeVisible();
  await waitForTable(page);
  await shot(page, "run-checks");
  await page.getByRole("button", { name: "Back", exact: true }).click();
  await waitForTable(page);

  await page.getByRole("button", { name: "+ Add" }).click();
  await expect(page.getByRole("button", { name: "Start Test(s)" })).toBeVisible();
  const cases = await waitForTable(page);
  const commentColumn = await cases.locator("thead th").evaluateAll((ths) =>
    ths.findIndex((th) => th.textContent?.trim().startsWith("Comment")),
  );
  expect(commentColumn).toBeGreaterThan(0);
  await shot(page, "select-tests", {
    mask: [cases.locator(`tbody tr td:nth-child(${commentColumn + 1})`)],
  });
  await page.getByRole("button", { name: "Cancel", exact: true }).click();
  await waitForTable(page);
});
