// The File Mapping Configuration flow (mapFileUF), the mapping worksheet:
// user/screens/flows/file-mapping-configuration.
//
// It leaves with Cancel: saving rewrites the source's mapping rows, which the
// Source Mapping images show.
import { expect, test } from "@playwright/test";
import { openFlowMenu, screenshotsFor, signIn, waitForTable } from "../../lib/shots";
import { press, selectRow } from "./steps";

const { shot } = screenshotsFor("user/screens/flows/file-mapping-configuration");

test("The mapping worksheet of the eligibility source", async ({ page }) => {
  await signIn(page);
  await openFlowMenu(page, "Source Mapping");
  const sources = await waitForTable(page);
  await selectRow(sources, "CI_D1_hc:Eligibility");
  await press(page, "Next");
  await waitForTable(page);
  await press(page, "Configure Mapping");

  await expect(page).toHaveURL(/\/flow\/mapFileUF\?table_name=CI_D1_hc%3AEligibility&object_type=hc%3AEligibility/);
  await expect(page.getByText("File Mapping Worksheet")).toBeVisible();
  await expect(page.getByText("hc:Member_ID*", { exact: true })).toBeVisible();
  await shot(page, "worksheet");

  // hc:Effective_Date is required. Emptied, its error shows, and then only
  // Save as Draft can save the worksheet.
  const effectiveDate = page.getByLabel("Input Column").first();
  await effectiveDate.fill("x");
  await effectiveDate.fill("");
  await effectiveDate.press("Tab");
  await expect(page.getByText("Input Column must be selected or either a default or an error message must be provided.")).toBeVisible();
  await expect(page.getByRole("button", { name: "Save", exact: true })).toBeDisabled();
  await expect(page.getByRole("button", { name: "Save as Draft", exact: true })).toBeEnabled();

  await press(page, "Cancel");
  await expect(page).toHaveURL(/\/flow\/fileMappingUF/);
});
