// The Load Client Configurations flow (loadConfigUF):
// user/screens/flows/load-client-configurations.
//
// It stops before Confirm: loading replaces the client's configuration in the
// database, which every other image is drawn from.
import { expect, test } from "@playwright/test";
import { openScreen, screenshotsFor, signIn, waitForTable } from "../../lib/shots";
import { press, selectRow } from "./steps";

const { shot } = screenshotsFor("user/screens/flows/load-client-configurations");

test("Load Client Configurations for CI, from the Workspaces screen, up to Confirm", async ({ page }) => {
  await signIn(page);
  await openScreen(page, "Workspaces");
  const workspaces = await waitForTable(page);
  await selectRow(workspaces, "jets_ws");
  await press(page, "Load Client Config");
  await expect(page).toHaveURL(/\/flow\/loadConfigUF\?workspace_name=jets_ws/);

  // 1. The clients.
  const clients = await waitForTable(page);
  await selectRow(clients, "Clinical Intel");
  await shot(page, "select-clients");
  await press(page, "Next");

  // 2. The confirmation.
  await waitForTable(page);
  await expect(page.getByText("Clients Selected")).toBeVisible();
  await expect(page.getByRole("button", { name: "Confirm", exact: true })).toBeEnabled();
});
