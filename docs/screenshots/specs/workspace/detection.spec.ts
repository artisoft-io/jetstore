// The Detection screen (user/screens/detection): the two detection tasks of
// 20_workspace.sql, and the shadow triage census of one, opened. The stack has
// no detection task, so Run detection now is disabled.
import { expect, test } from "@playwright/test";
import { openScreen, screenshotsFor, settle, signIn } from "../../lib/shots";

const { shot } = screenshotsFor("user/screens/detection");

test("Detection", async ({ page }) => {
  await signIn(page);
  await openScreen(page, "Detection");
  await expect(page.getByRole("button", { name: "Run detection now" })).toBeDisabled();
  await expect(page.locator("tbody tr").first()).toBeVisible();
  await settle(page);
  await shot(page, "detection");

  await page.locator("button.detection-toggle").first().click();
  await settle(page);
  await shot(page, "detection-triage-open");
});
