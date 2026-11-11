// The sign-in and registration forms (user/screens/sign-in). Both are taken
// before anyone signs in, and no password is typed into either.
import { expect, test } from "@playwright/test";
import { screenshotsFor, settle } from "../../lib/shots";

const { shot } = screenshotsFor("user/screens/sign-in");

test("sign-in and registration forms", async ({ page }) => {
  await page.addInitScript(() => {
    try {
      localStorage.setItem("jetstore-ide-theme", "light");
    } catch {
      /* the default theme is light */
    }
  });
  await page.goto("/");
  const form = page.locator("form.login-card");
  await expect(form.getByRole("button", { name: "Sign in" })).toBeVisible();
  await shot(form, "sign-in-form");

  // "Create one" leads to the registration form.
  await page.getByRole("link", { name: "Create one" }).click();
  await expect(page.getByRole("button", { name: "Create account" })).toBeVisible();
  await page.locator("#reg-name").fill("Sam Ortiz");
  await page.locator("#reg-email").fill("sam.ortiz@localhost.test");
  await settle(page);
  await shot(page.locator("form.login-card"), "register-form");

  // "Back to sign in" returns to the sign-in form; nothing is submitted.
  await page.getByRole("button", { name: "Back to sign in" }).click();
  await expect(page.getByRole("button", { name: "Sign in" })).toBeVisible();
});
