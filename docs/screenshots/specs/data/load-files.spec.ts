// The Load Files flow (loadFilesUF): user/screens/flows/load-files.
//
// It stops before Load Files & Done: loading needs the installation's bucket,
// and a load would add a row to File Loader Status that other images show.
import { expect, test } from "@playwright/test";
import { openFlowMenu, screenshotsFor, signIn, waitForTable } from "../../lib/shots";
import { press, selectRow } from "./steps";

const { shot } = screenshotsFor("user/screens/flows/load-files");

test("Load Files, from Source Mapping's Load Data, up to the load", async ({ page }) => {
  await signIn(page);
  await openFlowMenu(page, "Source Mapping");
  const sources = await waitForTable(page);
  await selectRow(sources, "CI_D1_hc:MedicalClaim");
  await press(page, "Load Data");

  // The flow opens at its second step, for the source chosen in Source Mapping.
  await expect(page).toHaveURL(/\/flow\/loadFilesUF/);
  const files = await waitForTable(page);
  await expect(files.locator("tbody tr")).toHaveCount(1);
  await expect(page.getByRole("button", { name: "Previous", exact: true })).toBeDisabled();
  await selectRow(files, "M021-M040_M007_M008_medical_claims.csv");
  await expect(page.getByRole("button", { name: "Load Files & Done", exact: true })).toBeEnabled();
  await shot(page, "select-files");
});
