// Process Errors and its two dialogs: user/screens/process-errors.
import { expect, test } from "@playwright/test";
import { screenshotsFor, signIn, waitForTable } from "../../lib/shots";
import { dialog, openRunScreen, press, scrollToLastColumns, selectRow } from "./steps";

const { shot } = screenshotsFor("user/screens/process-errors");

test("Process Errors of the run with errors, an error's input records and its rule session", async ({ page }) => {
  await signIn(page);
  await openRunScreen(page, "errors", "View Process Errors");
  const errors = await waitForTable(page);
  await expect(page).toHaveURL(/\/processErrors\/1767778200302$/);
  await expect(errors.locator("tbody tr")).toHaveCount(27);
  await selectRow(errors, "MED100007");
  // The error message is in the last columns.
  await scrollToLastColumns(errors);
  await shot(page, "process-errors");

  // The member's records in every input of the run, one table per input.
  await press(page, "View Input Records");
  await expect(dialog(page)).toContainText("Input Records for a Domain Key");
  await expect(dialog(page).getByText("Table hc:PharmacyClaim").first()).toBeVisible();
  await waitForTable(page, dialog(page).locator(".jets-datatable").first());
  await shot(dialog(page), "input-records");
  await press(dialog(page), "Close");

  // No rule session was saved for this error: the explorer opens empty.
  await press(page, "View Rule Session");
  await expect(dialog(page)).toContainText("Rete Session Explorer");
  await expect(dialog(page).getByText("No rows.")).toBeVisible();
  await press(dialog(page), "Close");
});
