// The Pull Workspace Changes flow (user/screens/flows/pull-workspace-changes),
// from Pull Workspace on the Workspaces screen: the options, the clients, and
// the confirmation page. The stack has git access off, so the flow is left
// with Cancel rather than Confirm.
import { expect, test } from "@playwright/test";
import { openScreen, screenshotsFor, settle, signIn, tick, waitForTable } from "../../lib/shots";

const { shot } = screenshotsFor("user/screens/flows/pull-workspace-changes");

test("Pull Workspace Changes flow", async ({ page }) => {
  await signIn(page);
  await openScreen(page, "Workspaces");
  const registry = await waitForTable(page);
  await tick(registry.locator("tbody tr").filter({ hasText: "jets_ws" }).getByRole("checkbox"));
  await page.getByRole("button", { name: "Pull Workspace", exact: true }).click();
  const options = await waitForTable(page);
  await shot(page, "pull-options");

  await tick(options.locator("tbody tr").filter({ hasText: "Compile workspace" }).getByRole("checkbox"));
  await tick(options.locator("tbody tr").filter({ hasText: "Load SELECTED client configurations" }).getByRole("checkbox"));
  await settle(page);
  await page.getByRole("button", { name: "Next", exact: true }).click();
  const clients = await waitForTable(page);
  await tick(clients.locator("tbody tr").filter({ hasText: "Clinical Intel" }).getByRole("checkbox"));
  await settle(page);
  await shot(page, "select-clients");

  await page.getByRole("button", { name: "Next", exact: true }).click();
  await expect(page.getByRole("button", { name: "Confirm", exact: true })).toBeVisible();
  await settle(page);
  await shot(page, "confirm");

  await page.getByRole("button", { name: "Cancel", exact: true }).click();
});
