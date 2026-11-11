// The example spec: Home's four tabs, for the contributors' page on screenshots
// (developer/contributing/screenshots). Copy it to start a spec of your own.
import { test } from "@playwright/test";
import { openScreen, openTab, screenshotsFor, signIn, waitForTable } from "../../lib/shots";

const { shot } = screenshotsFor("developer/contributing/screenshots");

test("Home, one image per tab", async ({ page }) => {
  await signIn(page);
  await openScreen(page, "Home");

  // Pipelines Status is the tab Home opens on.
  await waitForTable(page);
  await shot(page, "home-pipelines-status");

  await openTab(page, "Data Registry");
  await waitForTable(page);
  await shot(page, "home-data-registry");

  await openTab(page, "Session Registry");
  await waitForTable(page);
  await shot(page, "home-session-registry");

  // An image cropped to one element: the archived runs' table alone.
  await openTab(page, "Archived Pipelines");
  const table = await waitForTable(page);
  await shot(table, "home-archived-pipelines-table");
});
