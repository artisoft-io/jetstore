// The Start Pipeline flow (startPipelineUF): user/screens/flows/start-pipeline.
//
// It stops before Start Pipeline & Done: a run needs the installation's bucket,
// and the submitted run would add a row to the Pipelines Status tab that other
// images show.
import { expect, test } from "@playwright/test";
import { openScreen, screenshotsFor, signIn, waitForTable } from "../../lib/shots";
import { press, selectRow } from "./steps";

const { shot } = screenshotsFor("user/screens/flows/start-pipeline");

test("Start Pipeline for Patient_Profile_Template, up to the start", async ({ page }) => {
  await signIn(page);
  await openScreen(page, "Home");
  await waitForTable(page);
  await press(page, "Start Pipeline");
  await expect(page).toHaveURL(/\/flow\/startPipelineUF/);

  // 1. The pipeline configuration.
  const configs = await waitForTable(page);
  await selectRow(configs, "CI", "Patient_Profile_Template");
  await shot(page, "select-pipeline-configuration");
  await press(page, "Next");

  // 2. Its main input, from the input registry.
  const inputs = await waitForTable(page);
  await expect(inputs.locator("tbody tr")).toHaveCount(1);
  await selectRow(inputs, "1767604500201");
  await shot(page, "select-main-input");
  await press(page, "Next");

  // 3. The summary.
  await waitForTable(page);
  await expect(page.getByText("Run Pipeline Summary")).toBeVisible();
  await expect(page.getByRole("button", { name: "Start Pipeline & Done", exact: true })).toBeEnabled();
  await shot(page, "summary");
});
