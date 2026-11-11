// Home, its four tabs and its dialogs: user/screens/home.
import { expect, test } from "@playwright/test";
import { openScreen, openTab, screenshotsFor, signIn, waitForTable } from "../../lib/shots";
import { dialog, press, scrollToLastColumns, selectRow } from "./steps";

const { shot } = screenshotsFor("user/screens/home");

test("Home: the four tabs, a run's failure details and the session filters", async ({ page }) => {
  await signIn(page);
  await openScreen(page, "Home");

  // Pipelines Status, the tab Home opens on: every run, the latest first.
  const runs = await waitForTable(page);
  await expect(runs.locator("tbody tr")).toHaveCount(9);
  await shot(page, "pipelines-status");

  // The failed run's failure details.
  await selectRow(runs, "Patient_Profile_Template", "failed");
  await press(page, "View Failure Details");
  await expect(dialog(page)).toContainText("Pipeline Failure Details");
  await shot(dialog(page), "failure-details");
  await press(dialog(page), "Close");
  await expect(dialog(page)).toHaveCount(0);

  // Data Registry: the three files and the three tables mapped from them.
  await openTab(page, "Data Registry");
  const registry = await waitForTable(page);
  await expect(registry.locator("tbody tr")).toHaveCount(6);
  await shot(page, "data-registry");

  // Session Registry, and its filter dialog, cancelled.
  await openTab(page, "Session Registry");
  await waitForTable(page);
  await shot(page, "session-registry");
  await press(page, "Set Filters");
  await expect(dialog(page)).toContainText("Session Registry Filters");
  await shot(dialog(page), "session-registry-filters");
  await press(dialog(page), "Cancel");

  // Archived Pipelines.
  // Its last columns say when and why each run was archived.
  await openTab(page, "Archived Pipelines");
  const archived = await waitForTable(page);
  await scrollToLastColumns(archived);
  await shot(page, "archived-pipelines");
});
