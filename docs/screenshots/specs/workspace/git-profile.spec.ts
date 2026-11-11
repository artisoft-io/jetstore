// The Git Profile screen (user/screens/git-profile), reached from the user's
// name in the top bar. The form is filled with invented values and not
// submitted; no token is typed.
import { expect, test } from "@playwright/test";
import { screenshotsFor, settle, signIn } from "../../lib/shots";

const { shot } = screenshotsFor("user/screens/git-profile");

test("Git Profile", async ({ page }) => {
  await signIn(page);
  await page.getByRole("link", { name: "Admin", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Edit Git Profile" })).toBeVisible();
  await page.locator("#gitName").fill("Admin");
  await page.locator("#gitHandle").fill("ci-admin");
  await page.locator("#gitEmail").fill("admin@localhost.test");
  await settle(page);
  await shot(page, "git-profile");
});
