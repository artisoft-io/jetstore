// CPIPES Execution Details: user/screens/cpipes-execution-details.
import { expect, test } from "@playwright/test";
import { screenshotsFor, signIn, waitForTable } from "../../lib/shots";
import { openRunScreen } from "./steps";

const { shot } = screenshotsFor("user/screens/cpipes-execution-details");

test("CPIPES Execution Details of the completed run", async ({ page }) => {
  await signIn(page);
  await openRunScreen(page, "completed", "View Execution Stats");
  const steps = await waitForTable(page);
  await expect(page).toHaveURL(/\/executionStatsDetails\/1767608400301$/);
  await expect(steps.locator("tbody tr")).toHaveCount(7);
  await shot(page, "completed-run");
});
