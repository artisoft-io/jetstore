// user/how-to/look-at-loaded-data: browse the rows of a loaded file and of a
// table a pipeline wrote. The data is 00_base.sql's.
import { expect, test } from "@playwright/test";
import { openScreen, openTab, screenshotsFor, signIn, waitForTable } from "../../lib/shots";
import { selectRow } from "./support";

const { shot } = screenshotsFor("user/how-to/look-at-loaded-data");

test("Look at loaded data", async ({ page }) => {
  await signIn(page);

  await openScreen(page, "Home");
  await openTab(page, "Data Registry");
  await waitForTable(page);
  await selectRow(page, "1767604500201");
  await shot(page, "data-registry");

  await page.getByRole("button", { name: "View Loaded Data" }).click();
  await expect(page.getByRole("heading", { name: "Table hc:Eligibility" })).toBeVisible();
  await waitForTable(page);
  await shot(page, "domain-table");

  await openScreen(page, "Home");
  await openTab(page, "Data Registry");
  await waitForTable(page);
  await selectRow(page, "1767603600103");
  await page.getByRole("button", { name: "View Loaded Data" }).click();
  await expect(page.getByRole("heading", { name: "Table CI_D1_hc:PharmacyClaim" })).toBeVisible();
  await waitForTable(page);
  await shot(page, "staging-table");
});
