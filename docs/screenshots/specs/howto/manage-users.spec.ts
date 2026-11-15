// user/how-to/manage-users: activate an account and give it roles, and delete
// an account.
//
// The account is registered in the spec, through the sign-in page's Create
// one, as its owner would, since registering is the page's first step. Its
// password is generated here, typed into no image and kept nowhere. The page's
// last step deletes the account, so the stack ends as it began.
import { randomBytes } from "node:crypto";
import { expect, test } from "@playwright/test";
import { openFlowMenu, screenshotsFor, signIn, waitForTable } from "../../lib/shots";
import { hideRunValues, selectRow, undo } from "./support";

const { shot } = screenshotsFor("user/how-to/manage-users");
const NAME = "Robin Ashby";
const EMAIL = "robin.ashby@localhost.test";

test("Manage users", async ({ page }) => {
  // The person registers.
  await page.goto("/");
  await page.getByRole("link", { name: "Create one" }).click();
  await page.getByRole("textbox", { name: "Name" }).fill(NAME);
  await page.getByRole("textbox", { name: "Email" }).fill(EMAIL);
  await shot(page, "register");
  // The server refuses a password without a digit, an upper-case letter, a
  // lower-case letter and a symbol (user.Validate), and the form does not check
  // that first: about one random string in sixty has no digit, and the account
  // was then not created. The suffix meets every class.
  const password = `${randomBytes(18).toString("base64url")}Aa1!`;
  await page.getByRole("textbox", { name: "Password", exact: true }).fill(password);
  await page.getByRole("textbox", { name: "Confirm password" }).fill(password);
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page.getByText("Registration successful")).toBeVisible();

  // The administrator activates the account and gives it a role.
  await signIn(page);
  await openFlowMenu(page, "User Administration");
  const users = await waitForTable(page);
  await selectRow(page, EMAIL);
  await hideRunValues(page);
  await shot(page, "registered-users");

  await page.getByRole("button", { name: "Update User Profile" }).click();
  const dialog = page.getByRole("dialog", { name: "Update User Profile" });
  await expect(dialog).toBeVisible();
  await dialog.getByRole("combobox", { name: "Active User?" }).selectOption("Active");
  await selectRow(dialog, "ops_user");
  await hideRunValues(page);
  await shot(page, "update-user-profile");
  await dialog.getByRole("button", { name: "Submit" }).click();
  await expect(dialog).toBeHidden();
  const row = users.getByRole("row").filter({ hasText: EMAIL });
  await expect(row.getByRole("cell", { name: "1", exact: true })).toBeVisible();
  await hideRunValues(page);
  await shot(page, "user-active");

  // Delete the account.
  await selectRow(page, EMAIL);
  await page.getByRole("button", { name: "Delete User" }).click();
  await page.getByRole("button", { name: "OK", exact: true }).click();
  await expect(row).toHaveCount(0);
});

// Not part of the page: the account, if a failed step left it.
test.afterEach(async ({ page }) => {
  await undo(page, [`DELETE FROM jetsapi.users WHERE user_email = '${EMAIL}'`]);
});
