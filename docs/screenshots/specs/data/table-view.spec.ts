// Staging Table or Domain Table View: user/screens/table-view.
import { expect, test } from "@playwright/test";
import { openScreen, openTab, screenshotsFor, signIn, waitForTable } from "../../lib/shots";
import { press, selectRow } from "./steps";

const { shot } = screenshotsFor("user/screens/table-view");

test("A staging table and a domain table, from the Data Registry", async ({ page }) => {
  await signIn(page);
  await openScreen(page, "Home");
  await openTab(page, "Data Registry");
  let registry = await waitForTable(page);

  // The eligibility file, as it loaded into its staging table.
  await selectRow(registry, "CI_D1_hc:Eligibility");
  await press(page, "View Loaded Data");
  await waitForTable(page);
  await expect(page).toHaveURL(/\/domainTableViewer\/CI_D1_hc(:|%3A)Eligibility\/1767603600101$/);
  await expect(page.getByText("1–10 of 22")).toBeVisible();
  await shot(page, "staging-table");

  // The same members, after Map_Eligibility mapped them into hc:Eligibility.
  await page.goBack();
  registry = await waitForTable(page);
  await selectRow(registry, "domain_table", "hc:Eligibility", "1767604500201");
  await press(page, "View Loaded Data");
  await waitForTable(page);
  await expect(page.getByText("Table hc:Eligibility")).toBeVisible();
  await shot(page, "domain-table");
});
