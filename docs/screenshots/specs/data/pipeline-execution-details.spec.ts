// Pipeline Execution Details: user/screens/pipeline-execution-details.
import { expect, test } from "@playwright/test";
import { screenshotsFor, signIn, waitForTable } from "../../lib/shots";
import { openRunScreen } from "./steps";

const { shot } = screenshotsFor("user/screens/pipeline-execution-details");

test("Pipeline Execution Details of the failed run", async ({ page }) => {
  await signIn(page);
  await openRunScreen(page, "failed", "View Execution Details");
  const steps = await waitForTable(page);
  await expect(page).toHaveURL(/\/executionStatusDetails\/1767868200303$/);
  await expect(steps.locator("tbody tr")).toHaveCount(4);
  await shot(page, "failed-run");
});
