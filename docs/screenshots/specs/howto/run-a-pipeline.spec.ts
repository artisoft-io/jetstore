// user/how-to/run-a-pipeline: start a pipeline, watch it, and run it again.
//
// Not performed, because the stack has no S3 bucket: Start Pipeline & Done and
// Resubmit, which start a run that reads and writes S3. The spec goes up to
// each button; the images after show the runs 00_base.sql seeded.
import { expect, test } from "@playwright/test";
import { openScreen, screenshotsFor, signIn, waitForTable } from "../../lib/shots";
import { hideRunValues, selectRow } from "./support";

const { shot } = screenshotsFor("user/how-to/run-a-pipeline");

test("Run a pipeline", async ({ page }) => {
  await signIn(page);

  // Start: Start Pipeline on Pipelines Status, then the flow's three steps.
  await openScreen(page, "Home");
  await waitForTable(page);
  await page.getByRole("button", { name: "Start Pipeline" }).click();
  await expect(page.getByRole("heading", { name: "Start Pipeline", level: 1 })).toBeVisible();
  await waitForTable(page);
  await selectRow(page, "Map_Eligibility");
  await hideRunValues(page);
  await shot(page, "select-pipeline-configuration");
  await page.getByRole("button", { name: "Next" }).click();
  await expect(page.getByText("Select the main data source")).toBeVisible();
  await waitForTable(page);
  await selectRow(page, "1767603600101");
  await shot(page, "select-main-input");
  await page.getByRole("button", { name: "Next" }).click();
  await expect(page.getByRole("heading", { name: "Run Pipeline Summary" })).toBeVisible();
  await waitForTable(page);
  await expect(page.getByRole("button", { name: "Start Pipeline & Done" })).toBeEnabled();
  await page.getByRole("button", { name: "Cancel" }).click();

  // Watch: Pipelines Status, and a run's details.
  await openScreen(page, "Home");
  await waitForTable(page);
  await shot(page, "pipelines-status");
  await selectRow(page, "1767608400301");
  await page.getByRole("button", { name: "View Execution Details" }).click();
  await expect(page.getByRole("heading", { name: "Pipeline Execution Details" })).toBeVisible();
  await waitForTable(page);
  await shot(page, "execution-details");

  // Run again: a failed run, why it failed, and Resubmit.
  await openScreen(page, "Home");
  await waitForTable(page);
  await selectRow(page, "1767868200303");
  await expect(page.getByRole("button", { name: "Resubmit" })).toBeEnabled();
  await page.getByRole("button", { name: "View Failure Details" }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toContainText("reducing03");
  await shot(page, "failure-details");
  await dialog.getByRole("button", { name: /Close|Cancel|OK/ }).first().click();
  await expect(dialog).toBeHidden();
  await shot(page, "resubmit");
});
