// user/how-to/load-and-map-a-file: check a new file arrived, load it, check its
// columns and their mapping, and run its mapping pipeline.
//
// Not performed, because the stack has no S3 bucket: copying the file into the
// bucket, Load Files & Done (the loader reads the file from S3), and starting
// Map_Eligibility (see run-a-pipeline). The images after each show the result
// 00_base.sql seeded.
import { expect, test } from "@playwright/test";
import { openFlowMenu, openScreen, openTab, screenshotsFor, signIn, waitForTable } from "../../lib/shots";
import { hideRunValues, selectRow } from "./support";

const { shot } = screenshotsFor("user/how-to/load-and-map-a-file");

test("Load and map a file", async ({ page }) => {
  await signIn(page);

  // 1. The file is registered.
  await openScreen(page, "Home");
  await openTab(page, "Data Registry");
  await waitForTable(page);
  await selectRow(page, "1767603600101");
  await shot(page, "file-registered");

  // 2. Load it: Source Mapping, the source, Load Data, the file.
  await openFlowMenu(page, "Source Mapping");
  await waitForTable(page);
  await selectRow(page, "CI_D1_hc:Eligibility");
  await hideRunValues(page);
  await shot(page, "select-source");
  await page.getByRole("button", { name: "Load Data" }).click();
  await expect(page.getByRole("heading", { name: "Load Files", level: 1 })).toBeVisible();
  await waitForTable(page);
  await selectRow(page, "1767603600101");
  await expect(page.getByRole("button", { name: "Load Files & Done" })).toBeEnabled();
  await shot(page, "load-files");

  // 3. What the loader read.
  await openFlowMenu(page, "Source Mapping");
  await page.getByRole("button", { name: "Loader Status" }).click();
  await expect(page.getByRole("heading", { name: "File Loader Status" })).toBeVisible();
  await waitForTable(page);
  await shot(page, "file-loader-status");

  // 4. The source's mapping, against the file's columns.
  await openFlowMenu(page, "Source Mapping");
  await waitForTable(page);
  await selectRow(page, "CI_D1_hc:Eligibility");
  await page.getByRole("button", { name: "Next" }).click();
  await expect(page.getByText("File Mapping").first()).toBeVisible();
  await waitForTable(page);
  await hideRunValues(page);
  await shot(page, "file-mapping");

  // 5. The mapping pipeline's table, registered.
  await openScreen(page, "Home");
  await openTab(page, "Data Registry");
  await waitForTable(page);
  await selectRow(page, "1767604500201");
  await shot(page, "mapped-table-registered");
});
