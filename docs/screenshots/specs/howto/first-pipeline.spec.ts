// user/tutorials/first-pipeline: from a new file to a pipeline's output.
//
// Not performed, because the stack has no S3 bucket: the file landing in the
// bucket, Load Files & Done (the loader reads the file from S3), and Start
// Pipeline & Done (a run reads and writes S3). The images after each show the
// result 00_base.sql seeded.
import { expect, test } from "@playwright/test";
import { openFlowMenu, openScreen, openTab, screenshotsFor, signIn, waitForTable } from "../../lib/shots";
import { selectRow } from "./support";

const { shot } = screenshotsFor("user/tutorials/first-pipeline");

test("Your first pipeline", async ({ page }) => {
  await signIn(page);

  // 1. The file has arrived and is registered.
  await openScreen(page, "Home");
  await openTab(page, "Data Registry");
  await waitForTable(page);
  await shot(page, "data-registry");

  // 2. Load it into its staging table: Source Mapping, its source, Load Data.
  await openFlowMenu(page, "Source Mapping");
  await waitForTable(page);
  await selectRow(page, "CI_D1_hc:Eligibility");
  await page.getByRole("button", { name: "Load Data" }).click();
  await expect(page.getByRole("heading", { name: "Load Files", level: 1 })).toBeVisible();
  await waitForTable(page);
  await selectRow(page, "1767603600101");
  await expect(page.getByRole("button", { name: "Load Files & Done" })).toBeEnabled();
  await shot(page, "load-files");

  // 3. What the loader read: Loader Status, then the rows themselves.
  await openFlowMenu(page, "Source Mapping");
  await page.getByRole("button", { name: "Loader Status" }).click();
  await expect(page.getByRole("heading", { name: "File Loader Status" })).toBeVisible();
  await waitForTable(page);
  await selectRow(page, "1767603600101");
  await shot(page, "file-loader-status");
  await page.getByRole("button", { name: "View Loaded Data" }).click();
  await expect(page.getByRole("heading", { name: "Table CI_D1_hc:Eligibility" })).toBeVisible();
  await waitForTable(page);
  await shot(page, "staging-table");

  // 4. Start Map_Eligibility on the file.
  await openScreen(page, "Home");
  await waitForTable(page);
  await page.getByRole("button", { name: "Start Pipeline" }).click();
  await expect(page.getByRole("heading", { name: "Start Pipeline", level: 1 })).toBeVisible();
  await waitForTable(page);
  await selectRow(page, "Map_Eligibility");
  await page.getByRole("button", { name: "Next" }).click();
  await expect(page.getByText("Select the main data source")).toBeVisible();
  await waitForTable(page);
  await selectRow(page, "1767603600101");
  await page.getByRole("button", { name: "Next" }).click();
  await expect(page.getByRole("heading", { name: "Run Pipeline Summary" })).toBeVisible();
  await waitForTable(page);
  await shot(page, "start-pipeline-summary");
  await page.getByRole("button", { name: "Cancel" }).click();

  // 5. Watch the run on Pipelines Status, and read the table it wrote.
  await openScreen(page, "Home");
  await waitForTable(page);
  await selectRow(page, "Map_Eligibility");
  await shot(page, "pipelines-status");

  await openTab(page, "Data Registry");
  await waitForTable(page);
  await selectRow(page, "1767604500201");
  await page.getByRole("button", { name: "View Loaded Data" }).click();
  await expect(page.getByRole("heading", { name: "Table hc:Eligibility" })).toBeVisible();
  await waitForTable(page);
  await shot(page, "mapped-table");

  // 6. Patient_Profile_Template, which started by itself: its steps.
  await openScreen(page, "Home");
  await waitForTable(page);
  await selectRow(page, "1767608400301");
  await page.getByRole("button", { name: "View Execution Details" }).click();
  await expect(page.getByRole("heading", { name: "Pipeline Execution Details" })).toBeVisible();
  await waitForTable(page);
  await shot(page, "execution-details");

  // 7. Its output: the profiles, in the Query Tool.
  await openScreen(page, "Query Tool");
  await page.getByRole("textbox", { name: "Query" }).fill(
    [
      'SELECT "hc:Member_ID", "hc:Member_First_Name", "hc:Member_Last_Name", "hc:Member_Gender",',
      '       left("cintel:Claim_Summary", 70) AS claim_summary',
      'FROM "cintel:Patient_Profile"',
      "WHERE session_id = '1767608400301'",
      'ORDER BY "hc:Member_ID"',
    ].join("\n"),
  );
  await page.getByRole("button", { name: "Submit Query" }).click();
  await waitForTable(page);
  await expect(page.locator("main tbody tr")).toHaveCount(22);
  await shot(page, "patient-profiles");
});
