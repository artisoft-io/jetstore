// user/how-to/read-process-errors: find the errors a run raised, and the
// records behind one of them; the records are not shown, being mostly the
// data set's empty columns. The run and its errors are 00_base.sql's.
import { expect, test } from "@playwright/test";
import { openScreen, screenshotsFor, signIn, waitForTable } from "../../lib/shots";
import { selectRow } from "./support";

const { shot } = screenshotsFor("user/how-to/read-process-errors");

test("Read a run's process errors", async ({ page }) => {
  await signIn(page);

  await openScreen(page, "Home");
  await waitForTable(page);
  await selectRow(page, "1767778200302");
  await shot(page, "run-with-errors");

  await page.getByRole("button", { name: "View Process Errors" }).click();
  await expect(page.getByRole("heading", { name: "Pipeline Execution Errors", level: 1 })).toBeVisible();
  await waitForTable(page);
  await shot(page, "process-errors");

  await selectRow(page, "MED100007");
  await page.getByRole("button", { name: "View Input Records" }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByRole("heading", { name: "Table hc:Eligibility" })).toBeVisible();
  await waitForTable(page, dialog.locator(".jets-datatable").first());
});
