// The Rules Unit Tests flow (user/screens/flows/rules-unit-tests), from Rules
// Unit Tests on the Workspaces screen: the runs of 20_workspace.sql, the checks
// of one, its log, and the page that selects the tests to start. No test is
// started.
import { expect, test } from "@playwright/test";
import { openScreen, screenshotsFor, settle, signIn, tick, waitForTable } from "../../lib/shots";

const { shot } = screenshotsFor("user/screens/flows/rules-unit-tests");

test("Rules Unit Tests flow", async ({ page }) => {
  await signIn(page);
  await openScreen(page, "Workspaces");
  const registry = await waitForTable(page);
  await tick(registry.locator("tbody tr").filter({ hasText: "jets_ws" }).getByRole("checkbox"));
  await page.getByRole("button", { name: "Rules Unit Tests", exact: true }).click();
  const runs = await waitForTable(page);
  await expect(runs.locator("tbody tr").filter({ hasText: "20003" })).toBeVisible();
  await shot(page, "runs");

  await tick(runs.locator("tbody tr").filter({ hasText: "20003" }).getByRole("checkbox"));
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
  await waitForTable(page);
  await shot(page, "select-tests");
  await page.getByRole("button", { name: "Cancel", exact: true }).click();
  await waitForTable(page);
});
