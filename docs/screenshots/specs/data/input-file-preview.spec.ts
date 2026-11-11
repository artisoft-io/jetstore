// Input File Preview: user/screens/input-file-preview. No image: the screen
// reads the file from the installation's bucket, and the stack has none.
//
// The spec follows the way to it, the Load Files flow's Preview File button,
// whose address carries the file key, folders and all, as one encoded segment.
import { expect, test } from "@playwright/test";
import { openFlowMenu, signIn, waitForTable } from "../../lib/shots";
import { press, selectRow } from "./steps";

test("Input File Preview, from the Load Files flow", async ({ page }) => {
  await signIn(page);
  await openFlowMenu(page, "Source Mapping");
  const sources = await waitForTable(page);
  await selectRow(sources, "CI_D1_hc:Eligibility");
  await press(page, "Load Data");
  const files = await waitForTable(page);
  await selectRow(files, "M021-M040_M007_M008_eligibility.csv");
  await press(page, "Preview File");
  await expect(page).toHaveURL(/\/filePreviewPath\/jetstore%2Finput%2Fclient%3DCI%2F/);
  await expect(page.getByRole("heading", { name: "Input File Preview" })).toBeVisible();
});
